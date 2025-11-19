package main

import (
	"context"
	"log"

	"github.com/skabbio1976/vcenter"
)

// Detta exempel visar hur man klonar flera VMs parallellt med ServerRequest.
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

	// Definiera flera servrar att klona
	requests := []vcenter.ServerRequest{
		{
			Name:       "WebServer01",
			Template:   "Windows-2022-Template",
			CPUs:       2,
			MemoryGB:   4,
			Domain:     "example.com",
			DNSServers: []string{"192.168.1.1"},
		},
		{
			Name:       "WebServer02",
			Template:   "Windows-2022-Template",
			CPUs:       2,
			MemoryGB:   4,
			Domain:     "example.com",
			DNSServers: []string{"192.168.1.1"},
		},
		{
			Name:       "WebServer03",
			Template:   "Windows-2022-Template",
			CPUs:       2,
			MemoryGB:   4,
			Domain:     "example.com",
			DNSServers: []string{"192.168.1.1"},
		},
		{
			Name:       "AppServer01",
			Template:   "Windows-2022-Template",
			CPUs:       4,
			MemoryGB:   8,
			Domain:     "example.com",
			DNSServers: []string{"192.168.1.1"},
		},
	}

	log.Printf("Klonar %d VMs parallellt...\n", len(requests))

	// Klona alla VMs parallellt
	vms, errors := vcenter.CloneMultiple(
		ctx,
		client.Client,
		requests,
		"Datacenter1",
		"datastore1",
		"Resources",
		"Servers",
		"administrator@example.com",
		"DomainPassword123!",
		"LocalAdminPass123!",
		85, // Timezone
	)

	// Visa resultat
	successCount := 0
	failCount := 0

	for i, err := range errors {
		if err != nil {
			log.Printf("✗ %s: %v", requests[i].Name, err)
			failCount++
		} else {
			log.Printf("✓ %s: Lyckades", requests[i].Name)
			successCount++
		}
	}

	log.Printf("\n=== Sammanfattning ===")
	log.Printf("Totalt: %d", len(requests))
	log.Printf("Lyckades: %d", successCount)
	log.Printf("Misslyckades: %d", failCount)
	log.Printf("Skapade VMs: %d", len(vms))
}
