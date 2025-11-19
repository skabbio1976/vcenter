package main

import (
	"context"
	"log"

	"github.com/skabbio1976/vcenter"
	"github.com/vmware/govmomi/object"
)

// Detta exempel visar hur man utför power-operationer på flera VMs parallellt.
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

	// Lista med VM-namn att hantera
	vmNames := []string{
		"WebServer01",
		"WebServer02",
		"WebServer03",
		"AppServer01",
	}

	// Hitta alla VMs
	log.Println("Hittar VMs...")
	var vms []*object.VirtualMachine
	for _, name := range vmNames {
		vm, err := vcenter.GetVM(ctx, client.Client, name, "Datacenter1")
		if err != nil {
			log.Printf("Warning: Kunde inte hitta %s: %v", name, err)
			continue
		}
		vms = append(vms, vm)
		log.Printf("✓ Hittade: %s", name)
	}

	if len(vms) == 0 {
		log.Fatal("Inga VMs hittades")
	}

	// Starta alla VMs parallellt
	log.Printf("\nStartar %d VMs parallellt...", len(vms))
	errors := vcenter.BulkPowerOperation(ctx, vms, "on")

	successCount := 0
	for i, err := range errors {
		if err != nil {
			log.Printf("✗ %s: %v", vmNames[i], err)
		} else {
			log.Printf("✓ %s: Startad", vmNames[i])
			successCount++
		}
	}

	log.Printf("\n✓ %d/%d VMs startades framgångsrikt", successCount, len(vms))

	// Exempel på andra operationer:
	// errors = vcenter.BulkPowerOperation(ctx, vms, "off")     // Stäng av alla
	// errors = vcenter.BulkPowerOperation(ctx, vms, "restart") // Starta om alla
}
