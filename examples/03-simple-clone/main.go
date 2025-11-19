package main

import (
	"context"
	"log"

	"github.com/skabbio1976/vcenter"
)

// Detta exempel visar hur man klonar en enkel VM utan customization.
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

	log.Println("Startar kloning av VM...")

	// Klona VM
	vm, err := vcenter.CloneVM(
		ctx,
		client.Client,
		"Ubuntu-22.04-Template", // template name
		"TestServer01",          // new VM name
		"Datacenter1",           // datacenter
		"datastore1",            // datastore
		"Resources",             // resource pool
		"",                      // folder (tom = default VM folder)
	)
	if err != nil {
		log.Fatalf("Misslyckades att klona VM: %v", err)
	}

	log.Printf("✓ VM klonad: %s", vm.Name())

	// Ändra resurser (4 CPU, 8GB RAM)
	log.Println("Sätter CPU och minne...")
	err = vcenter.SetVMResources(ctx, vm, 4, 8192)
	if err != nil {
		log.Printf("Warning: Misslyckades att sätta resurser: %v", err)
	} else {
		log.Println("✓ Resurser uppdaterade: 4 CPU, 8GB RAM")
	}

	// Starta VM
	log.Println("Startar VM...")
	err = vcenter.PowerOnVM(ctx, vm)
	if err != nil {
		log.Printf("Warning: Misslyckades att starta VM: %v", err)
	} else {
		log.Println("✓ VM startad")
	}
}
