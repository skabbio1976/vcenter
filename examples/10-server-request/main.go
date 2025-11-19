package main

import (
	"context"
	"log"
	"time"

	"github.com/skabbio1976/vcenter"
)

// This example demonstrates how to use ServerRequest for structured VM configuration.
func main() {
	ctx := context.Background()

	// Connect to vCenter
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

	// Create a ServerRequest with static IP
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

	// Validate request
	if err := req.Validate(); err != nil {
		log.Fatalf("Validation error: %v", err)
	}

	log.Printf("Cloning %s from %s...\n", req.Name, req.Template)
	log.Printf("  CPU: %d, RAM: %dGB", req.CPUs, req.MemoryGB)
	log.Printf("  IP: %s, Gateway: %s", req.IPAddress, req.Gateway)

	// Clone VM with ServerRequest
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
		log.Fatalf("Failed to clone: %v", err)
	}

	log.Printf("✓ VM cloned: %s", vm.Name())

	// Wait for IP and Tools
	log.Println("Waiting for VMware Tools and IP configuration...")
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
			log.Println("✓ IP address matches configuration")
		}
	}

	log.Println("\n✓ Server ready to use!")
	log.Printf("  Hostname: %s.%s", req.Name, req.Domain)
	log.Printf("  IP: %s", req.IPAddress)
	log.Printf("  CPU: %d, RAM: %dGB", req.CPUs, req.MemoryGB)
}
