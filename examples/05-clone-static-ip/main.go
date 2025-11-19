package main

import (
	"context"
	"log"
	"time"

	"github.com/skabbio1976/vcenter"
)

// Detta exempel visar hur man klonar en Windows VM med statisk IP-adress.
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

	// Skapa Windows customization spec med statisk IP
	customization := vcenter.NewWindowsCustomizationStaticIP(
		"DBServer01",                     // Computer name
		"example.com",                    // AD Domain
		"administrator@example.com",      // Domain admin user
		"DomainPassword123!",             // Domain password
		"LocalAdminPass123!",             // Local admin password
		85,                               // Timezone (W. Europe)
		"192.168.1.100",                  // Static IP address
		"255.255.255.0",                  // Subnet mask
		"192.168.1.1",                    // Default gateway
		[]string{"192.168.1.1", "192.168.1.2"}, // DNS servers
		[]string{"example.com"},          // DNS suffixes
	)

	log.Println("Klonar Windows VM med statisk IP...")

	// Klona VM
	vm, err := vcenter.CloneVMWithCustomization(
		ctx,
		client.Client,
		"Windows-2022-Template",
		"DBServer01",
		"Datacenter1",
		"datastore1",
		"Resources",
		"DatabaseServers",
		customization,
	)
	if err != nil {
		log.Fatalf("Misslyckades att klona VM: %v", err)
	}

	log.Printf("✓ VM klonad: %s", vm.Name())

	// Vänta på VMware Tools och IP
	log.Println("Väntar på VMware Tools och IP-konfiguration...")
	err = vcenter.WaitForTools(ctx, vm)
	if err != nil {
		log.Printf("Warning: %v", err)
	}

	ip, err := vcenter.WaitForIP(ctx, vm, 10*time.Minute)
	if err != nil {
		log.Printf("Warning: %v", err)
	} else {
		log.Printf("✓ VM IP: %s (ska vara 192.168.1.100)", ip)
	}

	log.Println("✓ Klart!")
}
