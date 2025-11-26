package vcenter

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25/mo"
)

// InventoryOptions controls the scope of ScanVCenter.
// Nil values fall back to True so you can selectively disable sections.
type InventoryOptions struct {
	IncludeTemplates  *bool
	IncludeFolders    *bool
	IncludeDatastores *bool
}

func boolOrDefault(ptr *bool, def bool) bool {
	if ptr == nil {
		return def
	}
	return *ptr
}

// VCenterInventory mirrors the structure exposed by py-vcenter.inventory.VCenterInventory.
type VCenterInventory struct {
	Datacenters     []string
	ComputeClusters map[string][]string
	StorageClusters map[string][]string
	Datastores      map[string][]string
	Templates       map[string][]string
	Folders         map[string][]string
	PortGroups      map[string][]string
}

// ToFlat flattens per-datacenter maps into unique global lists.
func (inv *VCenterInventory) ToFlat() map[string][]string {
	result := map[string][]string{
		"datacenters":      append([]string{}, inv.Datacenters...),
		"compute_clusters": {},
		"storage_clusters": {},
		"datastores":       {},
		"templates":        {},
		"folders":          {},
		"port_groups":      {},
	}

	flat := func(target string, values map[string][]string) {
		set := map[string]struct{}{}
		for _, list := range values {
			for _, item := range list {
				set[item] = struct{}{}
			}
		}
		keys := make([]string, 0, len(set))
		for key := range set {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		result[target] = keys
	}

	flat("compute_clusters", inv.ComputeClusters)
	flat("storage_clusters", inv.StorageClusters)
	flat("datastores", inv.Datastores)
	flat("templates", inv.Templates)
	flat("folders", inv.Folders)
	flat("port_groups", inv.PortGroups)
	return result
}

// ScanVCenter collects datacenters, clusters, templates, folders, and networks similar to the Python API.
func ScanVCenter(ctx context.Context, client *govmomi.Client, opts InventoryOptions) (*VCenterInventory, error) {
	finder := find.NewFinder(client.Client, true)
	viewMgr := view.NewManager(client.Client)

	includeTemplates := boolOrDefault(opts.IncludeTemplates, true)
	includeFolders := boolOrDefault(opts.IncludeFolders, true)
	includeDatastores := boolOrDefault(opts.IncludeDatastores, true)

	datacenters, err := finder.DatacenterList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("list datacenters: %w", err)
	}

	inventory := &VCenterInventory{
		Datacenters:     []string{},
		ComputeClusters: map[string][]string{},
		StorageClusters: map[string][]string{},
		Datastores:      map[string][]string{},
		Templates:       map[string][]string{},
		Folders:         map[string][]string{},
		PortGroups:      map[string][]string{},
	}

	for _, dc := range datacenters {
		dcName := dc.Name()
		inventory.Datacenters = append(inventory.Datacenters, dcName)
		inventory.ComputeClusters[dcName] = []string{}
		inventory.StorageClusters[dcName] = []string{}
		inventory.Datastores[dcName] = []string{}
		inventory.Templates[dcName] = []string{}
		inventory.Folders[dcName] = []string{}
		inventory.PortGroups[dcName] = []string{}

		finder.SetDatacenter(dc)

		// Compute clusters
		clusters, err := finder.ClusterComputeResourceList(ctx, "*")
		if err == nil {
			for _, cluster := range clusters {
				inventory.ComputeClusters[dcName] = append(inventory.ComputeClusters[dcName], cluster.Name())
			}
		}

		// Storage clusters
		if pods, err := collectStoragePods(ctx, viewMgr, dc); err == nil {
			inventory.StorageClusters[dcName] = pods
		}

		// Datastores
		if includeDatastores {
			datastores, err := finder.DatastoreList(ctx, "*")
			if err == nil {
				for _, ds := range datastores {
					inventory.Datastores[dcName] = append(inventory.Datastores[dcName], ds.Name())
				}
			}
		}

		// Templates
		if includeTemplates {
			templates, err := collectTemplates(ctx, viewMgr, dc)
			if err != nil {
				return nil, err
			}
			inventory.Templates[dcName] = templates
		}

		// Folders
		if includeFolders {
			folders, err := dc.Folders(ctx)
			if err == nil {
				paths, err := collectFolderPaths(ctx, folders.VmFolder, dcName)
				if err != nil {
					return nil, err
				}
				inventory.Folders[dcName] = paths
			}
		}

		// Port groups / networks
		networks, err := collectNetworks(ctx, viewMgr, dc)
		if err != nil {
			return nil, err
		}
		inventory.PortGroups[dcName] = networks

		// Sort per-datacenter lists for determinism
		sort.Strings(inventory.ComputeClusters[dcName])
		sort.Strings(inventory.StorageClusters[dcName])
		sort.Strings(inventory.Datastores[dcName])
		sort.Strings(inventory.Templates[dcName])
		sort.Strings(inventory.Folders[dcName])
		sort.Strings(inventory.PortGroups[dcName])
	}

	sort.Strings(inventory.Datacenters)
	return inventory, nil
}

func collectTemplates(ctx context.Context, viewMgr *view.Manager, dc *object.Datacenter) ([]string, error) {
	vmView, err := viewMgr.CreateContainerView(ctx, dc.Reference(), []string{"VirtualMachine"}, true)
	if err != nil {
		return nil, fmt.Errorf("create VM view: %w", err)
	}
	defer vmView.Destroy(ctx)

	var vms []mo.VirtualMachine
	if err := vmView.Retrieve(ctx, []string{"VirtualMachine"}, []string{"name", "config.template"}, &vms); err != nil {
		return nil, fmt.Errorf("retrieve templates: %w", err)
	}

	list := []string{}
	for _, vm := range vms {
		if vm.Config != nil && vm.Config.Template {
			list = append(list, vm.Name)
		}
	}
	return list, nil
}

func collectFolderPaths(ctx context.Context, folder *object.Folder, dcName string) ([]string, error) {
	acc := []string{}
	err := collectFolderPathsRecursive(ctx, folder, dcName, &acc)
	if err != nil {
		return nil, err
	}
	return acc, nil
}

func collectFolderPathsRecursive(ctx context.Context, folder *object.Folder, dcName string, acc *[]string) error {
	children, err := folder.Children(ctx)
	if err != nil {
		return fmt.Errorf("list folder children: %w", err)
	}
	for _, child := range children {
		switch item := child.(type) {
		case *object.Folder:
			path := item.InventoryPath
			prefix := fmt.Sprintf("/%s/vm/", dcName)
			if strings.HasPrefix(path, prefix) {
				relative := strings.TrimPrefix(path, prefix)
				if relative != "" {
					*acc = append(*acc, relative)
				}
			}
			if err := collectFolderPathsRecursive(ctx, item, dcName, acc); err != nil {
				return err
			}
		}
	}
	return nil
}

func collectNetworks(ctx context.Context, viewMgr *view.Manager, dc *object.Datacenter) ([]string, error) {
	type networkType struct {
		kind  string
		props []string
	}

	targets := []networkType{
		{kind: "Network", props: []string{"name"}},
		{kind: "DistributedVirtualPortgroup", props: []string{"name"}},
	}

	seen := map[string]struct{}{}
	for _, t := range targets {
		v, err := viewMgr.CreateContainerView(ctx, dc.Reference(), []string{t.kind}, true)
		if err != nil {
			return nil, fmt.Errorf("create %s view: %w", t.kind, err)
		}

		switch t.kind {
		case "Network":
			var networks []mo.Network
			if err := v.Retrieve(ctx, []string{t.kind}, t.props, &networks); err == nil {
				for _, n := range networks {
					if n.Name != "" {
						seen[n.Name] = struct{}{}
					}
				}
			}
		case "DistributedVirtualPortgroup":
			var groups []mo.DistributedVirtualPortgroup
			if err := v.Retrieve(ctx, []string{t.kind}, t.props, &groups); err == nil {
				for _, g := range groups {
					if g.Name != "" {
						seen[g.Name] = struct{}{}
					}
				}
			}
		}
		v.Destroy(ctx)
	}

	acc := make([]string, 0, len(seen))
	for name := range seen {
		acc = append(acc, name)
	}
	sort.Strings(acc)
	return acc, nil
}

func collectStoragePods(ctx context.Context, viewMgr *view.Manager, dc *object.Datacenter) ([]string, error) {
	view, err := viewMgr.CreateContainerView(ctx, dc.Reference(), []string{"StoragePod"}, true)
	if err != nil {
		return nil, fmt.Errorf("create StoragePod view: %w", err)
	}
	defer view.Destroy(ctx)

	var pods []mo.StoragePod
	if err := view.Retrieve(ctx, []string{"StoragePod"}, []string{"name"}, &pods); err != nil {
		return nil, fmt.Errorf("retrieve storage pods: %w", err)
	}

	names := make([]string, 0, len(pods))
	for _, pod := range pods {
		names = append(names, pod.Name)
	}
	sort.Strings(names)
	return names, nil
}
