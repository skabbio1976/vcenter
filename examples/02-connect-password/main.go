package main

import (
	"context"
	"log"

	"github.com/skabbio1976/vcenter"
)

// Detta exempel visar hur man ansluter till vCenter med username och password.
// Använder go-vcenter-auth för session caching.
func main() {
	ctx := context.Background()

	// Konfigurera anslutning
	config := vcenter.ConnectConfig{
		Host:       "vcenter.example.com",
		Username:   "administrator@vsphere.local",
		Password:   "YourPassword",
		Insecure:   true, // skip TLS verification
		Datacenter: "Datacenter1",
	}

	// Anslut
	client, err := vcenter.ConnectWithPassword(ctx, config)
	if err != nil {
		log.Fatalf("Misslyckades att ansluta: %v", err)
	}
	defer client.Logout(ctx)

	log.Println("✓ Ansluten till vCenter")
	log.Printf("✓ Datacenter: %s", client.GetDatacenterName())
	log.Println("✓ Session är cachad och kan återanvändas")
}
