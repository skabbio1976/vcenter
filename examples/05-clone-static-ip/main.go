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
	customization := vcenter.NewWindowsCustomization(vcenter.WindowsCustomizationConfig{
		ComputerName:   "DBServer01",
		AdminPassword:  "LocalAdminPass123!",
		Timezone:       85, // W. Europe Standard Time
		Domain:         "example.com",
		DomainUser:     "administrator@example.com",
		DomainPassword: "DomainPassword123!",
		// Static IP configuration using Adapters
		Adapters: []vcenter.NetworkAdapter{{
			IPAddress:  "192.168.1.100",
			SubnetMask: "255.255.255.0",
			Gateway:    "192.168.1.1",
			DNSServers: []string{"192.168.1.1", "192.168.1.2"},
		}},
		GlobalDNS:   []string{"192.168.1.1", "192.168.1.2"},
		DNSSuffixes: []string{"example.com"},
	})

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

	log.Printf("VM cloned: %s", vm.Name())

	// Wait for customization to complete
	log.Println("Waiting for customization to complete...")
	err = vcenter.WaitForCustomization(ctx, vm, 15*time.Minute)
	if err != nil {
		log.Printf("Warning: %v", err)
	} else {
		log.Println("Customization completed!")
	}

	// Verify IP address
	ip, err := vcenter.WaitForIP(ctx, vm, 10*time.Minute)
	if err != nil {
		log.Printf("Warning: %v", err)
	} else {
		log.Printf("VM IP: %s (expected: 192.168.1.100)", ip)
	}

	log.Println("Done!")
}
