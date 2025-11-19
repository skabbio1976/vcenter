package main

import (
	"context"
	"log"

	"github.com/skabbio1976/vcenter"
)

// This example demonstrates how to connect to vCenter with Windows SSPI/Kerberos.
// Only works on Windows and uses the logged-in user's credentials.
func main() {
	ctx := context.Background()

	// Connect with SSPI (Windows integrated authentication)
	client, err := vcenter.ConnectWithSSPI(
		ctx,
		"vcenter.example.com", // vCenter hostname
		true,                  // insecure - skip TLS verification
		"Datacenter1",         // datacenter name
	)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Logout(ctx)

	log.Println("✓ Connected to vCenter with SSPI")

	// Display datacenter information
	dc := client.GetDatacenter()
	if dc != nil {
		log.Printf("✓ Datacenter: %s", client.GetDatacenterName())
	}

	log.Println("✓ Session active")
}
