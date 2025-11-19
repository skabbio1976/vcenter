package main

import (
	"context"
	"log"
	"time"

	"github.com/skabbio1976/vcenter"
)

// Detta exempel visar hur man använder ServerRequest för strukturerad VM-konfiguration.
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

	// Skapa en ServerRequest med statisk IP
	req := vcenter.ServerRequest{
		Name:        "AppServer01",
		Template:    "Windows-2022-Template",
		CPUs:        4,
		MemoryGB:    16,
		DiskGB:      100,
		Domain:      "example.com",
		IPAddress:   "192.168.1.101",
		SubnetMask:  "255.255.255.0",
		Gateway:     "192.168.1.1",
		DNSServers:  []string{"192.168.1.1", "192.168.1.2"},
		DNSSuffixes: []string{"example.com", "local"},
	}

	// Validera request
	if err := req.Validate(); err != nil {
		log.Fatalf("Valideringsfel: %v", err)
	}

	log.Printf("Klonar %s från %s...\n", req.Name, req.Template)
	log.Printf("  CPU: %d, RAM: %dGB", req.CPUs, req.MemoryGB)
	log.Printf("  IP: %s, Gateway: %s", req.IPAddress, req.Gateway)

	// Klona VM med ServerRequest
	vm, err := vcenter.CloneFromRequest(
		ctx,
		client.Client,
		req,
		"Datacenter1",
		"datastore1",
		"Resources",
		"ApplicationServers",
		"administrator@example.com",
		"DomainPassword123!",
		"LocalAdminPass123!",
		85, // Timezone
	)
	if err != nil {
		log.Fatalf("Misslyckades att klona: %v", err)
	}

	log.Printf("✓ VM klonad: %s", vm.Name())

	// Vänta på IP och Tools
	log.Println("Väntar på VMware Tools och IP-konfiguration...")
	err = vcenter.WaitForTools(ctx, vm)
	if err != nil {
		log.Printf("Warning: %v", err)
	}

	ip, err := vcenter.WaitForIP(ctx, vm, 10*time.Minute)
	if err != nil {
		log.Printf("Warning: %v", err)
	} else {
		log.Printf("✓ VM IP: %s", ip)
		if ip == req.IPAddress {
			log.Println("✓ IP-adressen matchar konfigurationen")
		}
	}

	log.Println("\n✓ Server klar att användas!")
	log.Printf("  Hostname: %s.%s", req.Name, req.Domain)
	log.Printf("  IP: %s", req.IPAddress)
	log.Printf("  CPU: %d, RAM: %dGB", req.CPUs, req.MemoryGB)
}
