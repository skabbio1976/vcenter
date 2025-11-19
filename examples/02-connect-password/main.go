package main

import (
	"context"
	"log"

	"github.com/skabbio1976/vcenter"
)

// This example demonstrates how to connect to vCenter with username and password.
// Uses go-vcenter-auth for session caching.
func main() {
	ctx := context.Background()

	// Configure connection
	config := vcenter.ConnectConfig{
		Host:       "vcenter.example.com",
		Username:   "administrator@vsphere.local",
		Password:   "YourPassword",
		Insecure:   true, // skip TLS verification
		Datacenter: "Datacenter1",
	}

	// Connect
	client, err := vcenter.ConnectWithPassword(ctx, config)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Logout(ctx)

	log.Println("✓ Connected to vCenter")
	log.Printf("✓ Datacenter: %s", client.GetDatacenterName())
	log.Println("✓ Session is cached and can be reused")
}
