# vcenter - PowerCLI-inspired Go API for VMware vCenter

A user-friendly Go library for VMware vCenter, inspired by PowerCLI. This package wraps [govmomi](https://github.com/vmware/govmomi) and provides a simpler, more intuitive API for common vCenter operations.

## Features

- **Easy Authentication**
  - Username/password with session caching
  - Windows SSPI/Kerberos (single sign-on)
  - Automatic session management via [go-vcenter-auth](https://github.com/skabbio1976/go-vcenter-auth)

- **VM Management**
  - Clone VMs from templates
  - Windows and Linux customization (with domain join for Windows)
  - CPU and memory configuration
  - Power operations (on/off/restart)
  - Delete and unregister VMs
  - Get detailed VM information
  - Support for datastore clusters (Storage DRS)

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

### Clone with Windows customization (domain join)

```go
// Create customization spec for domain join
customization := vcenter.NewWindowsCustomization(
    "WebServer01",                    // computer name
    "example.com",                    // domain
    "administrator@example.com",      // domain admin user
    "domainPassword",                 // domain password
    "localAdminPassword",             // local admin password
    85,                               // timezone (85 = W. Europe Standard Time)
    []string{"192.168.1.1"},          // DNS servers
    []string{"example.com"},          // DNS suffixes
)

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

// Wait for VMware Tools to be ready
err = vcenter.WaitForTools(ctx, vm)
if err != nil {
    log.Printf("Warning: VMware Tools timeout: %v\n", err)
}

// Get IP address
ip, err := vcenter.WaitForIP(ctx, vm, 10*time.Minute)
if err != nil {
    log.Printf("Warning: IP timeout: %v\n", err)
} else {
    log.Printf("VM IP: %s\n", ip)
}
```

### Clone with static IP

```go
customization := vcenter.NewWindowsCustomizationStaticIP(
    "DBServer01",
    "example.com",
    "administrator@example.com",
    "domainPassword",
    "localAdminPassword",
    85,                          // timezone
    "192.168.1.100",            // IP address
    "255.255.255.0",            // subnet mask
    "192.168.1.1",              // gateway
    []string{"192.168.1.1"},    // DNS servers
    []string{"example.com"},    // DNS suffixes
)

vm, err := vcenter.CloneVMWithCustomization(ctx, client.Client,
    "Windows-2022-Template", "DBServer01", "Datacenter1",
    "datastore1", "Resources", "", customization)
```

### Use ServerRequest for structured configuration

```go
req := vcenter.ServerRequest{
    Name:        "AppServer01",
    Template:    "Windows-2022-Template",
    CPUs:        4,
    MemoryGB:    16,
    Domain:      "example.com",
    IPAddress:   "192.168.1.101",
    SubnetMask:  "255.255.255.0",
    Gateway:     "192.168.1.1",
    DNSServers:  []string{"192.168.1.1", "192.168.1.2"},
    DNSSuffixes: []string{"example.com"},
}

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

### Clone multiple VMs in parallel

```go
requests := []vcenter.ServerRequest{
    {
        Name:     "Web01",
        Template: "Windows-2022-Template",
        CPUs:     2,
        MemoryGB: 4,
        Domain:   "example.com",
        DNSServers: []string{"192.168.1.1"},
    },
    {
        Name:     "Web02",
        Template: "Windows-2022-Template",
        CPUs:     2,
        MemoryGB: 4,
        Domain:   "example.com",
        DNSServers: []string{"192.168.1.1"},
    },
    {
        Name:     "Web03",
        Template: "Windows-2022-Template",
        CPUs:     2,
        MemoryGB: 4,
        Domain:   "example.com",
        DNSServers: []string{"192.168.1.1"},
    },
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

// Check results
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
// Find a VM
vm, err := vcenter.GetVM(ctx, client.Client, "WebServer01", "Datacenter1")
if err != nil {
    log.Fatal(err)
}

// Power on VM
err = vcenter.PowerOnVM(ctx, vm)
if err != nil {
    log.Fatal(err)
}

// Power off VM
err = vcenter.PowerOffVM(ctx, vm)
if err != nil {
    log.Fatal(err)
}

// Restart VM (graceful with VMware Tools, otherwise hard reset)
err = vcenter.RestartVM(ctx, vm)
if err != nil {
    log.Fatal(err)
}
```

### Bulk power operations

```go
// Find multiple VMs
vms := []*object.VirtualMachine{vm1, vm2, vm3}

// Power on all VMs in parallel
errors := vcenter.BulkPowerOperation(ctx, vms, "on")
for i, err := range errors {
    if err != nil {
        log.Printf("VM %d: %v\n", i, err)
    }
}

// Other operations: "off", "restart"
```

### Disk management

```go
// Add a 100GB disk
err = vcenter.AddDisk(ctx, vm, 100, "datastore1")
if err != nil {
    log.Fatal(err)
}

// Extend a disk from 100GB to 200GB
err = vcenter.ExtendDisk(ctx, vm, "Hard disk 2", 200)
if err != nil {
    log.Fatal(err)
}

// Remove a disk (WARNING: Permanently deletes data!)
err = vcenter.RemoveDisk(ctx, vm, "Hard disk 3")
if err != nil {
    log.Fatal(err)
}
```

### Network management

```go
// Add a VMXNET3 network adapter
err = vcenter.AddNetworkAdapter(ctx, vm, "Production-VLAN100")
if err != nil {
    log.Fatal(err)
}

// Change network on an existing adapter
err = vcenter.ChangeNetwork(ctx, vm, "Network adapter 1", "DMZ-VLAN200")
if err != nil {
    log.Fatal(err)
}
```

### Change CPU and memory

```go
// Set 4 CPUs and 8GB RAM
err = vcenter.SetVMResources(ctx, vm, 4, 8192)
if err != nil {
    log.Fatal(err)
}
```

### Linux customization

```go
// Clone Linux VM with DHCP
customization := vcenter.NewLinuxCustomization(
    "webserver01",                        // hostname
    "example.com",                        // domain (optional)
    []string{"192.168.1.1"},              // DNS servers (optional)
    []string{"example.com"},              // DNS suffixes (optional)
)

vm, err := vcenter.CloneVMWithCustomization(
    ctx, client.Client,
    "Ubuntu-22.04-Template", "webserver01",
    "Datacenter1", "datastore1", "Resources", "",
    customization,
)

// Clone Linux VM with static IP
customization := vcenter.NewLinuxCustomizationStaticIP(
    "webserver02",                        // hostname
    "192.168.1.150",                      // IP address
    "255.255.255.0",                      // netmask
    "192.168.1.1",                        // gateway
    []string{"192.168.1.1"},              // DNS servers
    "example.com",                        // domain (optional)
    []string{"example.com"},              // DNS suffixes (optional)
)
```

### Snapshot operations

```go
// Create a snapshot
err = vcenter.CreateSnapshot(ctx, vm, "Before Update",
    "Snapshot before applying updates", false, true)
if err != nil {
    log.Fatal(err)
}

// List all snapshots
snapshots, err := vcenter.ListSnapshots(ctx, vm)
if err != nil {
    log.Fatal(err)
}
for _, snap := range snapshots {
    fmt.Printf("Snapshot: %s (created: %s)\n", snap.Name, snap.CreateTime)
}

// Revert to snapshot
err = vcenter.RevertToSnapshot(ctx, vm, "Before Update", false)
if err != nil {
    log.Fatal(err)
}

// Delete a specific snapshot
err = vcenter.DeleteSnapshot(ctx, vm, "Before Update", false, true)
if err != nil {
    log.Fatal(err)
}

// Delete all snapshots
err = vcenter.DeleteAllSnapshots(ctx, vm, true)
if err != nil {
    log.Fatal(err)
}

// Get current snapshot
current, err := vcenter.GetCurrentSnapshot(ctx, vm)
if err != nil {
    log.Fatal(err)
}
if current != nil {
    fmt.Printf("Current snapshot: %s\n", current.Name)
}
```

### Get detailed VM information

```go
info, err := vcenter.GetVMInfo(ctx, vm)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("VM: %s\n", info.Name)
fmt.Printf("Power State: %s\n", info.PowerState)
fmt.Printf("CPUs: %d (%d sockets, %d cores per socket)\n",
    info.CPUCount, info.CPUSockets, info.CPUCoresPerSocket)
fmt.Printf("Memory: %.2f GB\n", info.MemoryGB)
fmt.Printf("Guest OS: %s\n", info.GuestOSFullName)
fmt.Printf("IP Address: %s\n", info.GuestIPAddress)
fmt.Printf("Hostname: %s\n", info.GuestHostname)
fmt.Printf("Tools Status: %s\n", info.ToolsStatus)

// Network information
for _, net := range info.Networks {
    fmt.Printf("Network: %s - MAC: %s - IPs: %v\n",
        net.Network, net.MACAddress, net.IPAddresses)
}
```

### Delete or unregister VM

```go
// Delete VM and remove all files from datastore
err = vcenter.DeleteVM(ctx, vm, true, false)
if err != nil {
    log.Fatal(err)
}

// Unregister VM from inventory (keep files)
err = vcenter.UnregisterVM(ctx, vm)
if err != nil {
    log.Fatal(err)
}

// Force delete a powered-on VM
err = vcenter.DeleteVM(ctx, vm, true, true)
if err != nil {
    log.Fatal(err)
}
```

### CD/DVD operations

```go
// Mount an ISO file
err = vcenter.MountISO(ctx, vm, "ISOs/windows.iso", "datastore1", true)
if err != nil {
    log.Fatal(err)
}

// Unmount ISO
err = vcenter.UnmountISO(ctx, vm, true)
if err != nil {
    log.Fatal(err)
}

// Connect CD/DVD drive
err = vcenter.ConnectCDROM(ctx, vm)
if err != nil {
    log.Fatal(err)
}

// Disconnect CD/DVD drive
err = vcenter.DisconnectCDROM(ctx, vm)
if err != nil {
    log.Fatal(err)
}
```

### Using Datastore Clusters (Storage DRS)

All clone functions support both regular datastores and datastore clusters. When you specify
a datastore cluster, the package automatically uses Storage DRS to select the best datastore
based on space and I/O load.

```go
// Clone to a datastore cluster instead of a specific datastore
vm, err := vcenter.CloneVM(
    ctx,
    client.Client,
    "Windows-2022-Template",
    "WebServer01",
    "Datacenter1",
    "Production-DatastoreCluster",  // Datastore cluster name
    "Resources",
    "WebServers",
)

// Also works with CloneFromRequest and CloneMultiple
req := vcenter.ServerRequest{
    Name:     "AppServer01",
    Template: "Windows-2022-Template",
    // ... other settings
}

vm, err = vcenter.CloneFromRequest(
    ctx,
    client.Client,
    req,
    "Datacenter1",
    "Production-DatastoreCluster",  // Datastore cluster automatically detected
    "Resources",
    "AppServers",
    "administrator@example.com",
    "domainPassword",
    "localAdminPassword",
    85,
)
```

**Note:** The package automatically detects whether the specified name is a datastore cluster
or a regular datastore. If it's a datastore cluster, Storage DRS will select the optimal
datastore based on vCenter's recommendation engine.

## Error Handling

The package uses custom error types for better error handling:

```go
vm, err := vcenter.GetVM(ctx, client.Client, "NonExistent", "DC1")
if err != nil {
    var notFoundErr *vcenter.NotFoundError
    if errors.As(err, &notFoundErr) {
        log.Printf("Resource not found: %s\n", notFoundErr)
    }
}

req := vcenter.ServerRequest{Name: ""}
err = req.Validate()
if err != nil {
    var validationErr *vcenter.ValidationError
    if errors.As(err, &validationErr) {
        log.Printf("Validation error on field %s: %s\n",
            validationErr.Field, validationErr.Message)
    }
}
```

## Windows Timezone IDs

Common timezone IDs for Windows customization:

- `4` - Eastern Standard Time (EST)
- `15` - U.S. Eastern Standard Time
- `20` - Central Standard Time
- `35` - Mountain Standard Time
- `85` - W. Europe Standard Time (Stockholm, Berlin, Paris)
- `105` - Pacific Standard Time (PST)
- `110` - Alaska Standard Time
- `220` - UTC

Full list: https://docs.microsoft.com/en-us/previous-versions/windows/embedded/ms912391(v=winembedded.11)

## License

MIT License - See LICENSE file for details.

## Contributing

Pull requests are welcome! For major changes, please open an issue first to discuss what you would like to change.

## Credits

- Built on [govmomi](https://github.com/vmware/govmomi)
- Uses [go-vcenter-auth](https://github.com/skabbio1976/go-vcenter-auth) for authentication
- Inspired by VMware PowerCLI
