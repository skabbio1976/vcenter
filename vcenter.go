package vcenter

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
)

// CloneVM clones a virtual machine from a template.
//
// The function creates a new VM based on the specified template and places it
// in the specified datacenter, datastore, and resource pool.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - client: govmomi.Client for vCenter connection
//   - templateName: The name of the template to clone from
//   - vmName: The name of the new VM
//   - datacenter: The name of the datacenter
//   - datastore: The name of the datastore where the VM should be created
//   - resourcePool: The name of the resource pool (e.g. "Resources")
//   - folder: The name of the VM folder (empty string for default VM folder)
//
// Returns the newly created VM or an error if cloning fails.
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

	ds, err := finder.Datastore(ctx, datastore)
	if err != nil {
		return nil, fmt.Errorf("datastore not found: %w", err)
	}

	pool, err := finder.ResourcePool(ctx, resourcePool)
	if err != nil {
		return nil, fmt.Errorf("resource pool not found: %w", err)
	}

	var vmFolder *object.Folder
	if folder != "" {
		vmFolder, err = finder.Folder(ctx, folder)
		if err != nil {
			return nil, fmt.Errorf("folder not found: %w", err)
		}
	} else {
		folders, err := dc.Folders(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get folders: %w", err)
		}
		vmFolder = folders.VmFolder
	}

	relocateSpec := types.VirtualMachineRelocateSpec{
		Datastore:    types.NewReference(ds.Reference()),
		Pool:         types.NewReference(pool.Reference()),
		DiskMoveType: string(types.VirtualMachineRelocateDiskMoveOptionsMoveAllDiskBackingsAndAllowSharing),
	}

	cloneSpec := types.VirtualMachineCloneSpec{
		Location: relocateSpec,
		PowerOn:  false,
		Template: false,
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

// CloneVMWithCustomization clones a virtual machine with Windows customization.
//
// The function clones a VM from a template and applies Windows customizations
// such as computer name, domain join, IP configuration, and timezone.
// The VM is started automatically after cloning so that customization can run.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - client: govmomi.Client for vCenter connection
//   - templateName: The name of the template to clone from
//   - vmName: The name of the new VM
//   - datacenter: The name of the datacenter
//   - datastore: The name of the datastore where the VM should be created
//   - resourcePool: The name of the resource pool
//   - folder: The name of the VM folder (empty string for default)
//   - customization: CustomizationSpec with all Windows settings
//
// Returns the newly created VM or an error.
//
// Example:
//
//	customization := vcenter.NewWindowsCustomization(...)
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

	ds, err := finder.Datastore(ctx, datastore)
	if err != nil {
		return nil, fmt.Errorf("datastore not found: %w", err)
	}

	pool, err := finder.ResourcePool(ctx, resourcePool)
	if err != nil {
		return nil, fmt.Errorf("resource pool not found: %w", err)
	}

	var vmFolder *object.Folder
	if folder != "" {
		vmFolder, err = finder.Folder(ctx, folder)
		if err != nil {
			return nil, fmt.Errorf("folder not found: %w", err)
		}
	} else {
		folders, err := dc.Folders(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get folders: %w", err)
		}
		vmFolder = folders.VmFolder
	}

	relocateSpec := types.VirtualMachineRelocateSpec{
		Datastore:    types.NewReference(ds.Reference()),
		Pool:         types.NewReference(pool.Reference()),
		DiskMoveType: string(types.VirtualMachineRelocateDiskMoveOptionsMoveAllDiskBackingsAndAllowSharing),
	}

	cloneSpec := types.VirtualMachineCloneSpec{
		Location:      relocateSpec,
		PowerOn:       true, // Must be true for customization
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

// NewWindowsCustomization creates a Windows customization spec for domain join with DHCP.
//
// The function generates a CustomizationSpec that can be used when cloning VMs
// to automatically:
//   - Set computer name
//   - Join Active Directory domain
//   - Configure local administrator password
//   - Set timezone
//   - Configure DNS servers (IP retrieved from DHCP)
//
// Parameters:
//   - computerName: The computer name in Windows
//   - domain: AD domain to join (e.g. "example.com")
//   - domainUser: Domain admin user (e.g. "administrator@example.com")
//   - domainPassword: Password for domain admin
//   - adminPassword: Local administrator password
//   - timezone: Windows timezone ID (85 for W. Europe, 110 for Pacific, etc)
//   - dnsServers: List of DNS server IP addresses
//   - dnsSuffixes: List of DNS search suffixes
//
// Returns a CustomizationSpec ready to be used with CloneVMWithCustomization.
//
// Example:
//
//	spec := vcenter.NewWindowsCustomization("WebServer01", "example.com",
//	    "admin@example.com", "domainpass", "adminpass", 85,
//	    []string{"192.168.1.1", "192.168.1.2"},
//	    []string{"example.com"})
func NewWindowsCustomization(
	computerName string,
	domain string,
	domainUser string,
	domainPassword string,
	adminPassword string,
	timezone int,
	dnsServers []string,
	dnsSuffixes []string,
) *types.CustomizationSpec {

	return &types.CustomizationSpec{
		Identity: &types.CustomizationSysprep{
			GuiUnattended: types.CustomizationGuiUnattended{
				Password: &types.CustomizationPassword{
					Value:     adminPassword,
					PlainText: true,
				},
				TimeZone:       int32(timezone),
				AutoLogon:      false,
				AutoLogonCount: 1,
			},
			UserData: types.CustomizationUserData{
				FullName: "Administrator",
				OrgName:  "Organization",
				ComputerName: &types.CustomizationFixedName{
					Name: computerName,
				},
			},
			Identification: types.CustomizationIdentification{
				JoinDomain:  domain,
				DomainAdmin: domainUser,
				DomainAdminPassword: &types.CustomizationPassword{
					Value:     domainPassword,
					PlainText: true,
				},
			},
		},
		GlobalIPSettings: types.CustomizationGlobalIPSettings{
			DnsServerList: dnsServers,
			DnsSuffixList: dnsSuffixes,
		},
		NicSettingMap: []types.CustomizationAdapterMapping{
			{
				Adapter: types.CustomizationIPSettings{
					Ip: &types.CustomizationDhcpIpGenerator{},
				},
			},
		},
	}
}

// NewWindowsCustomizationStaticIP creates a Windows customization spec with a static IP address.
//
// The function is identical to NewWindowsCustomization but configures a static
// IP address instead of DHCP.
//
// Parameters:
//   - computerName: The computer name in Windows
//   - domain: AD domain to join
//   - domainUser: Domain admin user
//   - domainPassword: Password for domain admin
//   - adminPassword: Local administrator password
//   - timezone: Windows timezone ID
//   - ipAddress: Static IP address (e.g. "192.168.1.100")
//   - subnetMask: Subnet mask (e.g. "255.255.255.0")
//   - gateway: Default gateway IP address
//   - dnsServers: List of DNS server IP addresses
//   - dnsSuffixes: List of DNS search suffixes
//
// Returns a CustomizationSpec with static IP configuration.
//
// Example:
//
//	spec := vcenter.NewWindowsCustomizationStaticIP("WebServer01", "example.com",
//	    "admin@example.com", "domainpass", "adminpass", 85,
//	    "192.168.1.100", "255.255.255.0", "192.168.1.1",
//	    []string{"192.168.1.1"}, []string{"example.com"})
func NewWindowsCustomizationStaticIP(
	computerName string,
	domain string,
	domainUser string,
	domainPassword string,
	adminPassword string,
	timezone int,
	ipAddress string,
	subnetMask string,
	gateway string,
	dnsServers []string,
	dnsSuffixes []string,
) *types.CustomizationSpec {

	return &types.CustomizationSpec{
		Identity: &types.CustomizationSysprep{
			GuiUnattended: types.CustomizationGuiUnattended{
				Password: &types.CustomizationPassword{
					Value:     adminPassword,
					PlainText: true,
				},
				TimeZone:       int32(timezone),
				AutoLogon:      false,
				AutoLogonCount: 1,
			},
			UserData: types.CustomizationUserData{
				FullName: "Administrator",
				OrgName:  "Organization",
				ComputerName: &types.CustomizationFixedName{
					Name: computerName,
				},
			},
			Identification: types.CustomizationIdentification{
				JoinDomain:  domain,
				DomainAdmin: domainUser,
				DomainAdminPassword: &types.CustomizationPassword{
					Value:     domainPassword,
					PlainText: true,
				},
			},
		},
		GlobalIPSettings: types.CustomizationGlobalIPSettings{
			DnsServerList: dnsServers,
			DnsSuffixList: dnsSuffixes,
		},
		NicSettingMap: []types.CustomizationAdapterMapping{
			{
				Adapter: types.CustomizationIPSettings{
					Ip: &types.CustomizationFixedIp{
						IpAddress: ipAddress,
					},
					SubnetMask:    subnetMask,
					Gateway:       []string{gateway},
					DnsServerList: dnsServers,
				},
			},
		},
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
			var props types.ManagedObjectReference
			err := vm.Properties(ctx, vm.Reference(), []string{"guest.toolsRunningStatus"}, &props)
			if err != nil {
				continue
			}

			var vmProps struct {
				Guest types.GuestInfo `mo:"guest"`
			}
			err = vm.Properties(ctx, vm.Reference(), []string{"guest"}, &vmProps)
			if err != nil {
				continue
			}

			if vmProps.Guest.ToolsRunningStatus == string(types.VirtualMachineToolsRunningStatusGuestToolsRunning) {
				return nil
			}
		}
	}
}

// ============================================================================
// JSON/Config Parsing
// ============================================================================

// ServerRequest represents a server order with all necessary configuration
type ServerRequest struct {
	Name        string   `json:"name"`
	Template    string   `json:"template"`
	CPUs        int32    `json:"cpus"`
	MemoryGB    int      `json:"memory_gb"`
	DiskGB      int      `json:"disk_gb"`
	Domain      string   `json:"domain"`
	IPAddress   string   `json:"ip_address,omitempty"`
	SubnetMask  string   `json:"subnet_mask,omitempty"` // E.g. "255.255.255.0"
	Gateway     string   `json:"gateway,omitempty"`     // Default gateway
	DNSServers  []string `json:"dns_servers"`
	DNSSuffixes []string `json:"dns_suffixes,omitempty"` // DNS search suffixes
}

// Validate validates ServerRequest and returns a ValidationError if something is wrong
func (r *ServerRequest) Validate() error {
	if r.Name == "" {
		return &ValidationError{Field: "Name", Message: "name is required"}
	}
	if r.Template == "" {
		return &ValidationError{Field: "Template", Message: "template is required"}
	}
	if r.IPAddress != "" {
		if r.SubnetMask == "" {
			return &ValidationError{Field: "SubnetMask", Message: "subnet mask is required when using static IP"}
		}
		if r.Gateway == "" {
			return &ValidationError{Field: "Gateway", Message: "gateway is required when using static IP"}
		}
	}
	return nil
}

// CloneFromRequest clones a virtual machine based on a ServerRequest struct.
//
// The function validates the request, creates a customization spec based on IP configuration
// (static or DHCP), clones the VM, and sets CPU/memory if specified.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - client: govmomi.Client for vCenter connection
//   - req: ServerRequest with all VM configuration
//   - datacenter: The name of the datacenter
//   - datastore: The name of the datastore
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
//	    Name: "WebServer01",
//	    Template: "Win2022-Template",
//	    CPUs: 4,
//	    MemoryGB: 8,
//	    Domain: "example.com",
//	    IPAddress: "192.168.1.100",
//	    SubnetMask: "255.255.255.0",
//	    Gateway: "192.168.1.1",
//	    DNSServers: []string{"192.168.1.1"},
//	}
//	vm, err := vcenter.CloneFromRequest(ctx, client, req, "DC1",
//	    "datastore1", "Resources", "", "admin@example.com",
//	    "domainpass", "adminpass", 85)
func CloneFromRequest(ctx context.Context, client *govmomi.Client, req ServerRequest, datacenter, datastore, resourcePool, folder string, domainUser, domainPassword, adminPassword string, timezone int) (*object.VirtualMachine, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Create DNS suffixes if not specified
	dnsSuffixes := req.DNSSuffixes
	if len(dnsSuffixes) == 0 && req.Domain != "" {
		dnsSuffixes = []string{req.Domain}
	}

	// Create customization spec
	var customization *types.CustomizationSpec
	if req.Domain != "" {
		if req.IPAddress != "" {
			// Static IP
			customization = NewWindowsCustomizationStaticIP(
				req.Name,
				req.Domain,
				domainUser,
				domainPassword,
				adminPassword,
				timezone,
				req.IPAddress,
				req.SubnetMask,
				req.Gateway,
				req.DNSServers,
				dnsSuffixes,
			)
		} else {
			// DHCP
			customization = NewWindowsCustomization(
				req.Name,
				req.Domain,
				domainUser,
				domainPassword,
				adminPassword,
				timezone,
				req.DNSServers,
				dnsSuffixes,
			)
		}
	}

	// Clone VM
	var vm *object.VirtualMachine
	var err error
	if customization != nil {
		vm, err = CloneVMWithCustomization(ctx, client, req.Template, req.Name, datacenter, datastore, resourcePool, folder, customization)
	} else {
		vm, err = CloneVM(ctx, client, req.Template, req.Name, datacenter, datastore, resourcePool, folder)
	}

	if err != nil {
		return nil, err
	}

	// Change resources if specified
	if req.CPUs > 0 || req.MemoryGB > 0 {
		cpus := req.CPUs
		if cpus == 0 {
			cpus = 2 // Default
		}
		memoryMB := int64(req.MemoryGB * 1024)
		if memoryMB == 0 {
			memoryMB = 4096 // Default 4GB
		}

		err = SetVMResources(ctx, vm, cpus, memoryMB)
		if err != nil {
			return vm, fmt.Errorf("VM created but failed to set resources: %w", err)
		}
	}

	return vm, nil
}

// ============================================================================
// Batch Operations
// ============================================================================

// CloneMultiple clones multiple virtual machines in parallel.
//
// The function uses goroutines to clone multiple VMs simultaneously, which
// greatly improves performance compared to sequential cloning.
//
// Parameters: Same as CloneFromRequest, plus a list of ServerRequests
//
// Returns:
//   - A slice with successfully cloned VMs
//   - A slice with errors (indexed same as requests, nil for successful ones)
//
// Example:
//
//	requests := []vcenter.ServerRequest{
//	    {Name: "Web01", Template: "Win2022-Template", ...},
//	    {Name: "Web02", Template: "Win2022-Template", ...},
//	}
//	vms, errors := vcenter.CloneMultiple(ctx, client, requests,
//	    "DC1", "datastore1", "Resources", "", "admin@example.com",
//	    "domainpass", "adminpass", 85)
//	for i, err := range errors {
//	    if err != nil {
//	        log.Printf("Failed to clone %s: %v", requests[i].Name, err)
//	    }
//	}
func CloneMultiple(ctx context.Context, client *govmomi.Client, requests []ServerRequest, datacenter, datastore, resourcePool, folder string, domainUser, domainPassword, adminPassword string, timezone int) ([]*object.VirtualMachine, []error) {
	var wg sync.WaitGroup
	vms := make([]*object.VirtualMachine, len(requests))
	errors := make([]error, len(requests))

	for i, req := range requests {
		wg.Add(1)
		go func(idx int, r ServerRequest) {
			defer wg.Done()
			vm, err := CloneFromRequest(ctx, client, r, datacenter, datastore, resourcePool, folder, domainUser, domainPassword, adminPassword, timezone)
			vms[idx] = vm
			errors[idx] = err
		}(i, req)
	}

	wg.Wait()

	// Collect all VMs that were successful
	successfulVMs := make([]*object.VirtualMachine, 0)
	var firstError error
	for i, vm := range vms {
		if errors[i] == nil && vm != nil {
			successfulVMs = append(successfulVMs, vm)
		} else if errors[i] != nil && firstError == nil {
			firstError = errors[i]
		}
	}

	return successfulVMs, errors
}

// BulkPowerOperation performs power operations on multiple virtual machines in parallel.
//
// The function uses goroutines to perform operations on multiple VMs simultaneously.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - vms: List of VirtualMachine objects to operate on
//   - operation: Operation to perform ("on", "off", or "restart")
//
// Returns a slice with errors (indexed same as vms, nil for successful ones).
//
// Example:
//
//	errors := vcenter.BulkPowerOperation(ctx, vms, "on")
//	for i, err := range errors {
//	    if err != nil {
//	        log.Printf("Failed to power on VM %d: %v", i, err)
//	    }
//	}
func BulkPowerOperation(ctx context.Context, vms []*object.VirtualMachine, operation string) []error {
	var wg sync.WaitGroup
	errors := make([]error, len(vms))

	for i, vm := range vms {
		wg.Add(1)
		go func(idx int, v *object.VirtualMachine) {
			defer wg.Done()
			var err error
			switch operation {
			case "on":
				err = PowerOnVM(ctx, v)
			case "off":
				err = PowerOffVM(ctx, v)
			case "restart":
				err = RestartVM(ctx, v)
			default:
				err = fmt.Errorf("unknown operation: %s (valid: on, off, restart)", operation)
			}
			errors[idx] = err
		}(i, vm)
	}

	wg.Wait()
	return errors
}

// ============================================================================
// Disk Operations
// ============================================================================

// AddDisk adds a new disk to a virtual machine.
//
// The disk is created as thin provisioned VMDK on the specified datastore.
// The VM can be running during the operation (hot-add, if supported by VM).
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - vm: VirtualMachine object to add disk to
//   - sizeGB: Disk size in GB (e.g. 100 for 100GB)
//   - datastoreName: The name of the datastore where the disk should be created
//
// Returns nil on success, otherwise an error.
//
// Example:
//
//	err := vcenter.AddDisk(ctx, vm, 100, "datastore1") // Add 100GB disk
func AddDisk(ctx context.Context, vm *object.VirtualMachine, sizeGB int, datastoreName string) error {
	var vprops struct {
		Config struct {
			Hardware struct {
				Device []types.BaseVirtualDevice
			}
		}
	}

	err := vm.Properties(ctx, vm.Reference(), []string{"config.hardware.device"}, &vprops)
	if err != nil {
		return fmt.Errorf("failed to get VM properties: %w", err)
	}

	devices := object.VirtualDeviceList(vprops.Config.Hardware.Device)

	// Find SCSI controller
	controller, err := devices.FindSCSIController("")
	if err != nil {
		return fmt.Errorf("failed to find SCSI controller: %w", err)
	}

	// Find datastore
	finder := find.NewFinder(vm.Client(), true)
	ds, err := finder.Datastore(ctx, datastoreName)
	if err != nil {
		return fmt.Errorf("datastore not found: %w", err)
	}

	// Create new disk
	disk := &types.VirtualDisk{
		VirtualDevice: types.VirtualDevice{
			Key: devices.NewKey(),
			Backing: &types.VirtualDiskFlatVer2BackingInfo{
				DiskMode:        string(types.VirtualDiskModePersistent),
				ThinProvisioned: types.NewBool(true),
				VirtualDeviceFileBackingInfo: types.VirtualDeviceFileBackingInfo{
					Datastore: types.NewReference(ds.Reference()),
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
	var vprops struct {
		Config struct {
			Hardware struct {
				Device []types.BaseVirtualDevice
			}
		}
	}

	err := vm.Properties(ctx, vm.Reference(), []string{"config.hardware.device"}, &vprops)
	if err != nil {
		return fmt.Errorf("failed to get VM properties: %w", err)
	}

	devices := object.VirtualDeviceList(vprops.Config.Hardware.Device)

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
	var vprops struct {
		Config struct {
			Hardware struct {
				Device []types.BaseVirtualDevice
			}
		}
	}

	err := vm.Properties(ctx, vm.Reference(), []string{"config.hardware.device"}, &vprops)
	if err != nil {
		return fmt.Errorf("failed to get VM properties: %w", err)
	}

	devices := object.VirtualDeviceList(vprops.Config.Hardware.Device)

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
	var vprops struct {
		Config struct {
			Hardware struct {
				Device []types.BaseVirtualDevice
			}
		}
	}

	err := vm.Properties(ctx, vm.Reference(), []string{"config.hardware.device"}, &vprops)
	if err != nil {
		return fmt.Errorf("failed to get VM properties: %w", err)
	}

	devices := object.VirtualDeviceList(vprops.Config.Hardware.Device)

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
	var vprops struct {
		Config struct {
			Hardware struct {
				Device []types.BaseVirtualDevice
			}
		}
	}

	err := vm.Properties(ctx, vm.Reference(), []string{"config.hardware.device"}, &vprops)
	if err != nil {
		return fmt.Errorf("failed to get VM properties: %w", err)
	}

	devices := object.VirtualDeviceList(vprops.Config.Hardware.Device)

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
