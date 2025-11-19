package main

import (
	"context"
	"log"
	"time"

	"github.com/skabbio1976/vcenter"
)

// This example demonstrates how to clone a Windows VM with a static IP address.
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

	// Create Windows customization spec with static IP
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

	log.Println("Cloning Windows VM with static IP...")

	// Clone VM
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
		log.Fatalf("Failed to clone VM: %v", err)
	}

	log.Printf("✓ VM cloned: %s", vm.Name())

	// Wait for VMware Tools and IP
	log.Println("Waiting for VMware Tools and IP configuration...")
	err = vcenter.WaitForTools(ctx, vm)
	if err != nil {
		log.Printf("Warning: %v", err)
	}

	ip, err := vcenter.WaitForIP(ctx, vm, 10*time.Minute)
	if err != nil {
		log.Printf("Warning: %v", err)
	} else {
		log.Printf("✓ VM IP: %s (should be 192.168.1.100)", ip)
	}

	log.Println("✓ Done!")
}
