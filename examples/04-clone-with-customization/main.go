package main

import (
	"context"
	"log"
	"time"

	"github.com/skabbio1976/vcenter"
)

// Detta exempel visar hur man klonar en Windows VM med domain join och DHCP.
func main() {
	ctx := context.Background()

	// Anslut till vCenter
	config := vcenter.ConnectConfig{
		Host:       "vcenter.example.com",
		Username:   "administrator@vsphere.local",
		Password:   "YourPassword",
		Insecure:   true,
		Datacenter: "Datacenter1",
	}

	client, err := vcenter.ConnectWithPassword(ctx, config)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Logout(ctx)

	// Skapa Windows customization spec med DHCP
	customization := vcenter.NewWindowsCustomization(
		"WebServer01",                    // Computer name
		"example.com",                    // AD Domain
		"administrator@example.com",      // Domain admin user
		"DomainPassword123!",             // Domain password
		"LocalAdminPass123!",             // Local admin password
		85,                               // Timezone (85 = W. Europe Standard Time)
		[]string{"192.168.1.1"},          // DNS servers
		[]string{"example.com"},          // DNS suffixes
	)

	log.Println("Startar kloning med Windows customization...")

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
		log.Fatalf("Misslyckades att klona VM: %v", err)
	}

	log.Printf("✓ VM klonad: %s", vm.Name())
	log.Println("VM startar automatiskt för customization...")

	// Vänta på VMware Tools
	log.Println("Väntar på VMware Tools...")
	err = vcenter.WaitForTools(ctx, vm)
	if err != nil {
		log.Printf("Warning: VMware Tools timeout: %v", err)
	} else {
		log.Println("✓ VMware Tools är redo")
	}

	// Vänta på IP-adress
	log.Println("Väntar på IP-adress...")
	ip, err := vcenter.WaitForIP(ctx, vm, 10*time.Minute)
	if err != nil {
		log.Printf("Warning: IP timeout: %v", err)
	} else {
		log.Printf("✓ VM IP: %s", ip)
	}

	log.Println("✓ Klart! VM är nu domain-joinad och redo att använda")
}
