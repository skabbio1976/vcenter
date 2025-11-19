package main

import (
	"context"
	"log"

	"github.com/skabbio1976/vcenter"
)

// Detta exempel visar hur man hanterar diskar på en VM.
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

	// Hitta VM
	vm, err := vcenter.GetVM(ctx, client.Client, "TestServer01", "Datacenter1")
	if err != nil {
		log.Fatalf("Kunde inte hitta VM: %v", err)
	}

	log.Printf("Arbetar med VM: %s\n", vm.Name())

	// Lägg till en 100GB disk
	log.Println("\n1. Lägger till 100GB disk...")
	err = vcenter.AddDisk(ctx, vm, 100, "datastore1")
	if err != nil {
		log.Printf("✗ Misslyckades: %v", err)
	} else {
		log.Println("✓ 100GB disk tillagd (Hard disk 2)")
	}

	// Lägg till en 50GB disk
	log.Println("\n2. Lägger till 50GB disk...")
	err = vcenter.AddDisk(ctx, vm, 50, "datastore1")
	if err != nil {
		log.Printf("✗ Misslyckades: %v", err)
	} else {
		log.Println("✓ 50GB disk tillagd (Hard disk 3)")
	}

	// Utöka en disk
	log.Println("\n3. Utökar Hard disk 2 från 100GB till 200GB...")
	err = vcenter.ExtendDisk(ctx, vm, "Hard disk 2", 200)
	if err != nil {
		log.Printf("✗ Misslyckades: %v", err)
	} else {
		log.Println("✓ Disk utökad till 200GB")
		log.Println("  OBS: Partitionen i gästsystemet måste utökas manuellt")
	}

	// Ta bort en disk
	log.Println("\n4. Tar bort Hard disk 3...")
	err = vcenter.RemoveDisk(ctx, vm, "Hard disk 3")
	if err != nil {
		log.Printf("✗ Misslyckades: %v", err)
	} else {
		log.Println("✓ Disk borttagen (VMDK-filen är raderad)")
	}

	log.Println("\n✓ Disk-operationer klara")
}
