package main

import (
	"context"
	"log"

	"github.com/skabbio1976/vcenter"
)

// Detta exempel visar hur man hanterar nätverkskort på en VM.
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

	// Lägg till ett nytt nätverkskort
	log.Println("\n1. Lägger till nätverkskort på Production-VLAN100...")
	err = vcenter.AddNetworkAdapter(ctx, vm, "Production-VLAN100")
	if err != nil {
		log.Printf("✗ Misslyckades: %v", err)
	} else {
		log.Println("✓ VMXNET3 nätverkskort tillagt (Network adapter 2)")
	}

	// Lägg till ett till nätverkskort
	log.Println("\n2. Lägger till nätverkskort på Storage-VLAN200...")
	err = vcenter.AddNetworkAdapter(ctx, vm, "Storage-VLAN200")
	if err != nil {
		log.Printf("✗ Misslyckades: %v", err)
	} else {
		log.Println("✓ VMXNET3 nätverkskort tillagt (Network adapter 3)")
	}

	// Byt nätverk på ett befintligt kort
	log.Println("\n3. Byter nätverk på Network adapter 1 till DMZ-VLAN300...")
	err = vcenter.ChangeNetwork(ctx, vm, "Network adapter 1", "DMZ-VLAN300")
	if err != nil {
		log.Printf("✗ Misslyckades: %v", err)
	} else {
		log.Println("✓ Nätverk bytt till DMZ-VLAN300")
	}

	log.Println("\n✓ Nätverks-operationer klara")
	log.Println("\nVM har nu följande nätverkskort:")
	log.Println("  - Network adapter 1: DMZ-VLAN300")
	log.Println("  - Network adapter 2: Production-VLAN100")
	log.Println("  - Network adapter 3: Storage-VLAN200")
}
