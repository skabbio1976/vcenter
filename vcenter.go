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

// CloneVM klonar en virtuell maskin från en template.
//
// Funktionen skapar en ny VM baserat på den angivna templaten och placerar den
// i det specificerade datacentret, datastore och resource pool.
//
// Parametrar:
//   - ctx: Context för timeout och cancellation
//   - client: govmomi.Client för vCenter-anslutning
//   - templateName: Namnet på templaten att klona från
//   - vmName: Namnet på den nya VM:en
//   - datacenter: Namnet på datacentret
//   - datastore: Namnet på datastore där VM:en ska skapas
//   - resourcePool: Namnet på resource pool (t.ex. "Resources")
//   - folder: Namnet på VM-mappen (tom sträng för default VM folder)
//
// Returnerar den nyskapade VM:en eller ett fel om kloningen misslyckas.
//
// Exempel:
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

// CloneVMWithCustomization klonar en virtuell maskin med Windows customization.
//
// Funktionen klonar en VM från en template och applicerar Windows-anpassningar
// såsom datornamn, domain join, IP-konfiguration och timezone.
// VM:en startas automatiskt efter kloning för att customization ska köras.
//
// Parametrar:
//   - ctx: Context för timeout och cancellation
//   - client: govmomi.Client för vCenter-anslutning
//   - templateName: Namnet på templaten att klona från
//   - vmName: Namnet på den nya VM:en
//   - datacenter: Namnet på datacentret
//   - datastore: Namnet på datastore där VM:en ska skapas
//   - resourcePool: Namnet på resource pool
//   - folder: Namnet på VM-mappen (tom sträng för default)
//   - customization: CustomizationSpec med alla Windows-inställningar
//
// Returnerar den nyskapade VM:en eller ett fel.
//
// Exempel:
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
		PowerOn:       true, // Måste vara true för customization
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

// NewWindowsCustomization skapar en Windows customization spec för domain join med DHCP.
//
// Funktionen genererar en CustomizationSpec som kan användas vid VM-kloning
// för att automatiskt:
//   - Sätta datornamn
//   - Joina Active Directory domain
//   - Konfigurera lokal administrator-lösenord
//   - Sätta timezone
//   - Konfigurera DNS-servrar (IP hämtas från DHCP)
//
// Parametrar:
//   - computerName: Datornamnet i Windows
//   - domain: AD domain att joina (t.ex. "example.com")
//   - domainUser: Domain admin-användare (t.ex. "administrator@example.com")
//   - domainPassword: Lösenord för domain admin
//   - adminPassword: Lokalt administrator-lösenord
//   - timezone: Windows timezone ID (85 för W. Europe, 110 för Pacific, etc)
//   - dnsServers: Lista med DNS-server IP-adresser
//   - dnsSuffixes: Lista med DNS search suffixes
//
// Returnerar en CustomizationSpec redo att användas med CloneVMWithCustomization.
//
// Exempel:
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

// NewWindowsCustomizationStaticIP skapar en Windows customization spec med statisk IP-adress.
//
// Funktionen är identisk med NewWindowsCustomization men konfigurerar statisk
// IP-adress istället för DHCP.
//
// Parametrar:
//   - computerName: Datornamnet i Windows
//   - domain: AD domain att joina
//   - domainUser: Domain admin-användare
//   - domainPassword: Lösenord för domain admin
//   - adminPassword: Lokalt administrator-lösenord
//   - timezone: Windows timezone ID
//   - ipAddress: Statisk IP-adress (t.ex. "192.168.1.100")
//   - subnetMask: Subnätmask (t.ex. "255.255.255.0")
//   - gateway: Default gateway IP-adress
//   - dnsServers: Lista med DNS-server IP-adresser
//   - dnsSuffixes: Lista med DNS search suffixes
//
// Returnerar en CustomizationSpec med statisk IP-konfiguration.
//
// Exempel:
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

// SetVMResources ändrar CPU och minne på en virtuell maskin.
//
// VM:en behöver vara avstängd för att ändra dessa resurser (beroende på VM configuration).
//
// Parametrar:
//   - ctx: Context för timeout och cancellation
//   - vm: VirtualMachine-objektet att modifiera
//   - numCPUs: Antal CPU:er (t.ex. 2, 4, 8)
//   - memoryMB: Minne i MB (t.ex. 4096 för 4GB, 8192 för 8GB)
//
// Returnerar nil vid framgång, annars ett fel.
//
// Exempel:
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

// GetVM hittar en virtuell maskin baserat på namn.
//
// Om datacenter är tomt söks det i alla datacenter.
//
// Parametrar:
//   - ctx: Context för timeout och cancellation
//   - client: govmomi.Client för vCenter-anslutning
//   - vmName: Namnet på VM:en att hitta
//   - datacenter: Namnet på datacentret (tom sträng för att söka överallt)
//
// Returnerar VirtualMachine-objektet eller ett NotFoundError.
//
// Exempel:
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

// PowerOnVM startar en virtuell maskin.
//
// Funktionen väntar tills power-on operationen är klar.
//
// Returnerar nil vid framgång, annars ett fel.
//
// Exempel:
//
//	err := vcenter.PowerOnVM(ctx, vm)
func PowerOnVM(ctx context.Context, vm *object.VirtualMachine) error {
	task, err := vm.PowerOn(ctx)
	if err != nil {
		return fmt.Errorf("failed to start power on: %w", err)
	}

	return task.Wait(ctx)
}

// PowerOffVM stänger av en virtuell maskin.
//
// Detta är en "hard" power off (motsvarar att dra ur elkabeln).
// För graceful shutdown, använd guest.Shutdown() istället.
//
// Returnerar nil vid framgång, annars ett fel.
//
// Exempel:
//
//	err := vcenter.PowerOffVM(ctx, vm)
func PowerOffVM(ctx context.Context, vm *object.VirtualMachine) error {
	task, err := vm.PowerOff(ctx)
	if err != nil {
		return fmt.Errorf("failed to start power off: %w", err)
	}

	return task.Wait(ctx)
}

// RestartVM startar om en virtuell maskin.
//
// Funktionen försöker först en graceful restart med VMware Tools (RebootGuest).
// Om det misslyckas görs en hard reset istället.
//
// Returnerar nil vid framgång, annars ett fel.
//
// Exempel:
//
//	err := vcenter.RestartVM(ctx, vm)
func RestartVM(ctx context.Context, vm *object.VirtualMachine) error {
	err := vm.RebootGuest(ctx)
	if err != nil {
		// Om guest tools inte fungerar, gör hard reset
		task, err := vm.Reset(ctx)
		if err != nil {
			return fmt.Errorf("failed to restart VM: %w", err)
		}
		return task.Wait(ctx)
	}
	return nil
}

// WaitForIP väntar tills VM:en får en routable IP-adress.
//
// Funktionen pollar VM:en var 2:a sekund tills en IP-adress hittas eller timeout nås.
// Kräver att VMware Tools är installerat och körs i gästsystemet.
//
// Parametrar:
//   - ctx: Context för timeout och cancellation
//   - vm: VirtualMachine-objektet att vänta på
//   - timeout: Max tid att vänta (t.ex. 5*time.Minute)
//
// Returnerar IP-adressen eller ett timeout-fel.
//
// Exempel:
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

// WaitForTools väntar på att VMware Tools ska bli redo.
//
// Funktionen pollar VM:en var 2:a sekund tills VMware Tools är running.
// Max väntetid är 5 minuter.
//
// Returnerar nil när Tools är redo, annars ett timeout-fel.
//
// Exempel:
//
//	err := vcenter.WaitForTools(ctx, vm)
//	if err == nil {
//	    // VMware Tools är nu redo att använda
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

// ServerRequest representerar en server-beställning med all nödvändig konfiguration
type ServerRequest struct {
	Name        string   `json:"name"`
	Template    string   `json:"template"`
	CPUs        int32    `json:"cpus"`
	MemoryGB    int      `json:"memory_gb"`
	DiskGB      int      `json:"disk_gb"`
	Domain      string   `json:"domain"`
	IPAddress   string   `json:"ip_address,omitempty"`
	SubnetMask  string   `json:"subnet_mask,omitempty"` // T.ex. "255.255.255.0"
	Gateway     string   `json:"gateway,omitempty"`     // Default gateway
	DNSServers  []string `json:"dns_servers"`
	DNSSuffixes []string `json:"dns_suffixes,omitempty"` // DNS search suffixes
}

// Validate validerar ServerRequest och returnerar ett ValidationError om något är fel
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

// CloneFromRequest klonar en virtuell maskin baserat på en ServerRequest-struct.
//
// Funktionen validerar requesten, skapar customization spec baserat på IP-konfiguration
// (statisk eller DHCP), klonar VM:en och sätter CPU/minne om specificerat.
//
// Parametrar:
//   - ctx: Context för timeout och cancellation
//   - client: govmomi.Client för vCenter-anslutning
//   - req: ServerRequest med all VM-konfiguration
//   - datacenter: Namnet på datacentret
//   - datastore: Namnet på datastore
//   - resourcePool: Namnet på resource pool
//   - folder: Namnet på VM-mappen (tom för default)
//   - domainUser: Domain admin för domain join
//   - domainPassword: Lösenord för domain admin
//   - adminPassword: Lokalt admin-lösenord
//   - timezone: Windows timezone ID
//
// Returnerar den nyskapade VM:en eller ett ValidationError/OperationError.
//
// Exempel:
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
	// Validera request
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Skapa DNS suffixes om inte specificerat
	dnsSuffixes := req.DNSSuffixes
	if len(dnsSuffixes) == 0 && req.Domain != "" {
		dnsSuffixes = []string{req.Domain}
	}

	// Skapa customization spec
	var customization *types.CustomizationSpec
	if req.Domain != "" {
		if req.IPAddress != "" {
			// Statisk IP
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

	// Klona VM
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

	// Ändra resources om specificerat
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

// CloneMultiple klonar flera virtuella maskiner parallellt.
//
// Funktionen använder goroutines för att klona flera VMs samtidigt, vilket
// kraftigt förbättrar prestandan jämfört med sekventiell kloning.
//
// Parametrar: Samma som CloneFromRequest, plus en lista av ServerRequests
//
// Returnerar:
//   - En slice med framgångsrikt klonade VMs
//   - En slice med errors (indexerad samma som requests, nil för lyckade)
//
// Exempel:
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

	// Samla alla VMs som lyckades
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

// BulkPowerOperation utför power-operationer på flera virtuella maskiner parallellt.
//
// Funktionen använder goroutines för att utföra operationer på flera VMs samtidigt.
//
// Parametrar:
//   - ctx: Context för timeout och cancellation
//   - vms: Lista med VirtualMachine-objekt att operera på
//   - operation: Operation att utföra ("on", "off", eller "restart")
//
// Returnerar en slice med errors (indexerad samma som vms, nil för lyckade).
//
// Exempel:
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

// AddDisk lägger till en ny disk på en virtuell maskin.
//
// Disken skapas som thin provisioned VMDK på den angivna datastore.
// VM:en kan vara igång under operationen (hot-add, om stöds av VM).
//
// Parametrar:
//   - ctx: Context för timeout och cancellation
//   - vm: VirtualMachine-objektet att lägga till disk på
//   - sizeGB: Diskstorlek i GB (t.ex. 100 för 100GB)
//   - datastoreName: Namnet på datastore där disken ska skapas
//
// Returnerar nil vid framgång, annars ett fel.
//
// Exempel:
//
//	err := vcenter.AddDisk(ctx, vm, 100, "datastore1") // Lägg till 100GB disk
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

	// Hitta SCSI controller
	controller, err := devices.FindSCSIController("")
	if err != nil {
		return fmt.Errorf("failed to find SCSI controller: %w", err)
	}

	// Hitta datastore
	finder := find.NewFinder(vm.Client(), true)
	ds, err := finder.Datastore(ctx, datastoreName)
	if err != nil {
		return fmt.Errorf("datastore not found: %w", err)
	}

	// Skapa ny disk
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

// ExtendDisk utökar en befintlig disk till en ny storlek.
//
// Disken identifieras med sitt label (t.ex. "Hard disk 2").
// Den nya storleken måste vara större än nuvarande storlek.
// VM:en kan vara igång under operationen.
//
// OBS: Detta utökar bara disken i vSphere. Du behöver manuellt utöka
// partitionen i gästoperativsystemet efteråt.
//
// Parametrar:
//   - ctx: Context för timeout och cancellation
//   - vm: VirtualMachine-objektet
//   - diskLabel: Namnet/label på disken (t.ex. "Hard disk 2")
//   - newSizeGB: Ny storlek i GB (måste vara större än nuvarande)
//
// Returnerar nil vid framgång, annars ett ValidationError eller OperationError.
//
// Exempel:
//
//	err := vcenter.ExtendDisk(ctx, vm, "Hard disk 2", 200) // Utöka till 200GB
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

	// Hitta disken baserat på label
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

	// Kontrollera att nya storleken är större än nuvarande
	currentSizeGB := disk.CapacityInKB / (1024 * 1024)
	if int64(newSizeGB) <= currentSizeGB {
		return fmt.Errorf("new size (%d GB) must be larger than current size (%d GB)", newSizeGB, currentSizeGB)
	}

	// Uppdatera disk storlek
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

// RemoveDisk tar bort en disk från en virtuell maskin.
//
// Både disken från VM-konfigurationen och VMDK-filen raderas permanent.
// VM:en kan vara igång under operationen (hot-remove, om stöds av VM).
//
// VARNING: Detta raderar data permanent!
//
// Parametrar:
//   - ctx: Context för timeout och cancellation
//   - vm: VirtualMachine-objektet
//   - diskLabel: Namnet/label på disken att ta bort (t.ex. "Hard disk 2")
//
// Returnerar nil vid framgång, annars ett NotFoundError eller OperationError.
//
// Exempel:
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

	// Hitta disken baserat på label
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

// AddNetworkAdapter lägger till ett nytt nätverkskort på en virtuell maskin.
//
// Nätverkskortet skapas som VMXNET3 (VMware paravirtualized adapter).
// VM:en kan vara igång under operationen (hot-add, om stöds av VM).
//
// Parametrar:
//   - ctx: Context för timeout och cancellation
//   - vm: VirtualMachine-objektet
//   - networkName: Namnet på nätverket/portgruppen att ansluta till
//
// Returnerar nil vid framgång, annars ett NotFoundError eller OperationError.
//
// Exempel:
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

	// Hitta nätverket
	finder := find.NewFinder(vm.Client(), true)
	network, err := finder.Network(ctx, networkName)
	if err != nil {
		return fmt.Errorf("network not found: %w", err)
	}

	// Skapa nätverkskort (VMXNET3)
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

// ChangeNetwork byter nätverk på ett befintligt nätverkskort.
//
// Nätverkskortet identifieras med sitt label (t.ex. "Network adapter 1").
// VM:en kan vara igång under operationen.
//
// Parametrar:
//   - ctx: Context för timeout och cancellation
//   - vm: VirtualMachine-objektet
//   - adapterLabel: Namnet/label på nätverkskortet (t.ex. "Network adapter 1")
//   - newNetworkName: Namnet på det nya nätverket/portgruppen
//
// Returnerar nil vid framgång, annars ett NotFoundError eller OperationError.
//
// Exempel:
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

	// Hitta nätverkskortet baserat på label
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

	// Hitta nya nätverket
	finder := find.NewFinder(vm.Client(), true)
	network, err := finder.Network(ctx, newNetworkName)
	if err != nil {
		return fmt.Errorf("network not found: %w", err)
	}

	backing, err := network.EthernetCardBackingInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to get network backing: %w", err)
	}

	// Uppdatera backing
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
