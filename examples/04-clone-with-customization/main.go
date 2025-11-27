package main

import (
	"context"
	"log"
	"time"

	"github.com/skabbio1976/vcenter"
)

// This example demonstrates how to clone a Windows VM with domain join and DHCP.
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

	// Create Windows customization spec with DHCP and domain join
	customization := vcenter.NewWindowsCustomization(vcenter.WindowsCustomizationConfig{
		ComputerName:   "WebServer01",
		AdminPassword:  "LocalAdminPass123!",
		Timezone:       85, // W. Europe Standard Time
		Domain:         "example.com",
		DomainUser:     "administrator@example.com",
		DomainPassword: "DomainPassword123!",
		GlobalDNS:      []string{"192.168.1.1"},
		DNSSuffixes:    []string{"example.com"},
	})

	log.Println("Starting clone with Windows customization...")

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
		log.Fatalf("Failed to clone VM: %v", err)
	}

	log.Printf("VM cloned: %s", vm.Name())
	log.Println("VM starts automatically for customization...")

	// Wait for customization to complete (recommended approach)
	log.Println("Waiting for customization to complete...")
	err = vcenter.WaitForCustomization(ctx, vm, 15*time.Minute)
	if err != nil {
		log.Printf("Warning: Customization timeout: %v", err)
	} else {
		log.Println("Customization completed!")
	}

	// Wait for IP address
	log.Println("Waiting for IP address...")
	ip, err := vcenter.WaitForIP(ctx, vm, 10*time.Minute)
	if err != nil {
		log.Printf("Warning: IP timeout: %v", err)
	} else {
		log.Printf("VM IP: %s", ip)
	}

	log.Println("Done! VM is now domain-joined and ready to use")
}
