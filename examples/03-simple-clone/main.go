package main

import (
	"context"
	"log"

	"github.com/skabbio1976/vcenter"
)

// This example demonstrates how to clone a simple VM without customization.
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

	log.Println("Starting VM clone...")

	// Clone VM
	vm, err := vcenter.CloneVM(
		ctx,
		client.Client,
		"Ubuntu-22.04-Template", // template name
		"TestServer01",          // new VM name
		"Datacenter1",           // datacenter
		"datastore1",            // datastore
		"Resources",             // resource pool
		"",                      // folder (empty = default VM folder)
	)
	if err != nil {
		log.Fatalf("Failed to clone VM: %v", err)
	}

	log.Printf("✓ VM cloned: %s", vm.Name())

	// Set resources (4 CPU, 8GB RAM)
	log.Println("Setting CPU and memory...")
	err = vcenter.SetVMResources(ctx, vm, 4, 8192)
	if err != nil {
		log.Printf("Warning: Failed to set resources: %v", err)
	} else {
		log.Println("✓ Resources updated: 4 CPU, 8GB RAM")
	}

	// Power on VM
	log.Println("Starting VM...")
	err = vcenter.PowerOnVM(ctx, vm)
	if err != nil {
		log.Printf("Warning: Failed to start VM: %v", err)
	} else {
		log.Println("✓ VM started")
	}
}
