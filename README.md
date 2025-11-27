# vcenter - PowerCLI-inspired Go API for VMware vCenter

A user-friendly Go library for VMware vCenter, inspired by PowerCLI. This package wraps [govmomi](https://github.com/vmware/govmomi) and provides a simpler, more intuitive API for common vCenter operations.

## Features

- **Easy Authentication**
  - Username/password with session caching
  - Windows SSPI/Kerberos (single sign-on)
  - Automatic session management via [go-vcenter-auth](https://github.com/skabbio1976/go-vcenter-auth)

- **VM Management**
  - Clone VMs from templates
  - Windows and Linux customization (domain join, static IP, multi-NIC)
  - CPU and memory configuration
  - Power operations (on/off/restart)
  - Delete and unregister VMs
  - Get detailed VM information
  - Support for datastore clusters (Storage DRS)

- **Guest Customization**
  - Windows Sysprep with domain join or workgroup
  - Linux customization
  - Multi-NIC support with per-adapter IP configuration
  - MachineObjectOU for AD placement
  - Autologon support
  - **Reliable customization completion detection** (hostname-based)

- **Snapshot Operations**
  - Create, delete, revert snapshots
  - List all snapshots
  - Get current snapshot
  - Delete all snapshots

- **Batch Operations**
  - Parallel VM cloning with goroutines
  - Bulk power operations

- **Disk Management**
  - Add new disks
  - Extend existing disks
  - Remove disks

- **Network Management**
  - Add network adapters
  - Change network on existing adapters

- **CD/DVD Operations**
  - Mount/unmount ISO files
  - Connect/disconnect CD/DVD drives

- **Guest Operations** (requires VMware Tools)
  - File upload/download
  - Script execution
  - Directory operations

## Installation

```bash
go get github.com/skabbio1976/vcenter
```

## Quick Start

### Connect with SSPI (Windows)

```go
package main

import (
    "context"
    "log"

    "github.com/skabbio1976/vcenter"
)

func main() {
    ctx := context.Background()

    // Connect with Windows integrated auth (SSPI)
    client, err := vcenter.ConnectWithSSPI(
        ctx,
        "vcenter.example.com",
        true,          // insecure (skip TLS verification)
        "Datacenter1", // datacenter name
    )
    if err != nil {
        log.Fatal(err)
    }
    defer client.Logout(ctx)

    log.Println("Connected to vCenter!")
}
```

### Connect with username/password

```go
config := vcenter.ConnectConfig{
    Host:       "vcenter.example.com",
    Username:   "administrator@vsphere.local",
    Password:   "password",
    Insecure:   true,
    Datacenter: "Datacenter1",
}

client, err := vcenter.ConnectWithPassword(ctx, config)
if err != nil {
    log.Fatal(err)
}
defer client.Logout(ctx)
```

## Exported Types and Functions

### Types

| Type | Description |
|------|-------------|
| `NetworkAdapter` | Network adapter configuration for multi-NIC customization |
| `WindowsCustomizationConfig` | All settings for Windows customization (domain, IP, autologon, etc.) |
| `LinuxCustomizationConfig` | All settings for Linux customization |
| `ServerRequest` | Structured VM request with full configuration |
| `SnapshotInfo` | Snapshot metadata |
| `VMInfo` | Detailed VM information |
| `NetworkInfo` | Network adapter information |

### Customization Functions

| Function | Description |
|----------|-------------|
| `NewWindowsCustomization(cfg)` | Create Windows customization spec (supports all scenarios) |
| `NewLinuxCustomization(cfg)` | Create Linux customization spec (supports all scenarios) |

### Clone Functions

| Function | Description |
|----------|-------------|
| `CloneVM()` | Clone VM without customization (powered off) |
| `CloneVMWithCustomization()` | Clone VM with customization (powers on) |
| `CloneFromRequest()` | Clone from ServerRequest with correct operation order |
| `CloneMultiple()` | Clone multiple VMs in parallel |

### Wait Functions

| Function | Description |
|----------|-------------|
| `WaitForCustomization()` | Wait for guest customization to complete (hostname-based detection) |
| `WaitForTools()` | Wait for VMware Tools to be ready |
| `WaitForIP()` | Wait for VM to get a routable IP address |

### Power Functions

| Function | Description |
|----------|-------------|
| `PowerOnVM()` | Power on a VM |
| `PowerOffVM()` | Power off a VM |
| `RestartVM()` | Restart a VM (graceful or hard) |
| `BulkPowerOperation()` | Power operations on multiple VMs |

### Disk Functions

| Function | Description |
|----------|-------------|
| `AddDisk()` | Add a new disk to VM |
| `ExtendDisk()` | Extend existing disk |
| `RemoveDisk()` | Remove disk from VM |

### Network Functions

| Function | Description |
|----------|-------------|
| `AddNetworkAdapter()` | Add network adapter |
| `ChangeNetwork()` | Change network on adapter |

### Snapshot Functions

| Function | Description |
|----------|-------------|
| `CreateSnapshot()` | Create snapshot |
| `DeleteSnapshot()` | Delete snapshot |
| `ListSnapshots()` | List all snapshots |
| `RevertToSnapshot()` | Revert to snapshot |
| `DeleteAllSnapshots()` | Delete all snapshots |
| `GetCurrentSnapshot()` | Get current snapshot |

### Other Functions

| Function | Description |
|----------|-------------|
| `GetVM()` | Find VM by name |
| `GetVMInfo()` | Get detailed VM info |
| `SetVMResources()` | Change CPU/memory |
| `DeleteVM()` | Delete VM |
| `UnregisterVM()` | Unregister VM |
| `MountISO()` / `UnmountISO()` | ISO operations |
| `ConnectCDROM()` / `DisconnectCDROM()` | CD/DVD operations |

## Examples

### Clone a VM

```go
vm, err := vcenter.CloneVM(
    ctx,
    client.Client,
    "Windows-2022-Template", // template name
    "WebServer01",           // new VM name
    "Datacenter1",           // datacenter
    "datastore1",            // datastore
    "Resources",             // resource pool
    "WebServers",            // folder (or "" for default)
)
if err != nil {
    log.Fatal(err)
}

log.Printf("VM cloned: %s\n", vm.Name())
```

### Clone with Windows customization (domain join, DHCP)

```go
// Create customization spec for domain join with DHCP
customization := vcenter.NewWindowsCustomization(vcenter.WindowsCustomizationConfig{
    ComputerName:   "WebServer01",
    AdminPassword:  "localAdminPassword",
    Timezone:       85, // W. Europe Standard Time
    Domain:         "example.com",
    DomainUser:     "administrator@example.com",
    DomainPassword: "domainPassword",
    GlobalDNS:      []string{"192.168.1.1"},
    DNSSuffixes:    []string{"example.com"},
})

// Clone VM with customization
vm, err := vcenter.CloneVMWithCustomization(
    ctx,
    client.Client,
    "Windows-2022-Template",
    "WebServer01",
    "Datacenter1",
    "datastore1",
    "Resources",
    "WebServers",
    customization,
)
if err != nil {
    log.Fatal(err)
}

// Wait for customization to complete (recommended!)
err = vcenter.WaitForCustomization(ctx, vm, 15*time.Minute)
if err != nil {
    log.Printf("Warning: Customization timeout: %v\n", err)
}

log.Printf("VM ready! IP: check with GetVMInfo()")
```

### Clone with static IP

```go
customization := vcenter.NewWindowsCustomization(vcenter.WindowsCustomizationConfig{
    ComputerName:   "DBServer01",
    AdminPassword:  "localAdminPassword",
    Timezone:       85,
    Domain:         "example.com",
    DomainUser:     "administrator@example.com",
    DomainPassword: "domainPassword",
    // Static IP via Adapters
    Adapters: []vcenter.NetworkAdapter{{
        IPAddress:  "192.168.1.100",
        SubnetMask: "255.255.255.0",
        Gateway:    "192.168.1.1",
        DNSServers: []string{"192.168.1.1"},
    }},
    GlobalDNS:   []string{"192.168.1.1"},
    DNSSuffixes: []string{"example.com"},
})

vm, err := vcenter.CloneVMWithCustomization(ctx, client.Client,
    "Windows-2022-Template", "DBServer01", "Datacenter1",
    "datastore1", "Resources", "", customization)
```

### Clone with multi-NIC configuration

```go
customization := vcenter.NewWindowsCustomization(vcenter.WindowsCustomizationConfig{
    ComputerName:   "AppServer01",
    AdminPassword:  "localAdminPassword",
    Timezone:       85,
    Domain:         "example.com",
    DomainUser:     "administrator@example.com",
    DomainPassword: "domainPassword",
    MachineObjectOU: "OU=Servers,DC=example,DC=com", // Place in specific OU
    // Multiple network adapters
    Adapters: []vcenter.NetworkAdapter{
        {
            Network:    "Production-VLAN100",
            IPAddress:  "10.1.1.50",
            SubnetMask: "255.255.255.0",
            Gateway:    "10.1.1.1",
            DNSServers: []string{"10.1.1.10"},
        },
        {
            Network:    "Management-VLAN200",
            IPAddress:  "10.2.1.50",
            SubnetMask: "255.255.255.0",
            Gateway:    "10.2.1.1",
        },
    },
    GlobalDNS:      []string{"10.1.1.10"},
    DNSSuffixes:    []string{"example.com"},
    AutologonCount: 1, // Auto-login once for post-install scripts
})
```

### Clone standalone Windows (workgroup, no domain)

```go
customization := vcenter.NewWindowsCustomization(vcenter.WindowsCustomizationConfig{
    ComputerName:  "TestServer01",
    AdminPassword: "localAdminPassword",
    Timezone:      85,
    // No Domain = joins WORKGROUP
    Adapters: []vcenter.NetworkAdapter{{
        IPAddress:  "192.168.1.200",
        SubnetMask: "255.255.255.0",
        Gateway:    "192.168.1.1",
    }},
})
```

### Use ServerRequest for structured configuration (recommended for complex deployments)

```go
req := vcenter.ServerRequest{
    Name:     "AppServer01",
    Template: "Windows-2022-Template",
    CPUs:     4,
    MemoryGB: 16,
    // Multiple disks (D:, E:, F:, ...)
    DisksGB: []int{100, 200}, // D: 100GB, E: 200GB
    // Multi-NIC
    Adapters: []vcenter.NetworkAdapter{
        {IPAddress: "192.168.1.101", SubnetMask: "255.255.255.0", Gateway: "192.168.1.1"},
        {IPAddress: "10.0.0.101", SubnetMask: "255.255.255.0", Gateway: "10.0.0.1"},
    },
    Domain:          "example.com",
    MachineObjectOU: "OU=AppServers,DC=example,DC=com",
    DNSServers:      []string{"192.168.1.1", "192.168.1.2"},
    DNSSuffixes:     []string{"example.com"},
    AutologonCount:  1,
}

// CloneFromRequest implements the CORRECT operation order:
// 1. Clone with powerOn=false
// 2. Add disks (VM off)
// 3. Set CPU/memory (VM off)
// 4. Power on (triggers customization)
// 5. WaitForCustomization
vm, err := vcenter.CloneFromRequest(
    ctx,
    client.Client,
    req,
    "Datacenter1",
    "datastore1",
    "Resources",
    "AppServers",
    "administrator@example.com",
    "domainPassword",
    "localAdminPassword",
    85, // timezone
)
```

### Linux customization

```go
// Linux with DHCP
customization := vcenter.NewLinuxCustomization(vcenter.LinuxCustomizationConfig{
    Hostname:    "webserver01",
    Domain:      "example.com",
    GlobalDNS:   []string{"192.168.1.1"},
    DNSSuffixes: []string{"example.com"},
})

// Linux with static IP
customization := vcenter.NewLinuxCustomization(vcenter.LinuxCustomizationConfig{
    Hostname: "webserver02",
    Domain:   "example.com",
    Adapters: []vcenter.NetworkAdapter{{
        IPAddress:  "192.168.1.150",
        SubnetMask: "255.255.255.0",
        Gateway:    "192.168.1.1",
    }},
    GlobalDNS:   []string{"192.168.1.1"},
    DNSSuffixes: []string{"example.com"},
})

// Linux multi-NIC
customization := vcenter.NewLinuxCustomization(vcenter.LinuxCustomizationConfig{
    Hostname: "appserver01",
    Domain:   "example.com",
    Adapters: []vcenter.NetworkAdapter{
        {IPAddress: "10.1.1.20", SubnetMask: "255.255.255.0", Gateway: "10.1.1.1"},
        {IPAddress: "10.2.1.20", SubnetMask: "255.255.255.0", Gateway: "10.2.1.1"},
    },
    GlobalDNS: []string{"10.1.1.1"},
})

vm, err := vcenter.CloneVMWithCustomization(
    ctx, client.Client,
    "Ubuntu-22.04-Template", "webserver01",
    "Datacenter1", "datastore1", "Resources", "",
    customization,
)
```

### Clone multiple VMs in parallel

```go
requests := []vcenter.ServerRequest{
    {Name: "Web01", Template: "Windows-2022-Template", CPUs: 2, MemoryGB: 4, Domain: "example.com"},
    {Name: "Web02", Template: "Windows-2022-Template", CPUs: 2, MemoryGB: 4, Domain: "example.com"},
    {Name: "Web03", Template: "Windows-2022-Template", CPUs: 2, MemoryGB: 4, Domain: "example.com"},
}

vms, errors := vcenter.CloneMultiple(
    ctx,
    client.Client,
    requests,
    "Datacenter1",
    "datastore1",
    "Resources",
    "WebServers",
    "administrator@example.com",
    "domainPassword",
    "localAdminPassword",
    85,
)

for i, err := range errors {
    if err != nil {
        log.Printf("Failed to clone %s: %v\n", requests[i].Name, err)
    } else {
        log.Printf("Successfully cloned %s\n", requests[i].Name)
    }
}
```

### Power operations

```go
vm, err := vcenter.GetVM(ctx, client.Client, "WebServer01", "Datacenter1")
if err != nil {
    log.Fatal(err)
}

err = vcenter.PowerOnVM(ctx, vm)
err = vcenter.PowerOffVM(ctx, vm)
err = vcenter.RestartVM(ctx, vm) // Graceful with Tools, otherwise hard reset
```

### Bulk power operations

```go
vms := []*object.VirtualMachine{vm1, vm2, vm3}
errors := vcenter.BulkPowerOperation(ctx, vms, "on") // "on", "off", "restart"
```

### Disk management

```go
err = vcenter.AddDisk(ctx, vm, 100, "datastore1")    // Add 100GB disk
err = vcenter.ExtendDisk(ctx, vm, "Hard disk 2", 200) // Extend to 200GB
err = vcenter.RemoveDisk(ctx, vm, "Hard disk 3")      // Remove disk
```

### Network management

```go
err = vcenter.AddNetworkAdapter(ctx, vm, "Production-VLAN100")
err = vcenter.ChangeNetwork(ctx, vm, "Network adapter 1", "DMZ-VLAN200")
```

### Change CPU and memory

```go
err = vcenter.SetVMResources(ctx, vm, 4, 8192) // 4 CPUs, 8GB RAM
```

### Snapshot operations

```go
err = vcenter.CreateSnapshot(ctx, vm, "Before Update", "Snapshot before updates", false, true)
snapshots, err := vcenter.ListSnapshots(ctx, vm)
err = vcenter.RevertToSnapshot(ctx, vm, "Before Update", false)
err = vcenter.DeleteSnapshot(ctx, vm, "Before Update", false, true)
err = vcenter.DeleteAllSnapshots(ctx, vm, true)
current, err := vcenter.GetCurrentSnapshot(ctx, vm)
```

### Get detailed VM information

```go
info, err := vcenter.GetVMInfo(ctx, vm)
fmt.Printf("VM: %s\n", info.Name)
fmt.Printf("Power State: %s\n", info.PowerState)
fmt.Printf("CPUs: %d\n", info.CPUCount)
fmt.Printf("Memory: %.2f GB\n", info.MemoryGB)
fmt.Printf("IP Address: %s\n", info.GuestIPAddress)
fmt.Printf("Hostname: %s\n", info.GuestHostname)
```

### Delete or unregister VM

```go
err = vcenter.DeleteVM(ctx, vm, true, false)  // Delete VM and files
err = vcenter.UnregisterVM(ctx, vm)           // Unregister only (keep files)
err = vcenter.DeleteVM(ctx, vm, true, true)   // Force delete powered-on VM
```

### CD/DVD operations

```go
err = vcenter.MountISO(ctx, vm, "ISOs/windows.iso", "datastore1", true)
err = vcenter.UnmountISO(ctx, vm, true)
err = vcenter.ConnectCDROM(ctx, vm)
err = vcenter.DisconnectCDROM(ctx, vm)
```

### Using Datastore Clusters (Storage DRS)

All clone functions automatically detect datastore clusters and use Storage DRS:

```go
// Works with both regular datastores and datastore clusters
vm, err := vcenter.CloneVM(ctx, client.Client,
    "Windows-2022-Template", "WebServer01",
    "Datacenter1", "Production-DatastoreCluster", // <- datastore cluster
    "Resources", "WebServers")
```

## WaitForCustomization - The Important Function

`WaitForCustomization` detects when Windows Sysprep or Linux customization is complete by checking:

1. **Hostname matches VM name** (before sysprep: `WIN-XXXXXXX`, after: configured name)
2. **Valid IP address** (not link-local 169.254.x.x)
3. **VMware Tools running**

This is more reliable than event-based monitoring and works for both domain-joined and standalone VMs.

```go
// Always use after CloneVMWithCustomization
err = vcenter.WaitForCustomization(ctx, vm, 15*time.Minute)
if err != nil {
    log.Printf("Customization may still be running: %v", err)
}
```

## Error Handling

```go
var notFoundErr *vcenter.NotFoundError
if errors.As(err, &notFoundErr) {
    log.Printf("Resource not found: %s\n", notFoundErr)
}

var validationErr *vcenter.ValidationError
if errors.As(err, &validationErr) {
    log.Printf("Validation error on field %s: %s\n", validationErr.Field, validationErr.Message)
}
```

## Windows Timezone IDs

Common timezone IDs for Windows customization:

- `4` - Eastern Standard Time (EST)
- `20` - Central Standard Time
- `35` - Mountain Standard Time
- `85` - W. Europe Standard Time (Stockholm, Berlin, Paris)
- `105` - Pacific Standard Time (PST)
- `220` - UTC

Full list: https://docs.microsoft.com/en-us/previous-versions/windows/embedded/ms912391(v=winembedded.11)

## License

MIT License - See LICENSE file for details.

## Contributing

Pull requests are welcome! For major changes, please open an issue first.

## Credits

- Built on [govmomi](https://github.com/vmware/govmomi)
- Uses [go-vcenter-auth](https://github.com/skabbio1976/go-vcenter-auth) for authentication
- Inspired by VMware PowerCLI
