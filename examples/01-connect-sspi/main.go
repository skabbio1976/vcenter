package main

import (
	"context"
	"log"

	"github.com/skabbio1976/vcenter"
)

// Detta exempel visar hur man ansluter till vCenter med Windows SSPI/Kerberos.
// Fungerar endast på Windows och använder den inloggade användarens credentials.
func main() {
	ctx := context.Background()

	// Anslut med SSPI (Windows integrated authentication)
	client, err := vcenter.ConnectWithSSPI(
		ctx,
		"vcenter.example.com", // vCenter hostname
		true,                  // insecure - skip TLS verification
		"Datacenter1",         // datacenter name
	)
	if err != nil {
		log.Fatalf("Misslyckades att ansluta: %v", err)
	}
	defer client.Logout(ctx)

	log.Println("✓ Ansluten till vCenter med SSPI")

	// Visa datacenter-information
	dc := client.GetDatacenter()
	if dc != nil {
		log.Printf("✓ Datacenter: %s", client.GetDatacenterName())
	}

	log.Println("✓ Session aktiv")
}
