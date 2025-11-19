package main

import (
	"context"
	"log"

	"github.com/skabbio1976/vcenter"
)

// This example demonstrates how to manage network adapters on a VM.
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

	// Find VM
	vm, err := vcenter.GetVM(ctx, client.Client, "TestServer01", "Datacenter1")
	if err != nil {
		log.Fatalf("Could not find VM: %v", err)
	}

	log.Printf("Working with VM: %s\n", vm.Name())

	// Add a new network adapter
	log.Println("\n1. Adding network adapter to Production-VLAN100...")
	err = vcenter.AddNetworkAdapter(ctx, vm, "Production-VLAN100")
	if err != nil {
		log.Printf("✗ Failed: %v", err)
	} else {
		log.Println("✓ VMXNET3 network adapter added (Network adapter 2)")
	}

	// Add another network adapter
	log.Println("\n2. Adding network adapter to Storage-VLAN200...")
	err = vcenter.AddNetworkAdapter(ctx, vm, "Storage-VLAN200")
	if err != nil {
		log.Printf("✗ Failed: %v", err)
	} else {
		log.Println("✓ VMXNET3 network adapter added (Network adapter 3)")
	}

	// Change network on an existing adapter
	log.Println("\n3. Changing network on Network adapter 1 to DMZ-VLAN300...")
	err = vcenter.ChangeNetwork(ctx, vm, "Network adapter 1", "DMZ-VLAN300")
	if err != nil {
		log.Printf("✗ Failed: %v", err)
	} else {
		log.Println("✓ Network changed to DMZ-VLAN300")
	}

	log.Println("\n✓ Network operations complete")
	log.Println("\nVM now has the following network adapters:")
	log.Println("  - Network adapter 1: DMZ-VLAN300")
	log.Println("  - Network adapter 2: Production-VLAN100")
	log.Println("  - Network adapter 3: Storage-VLAN200")
}
