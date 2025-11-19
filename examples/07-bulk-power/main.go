package main

import (
	"context"
	"log"

	"github.com/skabbio1976/vcenter"
	"github.com/vmware/govmomi/object"
)

// This example demonstrates how to perform power operations on multiple VMs in parallel.
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

	// List of VM names to manage
	vmNames := []string{
		"WebServer01",
		"WebServer02",
		"WebServer03",
		"AppServer01",
	}

	// Find all VMs
	log.Println("Finding VMs...")
	var vms []*object.VirtualMachine
	for _, name := range vmNames {
		vm, err := vcenter.GetVM(ctx, client.Client, name, "Datacenter1")
		if err != nil {
			log.Printf("Warning: Could not find %s: %v", name, err)
			continue
		}
		vms = append(vms, vm)
		log.Printf("✓ Found: %s", name)
	}

	if len(vms) == 0 {
		log.Fatal("No VMs found")
	}

	// Power on all VMs in parallel
	log.Printf("\nStarting %d VMs in parallel...", len(vms))
	errors := vcenter.BulkPowerOperation(ctx, vms, "on")

	successCount := 0
	for i, err := range errors {
		if err != nil {
			log.Printf("✗ %s: %v", vmNames[i], err)
		} else {
			log.Printf("✓ %s: Started", vmNames[i])
			successCount++
		}
	}

	log.Printf("\n✓ %d/%d VMs started successfully", successCount, len(vms))

	// Examples of other operations:
	// errors = vcenter.BulkPowerOperation(ctx, vms, "off")     // Power off all
	// errors = vcenter.BulkPowerOperation(ctx, vms, "restart") // Restart all
}
