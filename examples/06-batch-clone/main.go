package main

import (
	"context"
	"log"
	"sync"

	"github.com/skabbio1976/vcenter"
)

// This example demonstrates how to clone multiple VMs in parallel using goroutines.
// The API does not provide a CloneMultiple function by design - users control their
// own concurrency, rate limiting, and error handling.
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

	// Define multiple servers to clone
	requests := []vcenter.ServerRequest{
		{
			Name:       "WebServer01",
			Template:   "Windows-2022-Template",
			CPUs:       2,
			MemoryGB:   4,
			Domain:     "example.com",
			DNSServers: []string{"192.168.1.1"},
		},
		{
			Name:       "WebServer02",
			Template:   "Windows-2022-Template",
			CPUs:       2,
			MemoryGB:   4,
			Domain:     "example.com",
			DNSServers: []string{"192.168.1.1"},
		},
		{
			Name:       "WebServer03",
			Template:   "Windows-2022-Template",
			CPUs:       2,
			MemoryGB:   4,
			Domain:     "example.com",
			DNSServers: []string{"192.168.1.1"},
		},
		{
			Name:       "AppServer01",
			Template:   "Windows-2022-Template",
			CPUs:       4,
			MemoryGB:   8,
			Domain:     "example.com",
			DNSServers: []string{"192.168.1.1"},
		},
	}

	log.Printf("Cloning %d VMs in parallel...\n", len(requests))

	// Clone all VMs in parallel using goroutines
	var wg sync.WaitGroup
	results := make([]error, len(requests))

	for i, req := range requests {
		wg.Add(1)
		go func(idx int, r vcenter.ServerRequest) {
			defer wg.Done()
			_, err := vcenter.CloneFromRequest(
				ctx,
				client.Client,
				r,
				"Datacenter1",
				"datastore1",
				"Resources",
				"Servers",
				"administrator@example.com",
				"DomainPassword123!",
				"LocalAdminPass123!",
				85, // Timezone
			)
			results[idx] = err
		}(i, req)
	}

	wg.Wait()

	// Show results
	successCount := 0
	failCount := 0

	for i, err := range results {
		if err != nil {
			log.Printf("✗ %s: %v", requests[i].Name, err)
			failCount++
		} else {
			log.Printf("✓ %s: Succeeded", requests[i].Name)
			successCount++
		}
	}

	log.Printf("\n=== Summary ===")
	log.Printf("Total: %d", len(requests))
	log.Printf("Succeeded: %d", successCount)
	log.Printf("Failed: %d", failCount)
}
