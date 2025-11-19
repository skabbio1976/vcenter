# vcenter - Kodexempel

Den här katalogen innehåller praktiska exempel som visar hur man använder vcenter-paketet för olika vCenter-operationer.

## Översikt

Exemplen är organiserade i numerisk ordning från enkel till avancerad användning:

### Anslutning

- **01-connect-sspi** - Anslut med Windows SSPI/Kerberos (single sign-on)
- **02-connect-password** - Anslut med username och password

### VM-kloning

- **03-simple-clone** - Enkel VM-kloning utan customization
- **04-clone-with-customization** - Klona Windows VM med domain join (DHCP)
- **05-clone-static-ip** - Klona Windows VM med statisk IP-adress
- **10-server-request** - Använd ServerRequest för strukturerad konfiguration

### Batch-operationer

- **06-batch-clone** - Klona flera VMs parallellt
- **07-bulk-power** - Power-operationer på flera VMs samtidigt

### VM-hantering

- **08-disk-operations** - Lägg till, utöka och ta bort diskar
- **09-network-operations** - Lägg till och ändra nätverkskort

## Hur man kör exemplen

### Förutsättningar

1. Go 1.21 eller senare installerat
2. Tillgång till en vCenter Server
3. En template att klona från

### Konfigurera exempel

Varje exempel innehåller platshållare som behöver uppdateras:

```go
// Uppdatera dessa värden
Host:       "vcenter.example.com",      // Din vCenter hostname
Username:   "administrator@vsphere.local",
Password:   "YourPassword",
Datacenter: "Datacenter1",              // Ditt datacenter
```

### Kör ett exempel

```bash
# Navigera till exempel-katalogen
cd examples/03-simple-clone

# Redigera main.go och uppdatera konfigurationen
vim main.go

# Kör exemplet
go run main.go
```

## Exempel-beskrivningar

### 01-connect-sspi

Visar hur man ansluter med Windows integrated authentication. Perfekt för Windows-miljöer där användare redan är autentiserade mot Active Directory.

**Kräver:** Windows-operativsystem

```bash
cd examples/01-connect-sspi
go run main.go
```

### 02-connect-password

Visar standardanslutning med username och password. Fungerar på alla plattformar och använder session caching för bättre prestanda.

```bash
cd examples/02-connect-password
go run main.go
```

### 03-simple-clone

Grundläggande VM-kloning utan Windows customization. Bra för:
- Linux VMs
- Templates utan sysprep
- Snabb kloning för test

```bash
cd examples/03-simple-clone
go run main.go
```

### 04-clone-with-customization

Klona Windows VM med:
- Domain join
- DHCP IP-konfiguration
- DNS-inställningar
- Timezone
- Automatisk väntan på VMware Tools och IP

```bash
cd examples/04-clone-with-customization
go run main.go
```

### 05-clone-static-ip

Samma som exemplet ovan men med statisk IP-adress istället för DHCP. Perfekt för servrar som behöver fasta IP-adresser.

```bash
cd examples/05-clone-static-ip
go run main.go
```

### 06-batch-clone

Visar hur man klonar flera VMs parallellt med goroutines. Dramatiskt snabbare än sekventiell kloning:

- 4 VMs klonas samtidigt
- Visar framgång/misslyckande för varje VM
- Använder ServerRequest för konfiguration

```bash
cd examples/06-batch-clone
go run main.go
```

**Prestanda:** Klonar 4 VMs på samma tid som det tar att klona 1 VM sekventiellt.

### 07-bulk-power

Parallella power-operationer på flera VMs:
- Power on
- Power off
- Restart

Perfekt för att starta/stänga hela miljöer samtidigt.

```bash
cd examples/07-bulk-power
go run main.go
```

### 08-disk-operations

Komplett exempel på disk-hantering:
- Lägg till nya diskar (thin provisioned)
- Utöka befintliga diskar
- Ta bort diskar

**OBS:** Kommer ihåg att utöka partitioner i gästsystemet efter disk-utökning.

```bash
cd examples/08-disk-operations
go run main.go
```

### 09-network-operations

Nätverkskort-hantering:
- Lägg till VMXNET3-adapters
- Byt nätverk/portgrupp
- Multi-NIC konfiguration

```bash
cd examples/09-network-operations
go run main.go
```

### 10-server-request

Avancerat exempel som visar ServerRequest struct för:
- Strukturerad server-konfiguration
- Input-validering
- Komplett server-provisionering med en funktion
- Automatisk verifiering av IP-konfiguration

```bash
cd examples/10-server-request
go run main.go
```

## Best Practices

### 1. Error Handling

Hantera alltid errors och använd type assertions för specifika error-typer:

```go
vm, err := vcenter.GetVM(ctx, client.Client, "NonExistent", "DC1")
if err != nil {
    var notFoundErr *vcenter.NotFoundError
    if errors.As(err, &notFoundErr) {
        log.Printf("VM hittades inte: %s", notFoundErr)
    }
}
```

### 2. Context och Timeout

Använd alltid context med timeout för långvariga operationer:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
defer cancel()

vm, err := vcenter.CloneVM(ctx, ...)
```

### 3. Resource Cleanup

Glöm inte att logga ut från vCenter:

```go
client, err := vcenter.ConnectWithPassword(ctx, config)
if err != nil {
    log.Fatal(err)
}
defer client.Logout(ctx)  // VIKTIGT!
```

### 4. Parallella Operationer

För batch-operationer, använd de inbyggda parallella funktionerna:

```go
// GOOD - Parallell kloning
vms, errors := vcenter.CloneMultiple(ctx, client, requests, ...)

// AVOID - Sekventiell kloning
for _, req := range requests {
    vm, err := vcenter.CloneFromRequest(ctx, client, req, ...)
}
```

### 5. VMware Tools

Vänta alltid på VMware Tools innan IP-operationer:

```go
err = vcenter.WaitForTools(ctx, vm)
if err == nil {
    ip, err := vcenter.WaitForIP(ctx, vm, 10*time.Minute)
}
```

## Vanliga Problem

### Problem: "VM not found"

**Lösning:** Kontrollera att VM-namnet och datacenter är korrekt:

```go
// Sök i alla datacenter
vm, err := vcenter.GetVM(ctx, client.Client, "VMName", "")

// Sök i specifikt datacenter
vm, err := vcenter.GetVM(ctx, client.Client, "VMName", "DC1")
```

### Problem: Timeout vid WaitForIP

**Lösning:**
1. Kontrollera att VMware Tools är installerat
2. Öka timeout-värdet
3. Kontrollera nätverkskonfiguration

```go
// Öka timeout till 15 minuter
ip, err := vcenter.WaitForIP(ctx, vm, 15*time.Minute)
```

### Problem: "disk with label X not found"

**Lösning:** Kontrollera exakt label på disken i vCenter:

```go
// Använd exakt label från vCenter
err := vcenter.ExtendDisk(ctx, vm, "Hard disk 2", 200)
```

### Problem: Customization fungerar inte

**Lösning:**
1. Kontrollera att templaten har sysprep förberett
2. Verifiera domain credentials
3. Kontrollera DNS-konfiguration
4. Se till att PowerOn är true i CloneSpec

## Ytterligare Resurser

- [Huvuddokumentation](../README.md)
- [godoc](https://pkg.go.dev/github.com/skabbio1976/vcenter)
- [govmomi dokumentation](https://github.com/vmware/govmomi)
- [VMware vSphere API](https://developer.vmware.com/apis/968/vsphere)

## Bidra med Exempel

Har du ett användbart exempel? Skicka en pull request med:

1. Välorganiserad kod med kommentarer
2. Uppdatering av denna README
3. Testning mot en faktisk vCenter

## Support

Om du hittar buggar eller har frågor:
- Öppna ett issue på GitHub
- Kontrollera befintliga exempel först
- Inkludera felmeddelanden och Go-version
