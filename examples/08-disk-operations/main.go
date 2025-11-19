package main

import (
	"context"
	"log"

	"github.com/skabbio1976/vcenter"
)

// This example demonstrates how to manage disks on a VM.
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

	// Add a 100GB disk
	log.Println("\n1. Adding 100GB disk...")
	err = vcenter.AddDisk(ctx, vm, 100, "datastore1")
	if err != nil {
		log.Printf("✗ Failed: %v", err)
	} else {
		log.Println("✓ 100GB disk added (Hard disk 2)")
	}

	// Add a 50GB disk
	log.Println("\n2. Adding 50GB disk...")
	err = vcenter.AddDisk(ctx, vm, 50, "datastore1")
	if err != nil {
		log.Printf("✗ Failed: %v", err)
	} else {
		log.Println("✓ 50GB disk added (Hard disk 3)")
	}

	// Extend a disk
	log.Println("\n3. Extending Hard disk 2 from 100GB to 200GB...")
	err = vcenter.ExtendDisk(ctx, vm, "Hard disk 2", 200)
	if err != nil {
		log.Printf("✗ Failed: %v", err)
	} else {
		log.Println("✓ Disk extended to 200GB")
		log.Println("  NOTE: The partition in the guest system must be extended manually")
	}

	// Remove a disk
	log.Println("\n4. Removing Hard disk 3...")
	err = vcenter.RemoveDisk(ctx, vm, "Hard disk 3")
	if err != nil {
		log.Printf("✗ Failed: %v", err)
	} else {
		log.Println("✓ Disk removed (VMDK file is deleted)")
	}

	log.Println("\n✓ Disk operations complete")
}
