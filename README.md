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

- **Excel-based VM Provisioning**
  - Create Excel templates with dropdowns and validation
  - Validate Excel files before deployment
  - Convert Excel to JSON for automation pipelines
  - Multi-NIC, multi-disk support in Excel format

- **Configuration & Secrets Management**
  - JSON configuration files with validation
  - Encrypted credential storage (AES-256-GCM + Argon2)
  - Multiple key sources (environment variable, file, direct)
  - Password-based encryption for simpler workflows

- **Inventory Scanning**
  - Scan vCenter for all assets (templates, clusters, networks, etc.)
  - Per-datacenter or flattened global lists
  - Perfect for populating Excel dropdowns dynamically

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

> **Full examples:** See the [examples/](examples/) directory for complete, runnable code.

### Connect with SSPI (Windows)

> [Full example: examples/01-connect-sspi/](examples/01-connect-sspi/main.go)

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

> [Full example: examples/02-connect-password/](examples/02-connect-password/main.go)

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
| `ExcelValidValues` | Dropdown values for Excel template |
| `ExcelVMConfig` | VM configuration parsed from Excel |
| `VCenterConfig` | JSON configuration for vCenter connection and defaults |
| `Credential` | Single credential entry (server, username, password) |
| `CredentialStore` | Encrypted credential storage |
| `KeySource` | Key source for encryption (env, file, or direct) |
| `VCenterInventory` | Scanned vCenter assets (templates, clusters, networks, etc.) |
| `InventoryOptions` | Options for ScanVCenter (include/exclude sections) |
| `CustomizationExpected` | Expected hostname/domain/IP for WaitForCustomization |

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

### Wait Functions

| Function | Description |
|----------|-------------|
| `WaitForCustomization()` | Wait for guest customization to complete (verifies hostname, domain, and IP) |
| `WaitForTools()` | Wait for VMware Tools to be ready |
| `WaitForIP()` | Wait for VM to get a routable IP address |

### Power Functions

| Function | Description |
|----------|-------------|
| `PowerOnVM()` | Power on a VM |
| `PowerOffVM()` | Power off a VM |
| `RestartVM()` | Restart a VM (graceful or hard) |

### Disk Functions

| Function | Description |
|----------|-------------|
| `AddDisk()` | Add a new disk to VM (thin/thick/eagerzeroed provisioning) |
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

### Excel Functions

| Function | Description |
|----------|-------------|
| `CreateExcelTemplate()` | Create Excel template with dropdowns and validation |
| `ValidateExcel()` | Validate Excel file before deployment |
| `ExcelToJSON()` | Convert Excel rows to JSON (individual files + combined) |
| `DefaultExcelValidValues()` | Get default dropdown values |

### Configuration Functions

| Function | Description |
|----------|-------------|
| `LoadVCenterConfig()` | Load configuration from JSON file |
| `CreateVCenterConfigTemplate()` | Create config template with comments |
| `EnsureConfigFile()` | Create config if it doesn't exist |
| `DefaultVCenterConfig()` | Get default configuration values |

### Secrets/Credential Functions

| Function | Description |
|----------|-------------|
| `NewCredentialStore()` | Create empty credential store |
| `LoadEncryptedCredentialStore()` | Load encrypted credentials with KeySource |
| `LoadEncryptedCredentialStoreWithPassword()` | Load credentials with password |
| `GenerateEncryptionKey()` | Generate random 32-byte hex key |
| `KeySourceEnv()` | Use environment variable as key |
| `KeySourceFile()` | Use file contents as key |
| `KeySourceDirect()` | Use string directly as key |
| `EncryptString()` / `DecryptString()` | Encrypt/decrypt strings |

### Inventory Functions

| Function | Description |
|----------|-------------|
| `ScanVCenter()` | Scan vCenter for all assets |
| `VCenterInventory.ToFlat()` | Flatten per-datacenter maps to global lists |

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

### Guest Operations Functions (requires VMware Tools)

| Function | Description |
|----------|-------------|
| `UploadFileToVM()` | Upload file to guest OS |
| `DownloadFileFromVM()` | Download file from guest OS |
| `UploadDirectoryToVM()` | Upload entire directory (zip → upload → extract) |
| `DownloadDirectoryFromVM()` | Download entire directory (zip → download → extract) |
| `RunScriptOnVM()` | Execute script/program on guest OS |
| `UploadAndRunScript()` | Upload and execute script (convenience) |

## Examples

### Clone a VM

> [Full example: examples/03-simple-clone/](examples/03-simple-clone/main.go)

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

> [Full example: examples/04-clone-with-customization/](examples/04-clone-with-customization/main.go)

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
err = vcenter.WaitForCustomization(ctx, vm, 15*time.Minute, vcenter.CustomizationExpected{
    Hostname: "WebServer01",
    Domain:   "example.com",
    IP:       "dhcp", // or specific IP like "192.168.1.100"
})
if err != nil {
    log.Printf("Warning: Customization timeout: %v\n", err)
}

log.Printf("VM ready! IP: check with GetVMInfo()")
```

### Clone with static IP

> [Full example: examples/05-clone-static-ip/](examples/05-clone-static-ip/main.go)

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

> [Full example: examples/10-server-request/](examples/10-server-request/main.go)

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

### Clone multiple VMs in parallel (using goroutines)

> [Full example: examples/06-batch-clone/](examples/06-batch-clone/main.go)

The API does not provide batch functions by design - you control your own concurrency:

```go
requests := []vcenter.ServerRequest{
    {Name: "Web01", Template: "Windows-2022-Template", CPUs: 2, MemoryGB: 4, Domain: "example.com"},
    {Name: "Web02", Template: "Windows-2022-Template", CPUs: 2, MemoryGB: 4, Domain: "example.com"},
    {Name: "Web03", Template: "Windows-2022-Template", CPUs: 2, MemoryGB: 4, Domain: "example.com"},
}

var wg sync.WaitGroup
results := make([]error, len(requests))

for i, req := range requests {
    wg.Add(1)
    go func(idx int, r vcenter.ServerRequest) {
        defer wg.Done()
        _, err := vcenter.CloneFromRequest(ctx, client.Client, r,
            "Datacenter1", "datastore1", "Resources", "WebServers",
            "administrator@example.com", "domainPassword", "localAdminPassword", 85)
        results[idx] = err
    }(i, req)
}

wg.Wait()

for i, err := range results {
    if err != nil {
        log.Printf("Failed: %s: %v\n", requests[i].Name, err)
    } else {
        log.Printf("Success: %s\n", requests[i].Name)
    }
}
```

### Power operations

> [Full example: examples/07-bulk-power/](examples/07-bulk-power/main.go)

```go
vm, err := vcenter.GetVM(ctx, client.Client, "WebServer01", "Datacenter1")
if err != nil {
    log.Fatal(err)
}

err = vcenter.PowerOnVM(ctx, vm)
err = vcenter.PowerOffVM(ctx, vm)
err = vcenter.RestartVM(ctx, vm) // Graceful with Tools, otherwise hard reset
```

### Disk management

> [Full example: examples/08-disk-operations/](examples/08-disk-operations/main.go)

```go
err = vcenter.AddDisk(ctx, vm, 100, "thin")            // Add 100GB thin disk
err = vcenter.AddDisk(ctx, vm, 100, "thick")           // Add 100GB thick disk
err = vcenter.AddDisk(ctx, vm, 100, "eagerzeroed")     // Add 100GB eager zeroed thick disk
err = vcenter.ExtendDisk(ctx, vm, "Hard disk 2", 200) // Extend to 200GB
err = vcenter.RemoveDisk(ctx, vm, "Hard disk 3")      // Remove disk
```

### Network management

> [Full example: examples/09-network-operations/](examples/09-network-operations/main.go)

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

## Excel-based VM Provisioning

> [Full example: examples/11-config-and-excel/](examples/11-config-and-excel/main.go)

Create professional Excel templates for VM requests, validate them, and convert to JSON for automation:

### Create Excel template

```go
// Create template with default dropdown values
err := vcenter.CreateExcelTemplate("vm_requests.xlsx", nil)

// Or customize the dropdown values
values := vcenter.DefaultExcelValidValues()
values.Templates = []string{"Win2022-Prod", "Win2022-Dev", "Ubuntu2204"}
values.PortGroups = []string{"VLAN-100-Prod", "VLAN-200-Dev", "VLAN-300-Mgmt"}
values.Domains = []string{"prod.example.com", "dev.example.com"}

err := vcenter.CreateExcelTemplate("vm_requests.xlsx", &values)
```

The template includes:
- **VM Requests** sheet with all columns (VM name, template, network, CPU, memory, disks, domain, etc.)
- **Valid Values** sheet with dropdown source data
- **Instruktioner** sheet with usage instructions
- Data validation dropdowns in key columns
- Example rows to guide users

### Validate Excel before deployment

```go
// Basic validation
valid, messages, err := vcenter.ValidateExcel("vm_requests.xlsx", nil, false)
if !valid {
    for _, msg := range messages {
        log.Println(msg)
    }
}

// Strict validation (warnings become errors)
valid, messages, err := vcenter.ValidateExcel("vm_requests.xlsx", nil, true)
```

Validates:
- Required columns exist
- No duplicate VM names
- Valid CPU/memory values
- Network configuration (IP requires subnet + gateway)
- Disk provisioning values (thin/thick)

### Convert Excel to JSON

```go
// Parse Excel and write JSON files
configs, err := vcenter.ExcelToJSON("vm_requests.xlsx", "output/")
// Creates:
//   output/DC-PROD-01.json
//   output/SQL-PROD-01.json
//   output/_all_vms.json (combined)

// Or just parse without writing files
configs, err := vcenter.ExcelToJSON("vm_requests.xlsx", "")
for _, cfg := range configs {
    log.Printf("VM: %s, Template: %s, CPUs: %d\n",
        cfg.VMName, cfg.Template, cfg.Hardware.NumCPUs)
}
```

### Excel columns

| Column | Description | Example |
|--------|-------------|---------|
| `vm_name` | VM name | `DC-PROD-01` |
| `template` | Template name | `Windows2022-Template` |
| `datacenter` | Datacenter | `DC-Stockholm` |
| `compute_cluster` | Compute cluster | `Prod-Cluster` |
| `storage_cluster` | Storage cluster (or datastore) | `VSAN-Cluster-01` |
| `folder` | VM folder path | `Production/Infrastructure` |
| `port_group_1` | First NIC network | `VLAN-100-Prod` |
| `ip_1` | IP (or "DHCP") | `10.20.30.10` |
| `subnet_mask_1` | Subnet mask | `255.255.255.0` |
| `gateway_1` | Default gateway | `10.20.30.1` |
| `dns_servers_1` | DNS (semicolon-separated) | `10.20.1.10;10.20.1.11` |
| `port_group_2/3` | Additional NICs (optional) | |
| `num_cpus` | CPU count | `4` |
| `memory_mb` | Memory in MB | `8192` |
| `disk_gb` | Extra disks (semicolon-separated) | `50;100;200` |
| `disk_provisioning` | thin or thick | `thin` |
| `server_role` | Server role | `DC`, `SQL`, `Web` |
| `hostname` | Computer name | `DC-PROD-01` |
| `domain` | Domain to join | `corp.example.com` |
| `domain_join_user` | Domain join account | `svc_domainjoin` |
| `ou_path` | AD OU path | `OU=Servers,DC=corp,DC=com` |
| `autologon_count` | Auto-login count | `3` |
| `timezone` | Windows timezone | `W. Europe Standard Time` |
| `run_once_commands` | Post-install commands | `powershell.exe -File setup.ps1` |

## Configuration Management

### Create and load configuration

```go
// Create a config template (with helpful comments)
err := vcenter.CreateVCenterConfigTemplate("vcenter_config.json")

// Or ensure config exists (creates if missing)
created, err := vcenter.EnsureConfigFile("vcenter_config.json")
if created {
    log.Println("Created new config template - please edit it")
}

// Load configuration
cfg, err := vcenter.LoadVCenterConfig("vcenter_config.json")
if err != nil {
    log.Fatal(err)
}

// Use with ConnectWithPassword
client, err := vcenter.ConnectWithPassword(ctx, vcenter.ConnectConfig{
    Host:       cfg.VCenter,
    Username:   cfg.Username,
    Password:   cfg.Password,
    Insecure:   !cfg.VerifySSL,
    Datacenter: cfg.Datacenter,
})
```

### Configuration file format

```json
{
  "vcenter": "vcenter.example.com",
  "username": "administrator@vsphere.local",
  "password": "your-password",
  "datacenter": "Datacenter",
  "cluster": "Compute-Cluster",
  "resource_pool": "Resources",
  "storage_cluster": "VSAN-Cluster-01",
  "folder": "Production/Servers",
  "domain_user": "svc_domainjoin@corp.example.com",
  "domain_password": "DomainPassword",
  "admin_password": "LocalAdminPassword",
  "timezone": 85,
  "verify_ssl": false
}
```

## Secrets Management

Secure credential storage with AES-256-GCM encryption and Argon2 key derivation.

### Store credentials encrypted

```go
// Create credential store
store := vcenter.NewCredentialStore()
store.AddCredential("production", vcenter.Credential{
    Server:   "vcenter-prod.example.com",
    Username: "administrator@vsphere.local",
    Password: "SuperSecretPassword",
    Insecure: false,
})
store.AddCredential("development", vcenter.Credential{
    Server:   "vcenter-dev.example.com",
    Username: "admin@vsphere.local",
    Password: "DevPassword",
    Insecure: true,
})

// Option 1: Encrypt with generated key (store key securely!)
key, _ := vcenter.GenerateEncryptionKey()
err := store.SaveEncrypted("credentials.enc", vcenter.KeySourceDirect(key))
fmt.Printf("Encryption key (save this securely): %s\n", key)

// Option 2: Encrypt with environment variable
os.Setenv("VCENTER_KEY", "your-secret-key")
err := store.SaveEncrypted("credentials.enc", vcenter.KeySourceEnv("VCENTER_KEY"))

// Option 3: Encrypt with key from file
err := store.SaveEncrypted("credentials.enc", vcenter.KeySourceFile("/path/to/keyfile"))

// Option 4: Simple password-based encryption
err := store.SaveEncryptedWithPassword("credentials.enc", "master-password")
```

### Load encrypted credentials

```go
// Load with KeySource
store, err := vcenter.LoadEncryptedCredentialStore("credentials.enc",
    vcenter.KeySourceEnv("VCENTER_KEY"))

// Or with password
store, err := vcenter.LoadEncryptedCredentialStoreWithPassword("credentials.enc",
    "master-password")

// Get specific credential
cred, ok := store.GetCredential("production")
if ok {
    client, err := vcenter.ConnectWithPassword(ctx, vcenter.ConnectConfig{
        Host:       cred.Server,
        Username:   cred.Username,
        Password:   cred.Password,
        Insecure:   cred.Insecure,
        Datacenter: "Datacenter1",
    })
}

// List all credential names
names := store.ListNames() // ["development", "production"]
```

### Encrypt individual strings

```go
// Encrypt sensitive data
encrypted, err := vcenter.EncryptString("secret-data", vcenter.KeySourceEnv("MY_KEY"))

// Decrypt
decrypted, err := vcenter.DecryptString(encrypted, vcenter.KeySourceEnv("MY_KEY"))

// Or with password
encrypted, err := vcenter.EncryptStringWithPassword("secret-data", "password")
decrypted, err := vcenter.DecryptStringWithPassword(encrypted, "password")
```

## Inventory Scanning

Scan vCenter for all available assets - perfect for populating Excel dropdowns dynamically.

### Scan all assets

```go
// Scan everything
inventory, err := vcenter.ScanVCenter(ctx, client.Client, vcenter.InventoryOptions{})
if err != nil {
    log.Fatal(err)
}

// Access per-datacenter
for _, dc := range inventory.Datacenters {
    fmt.Printf("Datacenter: %s\n", dc)
    fmt.Printf("  Templates: %v\n", inventory.Templates[dc])
    fmt.Printf("  Clusters: %v\n", inventory.ComputeClusters[dc])
    fmt.Printf("  Storage Clusters: %v\n", inventory.StorageClusters[dc])
    fmt.Printf("  Datastores: %v\n", inventory.Datastores[dc])
    fmt.Printf("  Port Groups: %v\n", inventory.PortGroups[dc])
    fmt.Printf("  Folders: %v\n", inventory.Folders[dc])
}
```

### Flatten to global lists

```go
// Get unique values across all datacenters
flat := inventory.ToFlat()

fmt.Println("All templates:", flat["templates"])
fmt.Println("All port groups:", flat["port_groups"])
fmt.Println("All clusters:", flat["compute_clusters"])
```

### Selective scanning

```go
// Skip expensive operations
falseVal := false
inventory, err := vcenter.ScanVCenter(ctx, client.Client, vcenter.InventoryOptions{
    IncludeTemplates:  &falseVal, // Skip template scan
    IncludeFolders:    &falseVal, // Skip folder scan
    IncludeDatastores: nil,       // nil = include (default)
})
```

### Generate Excel template from live vCenter

```go
// Scan vCenter
inventory, _ := vcenter.ScanVCenter(ctx, client.Client, vcenter.InventoryOptions{})
flat := inventory.ToFlat()

// Create Excel with real values from vCenter
values := vcenter.ExcelValidValues{
    Templates:       flat["templates"],
    Datacenters:     flat["datacenters"],
    ComputeClusters: flat["compute_clusters"],
    StorageClusters: flat["storage_clusters"],
    Datastores:      flat["datastores"],
    PortGroups:      flat["port_groups"],
    // Keep defaults for other fields
    Domains:         []string{"corp.example.com", "dev.example.com"},
    Timezones:       []string{"W. Europe Standard Time", "UTC"},
    CPUOptions:      []int{2, 4, 8, 16},
    MemoryOptions:   []int{4096, 8192, 16384, 32768},
    DiskOptions:     []int{50, 100, 200, 500},
}

err := vcenter.CreateExcelTemplate("vm_requests.xlsx", &values)
```

### Inventory structure

```go
type VCenterInventory struct {
    Datacenters     []string            // ["DC-Stockholm", "DC-Göteborg"]
    ComputeClusters map[string][]string // {"DC-Stockholm": ["Prod-Cluster", "Dev-Cluster"]}
    StorageClusters map[string][]string // {"DC-Stockholm": ["VSAN-Cluster-01"]}
    Datastores      map[string][]string // {"DC-Stockholm": ["datastore1", "datastore2"]}
    Templates       map[string][]string // {"DC-Stockholm": ["Win2022-Template", "Ubuntu-Template"]}
    Folders         map[string][]string // {"DC-Stockholm": ["Production/Web", "Production/DB"]}
    PortGroups      map[string][]string // {"DC-Stockholm": ["VLAN-100", "VLAN-200"]}
}
```

## Guest Operations (Post-Deployment Configuration)

> [Full example: examples/12-guest-operations/](examples/12-guest-operations/main.go)

Execute scripts and transfer files on VMs via VMware Tools - perfect for post-deployment configuration.

### Upload file to VM

```go
err := vcenter.UploadFileToVM(ctx, vm,
    "Administrator", "password",
    "/local/config.xml",           // local path
    "C:\\temp\\config.xml",        // remote path on VM
    true,                          // overwrite if exists
)
```

### Download file from VM

```go
err := vcenter.DownloadFileFromVM(ctx, vm,
    "Administrator", "password",
    "C:\\logs\\install.log",       // remote path on VM
    "/local/install.log",          // local path
)
```

### Run script on VM

```go
// Run PowerShell script (don't wait)
pid, err := vcenter.RunScriptOnVM(ctx, vm,
    "Administrator", "password",
    "C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe",
    []string{"-File", "C:\\temp\\setup.ps1"},
    "",     // working directory (optional)
    false,  // don't wait for completion
)

// Run and wait for completion
pid, err := vcenter.RunScriptOnVM(ctx, vm,
    "Administrator", "password",
    "C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe",
    []string{"-ExecutionPolicy", "Bypass", "-File", "C:\\temp\\setup.ps1"},
    "C:\\temp",  // working directory
    true,        // wait for completion (5 min timeout)
)

// Linux example
pid, err := vcenter.RunScriptOnVM(ctx, vm,
    "root", "password",
    "/bin/bash",
    []string{"/tmp/setup.sh"},
    "/tmp",
    true,
)
```

### Upload and run script (convenience function)

```go
// Upload local script and execute it
pid, err := vcenter.UploadAndRunScript(ctx, vm,
    "Administrator", "password",
    "/local/scripts/install-app.ps1",   // local script
    "C:\\temp\\install-app.ps1",        // remote destination
    "C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe",
    []string{"-ExecutionPolicy", "Bypass", "-File", "C:\\temp\\install-app.ps1"},
    true,  // wait for completion
)
```

### Upload/download entire directories

Much faster than file-by-file transfer - uses zip compression (single HTTP request).

```go
// Upload directory to Windows VM
err := vcenter.UploadDirectoryToVM(ctx, vm,
    "Administrator", "password",
    "/local/app-configs",       // local directory
    "C:\\App\\configs",         // remote destination
    true,                       // isWindows
)

// Upload directory to Linux VM
err := vcenter.UploadDirectoryToVM(ctx, vm,
    "root", "password",
    "/local/app-configs",
    "/opt/app/configs",
    false,  // isWindows = false for Linux
)

// Download directory from Windows VM
err := vcenter.DownloadDirectoryFromVM(ctx, vm,
    "Administrator", "password",
    "C:\\App\\logs",            // remote directory
    "/local/collected-logs",    // local destination
    true,
)

// Download directory from Linux VM
err := vcenter.DownloadDirectoryFromVM(ctx, vm,
    "root", "password",
    "/var/log/app",
    "/local/collected-logs",
    false,
)
```

**How it works:**
- **Upload**: zip locally → upload zip → extract with PowerShell/unzip → delete zip
- **Download**: zip on guest → download zip → extract locally → delete remote zip
- **Performance**: 100+ files in one HTTP request vs 100+ separate requests

### Complete post-deployment workflow

```go
// After WaitForCustomization completes...
err := vcenter.WaitForCustomization(ctx, vm, 15*time.Minute)
if err != nil {
    log.Fatal(err)
}

// Upload configuration files
err = vcenter.UploadFileToVM(ctx, vm, "Administrator", adminPass,
    "configs/app.config", "C:\\App\\app.config", true)

// Run installation script
pid, err := vcenter.UploadAndRunScript(ctx, vm,
    "Administrator", adminPass,
    "scripts/install.ps1", "C:\\temp\\install.ps1",
    "powershell.exe",
    []string{"-ExecutionPolicy", "Bypass", "-File", "C:\\temp\\install.ps1"},
    true,
)

// Download logs for verification
err = vcenter.DownloadFileFromVM(ctx, vm, "Administrator", adminPass,
    "C:\\temp\\install.log", "logs/"+vmName+"_install.log")
```

## WaitForCustomization - The Important Function

`WaitForCustomization` waits until the VM has the expected hostname, domain suffix, and IP address. This ensures the customization (Windows Sysprep or Linux cloud-init) has fully completed.

**Parameters:**
- `vm` - The virtual machine to monitor
- `timeout` - Maximum wait time (recommended: 10-15 minutes)
- `expected` - What values to wait for:
  - `Hostname` - Expected computer name (without domain)
  - `Domain` - Expected domain suffix (empty for standalone/workgroup)
  - `IP` - Expected IP address, or "dhcp" for any valid IP

**Behavior by scenario:**

| Scenario | Expected Hostname | Expected IP |
|----------|-------------------|-------------|
| Domain-joined + static IP | `hostname.domain` | exact IP match |
| Domain-joined + DHCP | `hostname.domain` | any valid IP |
| Standalone + static IP | `hostname` (exact) | exact IP match |
| Standalone + DHCP | `hostname` (exact) | any valid IP |

```go
// Domain-joined VM with static IP
err = vcenter.WaitForCustomization(ctx, vm, 15*time.Minute, vcenter.CustomizationExpected{
    Hostname: "srv001",
    Domain:   "corp.example.com",
    IP:       "192.168.1.100",
})

// Domain-joined VM with DHCP
err = vcenter.WaitForCustomization(ctx, vm, 15*time.Minute, vcenter.CustomizationExpected{
    Hostname: "srv002",
    Domain:   "corp.example.com",
    IP:       "dhcp",
})

// Standalone VM (workgroup) with static IP
err = vcenter.WaitForCustomization(ctx, vm, 15*time.Minute, vcenter.CustomizationExpected{
    Hostname: "testvm01",
    IP:       "192.168.1.200",
})

// Standalone VM with DHCP
err = vcenter.WaitForCustomization(ctx, vm, 15*time.Minute, vcenter.CustomizationExpected{
    Hostname: "testvm02",
    IP:       "dhcp",
})
```

**Note:** `CloneFromRequest` calls `WaitForCustomization` automatically with the correct expected values based on the `ServerRequest` configuration.

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
