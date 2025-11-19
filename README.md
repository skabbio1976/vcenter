# vcenter - PowerCLI-liknande Go API för VMware vCenter

Ett användarvänligt Go-bibliotek för VMware vCenter, inspirerat av PowerCLI. Paketet wrapprar [govmomi](https://github.com/vmware/govmomi) och erbjuder ett enklare och mer intuitivt API för vanliga vCenter-operationer.

## Funktioner

- **Enkel autentisering**
  - Username/password med session caching
  - Windows SSPI/Kerberos (single sign-on)
  - Automatisk session-hantering via [go-vcenter-auth](https://github.com/skabbio1976/go-vcenter-auth)

- **VM-hantering**
  - Klona VMs från templates
  - Windows customization med domain join
  - CPU och minnes-konfiguration
  - Power-operationer (on/off/restart)

- **Batch-operationer**
  - Parallell VM-kloning med goroutines
  - Bulk power-operationer

- **Disk-hantering**
  - Lägg till nya diskar
  - Utöka befintliga diskar
  - Ta bort diskar

- **Nätverks-hantering**
  - Lägg till nätverkskort
  - Byt nätverk på befintliga kort

## Installation

```bash
go get github.com/skabbio1976/vcenter
```

## Snabbstart

### Anslut med SSPI (Windows)

```go
package main

import (
    "context"
    "log"

    "github.com/skabbio1976/vcenter"
)

func main() {
    ctx := context.Background()

    // Anslut med Windows integrated auth (SSPI)
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

    log.Println("Ansluten till vCenter!")
}
```

### Anslut med username/password

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

## Exempel

### Klona en VM

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

log.Printf("VM klonades: %s\n", vm.Name())
```

### Klona med Windows customization (domain join)

```go
// Skapa customization spec för domain join
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

// Klona VM med customization
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

// Vänta på att VMware Tools blir redo
err = vcenter.WaitForTools(ctx, vm)
if err != nil {
    log.Printf("Warning: VMware Tools timeout: %v\n", err)
}

// Hämta IP-adress
ip, err := vcenter.WaitForIP(ctx, vm, 10*time.Minute)
if err != nil {
    log.Printf("Warning: IP timeout: %v\n", err)
} else {
    log.Printf("VM IP: %s\n", ip)
}
```

### Klona med statisk IP

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

### Använd ServerRequest för strukturerad konfiguration

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

### Klona flera VMs parallellt

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

// Kontrollera resultat
for i, err := range errors {
    if err != nil {
        log.Printf("Misslyckades att klona %s: %v\n", requests[i].Name, err)
    } else {
        log.Printf("Lyckades klona %s\n", requests[i].Name)
    }
}
```

### Power-operationer

```go
// Hitta en VM
vm, err := vcenter.GetVM(ctx, client.Client, "WebServer01", "Datacenter1")
if err != nil {
    log.Fatal(err)
}

// Starta VM
err = vcenter.PowerOnVM(ctx, vm)
if err != nil {
    log.Fatal(err)
}

// Stäng av VM
err = vcenter.PowerOffVM(ctx, vm)
if err != nil {
    log.Fatal(err)
}

// Starta om VM (graceful med VMware Tools, annars hard reset)
err = vcenter.RestartVM(ctx, vm)
if err != nil {
    log.Fatal(err)
}
```

### Bulk power-operationer

```go
// Hitta flera VMs
vms := []*object.VirtualMachine{vm1, vm2, vm3}

// Starta alla VMs parallellt
errors := vcenter.BulkPowerOperation(ctx, vms, "on")
for i, err := range errors {
    if err != nil {
        log.Printf("VM %d: %v\n", i, err)
    }
}

// Andra operationer: "off", "restart"
```

### Disk-hantering

```go
// Lägg till en 100GB disk
err = vcenter.AddDisk(ctx, vm, 100, "datastore1")
if err != nil {
    log.Fatal(err)
}

// Utöka en disk från 100GB till 200GB
err = vcenter.ExtendDisk(ctx, vm, "Hard disk 2", 200)
if err != nil {
    log.Fatal(err)
}

// Ta bort en disk (VARNING: Raderar data permanent!)
err = vcenter.RemoveDisk(ctx, vm, "Hard disk 3")
if err != nil {
    log.Fatal(err)
}
```

### Nätverks-hantering

```go
// Lägg till ett VMXNET3 nätverkskort
err = vcenter.AddNetworkAdapter(ctx, vm, "Production-VLAN100")
if err != nil {
    log.Fatal(err)
}

// Byt nätverk på ett befintligt kort
err = vcenter.ChangeNetwork(ctx, vm, "Network adapter 1", "DMZ-VLAN200")
if err != nil {
    log.Fatal(err)
}
```

### Ändra CPU och minne

```go
// Sätt 4 CPUs och 8GB RAM
err = vcenter.SetVMResources(ctx, vm, 4, 8192)
if err != nil {
    log.Fatal(err)
}
```

## Error Handling

Paketet använder custom error types för bättre felhantering:

```go
vm, err := vcenter.GetVM(ctx, client.Client, "NonExistent", "DC1")
if err != nil {
    var notFoundErr *vcenter.NotFoundError
    if errors.As(err, &notFoundErr) {
        log.Printf("Resursen hittades inte: %s\n", notFoundErr)
    }
}

req := vcenter.ServerRequest{Name: ""}
err = req.Validate()
if err != nil {
    var validationErr *vcenter.ValidationError
    if errors.As(err, &validationErr) {
        log.Printf("Valideringsfel på fält %s: %s\n",
            validationErr.Field, validationErr.Message)
    }
}
```

## Windows Timezone IDs

Vanliga timezone IDs för Windows customization:

- `4` - Eastern Standard Time (EST)
- `15` - U.S. Eastern Standard Time
- `20` - Central Standard Time
- `35` - Mountain Standard Time
- `85` - W. Europe Standard Time (Stockholm, Berlin, Paris)
- `105` - Pacific Standard Time (PST)
- `110` - Alaska Standard Time
- `220` - UTC

Fullständig lista: https://docs.microsoft.com/en-us/previous-versions/windows/embedded/ms912391(v=winembedded.11)

## Licens

MIT License - Se LICENSE filen för detaljer.

## Bidra

Pull requests är välkomna! För större ändringar, öppna först ett issue för att diskutera vad du vill ändra.

## Credits

- Baserat på [govmomi](https://github.com/vmware/govmomi)
- Använder [go-vcenter-auth](https://github.com/skabbio1976/go-vcenter-auth) för autentisering
- Inspirerat av VMware PowerCLI
