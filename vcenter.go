package vcenter

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/guest"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// resolveDatastoreOrCluster attempts to find either a datastore cluster or a regular datastore.
// It first tries to find a datastore cluster (StoragePod), and if that fails, tries to find
// a regular datastore. This allows the clone functions to accept either type.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - finder: The Finder instance (must have datacenter set)
//   - name: The name of the datastore or datastore cluster
//
// Returns:
//   - isCluster: true if the name refers to a datastore cluster, false if regular datastore
//   - datastoreRef: Reference to the datastore (nil if cluster)
//   - clusterRef: Reference to the datastore cluster/StoragePod (nil if regular datastore)
//   - error: Any error that occurred during lookup
func resolveDatastoreOrCluster(
	ctx context.Context,
	finder *find.Finder,
	name string,
) (isCluster bool, datastoreRef *types.ManagedObjectReference, clusterRef *types.ManagedObjectReference, err error) {
	// Try to find as datastore cluster first
	storagePod, err := finder.DatastoreCluster(ctx, name)
	if err == nil {
		// Found a datastore cluster
		ref := storagePod.Reference()
		return true, nil, &ref, nil
	}

	// Not a cluster, try as regular datastore
	ds, err := finder.Datastore(ctx, name)
	if err != nil {
		return false, nil, nil, fmt.Errorf("neither datastore cluster nor datastore found with name '%s': %w", name, err)
	}

	ref := ds.Reference()
	return false, &ref, nil, nil
}

// getStoragePlacementResult uses Storage DRS to get a recommended datastore from a cluster.
// This function is called when cloning to a datastore cluster to let vSphere choose the
// best datastore based on space and I/O load.
func getStoragePlacementResult(
	ctx context.Context,
	client *govmomi.Client,
	template *object.VirtualMachine,
	vmFolder *object.Folder,
	vmName string,
	pool *object.ResourcePool,
	storagePod types.ManagedObjectReference,
	powerOn bool,
	customization *types.CustomizationSpec,
) (*types.ManagedObjectReference, error) {
	// Get storage resource manager
	storageRM := object.NewStorageResourceManager(client.Client)

	// Create clone spec with customization if provided
	relocateSpec := types.VirtualMachineRelocateSpec{
		Pool:         types.NewReference(pool.Reference()),
		DiskMoveType: string(types.VirtualMachineRelocateDiskMoveOptionsMoveAllDiskBackingsAndAllowSharing),
	}

	cloneSpec := types.VirtualMachineCloneSpec{
		Location:      relocateSpec,
		PowerOn:       powerOn,
		Template:      false,
		Customization: customization,
	}

	podSelectionSpec := types.StorageDrsPodSelectionSpec{
		StoragePod: &storagePod,
	}

	placementSpec := types.StoragePlacementSpec{
		Type:             string(types.StoragePlacementSpecPlacementTypeClone),
		Vm:               types.NewReference(template.Reference()),
		PodSelectionSpec: podSelectionSpec,
		CloneSpec:        &cloneSpec,
		CloneName:        vmName,
		Folder:           types.NewReference(vmFolder.Reference()),
		ResourcePool:     types.NewReference(pool.Reference()),
	}

	// Get recommendation
	result, err := storageRM.RecommendDatastores(ctx, placementSpec)
	if err != nil {
		return nil, fmt.Errorf("failed to get storage placement recommendation: %w", err)
	}

	if len(result.Recommendations) == 0 {
		return nil, fmt.Errorf("no storage placement recommendations available")
	}

	// Use the first (best) recommendation
	recommendation := result.Recommendations[0]

	// Apply the recommendation
	task, err := storageRM.ApplyStorageDrsRecommendation(ctx, []string{recommendation.Key})
	if err != nil {
		return nil, fmt.Errorf("failed to apply storage placement recommendation: %w", err)
	}

	// Wait for the task to complete
	info, err := task.WaitForResult(ctx)
	if err != nil {
		return nil, fmt.Errorf("storage placement task failed: %w", err)
	}

	// The result is an ApplyStorageRecommendationResult containing VM reference
	// Try direct ManagedObjectReference first
	if vmRef, ok := info.Result.(types.ManagedObjectReference); ok {
		return &vmRef, nil
	}

	// Try ApplyStorageRecommendationResult
	if applyResult, ok := info.Result.(types.ApplyStorageRecommendationResult); ok {
		if applyResult.Vm != nil {
			return applyResult.Vm, nil
		}
	}

	// Try as a pointer
	if applyResult, ok := info.Result.(*types.ApplyStorageRecommendationResult); ok {
		if applyResult != nil && applyResult.Vm != nil {
			return applyResult.Vm, nil
		}
	}

	return nil, fmt.Errorf("unexpected result type from storage placement: %T", info.Result)
}

// cloneVMInternal is the internal clone function that supports all options.
// This is used by the public functions CloneVM and CloneFromRequest.
func cloneVMInternal(
	ctx context.Context,
	client *govmomi.Client,
	templateName string,
	vmName string,
	datacenter string,
	datastore string,
	resourcePool string,
	folder string,
	customization *types.CustomizationSpec,
	powerOn bool,
) (*object.VirtualMachine, error) {

	finder := find.NewFinder(client.Client, true)

	dc, err := finder.Datacenter(ctx, datacenter)
	if err != nil {
		return nil, fmt.Errorf("datacenter not found: %w", err)
	}
	finder.SetDatacenter(dc)

	template, err := finder.VirtualMachine(ctx, templateName)
	if err != nil {
		return nil, fmt.Errorf("template not found: %w", err)
	}

	pool, err := finder.ResourcePool(ctx, resourcePool)
	if err != nil {
		return nil, fmt.Errorf("resource pool not found: %w", err)
	}

	var vmFolder *object.Folder
	if folder != "" {
		// Try the folder path as-is first
		vmFolder, err = finder.Folder(ctx, folder)
		if err != nil {
			// If not found or ambiguous, try with full path under vm folder
			fullPath := fmt.Sprintf("/%s/vm/%s", datacenter, folder)
			vmFolder, err = finder.Folder(ctx, fullPath)
			if err != nil {
				return nil, fmt.Errorf("folder not found: %w", err)
			}
		}
	} else {
		folders, err := dc.Folders(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get folders: %w", err)
		}
		vmFolder = folders.VmFolder
	}

	// Resolve datastore or datastore cluster
	isCluster, datastoreRef, clusterRef, err := resolveDatastoreOrCluster(ctx, finder, datastore)
	if err != nil {
		return nil, err
	}

	// If it's a datastore cluster, use Storage DRS placement
	if isCluster {
		vmRef, err := getStoragePlacementResult(ctx, client, template, vmFolder, vmName, pool, *clusterRef, powerOn, customization)
		if err != nil {
			return nil, err
		}
		vm := object.NewVirtualMachine(client.Client, *vmRef)
		return vm, nil
	}

	// Regular datastore - use normal clone
	relocateSpec := types.VirtualMachineRelocateSpec{
		Datastore:    datastoreRef,
		Pool:         types.NewReference(pool.Reference()),
		DiskMoveType: string(types.VirtualMachineRelocateDiskMoveOptionsMoveAllDiskBackingsAndAllowSharing),
	}

	cloneSpec := types.VirtualMachineCloneSpec{
		Location:      relocateSpec,
		PowerOn:       powerOn,
		Template:      false,
		Customization: customization,
	}

	task, err := template.Clone(ctx, vmFolder, vmName, cloneSpec)
	if err != nil {
		return nil, fmt.Errorf("failed to start clone: %w", err)
	}

	info, err := task.WaitForResult(ctx)
	if err != nil {
		return nil, fmt.Errorf("clone failed: %w", err)
	}

	vm := object.NewVirtualMachine(client.Client, info.Result.(types.ManagedObjectReference))
	return vm, nil
}

// CloneVM clones a virtual machine from a template.
//
// The function creates a new VM based on the specified template and places it
// in the specified datacenter, datastore, and resource pool.
// The VM is created powered off by default.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - client: govmomi.Client for vCenter connection
//   - templateName: The name of the template to clone from
//   - vmName: The name of the new VM
//   - datacenter: The name of the datacenter
//   - datastore: The name of the datastore or datastore cluster where the VM should be created
//   - resourcePool: The name of the resource pool (e.g. "Resources")
//   - folder: The name of the VM folder (empty string for default VM folder)
//
// Returns the newly created VM (powered off) or an error if cloning fails.
//
// Example:
//
//	vm, err := vcenter.CloneVM(ctx, client, "Win2022-Template", "WebServer01",
//	    "DC1", "datastore1", "Resources", "WebServers")
func CloneVM(
	ctx context.Context,
	client *govmomi.Client,
	templateName string,
	vmName string,
	datacenter string,
	datastore string,
	resourcePool string,
	folder string,
) (*object.VirtualMachine, error) {
	return cloneVMInternal(ctx, client, templateName, vmName, datacenter, datastore, resourcePool, folder, nil, false)
}

// CloneVMWithCustomization clones a virtual machine with Windows/Linux customization.
//
// The function clones a VM from a template and applies customizations
// such as computer name, domain join, IP configuration, and timezone.
// The VM is started automatically after cloning so that customization can run.
//
// NOTE: For proper customization with additional disks or resource changes,
// use CloneFromRequest instead which implements the correct operation order.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - client: govmomi.Client for vCenter connection
//   - templateName: The name of the template to clone from
//   - vmName: The name of the new VM
//   - datacenter: The name of the datacenter
//   - datastore: The name of the datastore or datastore cluster where the VM should be created
//   - resourcePool: The name of the resource pool
//   - folder: The name of the VM folder (empty string for default)
//   - customization: CustomizationSpec with all Windows/Linux settings
//
// Returns the newly created VM or an error.
//
// Example:
//
//	customization := vcenter.NewWindowsCustomization(vcenter.WindowsCustomizationConfig{...})
//	vm, err := vcenter.CloneVMWithCustomization(ctx, client,
//	    "Win2022-Template", "WebServer01", "DC1", "datastore1",
//	    "Resources", "", customization)
func CloneVMWithCustomization(
	ctx context.Context,
	client *govmomi.Client,
	templateName string,
	vmName string,
	datacenter string,
	datastore string,
	resourcePool string,
	folder string,
	customization *types.CustomizationSpec,
) (*object.VirtualMachine, error) {
	return cloneVMInternal(ctx, client, templateName, vmName, datacenter, datastore, resourcePool, folder, customization, true)
}

// ============================================================================
// Customization Types and Functions
// ============================================================================

// NetworkAdapter describes the configuration for a network adapter during customization.
// Used in multi-NIC scenarios where each adapter can have different IP settings.
type NetworkAdapter struct {
	Network    string   `json:"network"`               // Port group/network name
	IPAddress  string   `json:"ip_address,omitempty"`  // Empty = DHCP
	SubnetMask string   `json:"subnet_mask,omitempty"` // Required if IPAddress is set
	Gateway    string   `json:"gateway,omitempty"`     // Required if IPAddress is set
	DNSServers []string `json:"dns_servers,omitempty"` // Per-adapter DNS servers (optional)
}

// WindowsCustomizationConfig contains all settings for Windows VM customization.
// This unified config supports all scenarios: domain join, workgroup, DHCP, static IP,
// single NIC, and multi-NIC configurations.
type WindowsCustomizationConfig struct {
	ComputerName  string // Required: Windows computer name (max 15 chars)
	AdminPassword string // Required: Local administrator password

	// Timezone - Windows timezone ID (e.g., 85 for W. Europe, 110 for Pacific)
	Timezone int

	// Domain join settings (optional - if Domain is empty, joins WORKGROUP)
	Domain          string // AD domain to join (e.g., "example.com")
	DomainUser      string // Domain admin user (e.g., "admin@example.com")
	DomainPassword  string // Password for domain admin
	MachineObjectOU string // OU for computer object (e.g., "OU=Servers,DC=example,DC=com")

	// Network settings
	Adapters    []NetworkAdapter // Network adapters (empty = one DHCP adapter)
	GlobalDNS   []string         // Global DNS servers
	DNSSuffixes []string         // DNS search suffixes

	// Autologon settings
	AutologonCount int // Number of auto-logons (0 = disabled)
}

// LinuxCustomizationConfig contains all settings for Linux VM customization.
// Supports DHCP, static IP, single NIC, and multi-NIC configurations.
type LinuxCustomizationConfig struct {
	Hostname string // Required: Linux hostname
	Domain   string // Domain name (optional)

	// Network settings
	Adapters    []NetworkAdapter // Network adapters (empty = one DHCP adapter)
	GlobalDNS   []string         // Global DNS servers
	DNSSuffixes []string         // DNS search suffixes
}

// NewWindowsCustomization creates a Windows customization spec from a config struct.
//
// This function replaces the older separate functions and supports all scenarios:
//   - Domain join with DHCP
//   - Domain join with static IP
//   - Workgroup (standalone) with DHCP
//   - Workgroup (standalone) with static IP
//   - Single or multiple network adapters
//   - MachineObjectOU for specific OU placement
//   - Autologon for post-install scripts
//
// Example - Domain join with DHCP:
//
//	spec := vcenter.NewWindowsCustomization(vcenter.WindowsCustomizationConfig{
//	    ComputerName:  "WebServer01",
//	    AdminPassword: "SecurePass123!",
//	    Timezone:      85,
//	    Domain:        "example.com",
//	    DomainUser:    "admin@example.com",
//	    DomainPassword: "DomainPass!",
//	    GlobalDNS:     []string{"192.168.1.1", "192.168.1.2"},
//	    DNSSuffixes:   []string{"example.com"},
//	})
//
// Example - Standalone with static IP:
//
//	spec := vcenter.NewWindowsCustomization(vcenter.WindowsCustomizationConfig{
//	    ComputerName:  "TestServer",
//	    AdminPassword: "SecurePass123!",
//	    Timezone:      85,
//	    Adapters: []vcenter.NetworkAdapter{{
//	        IPAddress:  "192.168.1.100",
//	        SubnetMask: "255.255.255.0",
//	        Gateway:    "192.168.1.1",
//	        DNSServers: []string{"192.168.1.1"},
//	    }},
//	})
//
// Example - Multi-NIC configuration:
//
//	spec := vcenter.NewWindowsCustomization(vcenter.WindowsCustomizationConfig{
//	    ComputerName:  "WebServer01",
//	    AdminPassword: "SecurePass123!",
//	    Timezone:      85,
//	    Domain:        "example.com",
//	    DomainUser:    "admin@example.com",
//	    DomainPassword: "DomainPass!",
//	    Adapters: []vcenter.NetworkAdapter{
//	        {Network: "Production", IPAddress: "10.1.1.10", SubnetMask: "255.255.255.0", Gateway: "10.1.1.1"},
//	        {Network: "Management", IPAddress: "10.2.1.10", SubnetMask: "255.255.255.0", Gateway: "10.2.1.1"},
//	    },
//	    GlobalDNS:   []string{"10.1.1.1"},
//	    DNSSuffixes: []string{"example.com"},
//	})
func NewWindowsCustomization(cfg WindowsCustomizationConfig) *types.CustomizationSpec {
	// Build identification (domain join or workgroup)
	identification := types.CustomizationIdentification{}
	if cfg.Domain != "" {
		identification.JoinDomain = cfg.Domain
		identification.DomainAdmin = cfg.DomainUser
		identification.DomainAdminPassword = &types.CustomizationPassword{
			Value:     cfg.DomainPassword,
			PlainText: true,
		}
		// Set the OU path for the computer object if specified
		if cfg.MachineObjectOU != "" {
			identification.DomainOU = cfg.MachineObjectOU
		}
	} else {
		identification.JoinWorkgroup = "WORKGROUP"
	}

	// Handle autologon
	autoLogon := cfg.AutologonCount > 0
	autoLogonCount := int32(1)
	if cfg.AutologonCount > 0 {
		autoLogonCount = int32(cfg.AutologonCount)
	}

	// Build sysprep identity
	sysprep := &types.CustomizationSysprep{
		GuiUnattended: types.CustomizationGuiUnattended{
			Password: &types.CustomizationPassword{
				Value:     cfg.AdminPassword,
				PlainText: true,
			},
			TimeZone:       int32(cfg.Timezone),
			AutoLogon:      autoLogon,
			AutoLogonCount: autoLogonCount,
		},
		UserData: types.CustomizationUserData{
			FullName: "Administrator",
			OrgName:  "Organization",
			ComputerName: &types.CustomizationFixedName{
				Name: cfg.ComputerName,
			},
		},
		Identification: identification,
	}

	// Build network adapter mappings
	var nicMappings []types.CustomizationAdapterMapping

	if len(cfg.Adapters) == 0 {
		// Default: single adapter with DHCP
		nicMappings = []types.CustomizationAdapterMapping{{
			Adapter: types.CustomizationIPSettings{
				Ip: &types.CustomizationDhcpIpGenerator{},
			},
		}}
	} else {
		// Build mapping for each adapter
		for _, adapter := range cfg.Adapters {
			mapping := types.CustomizationAdapterMapping{
				Adapter: types.CustomizationIPSettings{},
			}

			if adapter.IPAddress != "" {
				// Static IP configuration
				mapping.Adapter.Ip = &types.CustomizationFixedIp{
					IpAddress: adapter.IPAddress,
				}
				mapping.Adapter.SubnetMask = adapter.SubnetMask
				if adapter.Gateway != "" {
					mapping.Adapter.Gateway = []string{adapter.Gateway}
				}
				if len(adapter.DNSServers) > 0 {
					mapping.Adapter.DnsServerList = adapter.DNSServers
				}
			} else {
				// DHCP configuration
				mapping.Adapter.Ip = &types.CustomizationDhcpIpGenerator{}
			}

			nicMappings = append(nicMappings, mapping)
		}
	}

	return &types.CustomizationSpec{
		Identity: sysprep,
		GlobalIPSettings: types.CustomizationGlobalIPSettings{
			DnsServerList: cfg.GlobalDNS,
			DnsSuffixList: cfg.DNSSuffixes,
		},
		NicSettingMap: nicMappings,
	}
}

// NewLinuxCustomization creates a Linux customization spec from a config struct.
//
// This function supports all Linux customization scenarios:
//   - DHCP configuration
//   - Static IP configuration
//   - Single or multiple network adapters
//
// Example - Simple DHCP:
//
//	spec := vcenter.NewLinuxCustomization(vcenter.LinuxCustomizationConfig{
//	    Hostname: "webserver01",
//	    Domain:   "example.com",
//	})
//
// Example - Static IP:
//
//	spec := vcenter.NewLinuxCustomization(vcenter.LinuxCustomizationConfig{
//	    Hostname: "webserver01",
//	    Domain:   "example.com",
//	    Adapters: []vcenter.NetworkAdapter{{
//	        IPAddress:  "192.168.1.100",
//	        SubnetMask: "255.255.255.0",
//	        Gateway:    "192.168.1.1",
//	    }},
//	    GlobalDNS:   []string{"192.168.1.1"},
//	    DNSSuffixes: []string{"example.com"},
//	})
//
// Example - Multi-NIC:
//
//	spec := vcenter.NewLinuxCustomization(vcenter.LinuxCustomizationConfig{
//	    Hostname: "appserver01",
//	    Domain:   "example.com",
//	    Adapters: []vcenter.NetworkAdapter{
//	        {IPAddress: "10.1.1.20", SubnetMask: "255.255.255.0", Gateway: "10.1.1.1"},
//	        {IPAddress: "10.2.1.20", SubnetMask: "255.255.255.0", Gateway: "10.2.1.1"},
//	    },
//	    GlobalDNS: []string{"10.1.1.1"},
//	})
func NewLinuxCustomization(cfg LinuxCustomizationConfig) *types.CustomizationSpec {
	// Build Linux identity
	linuxPrep := &types.CustomizationLinuxPrep{
		HostName: &types.CustomizationFixedName{
			Name: cfg.Hostname,
		},
	}
	if cfg.Domain != "" {
		linuxPrep.Domain = cfg.Domain
	}

	// Build network adapter mappings
	var nicMappings []types.CustomizationAdapterMapping

	if len(cfg.Adapters) == 0 {
		// Default: single adapter with DHCP
		nicMappings = []types.CustomizationAdapterMapping{{
			Adapter: types.CustomizationIPSettings{
				Ip: &types.CustomizationDhcpIpGenerator{},
			},
		}}
	} else {
		// Build mapping for each adapter
		for _, adapter := range cfg.Adapters {
			mapping := types.CustomizationAdapterMapping{
				Adapter: types.CustomizationIPSettings{},
			}

			if adapter.IPAddress != "" {
				// Static IP configuration
				mapping.Adapter.Ip = &types.CustomizationFixedIp{
					IpAddress: adapter.IPAddress,
				}
				mapping.Adapter.SubnetMask = adapter.SubnetMask
				if adapter.Gateway != "" {
					mapping.Adapter.Gateway = []string{adapter.Gateway}
				}
			} else {
				// DHCP configuration
				mapping.Adapter.Ip = &types.CustomizationDhcpIpGenerator{}
			}

			nicMappings = append(nicMappings, mapping)
		}
	}

	return &types.CustomizationSpec{
		Identity: linuxPrep,
		GlobalIPSettings: types.CustomizationGlobalIPSettings{
			DnsServerList: cfg.GlobalDNS,
			DnsSuffixList: cfg.DNSSuffixes,
		},
		NicSettingMap: nicMappings,
	}
}


// SetVMResources changes CPU and memory on a virtual machine.
//
// The VM needs to be powered off to change these resources (depending on VM configuration).
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - vm: VirtualMachine object to modify
//   - numCPUs: Number of CPUs (e.g. 2, 4, 8)
//   - memoryMB: Memory in MB (e.g. 4096 for 4GB, 8192 for 8GB)
//
// Returns nil on success, otherwise an error.
//
// Example:
//
//	err := vcenter.SetVMResources(ctx, vm, 4, 8192) // 4 CPU, 8GB RAM
func SetVMResources(ctx context.Context, vm *object.VirtualMachine, numCPUs int32, memoryMB int64) error {
	spec := types.VirtualMachineConfigSpec{
		NumCPUs:  numCPUs,
		MemoryMB: memoryMB,
	}

	task, err := vm.Reconfigure(ctx, spec)
	if err != nil {
		return fmt.Errorf("failed to reconfigure: %w", err)
	}

	return task.Wait(ctx)
}

// ============================================================================
// VM Helpers
// ============================================================================

// GetVM finds a virtual machine based on name.
//
// If datacenter is empty, search is performed in all datacenters.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - client: govmomi.Client for vCenter connection
//   - vmName: The name of the VM to find
//   - datacenter: The name of the datacenter (empty string to search everywhere)
//
// Returns the VirtualMachine object or a NotFoundError.
//
// Example:
//
//	vm, err := vcenter.GetVM(ctx, client, "WebServer01", "DC1")
func GetVM(ctx context.Context, client *govmomi.Client, vmName string, datacenter string) (*object.VirtualMachine, error) {
	finder := find.NewFinder(client.Client, true)

	if datacenter != "" {
		dc, err := finder.Datacenter(ctx, datacenter)
		if err != nil {
			return nil, fmt.Errorf("datacenter not found: %w", err)
		}
		finder.SetDatacenter(dc)
	}

	vm, err := finder.VirtualMachine(ctx, vmName)
	if err != nil {
		return nil, fmt.Errorf("VM not found: %w", err)
	}

	return vm, nil
}

// PowerOnVM powers on a virtual machine.
//
// The function waits until the power-on operation is complete.
//
// Returns nil on success, otherwise an error.
//
// Example:
//
//	err := vcenter.PowerOnVM(ctx, vm)
func PowerOnVM(ctx context.Context, vm *object.VirtualMachine) error {
	task, err := vm.PowerOn(ctx)
	if err != nil {
		return fmt.Errorf("failed to start power on: %w", err)
	}

	return task.Wait(ctx)
}

// PowerOffVM powers off a virtual machine.
//
// This is a "hard" power off (equivalent to pulling the power cable).
// For graceful shutdown, use guest.Shutdown() instead.
//
// Returns nil on success, otherwise an error.
//
// Example:
//
//	err := vcenter.PowerOffVM(ctx, vm)
func PowerOffVM(ctx context.Context, vm *object.VirtualMachine) error {
	task, err := vm.PowerOff(ctx)
	if err != nil {
		return fmt.Errorf("failed to start power off: %w", err)
	}

	return task.Wait(ctx)
}

// RestartVM restarts a virtual machine.
//
// The function first attempts a graceful restart using VMware Tools (RebootGuest).
// If that fails, a hard reset is performed instead.
//
// Returns nil on success, otherwise an error.
//
// Example:
//
//	err := vcenter.RestartVM(ctx, vm)
func RestartVM(ctx context.Context, vm *object.VirtualMachine) error {
	err := vm.RebootGuest(ctx)
	if err != nil {
		// If guest tools don't work, do hard reset
		task, err := vm.Reset(ctx)
		if err != nil {
			return fmt.Errorf("failed to restart VM: %w", err)
		}
		return task.Wait(ctx)
	}
	return nil
}

// WaitForIP waits until the VM gets a routable IP address.
//
// The function polls the VM every 2 seconds until an IP address is found or timeout is reached.
// Requires that VMware Tools is installed and running in the guest system.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - vm: VirtualMachine object to wait for
//   - timeout: Max time to wait (e.g. 5*time.Minute)
//
// Returns the IP address or a timeout error.
//
// Example:
//
//	ip, err := vcenter.WaitForIP(ctx, vm, 5*time.Minute)
//	fmt.Printf("VM IP: %s\n", ip)
func WaitForIP(ctx context.Context, vm *object.VirtualMachine, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("timeout waiting for IP address: %w", ctx.Err())
		case <-ticker.C:
			ip, err := vm.WaitForIP(ctx, true) // true = wait for routable IP
			if err == nil && ip != "" {
				return ip, nil
			}
		}
	}
}

// WaitForTools waits for VMware Tools to become ready.
//
// The function polls the VM every 2 seconds until VMware Tools is running.
// Max wait time is 5 minutes.
//
// Returns nil when Tools are ready, otherwise a timeout error.
//
// Example:
//
//	err := vcenter.WaitForTools(ctx, vm)
//	if err == nil {
//	    // VMware Tools are now ready to use
//	}
func WaitForTools(ctx context.Context, vm *object.VirtualMachine) error {
	timeout := 5 * time.Minute
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for VMware Tools: %w", ctx.Err())
		case <-ticker.C:
			var vmProps mo.VirtualMachine
			err := vm.Properties(ctx, vm.Reference(), []string{"guest"}, &vmProps)
			if err != nil {
				continue
			}

			if vmProps.Guest.ToolsRunningStatus == string(types.VirtualMachineToolsRunningStatusGuestToolsRunning) {
				return nil
			}
		}
	}
}

// CustomizationExpected specifies what values to wait for during customization.
type CustomizationExpected struct {
	// Hostname is the expected hostname (without domain suffix).
	// Required - customization waits until this matches.
	Hostname string

	// Domain is the expected domain suffix (e.g., "domain.local").
	// If set, waits for hostname to be "Hostname.Domain".
	// If empty, waits for hostname to match Hostname exactly.
	Domain string

	// IP is the expected IP address.
	// If set, waits for this exact IP.
	// If empty or "dhcp", accepts any valid (non-link-local) IP.
	IP string
}

// WaitForCustomization waits for VM guest customization (Windows Sysprep) to complete.
//
// The function detects customization completion by checking:
//  1. Hostname matches the expected value (with or without domain suffix)
//  2. IP address matches (exact match for static, any valid IP for DHCP)
//  3. VMware Tools is running
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - vm: VirtualMachine object to monitor
//   - timeout: Max time to wait (recommended: 10-15 minutes)
//   - expected: Expected hostname, domain, and IP configuration
//
// Returns nil when customization completes, otherwise a timeout error.
//
// Example:
//
//	// Domain-joined VM with static IP
//	err := vcenter.WaitForCustomization(ctx, vm, 10*time.Minute, vcenter.CustomizationExpected{
//	    Hostname: "srv001",
//	    Domain:   "domain.local",
//	    IP:       "192.168.1.100",
//	})
//
//	// Standalone VM with DHCP
//	err := vcenter.WaitForCustomization(ctx, vm, 10*time.Minute, vcenter.CustomizationExpected{
//	    Hostname: "srv002",
//	    IP:       "dhcp",
//	})
func WaitForCustomization(ctx context.Context, vm *object.VirtualMachine, timeout time.Duration, expected CustomizationExpected) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second) // Poll every 5 seconds
	defer ticker.Stop()

	startTime := time.Now()
	var lastLogTime time.Time
	vmName := vm.Name()

	// Build expected hostname string
	expectedHostname := strings.ToLower(expected.Hostname)
	expectedFQDN := expectedHostname
	if expected.Domain != "" {
		expectedFQDN = expectedHostname + "." + strings.ToLower(expected.Domain)
	}

	// Determine if we need exact IP match
	expectStaticIP := expected.IP != "" && strings.ToLower(expected.IP) != "dhcp"

	log.Printf("Waiting for customization to complete on %s...", vmName)
	log.Printf("  Expected: hostname=%s, ip=%s", expectedFQDN, func() string {
		if expectStaticIP {
			return expected.IP
		}
		return "(any valid)"
	}())

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("customization timeout after %v on %s: %w", timeout, vmName, ctx.Err())
		case <-ticker.C:
			var vmProps mo.VirtualMachine
			err := vm.Properties(ctx, vm.Reference(), []string{"guest"}, &vmProps)
			if err != nil {
				continue
			}

			hostname := strings.ToLower(vmProps.Guest.HostName)
			ip := vmProps.Guest.IpAddress
			toolsStatus := vmProps.Guest.ToolsRunningStatus

			// Check hostname match
			// For domain-joined: expect "hostname.domain"
			// For standalone: expect "hostname" exactly
			var hostnameMatches bool
			if expected.Domain != "" {
				// Domain-joined: must match full FQDN
				hostnameMatches = hostname == expectedFQDN
			} else {
				// Standalone: must match hostname exactly (no domain suffix)
				hostnameMatches = hostname == expectedHostname
			}

			// Check IP match
			var ipMatches bool
			if expectStaticIP {
				// Static IP: must match exactly
				ipMatches = ip == expected.IP
			} else {
				// DHCP: any valid IP (not link-local)
				ipMatches = ip != "" && !strings.HasPrefix(ip, "169.254.")
			}

			// Check VMware Tools is running
			toolsRunning := toolsStatus == string(types.VirtualMachineToolsRunningStatusGuestToolsRunning)

			if hostnameMatches && ipMatches && toolsRunning {
				log.Printf("Customization complete on %s (hostname=%s, ip=%s)", vmName, vmProps.Guest.HostName, ip)
				return nil
			}

			// Log status every 30 seconds
			if time.Since(lastLogTime) >= 30*time.Second {
				elapsed := int(time.Since(startTime).Seconds())
				hostnameDisplay := vmProps.Guest.HostName
				if hostnameDisplay == "" {
					hostnameDisplay = "(not set)"
				}
				ipDisplay := ip
				if ipDisplay == "" {
					ipDisplay = "(no IP)"
				}

				// Show what's missing
				var missing []string
				if !hostnameMatches {
					missing = append(missing, fmt.Sprintf("hostname want=%s", expectedFQDN))
				}
				if !ipMatches {
					if expectStaticIP {
						missing = append(missing, fmt.Sprintf("ip want=%s", expected.IP))
					} else {
						missing = append(missing, "ip want=(valid)")
					}
				}
				if !toolsRunning {
					missing = append(missing, "tools not running")
				}

				log.Printf("  Waiting for %s (%ds): hostname=%s, ip=%s [%s]",
					vmName, elapsed, hostnameDisplay, ipDisplay, strings.Join(missing, ", "))
				lastLogTime = time.Now()
			}
		}
	}
}

// ============================================================================
// JSON/Config Parsing
// ============================================================================

// ServerRequest represents a server order with all necessary configuration.
// Supports multi-NIC, multiple disks, MachineObjectOU, and autologon.
type ServerRequest struct {
	Name     string `json:"name"`
	Template string `json:"template"`
	CPUs     int32  `json:"cpus"`
	MemoryGB int    `json:"memory_gb"`

	// Disks - list of disk sizes in GB (for additional disks: D:, E:, F:, etc.)
	DisksGB          []int  `json:"disks_gb,omitempty"`
	DiskProvisioning string `json:"disk_provisioning,omitempty"` // thin, thick, or eagerzeroed

	// Network adapters (empty = single DHCP adapter with legacy IP fields)
	Adapters []NetworkAdapter `json:"adapters,omitempty"`

	// Legacy single-NIC fields (used if Adapters is empty)
	IPAddress  string `json:"ip_address,omitempty"`
	SubnetMask string `json:"subnet_mask,omitempty"`
	Gateway    string `json:"gateway,omitempty"`

	// Domain settings (optional - empty = workgroup)
	Domain          string `json:"domain,omitempty"`
	MachineObjectOU string `json:"machine_object_ou,omitempty"` // OU for computer object

	// DNS settings
	DNSServers  []string `json:"dns_servers,omitempty"`
	DNSSuffixes []string `json:"dns_suffixes,omitempty"`

	// Autologon (0 = disabled)
	AutologonCount int `json:"autologon_count,omitempty"`

	// Deprecated: Use DisksGB instead. Kept for backwards compatibility.
	DiskGB int `json:"disk_gb,omitempty"`
}

// Validate validates ServerRequest and returns a ValidationError if something is wrong
func (r *ServerRequest) Validate() error {
	if r.Name == "" {
		return &ValidationError{Field: "Name", Message: "name is required"}
	}
	if r.Template == "" {
		return &ValidationError{Field: "Template", Message: "template is required"}
	}

	// Validate legacy single-NIC fields
	if r.IPAddress != "" {
		if r.SubnetMask == "" {
			return &ValidationError{Field: "SubnetMask", Message: "subnet mask is required when using static IP"}
		}
		if r.Gateway == "" {
			return &ValidationError{Field: "Gateway", Message: "gateway is required when using static IP"}
		}
	}

	// Validate multi-NIC adapters
	for i, adapter := range r.Adapters {
		if adapter.IPAddress != "" {
			if adapter.SubnetMask == "" {
				return &ValidationError{Field: fmt.Sprintf("Adapters[%d].SubnetMask", i), Message: "subnet mask is required when using static IP"}
			}
			if adapter.Gateway == "" {
				return &ValidationError{Field: fmt.Sprintf("Adapters[%d].Gateway", i), Message: "gateway is required when using static IP"}
			}
		}
	}

	return nil
}

// CloneFromRequest clones a virtual machine based on a ServerRequest struct.
//
// The function implements the CORRECT operation order for VM cloning with customization:
//  1. Clone VM with powerOn=false (customization attached but doesn't run yet)
//  2. Add extra disks while VM is powered off (safe operation)
//  3. Change CPU/memory while VM is powered off (safe operation)
//  4. Power on VM (this triggers guest customization/sysprep)
//  5. Wait for customization to complete using WaitForCustomization
//
// This order is critical because:
//   - Sysprep only runs when the VM boots for the first time after clone
//   - Modifying hardware while powered off is safer
//   - Logging in before sysprep completes can break the installation
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - client: govmomi.Client for vCenter connection
//   - req: ServerRequest with all VM configuration
//   - datacenter: The name of the datacenter
//   - datastore: The name of the datastore or datastore cluster
//   - resourcePool: The name of the resource pool
//   - folder: The name of the VM folder (empty for default)
//   - domainUser: Domain admin for domain join
//   - domainPassword: Password for domain admin
//   - adminPassword: Local admin password
//   - timezone: Windows timezone ID
//
// Returns the newly created VM or a ValidationError/OperationError.
//
// Example:
//
//	req := vcenter.ServerRequest{
//	    Name:     "WebServer01",
//	    Template: "Win2022-Template",
//	    CPUs:     4,
//	    MemoryGB: 8,
//	    Domain:   "example.com",
//	    Adapters: []vcenter.NetworkAdapter{
//	        {IPAddress: "192.168.1.100", SubnetMask: "255.255.255.0", Gateway: "192.168.1.1"},
//	    },
//	    DNSServers:      []string{"192.168.1.1"},
//	    MachineObjectOU: "OU=Servers,DC=example,DC=com",
//	}
//	vm, err := vcenter.CloneFromRequest(ctx, client, req, "DC1",
//	    "datastore1", "Resources", "", "admin@example.com",
//	    "domainpass", "adminpass", 85)
func CloneFromRequest(ctx context.Context, client *govmomi.Client, req ServerRequest, datacenter, datastore, resourcePool, folder string, domainUser, domainPassword, adminPassword string, timezone int) (*object.VirtualMachine, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, err
	}

	log.Printf("Starting clone operation for %s...", req.Name)

	// Create DNS suffixes if not specified
	dnsSuffixes := req.DNSSuffixes
	if len(dnsSuffixes) == 0 && req.Domain != "" {
		dnsSuffixes = []string{req.Domain}
	}

	// Build network adapters from request
	var adapters []NetworkAdapter
	if len(req.Adapters) > 0 {
		// Use multi-NIC configuration
		adapters = req.Adapters
	} else if req.IPAddress != "" {
		// Legacy single static IP
		adapters = []NetworkAdapter{{
			IPAddress:  req.IPAddress,
			SubnetMask: req.SubnetMask,
			Gateway:    req.Gateway,
			DNSServers: req.DNSServers,
		}}
	}
	// else: empty adapters = DHCP

	// Create customization spec when domain settings or networking overrides are requested
	var customization *types.CustomizationSpec
	needsCustomization := req.Domain != "" || len(adapters) > 0 || len(req.DNSServers) > 0 || len(dnsSuffixes) > 0
	if needsCustomization {
		customization = NewWindowsCustomization(WindowsCustomizationConfig{
			ComputerName:    req.Name,
			AdminPassword:   adminPassword,
			Timezone:        timezone,
			Domain:          req.Domain,
			DomainUser:      domainUser,
			DomainPassword:  domainPassword,
			MachineObjectOU: req.MachineObjectOU,
			Adapters:        adapters,
			GlobalDNS:       req.DNSServers,
			DNSSuffixes:     dnsSuffixes,
			AutologonCount:  req.AutologonCount,
		})
		log.Printf("  Customization spec created: domain=%s, adapters=%d", req.Domain, len(adapters))
	}

	// === CORRECT ORDER OF OPERATIONS ===

	// Step 1: Clone VM with powerOn=false (customization attached but doesn't run yet)
	log.Printf("  Step 1: Cloning VM with powerOn=false...")
	vm, err := cloneVMInternal(ctx, client, req.Template, req.Name, datacenter, datastore, resourcePool, folder, customization, false)
	if err != nil {
		return nil, fmt.Errorf("clone failed: %w", err)
	}
	log.Printf("  Clone completed (VM is powered off)")

	// Step 2: Add extra disks while VM is powered off
	disks := req.DisksGB
	if len(disks) == 0 && req.DiskGB > 0 {
		// Legacy single disk field
		disks = []int{req.DiskGB}
	}

	if len(disks) > 0 {
		log.Printf("  Step 2: Adding %d disk(s) while VM is off...", len(disks))
		for i, diskSizeGB := range disks {
			diskLetter := string(rune('D' + i)) // D, E, F, ...
			log.Printf("    Adding %dGB disk (%s:)", diskSizeGB, diskLetter)
			err = AddDisk(ctx, vm, diskSizeGB)
			if err != nil {
				return vm, fmt.Errorf("VM created but failed to add disk %s: %w", diskLetter, err)
			}
		}
	}

	// Step 3: Change CPU/memory while VM is powered off
	if req.CPUs > 0 || req.MemoryGB > 0 {
		cpus := req.CPUs
		if cpus == 0 {
			cpus = 2 // Default
		}
		memoryMB := int64(req.MemoryGB * 1024)
		if memoryMB == 0 {
			memoryMB = 4096 // Default 4GB
		}

		log.Printf("  Step 3: Setting resources: %d CPUs, %dMB RAM", cpus, memoryMB)
		err = SetVMResources(ctx, vm, cpus, memoryMB)
		if err != nil {
			return vm, fmt.Errorf("VM created but failed to set resources: %w", err)
		}
	}

	// Step 4: Power on VM (this triggers guest customization/sysprep)
	log.Printf("  Step 4: Powering on VM (this triggers customization)...")
	err = PowerOnVM(ctx, vm)
	if err != nil {
		return vm, fmt.Errorf("VM created but failed to power on: %w", err)
	}
	log.Printf("  VM powered on successfully")

	// Step 5: Wait for customization to complete (if customization was applied)
	if customization != nil {
		log.Printf("  Step 5: Waiting for customization to complete...")
		customizationTimeout := 15 * time.Minute

		// Build expected values based on request
		expected := CustomizationExpected{
			Hostname: req.Name,
			Domain:   req.Domain,
		}

		// Determine expected IP
		if len(req.Adapters) > 0 {
			// Multi-NIC: use first adapter's IP
			expected.IP = req.Adapters[0].IPAddress
		} else {
			// Legacy single-NIC
			expected.IP = req.IPAddress
		}
		// Empty IP or "dhcp" means accept any valid IP
		if expected.IP == "" {
			expected.IP = "dhcp"
		}

		err = WaitForCustomization(ctx, vm, customizationTimeout, expected)
		if err != nil {
			// Don't fail the whole operation - VM is created, customization might still work
			log.Printf("  WARNING: Customization wait issue: %v", err)
		} else {
			log.Printf("  Customization completed successfully")
		}
	}

	log.Printf("Clone operation for %s completed successfully", req.Name)
	return vm, nil
}

// ============================================================================
// Disk Operations
// ============================================================================

// AddDisk adds a new disk to a virtual machine.
//
// The disk is created as thin provisioned VMDK on the specified datastore.
// The VM can be running during the operation (hot-add, if supported by VM).
//
// The disk is placed on the same datastore as the VM's existing OS disk.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - vm: VirtualMachine object to add disk to
//   - sizeGB: Disk size in GB (e.g. 100 for 100GB)
//
// Returns nil on success, otherwise an error.
//
// Example:
//
//	err := vcenter.AddDisk(ctx, vm, 100) // Add 100GB disk
func AddDisk(ctx context.Context, vm *object.VirtualMachine, sizeGB int) error {
	var vmProps mo.VirtualMachine
	err := vm.Properties(ctx, vm.Reference(), []string{"config.hardware.device"}, &vmProps)
	if err != nil {
		return fmt.Errorf("failed to get VM properties: %w", err)
	}

	devices := object.VirtualDeviceList(vmProps.Config.Hardware.Device)

	// Find SCSI controller
	controller, err := devices.FindSCSIController("")
	if err != nil {
		return fmt.Errorf("failed to find SCSI controller: %w", err)
	}

	// Get datastore from VM's existing disk (use same datastore as OS disk)
	var dsRef *types.ManagedObjectReference
	for _, device := range vmProps.Config.Hardware.Device {
		if disk, ok := device.(*types.VirtualDisk); ok {
			if backing, ok := disk.Backing.(*types.VirtualDiskFlatVer2BackingInfo); ok {
				dsRef = backing.Datastore
				break
			}
		}
	}
	if dsRef == nil {
		return fmt.Errorf("could not determine datastore from VM's existing disks")
	}

	// Create new disk
	disk := &types.VirtualDisk{
		VirtualDevice: types.VirtualDevice{
			Key: devices.NewKey(),
			Backing: &types.VirtualDiskFlatVer2BackingInfo{
				DiskMode:        string(types.VirtualDiskModePersistent),
				ThinProvisioned: types.NewBool(true),
				VirtualDeviceFileBackingInfo: types.VirtualDeviceFileBackingInfo{
					Datastore: dsRef,
				},
			},
		},
		CapacityInKB: int64(sizeGB) * 1024 * 1024,
	}

	devices.AssignController(disk, controller)

	configSpec := types.VirtualMachineConfigSpec{}
	configSpec.DeviceChange = []types.BaseVirtualDeviceConfigSpec{
		&types.VirtualDeviceConfigSpec{
			Operation:     types.VirtualDeviceConfigSpecOperationAdd,
			Device:        disk,
			FileOperation: types.VirtualDeviceConfigSpecFileOperationCreate,
		},
	}

	task, err := vm.Reconfigure(ctx, configSpec)
	if err != nil {
		return fmt.Errorf("failed to add disk: %w", err)
	}

	return task.Wait(ctx)
}

// ExtendDisk extends an existing disk to a new size.
//
// The disk is identified by its label (e.g. "Hard disk 2").
// The new size must be larger than the current size.
// The VM can be running during the operation.
//
// NOTE: This only extends the disk in vSphere. You need to manually extend
// the partition in the guest operating system afterwards.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - vm: VirtualMachine object
//   - diskLabel: The name/label of the disk (e.g. "Hard disk 2")
//   - newSizeGB: New size in GB (must be larger than current)
//
// Returns nil on success, otherwise a ValidationError or OperationError.
//
// Example:
//
//	err := vcenter.ExtendDisk(ctx, vm, "Hard disk 2", 200) // Extend to 200GB
func ExtendDisk(ctx context.Context, vm *object.VirtualMachine, diskLabel string, newSizeGB int) error {
	var vmProps mo.VirtualMachine
	err := vm.Properties(ctx, vm.Reference(), []string{"config.hardware.device"}, &vmProps)
	if err != nil {
		return fmt.Errorf("failed to get VM properties: %w", err)
	}

	devices := object.VirtualDeviceList(vmProps.Config.Hardware.Device)

	// Find disk by label
	var disk *types.VirtualDisk
	for _, device := range devices {
		if d, ok := device.(*types.VirtualDisk); ok {
			if dev := device.GetVirtualDevice(); dev.DeviceInfo != nil {
				label := dev.DeviceInfo.GetDescription().Label
				if label == diskLabel {
					disk = d
					break
				}
			}
		}
	}

	if disk == nil {
		return fmt.Errorf("disk with label %s not found", diskLabel)
	}

	// Check that new size is larger than current
	currentSizeGB := disk.CapacityInKB / (1024 * 1024)
	if int64(newSizeGB) <= currentSizeGB {
		return fmt.Errorf("new size (%d GB) must be larger than current size (%d GB)", newSizeGB, currentSizeGB)
	}

	// Update disk size
	disk.CapacityInKB = int64(newSizeGB) * 1024 * 1024

	configSpec := types.VirtualMachineConfigSpec{}
	configSpec.DeviceChange = []types.BaseVirtualDeviceConfigSpec{
		&types.VirtualDeviceConfigSpec{
			Operation: types.VirtualDeviceConfigSpecOperationEdit,
			Device:    disk,
		},
	}

	task, err := vm.Reconfigure(ctx, configSpec)
	if err != nil {
		return fmt.Errorf("failed to extend disk: %w", err)
	}

	return task.Wait(ctx)
}

// RemoveDisk removes a disk from a virtual machine.
//
// Both the disk from the VM configuration and the VMDK file are permanently deleted.
// The VM can be running during the operation (hot-remove, if supported by VM).
//
// WARNING: This permanently deletes data!
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - vm: VirtualMachine object
//   - diskLabel: The name/label of the disk to remove (e.g. "Hard disk 2")
//
// Returns nil on success, otherwise a NotFoundError or OperationError.
//
// Example:
//
//	err := vcenter.RemoveDisk(ctx, vm, "Hard disk 2")
func RemoveDisk(ctx context.Context, vm *object.VirtualMachine, diskLabel string) error {
	var vmProps mo.VirtualMachine
	err := vm.Properties(ctx, vm.Reference(), []string{"config.hardware.device"}, &vmProps)
	if err != nil {
		return fmt.Errorf("failed to get VM properties: %w", err)
	}

	devices := object.VirtualDeviceList(vmProps.Config.Hardware.Device)

	// Find disk by label
	var disk *types.VirtualDisk
	for _, device := range devices {
		if d, ok := device.(*types.VirtualDisk); ok {
			if dev := device.GetVirtualDevice(); dev.DeviceInfo != nil {
				label := dev.DeviceInfo.GetDescription().Label
				if label == diskLabel {
					disk = d
					break
				}
			}
		}
	}

	if disk == nil {
		return fmt.Errorf("disk with label %s not found", diskLabel)
	}

	configSpec := types.VirtualMachineConfigSpec{}
	configSpec.DeviceChange = []types.BaseVirtualDeviceConfigSpec{
		&types.VirtualDeviceConfigSpec{
			Operation:     types.VirtualDeviceConfigSpecOperationRemove,
			Device:        disk,
			FileOperation: types.VirtualDeviceConfigSpecFileOperationDestroy,
		},
	}

	task, err := vm.Reconfigure(ctx, configSpec)
	if err != nil {
		return fmt.Errorf("failed to remove disk: %w", err)
	}

	return task.Wait(ctx)
}

// ============================================================================
// Network Operations
// ============================================================================

// AddNetworkAdapter adds a new network adapter to a virtual machine.
//
// The network adapter is created as VMXNET3 (VMware paravirtualized adapter).
// The VM can be running during the operation (hot-add, if supported by VM).
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - vm: VirtualMachine object
//   - networkName: The name of the network/port group to connect to
//
// Returns nil on success, otherwise a NotFoundError or OperationError.
//
// Example:
//
//	err := vcenter.AddNetworkAdapter(ctx, vm, "Production-VLAN100")
func AddNetworkAdapter(ctx context.Context, vm *object.VirtualMachine, networkName string) error {
	var vmProps mo.VirtualMachine
	err := vm.Properties(ctx, vm.Reference(), []string{"config.hardware.device"}, &vmProps)
	if err != nil {
		return fmt.Errorf("failed to get VM properties: %w", err)
	}

	devices := object.VirtualDeviceList(vmProps.Config.Hardware.Device)

	// Find network
	finder := find.NewFinder(vm.Client(), true)
	network, err := finder.Network(ctx, networkName)
	if err != nil {
		return fmt.Errorf("network not found: %w", err)
	}

	// Create network adapter (VMXNET3)
	backing, err := network.EthernetCardBackingInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to get network backing: %w", err)
	}

	netdev, err := object.EthernetCardTypes().CreateEthernetCard("vmxnet3", backing)
	if err != nil {
		return fmt.Errorf("failed to create ethernet card: %w", err)
	}

	netdev.GetVirtualDevice().Key = devices.NewKey()

	configSpec := types.VirtualMachineConfigSpec{}
	configSpec.DeviceChange = []types.BaseVirtualDeviceConfigSpec{
		&types.VirtualDeviceConfigSpec{
			Operation: types.VirtualDeviceConfigSpecOperationAdd,
			Device:    netdev,
		},
	}

	task, err := vm.Reconfigure(ctx, configSpec)
	if err != nil {
		return fmt.Errorf("failed to add network adapter: %w", err)
	}

	return task.Wait(ctx)
}

// ChangeNetwork changes the network on an existing network adapter.
//
// The network adapter is identified by its label (e.g. "Network adapter 1").
// The VM can be running during the operation.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - vm: VirtualMachine object
//   - adapterLabel: The name/label of the network adapter (e.g. "Network adapter 1")
//   - newNetworkName: The name of the new network/port group
//
// Returns nil on success, otherwise a NotFoundError or OperationError.
//
// Example:
//
//	err := vcenter.ChangeNetwork(ctx, vm, "Network adapter 1", "DMZ-VLAN200")
func ChangeNetwork(ctx context.Context, vm *object.VirtualMachine, adapterLabel string, newNetworkName string) error {
	var vmProps mo.VirtualMachine
	err := vm.Properties(ctx, vm.Reference(), []string{"config.hardware.device"}, &vmProps)
	if err != nil {
		return fmt.Errorf("failed to get VM properties: %w", err)
	}

	devices := object.VirtualDeviceList(vmProps.Config.Hardware.Device)

	// Find network adapter by label
	var adapter types.BaseVirtualDevice
	for _, device := range devices {
		if _, ok := device.(types.BaseVirtualEthernetCard); ok {
			if dev := device.GetVirtualDevice(); dev.DeviceInfo != nil {
				label := dev.DeviceInfo.GetDescription().Label
				if label == adapterLabel {
					adapter = device
					break
				}
			}
		}
	}

	if adapter == nil {
		return fmt.Errorf("network adapter with label %s not found", adapterLabel)
	}

	// Find new network
	finder := find.NewFinder(vm.Client(), true)
	network, err := finder.Network(ctx, newNetworkName)
	if err != nil {
		return fmt.Errorf("network not found: %w", err)
	}

	backing, err := network.EthernetCardBackingInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to get network backing: %w", err)
	}

	// Update backing
	card := adapter.(types.BaseVirtualEthernetCard)
	card.GetVirtualEthernetCard().Backing = backing

	configSpec := types.VirtualMachineConfigSpec{}
	configSpec.DeviceChange = []types.BaseVirtualDeviceConfigSpec{
		&types.VirtualDeviceConfigSpec{
			Operation: types.VirtualDeviceConfigSpecOperationEdit,
			Device:    adapter,
		},
	}

	task, err := vm.Reconfigure(ctx, configSpec)
	if err != nil {
		return fmt.Errorf("failed to change network: %w", err)
	}

	return task.Wait(ctx)
}

// ============================================================================
// Snapshot Operations
// ============================================================================

// SnapshotInfo contains information about a VM snapshot
type SnapshotInfo struct {
	Name        string
	Description string
	CreateTime  string
	State       string
	ID          int32
	Level       int
}

// findSnapshotByName recursively searches for a snapshot by name in the snapshot tree
func findSnapshotByName(snapshotList []types.VirtualMachineSnapshotTree, snapshotName string) *types.ManagedObjectReference {
	for _, snapshot := range snapshotList {
		if snapshot.Name == snapshotName {
			return &snapshot.Snapshot
		}
		if len(snapshot.ChildSnapshotList) > 0 {
			result := findSnapshotByName(snapshot.ChildSnapshotList, snapshotName)
			if result != nil {
				return result
			}
		}
	}
	return nil
}

// extractSnapshotInfo recursively extracts snapshot information from the snapshot tree
func extractSnapshotInfo(snapshotTree types.VirtualMachineSnapshotTree, level int) []SnapshotInfo {
	snapshots := []SnapshotInfo{
		{
			Name:        snapshotTree.Name,
			Description: snapshotTree.Description,
			CreateTime:  snapshotTree.CreateTime.String(),
			State:       string(snapshotTree.State),
			ID:          snapshotTree.Id,
			Level:       level,
		},
	}

	// Process children recursively
	for _, child := range snapshotTree.ChildSnapshotList {
		snapshots = append(snapshots, extractSnapshotInfo(child, level+1)...)
	}

	return snapshots
}

// CreateSnapshot creates a snapshot of a virtual machine.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - vm: VirtualMachine object
//   - snapshotName: Name for the snapshot
//   - description: Optional description for the snapshot
//   - memory: Include VM memory in snapshot (default: false)
//     If true, creates a snapshot of the VM's memory state
//     Useful for capturing running state but increases snapshot size
//   - quiesce: Quiesce filesystem before snapshot (default: false)
//     Requires VMware Tools to be running
//     Ensures filesystem consistency by flushing buffers
//
// Returns nil on success, otherwise an error.
//
// Example:
//
//	// Simple disk-only snapshot
//	err := vcenter.CreateSnapshot(ctx, vm, "Before Update", "", false, false)
//
//	// Quiesced snapshot (requires VMware Tools)
//	err := vcenter.CreateSnapshot(ctx, vm, "Consistent Backup",
//	    "Pre-maintenance backup", false, true)
func CreateSnapshot(ctx context.Context, vm *object.VirtualMachine, snapshotName string, description string, memory bool, quiesce bool) error {
	task, err := vm.CreateSnapshot(ctx, snapshotName, description, memory, quiesce)
	if err != nil {
		return fmt.Errorf("failed to create snapshot: %w", err)
	}

	return task.Wait(ctx)
}

// DeleteSnapshot deletes a snapshot by name.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - vm: VirtualMachine object
//   - snapshotName: Name of the snapshot to delete
//   - removeChildren: Also remove child snapshots (default: false)
//     If false, children are preserved and promoted
//   - consolidate: Consolidate disk files after removal (default: true)
//     Merges snapshot deltas back into parent disk
//
// Returns nil on success, otherwise an error.
//
// Example:
//
//	// Delete single snapshot, keep children
//	err := vcenter.DeleteSnapshot(ctx, vm, "Before Update", false, true)
//
//	// Delete snapshot and all children
//	err := vcenter.DeleteSnapshot(ctx, vm, "Old Backup", true, true)
func DeleteSnapshot(ctx context.Context, vm *object.VirtualMachine, snapshotName string, removeChildren bool, consolidate bool) error {
	var props struct {
		Snapshot *types.VirtualMachineSnapshotInfo `mo:"snapshot"`
	}

	err := vm.Properties(ctx, vm.Reference(), []string{"snapshot"}, &props)
	if err != nil {
		return fmt.Errorf("failed to get snapshot info: %w", err)
	}

	if props.Snapshot == nil {
		return fmt.Errorf("VM has no snapshots")
	}

	// Find the snapshot
	snapshotRef := findSnapshotByName(props.Snapshot.RootSnapshotList, snapshotName)
	if snapshotRef == nil {
		return fmt.Errorf("snapshot %s not found", snapshotName)
	}

	// Create snapshot task request
	req := types.RemoveSnapshot_Task{
		This:           *snapshotRef,
		RemoveChildren: removeChildren,
		Consolidate:    &consolidate,
	}

	res, err := methods.RemoveSnapshot_Task(ctx, vm.Client(), &req)
	if err != nil {
		return fmt.Errorf("failed to remove snapshot: %w", err)
	}

	task := object.NewTask(vm.Client(), res.Returnval)
	return task.Wait(ctx)
}

// ListSnapshots lists all snapshots for a virtual machine.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - vm: VirtualMachine object
//
// Returns a list of SnapshotInfo objects containing snapshot details.
//
// Example:
//
//	snapshots, err := vcenter.ListSnapshots(ctx, vm)
//	for _, snap := range snapshots {
//	    fmt.Printf("%s: %s\n", snap.Name, snap.CreateTime)
//	}
func ListSnapshots(ctx context.Context, vm *object.VirtualMachine) ([]SnapshotInfo, error) {
	var props struct {
		Snapshot *types.VirtualMachineSnapshotInfo `mo:"snapshot"`
	}

	err := vm.Properties(ctx, vm.Reference(), []string{"snapshot"}, &props)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot info: %w", err)
	}

	if props.Snapshot == nil {
		return []SnapshotInfo{}, nil
	}

	var allSnapshots []SnapshotInfo
	for _, rootSnapshot := range props.Snapshot.RootSnapshotList {
		allSnapshots = append(allSnapshots, extractSnapshotInfo(rootSnapshot, 0)...)
	}

	return allSnapshots, nil
}

// RevertToSnapshot reverts VM to a specific snapshot.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - vm: VirtualMachine object
//   - snapshotName: Name of the snapshot to revert to
//   - suppressPowerOn: Don't power on VM after revert (default: false)
//     If false, VM will power on if it was on when snapshot was taken
//
// Returns nil on success, otherwise an error.
//
// Example:
//
//	// Revert to snapshot
//	err := vcenter.RevertToSnapshot(ctx, vm, "Before Update", false)
//
//	// Revert but keep VM powered off
//	err := vcenter.RevertToSnapshot(ctx, vm, "Clean State", true)
func RevertToSnapshot(ctx context.Context, vm *object.VirtualMachine, snapshotName string, suppressPowerOn bool) error {
	var props struct {
		Snapshot *types.VirtualMachineSnapshotInfo `mo:"snapshot"`
	}

	err := vm.Properties(ctx, vm.Reference(), []string{"snapshot"}, &props)
	if err != nil {
		return fmt.Errorf("failed to get snapshot info: %w", err)
	}

	if props.Snapshot == nil {
		return fmt.Errorf("VM has no snapshots")
	}

	// Find the snapshot
	snapshotRef := findSnapshotByName(props.Snapshot.RootSnapshotList, snapshotName)
	if snapshotRef == nil {
		return fmt.Errorf("snapshot %s not found", snapshotName)
	}

	// Create revert snapshot task request
	req := types.RevertToSnapshot_Task{
		This:            *snapshotRef,
		SuppressPowerOn: &suppressPowerOn,
	}

	res, err := methods.RevertToSnapshot_Task(ctx, vm.Client(), &req)
	if err != nil {
		return fmt.Errorf("failed to revert to snapshot: %w", err)
	}

	task := object.NewTask(vm.Client(), res.Returnval)
	return task.Wait(ctx)
}

// DeleteAllSnapshots deletes all snapshots for a virtual machine.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - vm: VirtualMachine object
//   - consolidate: Consolidate disk files after removal (default: true)
//
// Returns nil on success, otherwise an error.
//
// Example:
//
//	err := vcenter.DeleteAllSnapshots(ctx, vm, true)
func DeleteAllSnapshots(ctx context.Context, vm *object.VirtualMachine, consolidate bool) error {
	var props struct {
		Snapshot *types.VirtualMachineSnapshotInfo `mo:"snapshot"`
	}

	err := vm.Properties(ctx, vm.Reference(), []string{"snapshot"}, &props)
	if err != nil {
		return fmt.Errorf("failed to get snapshot info: %w", err)
	}

	if props.Snapshot == nil {
		// No snapshots to delete
		return nil
	}

	task, err := vm.RemoveAllSnapshot(ctx, &consolidate)
	if err != nil {
		return fmt.Errorf("failed to remove all snapshots: %w", err)
	}

	return task.Wait(ctx)
}

// GetCurrentSnapshot gets the current snapshot (the one the VM is currently on).
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - vm: VirtualMachine object
//
// Returns a SnapshotInfo object or nil if no current snapshot.
//
// Example:
//
//	current, err := vcenter.GetCurrentSnapshot(ctx, vm)
//	if current != nil {
//	    fmt.Printf("Current snapshot: %s\n", current.Name)
//	}
func GetCurrentSnapshot(ctx context.Context, vm *object.VirtualMachine) (*SnapshotInfo, error) {
	var props struct {
		Snapshot *types.VirtualMachineSnapshotInfo `mo:"snapshot"`
	}

	err := vm.Properties(ctx, vm.Reference(), []string{"snapshot"}, &props)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot info: %w", err)
	}

	if props.Snapshot == nil || props.Snapshot.CurrentSnapshot == nil {
		return nil, nil
	}

	// Find the current snapshot in the tree
	var findCurrent func([]types.VirtualMachineSnapshotTree) *SnapshotInfo
	findCurrent = func(snapshots []types.VirtualMachineSnapshotTree) *SnapshotInfo {
		for _, snap := range snapshots {
			if snap.Snapshot == *props.Snapshot.CurrentSnapshot {
				return &SnapshotInfo{
					Name:        snap.Name,
					Description: snap.Description,
					CreateTime:  snap.CreateTime.String(),
					State:       string(snap.State),
					ID:          snap.Id,
				}
			}
			if len(snap.ChildSnapshotList) > 0 {
				if result := findCurrent(snap.ChildSnapshotList); result != nil {
					return result
				}
			}
		}
		return nil
	}

	return findCurrent(props.Snapshot.RootSnapshotList), nil
}

// ============================================================================
// VM Lifecycle Management
// ============================================================================

// DeleteVM deletes a virtual machine.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - vm: VirtualMachine object
//   - deleteFromDisk: If true, destroy VM and delete all files from disk.
//     If false, only unregister from inventory (default: true)
//   - force: Allow deletion of powered-on VMs or VMs with snapshots (default: false)
//
// Returns nil on success, otherwise an error.
//
// Example:
//
//	// Delete VM and remove all files
//	err := vcenter.DeleteVM(ctx, vm, true, false)
//
//	// Unregister VM but keep files on datastore
//	err := vcenter.DeleteVM(ctx, vm, false, false)
//
//	// Force delete a powered-on VM
//	err := vcenter.DeleteVM(ctx, vm, true, true)
func DeleteVM(ctx context.Context, vm *object.VirtualMachine, deleteFromDisk bool, force bool) error {
	// Check power state
	var props struct {
		Runtime struct {
			PowerState types.VirtualMachinePowerState
		}
		Snapshot *types.VirtualMachineSnapshotInfo `mo:"snapshot"`
	}

	err := vm.Properties(ctx, vm.Reference(), []string{"runtime.powerState", "snapshot"}, &props)
	if err != nil {
		return fmt.Errorf("failed to get VM properties: %w", err)
	}

	if props.Runtime.PowerState == types.VirtualMachinePowerStatePoweredOn {
		if !force {
			return fmt.Errorf("VM is powered on. Power off the VM first or use force=true")
		}
		// Power off VM if force=true
		task, err := vm.PowerOff(ctx)
		if err != nil {
			return fmt.Errorf("failed to power off VM: %w", err)
		}
		err = task.Wait(ctx)
		if err != nil {
			return fmt.Errorf("failed to power off VM: %w", err)
		}
	}

	// Check for snapshots
	if props.Snapshot != nil {
		if !force {
			return fmt.Errorf("VM has snapshots. Remove snapshots first or use force=true")
		}
	}

	// Delete or unregister
	if deleteFromDisk {
		// Destroy VM and delete all files
		task, err := vm.Destroy(ctx)
		if err != nil {
			return fmt.Errorf("failed to destroy VM: %w", err)
		}
		return task.Wait(ctx)
	}

	// Unregister from inventory only (keep files on datastore)
	return vm.Unregister(ctx)
}

// UnregisterVM unregisters a virtual machine from inventory without deleting files.
//
// This is a convenience function that calls DeleteVM with deleteFromDisk=false.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - vm: VirtualMachine object
//
// Returns nil on success, otherwise an error.
//
// Example:
//
//	// Unregister VM (keep files on datastore)
//	err := vcenter.UnregisterVM(ctx, vm)
func UnregisterVM(ctx context.Context, vm *object.VirtualMachine) error {
	return DeleteVM(ctx, vm, false, false)
}

// ============================================================================
// VM Information
// ============================================================================

// VMInfo contains detailed information about a virtual machine
type VMInfo struct {
	Name              string
	PowerState        string
	CPUCount          int32
	CPUCoresPerSocket int32
	CPUSockets        int32
	MemoryMB          int64
	MemoryGB          float64
	Folder            string
	GuestOS           string
	GuestOSFullName   string
	GuestHostname     string
	GuestIPAddress    string
	Networks          []NetworkInfo
	Domain            string
	ToolsStatus       string
	ToolsVersion      string
	UUID              string
	InstanceUUID      string
	Datastore         string
	ResourcePool      string
	Annotation        string
}

// NetworkInfo contains network adapter information
type NetworkInfo struct {
	Label       string
	MACAddress  string
	Network     string
	Connected   bool
	AdapterType string
	IPAddresses []string
}

// GetVMInfo gets detailed information about a virtual machine.
//
// Returns comprehensive information about the VM including hardware configuration,
// network settings, guest OS details, and current state.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - vm: VirtualMachine object
//
// Returns a VMInfo struct with VM information.
//
// Example:
//
//	vm, _ := vcenter.GetVM(ctx, client, "WebServer01", "")
//	info, err := vcenter.GetVMInfo(ctx, vm)
//	fmt.Printf("VM: %s\n", info.Name)
//	fmt.Printf("CPUs: %d (%d sockets)\n", info.CPUCount, info.CPUSockets)
//	fmt.Printf("Memory: %.2f GB\n", info.MemoryGB)
//	fmt.Printf("IP: %s\n", info.GuestIPAddress)
//	for _, net := range info.Networks {
//	    fmt.Printf("  Network: %s - %v\n", net.Network, net.IPAddresses)
//	}
func GetVMInfo(ctx context.Context, vm *object.VirtualMachine) (*VMInfo, error) {
	var props struct {
		Config struct {
			Name          string
			Hardware      types.VirtualHardware
			GuestId       string
			GuestFullName string
			DatastoreUrl  []types.VirtualMachineConfigInfoDatastoreUrlPair
			UUID          string
			InstanceUuid  string
			Annotation    string
		}
		Summary struct {
			Runtime types.VirtualMachineRuntimeInfo
		}
		Guest        types.GuestInfo
		ResourcePool *types.ManagedObjectReference `mo:"resourcePool"`
		Parent       types.ManagedObjectReference
	}

	err := vm.Properties(ctx, vm.Reference(), []string{
		"config",
		"summary.runtime",
		"guest",
		"resourcePool",
		"parent",
	}, &props)
	if err != nil {
		return nil, fmt.Errorf("failed to get VM properties: %w", err)
	}

	// CPU information
	cpuCount := props.Config.Hardware.NumCPU
	coresPerSocket := props.Config.Hardware.NumCoresPerSocket
	cpuSockets := cpuCount / coresPerSocket
	if coresPerSocket == 0 {
		cpuSockets = cpuCount
	}

	// Memory information
	memoryMB := props.Config.Hardware.MemoryMB
	memoryGB := float64(memoryMB) / 1024.0

	// Folder path
	folderPath := "/"
	if props.Parent.Type == "Folder" {
		folder := object.NewFolder(vm.Client(), props.Parent)
		var folderProps struct {
			Name   string
			Parent *types.ManagedObjectReference `mo:"parent"`
		}
		err := folder.Properties(ctx, folder.Reference(), []string{"name", "parent"}, &folderProps)
		if err == nil && folderProps.Name != "vm" {
			folderPath = folderProps.Name
		}
	}

	// Network information
	networks := []NetworkInfo{}
	for _, device := range props.Config.Hardware.Device {
		if ethCard, ok := device.(types.BaseVirtualEthernetCard); ok {
			card := ethCard.GetVirtualEthernetCard()
			networkInfo := NetworkInfo{
				Label:       card.DeviceInfo.GetDescription().Label,
				MACAddress:  card.MacAddress,
				Network:     "Unknown",
				Connected:   false,
				AdapterType: fmt.Sprintf("%T", device),
				IPAddresses: []string{},
			}

			// Get network name
			if backing := card.Backing; backing != nil {
				switch b := backing.(type) {
				case *types.VirtualEthernetCardNetworkBackingInfo:
					networkInfo.Network = b.DeviceName
				case *types.VirtualEthernetCardDistributedVirtualPortBackingInfo:
					networkInfo.Network = "DVS"
				}
			}

			// Get connection status
			if card.Connectable != nil {
				networkInfo.Connected = card.Connectable.Connected
			}

			// Get IP addresses from guest info
			for _, nic := range props.Guest.Net {
				if nic.MacAddress == card.MacAddress {
					if nic.IpConfig != nil {
						for _, ipAddr := range nic.IpConfig.IpAddress {
							networkInfo.IPAddresses = append(networkInfo.IPAddresses, ipAddr.IpAddress)
						}
					}
					break
				}
			}

			networks = append(networks, networkInfo)
		}
	}

	// Guest information
	guestHostname := props.Guest.HostName
	guestIPAddress := props.Guest.IpAddress

	// Extract domain from hostname
	domain := ""
	if guestHostname != "" && len(guestHostname) > 0 {
		parts := make([]string, 0)
		for i, c := range guestHostname {
			if c == '.' {
				parts = append(parts, guestHostname[i+1:])
				break
			}
		}
		if len(parts) > 0 {
			domain = parts[0]
		}
	}

	// Tools status
	toolsStatus := string(props.Guest.ToolsRunningStatus)
	toolsVersion := props.Guest.ToolsVersion

	// Datastore
	datastoreName := ""
	if len(props.Config.DatastoreUrl) > 0 {
		datastoreName = props.Config.DatastoreUrl[0].Name
	}

	// Resource pool
	resourcePoolName := ""
	if props.ResourcePool != nil {
		rp := object.NewResourcePool(vm.Client(), *props.ResourcePool)
		var rpProps struct {
			Name string
		}
		err := rp.Properties(ctx, rp.Reference(), []string{"name"}, &rpProps)
		if err == nil {
			resourcePoolName = rpProps.Name
		}
	}

	return &VMInfo{
		Name:              props.Config.Name,
		PowerState:        string(props.Summary.Runtime.PowerState),
		CPUCount:          cpuCount,
		CPUCoresPerSocket: coresPerSocket,
		CPUSockets:        cpuSockets,
		MemoryMB:          int64(memoryMB),
		MemoryGB:          memoryGB,
		Folder:            folderPath,
		GuestOS:           props.Config.GuestId,
		GuestOSFullName:   props.Config.GuestFullName,
		GuestHostname:     guestHostname,
		GuestIPAddress:    guestIPAddress,
		Networks:          networks,
		Domain:            domain,
		ToolsStatus:       toolsStatus,
		ToolsVersion:      toolsVersion,
		UUID:              props.Config.UUID,
		InstanceUUID:      props.Config.InstanceUuid,
		Datastore:         datastoreName,
		ResourcePool:      resourcePoolName,
		Annotation:        props.Config.Annotation,
	}, nil
}

// ============================================================================
// CD/DVD Operations
// ============================================================================

// getCDROMDevice finds the first CD/DVD drive on a VM (helper function)
// Returns the device object with complete structure including current backing and connectable settings
func getCDROMDevice(ctx context.Context, vm *object.VirtualMachine) (*types.VirtualCdrom, object.VirtualDeviceList, error) {
	var props struct {
		Config struct {
			Hardware struct {
				Device []types.BaseVirtualDevice
			}
		}
	}

	err := vm.Properties(ctx, vm.Reference(), []string{"config.hardware.device"}, &props)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get VM properties: %w", err)
	}

	devices := object.VirtualDeviceList(props.Config.Hardware.Device)

	// Find CD-ROM device using govmomi's FindCdrom method
	cdrom, err := devices.FindCdrom("")
	if err != nil {
		return nil, nil, fmt.Errorf("no CD/DVD drive found on VM: %w", err)
	}

	return cdrom, devices, nil
}

// MountISO mounts an ISO file to a VM's CD/DVD drive.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - vm: VirtualMachine object
//   - isoPath: Path to ISO file on datastore (e.g., "ISOs/ubuntu-22.04.iso")
//   - datastoreName: Name of datastore containing the ISO (if empty, uses VM's datastore)
//   - connect: Connect the CD/DVD drive after mounting (default: true)
//
// Returns nil on success, otherwise an error.
//
// Example:
//
//	// Mount ISO from specific datastore
//	err := vcenter.MountISO(ctx, vm, "ISOs/windows.iso", "datastore1", true)
//
//	// Mount ISO from VM's datastore
//	err := vcenter.MountISO(ctx, vm, "ISOs/ubuntu.iso", "", true)
func MountISO(ctx context.Context, vm *object.VirtualMachine, isoPath string, datastoreName string, connect bool) error {
	// Get CD/DVD device
	cdrom, devices, err := getCDROMDevice(ctx, vm)
	if err != nil {
		return err
	}

	// Get datastore name if not specified
	if datastoreName == "" {
		var props struct {
			Config struct {
				DatastoreUrl []types.VirtualMachineConfigInfoDatastoreUrlPair
			}
		}
		err := vm.Properties(ctx, vm.Reference(), []string{"config.datastoreUrl"}, &props)
		if err != nil {
			return fmt.Errorf("failed to get VM datastores: %w", err)
		}
		if len(props.Config.DatastoreUrl) == 0 {
			return fmt.Errorf("could not determine datastore. Please specify datastoreName")
		}
		datastoreName = props.Config.DatastoreUrl[0].Name
	}

	// Build datastore path in VMware format: [datastore] path/to/file.iso
	// IMPORTANT: Do NOT include datastore name in the path itself!
	fullISOPath := fmt.Sprintf("[%s] %s", datastoreName, isoPath)

	// Use govmomi's InsertIso helper to set the correct backing
	// This ensures we follow govmomi's best practices
	devices.InsertIso(cdrom, fullISOPath)

	// Preserve existing connectable settings or create new ones
	if cdrom.Connectable == nil {
		cdrom.Connectable = &types.VirtualDeviceConnectInfo{}
	}

	// Update connection state
	cdrom.Connectable.Connected = connect
	cdrom.Connectable.StartConnected = connect
	cdrom.Connectable.AllowGuestControl = true

	// Create device change spec
	deviceSpec := types.VirtualDeviceConfigSpec{
		Operation: types.VirtualDeviceConfigSpecOperationEdit,
		Device:    cdrom,
	}

	// Create VM config spec
	configSpec := types.VirtualMachineConfigSpec{
		DeviceChange: []types.BaseVirtualDeviceConfigSpec{&deviceSpec},
	}

	// Reconfigure VM
	task, err := vm.Reconfigure(ctx, configSpec)
	if err != nil {
		return fmt.Errorf("failed to mount ISO: %w", err)
	}

	return task.Wait(ctx)
}

// UnmountISO unmounts ISO from a VM's CD/DVD drive.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - vm: VirtualMachine object
//   - disconnect: Disconnect the CD/DVD drive after unmounting (default: true)
//
// Returns nil on success, otherwise an error.
//
// Example:
//
//	// Unmount ISO and disconnect drive
//	err := vcenter.UnmountISO(ctx, vm, true)
//
//	// Unmount but keep drive connected
//	err := vcenter.UnmountISO(ctx, vm, false)
func UnmountISO(ctx context.Context, vm *object.VirtualMachine, disconnect bool) error {
	// Get CD/DVD device
	cdrom, devices, err := getCDROMDevice(ctx, vm)
	if err != nil {
		return err
	}

	// Use govmomi's EjectIso helper to set the default backing
	// This restores the CD-ROM to its default state (no ISO)
	devices.EjectIso(cdrom)

	// Preserve existing connectable settings or create new ones
	if cdrom.Connectable == nil {
		cdrom.Connectable = &types.VirtualDeviceConnectInfo{}
	}

	// Update connection state
	cdrom.Connectable.Connected = !disconnect
	cdrom.Connectable.StartConnected = false
	cdrom.Connectable.AllowGuestControl = true

	// Create device change spec
	deviceSpec := types.VirtualDeviceConfigSpec{
		Operation: types.VirtualDeviceConfigSpecOperationEdit,
		Device:    cdrom,
	}

	// Create VM config spec
	configSpec := types.VirtualMachineConfigSpec{
		DeviceChange: []types.BaseVirtualDeviceConfigSpec{&deviceSpec},
	}

	// Reconfigure VM
	task, err := vm.Reconfigure(ctx, configSpec)
	if err != nil {
		return fmt.Errorf("failed to unmount ISO: %w", err)
	}

	return task.Wait(ctx)
}

// ConnectCDROM connects a VM's CD/DVD drive.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - vm: VirtualMachine object
//
// Returns nil on success, otherwise an error.
//
// Example:
//
//	// Connect CD/DVD drive
//	err := vcenter.ConnectCDROM(ctx, vm)
func ConnectCDROM(ctx context.Context, vm *object.VirtualMachine) error {
	// Get CD/DVD device
	cdrom, _, err := getCDROMDevice(ctx, vm)
	if err != nil {
		return err
	}

	// Preserve existing connectable settings or create new ones
	if cdrom.Connectable == nil {
		cdrom.Connectable = &types.VirtualDeviceConnectInfo{}
	}

	// Update connection state
	cdrom.Connectable.Connected = true
	cdrom.Connectable.StartConnected = true
	cdrom.Connectable.AllowGuestControl = true

	// Create device change spec
	deviceSpec := types.VirtualDeviceConfigSpec{
		Operation: types.VirtualDeviceConfigSpecOperationEdit,
		Device:    cdrom,
	}

	// Create VM config spec
	configSpec := types.VirtualMachineConfigSpec{
		DeviceChange: []types.BaseVirtualDeviceConfigSpec{&deviceSpec},
	}

	// Reconfigure VM
	task, err := vm.Reconfigure(ctx, configSpec)
	if err != nil {
		return fmt.Errorf("failed to connect CDROM: %w", err)
	}

	return task.Wait(ctx)
}

// DisconnectCDROM disconnects a VM's CD/DVD drive.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - vm: VirtualMachine object
//
// Returns nil on success, otherwise an error.
//
// Example:
//
//	// Disconnect CD/DVD drive
//	err := vcenter.DisconnectCDROM(ctx, vm)
func DisconnectCDROM(ctx context.Context, vm *object.VirtualMachine) error {
	// Get CD/DVD device
	cdrom, _, err := getCDROMDevice(ctx, vm)
	if err != nil {
		return err
	}

	// Preserve existing connectable settings or create new ones
	if cdrom.Connectable == nil {
		cdrom.Connectable = &types.VirtualDeviceConnectInfo{}
	}

	// Update connection state
	cdrom.Connectable.Connected = false
	cdrom.Connectable.StartConnected = false
	cdrom.Connectable.AllowGuestControl = true

	// Create device change spec
	deviceSpec := types.VirtualDeviceConfigSpec{
		Operation: types.VirtualDeviceConfigSpecOperationEdit,
		Device:    cdrom,
	}

	// Create VM config spec
	configSpec := types.VirtualMachineConfigSpec{
		DeviceChange: []types.BaseVirtualDeviceConfigSpec{&deviceSpec},
	}

	// Reconfigure VM
	task, err := vm.Reconfigure(ctx, configSpec)
	if err != nil {
		return fmt.Errorf("failed to disconnect CDROM: %w", err)
	}

	return task.Wait(ctx)
}

// ============================================================================
// Guest Operations (via VMware Tools)
// ============================================================================

// NOTE: Guest operations require VMware Tools to be running on the guest VM.
// These functions use the govmomi/guest package for file transfer and script execution.
//
// For full functionality including directory operations, consider using the govmomi
// guest operations API directly. The functions below provide basic file and script
// operations.

// UploadFileToVM uploads a file to a virtual machine via VMware Tools.
//
// The VM must be powered on and have VMware Tools running.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - vm: VirtualMachine object
//   - guestUsername: Username for guest OS (e.g., "Administrator")
//   - guestPassword: Password for guest OS user
//   - localFilePath: Local file path to upload
//   - remoteFilePath: Destination path on guest OS (e.g., "C:\\temp\\script.ps1")
//   - overwrite: Overwrite existing file (default: true)
//
// Returns nil on success, otherwise an error.
//
// Example:
//
//	err := vcenter.UploadFileToVM(ctx, vm, "Administrator", "password",
//	    "/local/script.ps1", "C:\\temp\\script.ps1", true)
func UploadFileToVM(ctx context.Context, vm *object.VirtualMachine, guestUsername string, guestPassword string, localFilePath string, remoteFilePath string, overwrite bool) error {
	file, err := os.Open(localFilePath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat local file: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("local path %s is a directory", localFilePath)
	}

	ops := guest.NewOperationsManager(vm.Client(), vm.Reference())
	fileManager, err := ops.FileManager(ctx)
	if err != nil {
		return fmt.Errorf("failed to create guest file manager: %w", err)
	}

	auth := &types.NamePasswordAuthentication{
		Username: guestUsername,
		Password: guestPassword,
	}

	transferURL, err := fileManager.InitiateFileTransferToGuest(
		ctx,
		auth,
		remoteFilePath,
		&types.GuestFileAttributes{},
		info.Size(),
		overwrite,
	)
	if err != nil {
		return fmt.Errorf("failed to initiate file transfer: %w", err)
	}

	turl, err := fileManager.TransferURL(ctx, transferURL)
	if err != nil {
		return fmt.Errorf("failed to prepare transfer URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, turl.String(), file)
	if err != nil {
		return fmt.Errorf("failed to create upload request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := vm.Client().Client.Client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	return fmt.Errorf("file upload failed (status %d): %s", resp.StatusCode, string(body))
}

// DownloadFileFromVM downloads a file from a virtual machine via VMware Tools.
//
// The VM must be powered on and have VMware Tools running.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - vm: VirtualMachine object
//   - guestUsername: Username for guest OS (e.g., "Administrator")
//   - guestPassword: Password for guest OS user
//   - remoteFilePath: Source path on guest OS (e.g., "C:\\temp\\file.txt")
//   - localFilePath: Destination path on local machine
//
// Returns nil on success, otherwise an error.
//
// Example:
//
//	err := vcenter.DownloadFileFromVM(ctx, vm, "Administrator", "password",
//	    "C:\\temp\\log.txt", "/local/log.txt")
func DownloadFileFromVM(ctx context.Context, vm *object.VirtualMachine, guestUsername string, guestPassword string, remoteFilePath string, localFilePath string) error {
	ops := guest.NewOperationsManager(vm.Client(), vm.Reference())
	fileManager, err := ops.FileManager(ctx)
	if err != nil {
		return fmt.Errorf("failed to create guest file manager: %w", err)
	}

	auth := &types.NamePasswordAuthentication{
		Username: guestUsername,
		Password: guestPassword,
	}

	info, err := fileManager.InitiateFileTransferFromGuest(ctx, auth, remoteFilePath)
	if err != nil {
		return fmt.Errorf("failed to initiate file download: %w", err)
	}

	turl, err := fileManager.TransferURL(ctx, info.Url)
	if err != nil {
		return fmt.Errorf("failed to prepare transfer URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, turl.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create download request: %w", err)
	}

	resp, err := vm.Client().Client.Client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("file download failed (status %d): %s", resp.StatusCode, string(body))
	}

	localFile, err := os.Create(localFilePath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer localFile.Close()

	if _, err := io.Copy(localFile, resp.Body); err != nil {
		return fmt.Errorf("failed to write local file: %w", err)
	}

	return nil
}

// RunScriptOnVM executes a script on a virtual machine via VMware Tools.
//
// The VM must be powered on and have VMware Tools running.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - vm: VirtualMachine object
//   - guestUsername: Username for guest OS (e.g., "Administrator")
//   - guestPassword: Password for guest OS user
//   - scriptPath: Full path to script on guest OS (e.g., "C:\\temp\\script.ps1")
//   - scriptArgs: Arguments to pass to script
//   - workingDirectory: Optional working directory for script execution
//   - waitForCompletion: Wait for script to complete (default: false)
//
// Returns the process ID and error.
//
// Example:
//
//	pid, err := vcenter.RunScriptOnVM(ctx, vm, "Administrator", "password",
//	    "C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe",
//	    []string{"-File", "C:\\temp\\script.ps1"}, "", false)
func RunScriptOnVM(ctx context.Context, vm *object.VirtualMachine, guestUsername string, guestPassword string, scriptPath string, scriptArgs []string, workingDirectory string, waitForCompletion bool) (int64, error) {
	ops := guest.NewOperationsManager(vm.Client(), vm.Reference())
	processManager, err := ops.ProcessManager(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to create guest process manager: %w", err)
	}

	auth := &types.NamePasswordAuthentication{
		Username: guestUsername,
		Password: guestPassword,
	}

	arguments := strings.Join(scriptArgs, " ")
	spec := types.GuestProgramSpec{
		ProgramPath:      scriptPath,
		Arguments:        arguments,
		WorkingDirectory: workingDirectory,
	}

	pid, err := processManager.StartProgram(ctx, auth, &spec)
	if err != nil {
		return 0, fmt.Errorf("failed to start guest program: %w", err)
	}

	if !waitForCompletion {
		return pid, nil
	}

	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-waitCtx.Done():
			return pid, fmt.Errorf("timeout waiting for script completion: %w", waitCtx.Err())
		case <-ticker.C:
			procs, err := processManager.ListProcesses(ctx, auth, []int64{pid})
			if err != nil || len(procs) == 0 {
				continue
			}

			proc := procs[0]
			if proc.EndTime != nil {
				if proc.ExitCode != 0 {
					return pid, fmt.Errorf("script exited with code %d", proc.ExitCode)
				}
				return pid, nil
			}
		}
	}
}

// UploadAndRunScript uploads a script to VM and executes it (helper function).
//
// This is a convenience function that combines UploadFileToVM and
// RunScriptOnVM into a single operation.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - vm: VirtualMachine object
//   - guestUsername: Username for guest OS
//   - guestPassword: Password for guest OS user
//   - localScriptPath: Local script file path
//   - remoteScriptPath: Destination path on guest OS
//   - interpreterPath: Path to interpreter (e.g., "powershell.exe" or "/bin/bash")
//   - scriptArgs: Optional list of additional arguments
//   - waitForCompletion: Wait for script to complete (default: true)
//
// Returns the process ID and error.
//
// Example:
//
//	// PowerShell script on Windows
//	pid, err := vcenter.UploadAndRunScript(ctx, vm, "Administrator", "password",
//	    "/local/setup.ps1", "C:\\temp\\setup.ps1",
//	    "C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe",
//	    []string{"-ExecutionPolicy", "Bypass", "-File", "C:\\temp\\setup.ps1"}, true)
func UploadAndRunScript(ctx context.Context, vm *object.VirtualMachine, guestUsername string, guestPassword string, localScriptPath string, remoteScriptPath string, interpreterPath string, scriptArgs []string, waitForCompletion bool) (int64, error) {
	// Upload script file
	err := UploadFileToVM(ctx, vm, guestUsername, guestPassword, localScriptPath, remoteScriptPath, true)
	if err != nil {
		return 0, fmt.Errorf("failed to upload script: %w", err)
	}

	// Execute script
	pid, err := RunScriptOnVM(ctx, vm, guestUsername, guestPassword, interpreterPath, scriptArgs, "", waitForCompletion)
	if err != nil {
		return 0, fmt.Errorf("failed to run script: %w", err)
	}

	return pid, nil
}

// UploadDirectoryToVM uploads an entire directory to a VM by zipping it locally,
// uploading the zip, extracting on the guest, and cleaning up.
//
// This is much faster than uploading files one by one (single HTTP request vs many).
// Works on both Windows and Linux guests.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - vm: VirtualMachine object
//   - guestUsername: Username for guest OS
//   - guestPassword: Password for guest OS user
//   - localDir: Local directory to upload
//   - remoteDir: Destination directory on guest OS
//   - isWindows: True for Windows guest, false for Linux
//
// Returns nil on success, otherwise an error.
//
// Example:
//
//	err := vcenter.UploadDirectoryToVM(ctx, vm, "Administrator", "password",
//	    "/local/configs", "C:\\App\\configs", true)
func UploadDirectoryToVM(ctx context.Context, vm *object.VirtualMachine, guestUsername string, guestPassword string, localDir string, remoteDir string, isWindows bool) error {
	// Verify local directory exists
	info, err := os.Stat(localDir)
	if err != nil {
		return fmt.Errorf("failed to access local directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", localDir)
	}

	// Create temporary zip file
	tmpZip, err := os.CreateTemp("", "vcenter-upload-*.zip")
	if err != nil {
		return fmt.Errorf("failed to create temp zip file: %w", err)
	}
	tmpZipPath := tmpZip.Name()
	defer os.Remove(tmpZipPath)

	// Zip the directory
	if err := zipDirectory(localDir, tmpZip); err != nil {
		tmpZip.Close()
		return fmt.Errorf("failed to zip directory: %w", err)
	}
	tmpZip.Close()

	// Determine remote paths based on OS
	var remoteZipPath string
	var extractCmd string
	var extractArgs []string
	var cleanupCmd string
	var cleanupArgs []string

	if isWindows {
		remoteZipPath = "C:\\Windows\\Temp\\vcenter-upload-" + fmt.Sprintf("%d", time.Now().UnixNano()) + ".zip"
		extractCmd = "C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe"
		extractArgs = []string{
			"-NoProfile", "-NonInteractive", "-Command",
			fmt.Sprintf("Expand-Archive -Path '%s' -DestinationPath '%s' -Force", remoteZipPath, remoteDir),
		}
		cleanupCmd = "C:\\Windows\\System32\\cmd.exe"
		cleanupArgs = []string{"/c", "del", "/f", "/q", remoteZipPath}
	} else {
		remoteZipPath = "/tmp/vcenter-upload-" + fmt.Sprintf("%d", time.Now().UnixNano()) + ".zip"
		extractCmd = "/usr/bin/unzip"
		extractArgs = []string{"-o", "-q", remoteZipPath, "-d", remoteDir}
		cleanupCmd = "/bin/rm"
		cleanupArgs = []string{"-f", remoteZipPath}
	}

	// Upload zip file
	if err := UploadFileToVM(ctx, vm, guestUsername, guestPassword, tmpZipPath, remoteZipPath, true); err != nil {
		return fmt.Errorf("failed to upload zip file: %w", err)
	}

	// Extract on guest
	_, err = RunScriptOnVM(ctx, vm, guestUsername, guestPassword, extractCmd, extractArgs, "", true)
	if err != nil {
		// Try to clean up zip file even if extract failed
		RunScriptOnVM(ctx, vm, guestUsername, guestPassword, cleanupCmd, cleanupArgs, "", false)
		return fmt.Errorf("failed to extract zip on guest: %w", err)
	}

	// Clean up zip file on guest
	_, err = RunScriptOnVM(ctx, vm, guestUsername, guestPassword, cleanupCmd, cleanupArgs, "", true)
	if err != nil {
		// Non-fatal, just log
		log.Printf("Warning: failed to clean up remote zip file: %v", err)
	}

	return nil
}

// DownloadDirectoryFromVM downloads an entire directory from a VM by zipping it
// on the guest, downloading the zip, and extracting locally.
//
// This is much faster than downloading files one by one.
// Works on both Windows and Linux guests.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - vm: VirtualMachine object
//   - guestUsername: Username for guest OS
//   - guestPassword: Password for guest OS user
//   - remoteDir: Directory on guest OS to download
//   - localDir: Local destination directory
//   - isWindows: True for Windows guest, false for Linux
//
// Returns nil on success, otherwise an error.
//
// Example:
//
//	err := vcenter.DownloadDirectoryFromVM(ctx, vm, "Administrator", "password",
//	    "C:\\App\\logs", "/local/logs", true)
func DownloadDirectoryFromVM(ctx context.Context, vm *object.VirtualMachine, guestUsername string, guestPassword string, remoteDir string, localDir string, isWindows bool) error {
	// Determine remote paths and commands based on OS
	var remoteZipPath string
	var zipCmd string
	var zipArgs []string
	var cleanupCmd string
	var cleanupArgs []string

	if isWindows {
		remoteZipPath = "C:\\Windows\\Temp\\vcenter-download-" + fmt.Sprintf("%d", time.Now().UnixNano()) + ".zip"
		zipCmd = "C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe"
		zipArgs = []string{
			"-NoProfile", "-NonInteractive", "-Command",
			fmt.Sprintf("Compress-Archive -Path '%s\\*' -DestinationPath '%s' -Force", remoteDir, remoteZipPath),
		}
		cleanupCmd = "C:\\Windows\\System32\\cmd.exe"
		cleanupArgs = []string{"/c", "del", "/f", "/q", remoteZipPath}
	} else {
		remoteZipPath = "/tmp/vcenter-download-" + fmt.Sprintf("%d", time.Now().UnixNano()) + ".zip"
		zipCmd = "/usr/bin/zip"
		zipArgs = []string{"-r", "-q", remoteZipPath, "."}
		cleanupCmd = "/bin/rm"
		cleanupArgs = []string{"-f", remoteZipPath}
	}

	// Create zip on guest
	var workDir string
	if !isWindows {
		workDir = remoteDir // zip command needs to run from directory on Linux
	}
	_, err := RunScriptOnVM(ctx, vm, guestUsername, guestPassword, zipCmd, zipArgs, workDir, true)
	if err != nil {
		return fmt.Errorf("failed to create zip on guest: %w", err)
	}

	// Create temporary local zip file
	tmpZip, err := os.CreateTemp("", "vcenter-download-*.zip")
	if err != nil {
		RunScriptOnVM(ctx, vm, guestUsername, guestPassword, cleanupCmd, cleanupArgs, "", false)
		return fmt.Errorf("failed to create temp zip file: %w", err)
	}
	tmpZipPath := tmpZip.Name()
	tmpZip.Close()
	defer os.Remove(tmpZipPath)

	// Download zip file
	if err := DownloadFileFromVM(ctx, vm, guestUsername, guestPassword, remoteZipPath, tmpZipPath); err != nil {
		RunScriptOnVM(ctx, vm, guestUsername, guestPassword, cleanupCmd, cleanupArgs, "", false)
		return fmt.Errorf("failed to download zip file: %w", err)
	}

	// Clean up zip file on guest
	_, err = RunScriptOnVM(ctx, vm, guestUsername, guestPassword, cleanupCmd, cleanupArgs, "", true)
	if err != nil {
		log.Printf("Warning: failed to clean up remote zip file: %v", err)
	}

	// Extract locally
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return fmt.Errorf("failed to create local directory: %w", err)
	}

	if err := unzipFile(tmpZipPath, localDir); err != nil {
		return fmt.Errorf("failed to extract zip locally: %w", err)
	}

	return nil
}

// zipDirectory creates a zip archive of a directory
func zipDirectory(sourceDir string, targetFile *os.File) error {
	zipWriter := zip.NewWriter(targetFile)
	defer zipWriter.Close()

	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if relPath == "." {
			return nil
		}

		// Create zip header
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath)

		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})
}

// unzipFile extracts a zip archive to a directory
func unzipFile(zipPath string, destDir string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		destPath := filepath.Join(destDir, file.Name)

		// Security check: ensure path doesn't escape destination
		if !strings.HasPrefix(filepath.Clean(destPath), filepath.Clean(destDir)) {
			return fmt.Errorf("illegal file path in zip: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(destPath, file.Mode()); err != nil {
				return err
			}
			continue
		}

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		destFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return err
		}

		srcFile, err := file.Open()
		if err != nil {
			destFile.Close()
			return err
		}

		_, err = io.Copy(destFile, srcFile)
		srcFile.Close()
		destFile.Close()

		if err != nil {
			return err
		}
	}

	return nil
}
