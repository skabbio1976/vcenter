// Integration tests for vcenter package
//
// These tests require a real vCenter environment and are configured via test-config.json
//
// To run these tests:
// 1. Copy test-config.example.json to test-config.json
// 2. Update with your vCenter details
// 3. Run: go test -v -timeout 30m
//
// For airgapped environments:
// 1. Build on internet-connected machine: go test -c -o vcenter-integration-test
// 2. Copy binary and test-config.json to airgapped environment
// 3. Run: ./vcenter-integration-test -test.v

package vcenter_test

import (
	"context"
	"testing"
	"time"

	"github.com/skabbio1976/vcenter"
	"github.com/skabbio1976/vcenter/testdata"
)

// Test01_Authentication tests connection and authentication
func Test01_Authentication(t *testing.T) {
	testdata.SkipIfNoConfig(t)
	t.Log("\n=== Test 01: Authentication ===")

	ctx := context.Background()
	client, _ := testdata.GetTestClient(t, ctx)
	defer client.Logout(ctx)

	t.Log("✓ Successfully connected to vCenter")

	// Verify datacenter is set
	dc := client.GetDatacenterName()
	if dc == "" {
		t.Fatal("Datacenter not set")
	}
	t.Logf("✓ Using datacenter: %s", dc)
}

// Test02_GetVM tests finding and getting VM info
func Test02_GetVM(t *testing.T) {
	testdata.SkipIfNoConfig(t)
	t.Log("\n=== Test 02: Get VM Info ===")

	ctx := context.Background()
	client, config := testdata.GetTestClient(t, ctx)
	defer client.Logout(ctx)

	// Find template
	t.Logf("Looking for template: %s", config.TestResources.TemplateName)
	datacenter := client.GetDatacenterName()
	template, err := vcenter.GetVM(ctx, client.Client, config.TestResources.TemplateName, datacenter)
	testdata.AssertNoError(t, err, "Failed to get template")
	t.Logf("✓ Found template: %s", template.Name())

	// Get VM info
	t.Log("Getting template info...")
	info, err := vcenter.GetVMInfo(ctx, template)
	testdata.AssertNoError(t, err, "Failed to get VM info")

	t.Logf("✓ Template info:")
	t.Logf("  Name: %s", info.Name)
	t.Logf("  CPUs: %d", info.CPUCount)
	t.Logf("  Memory: %.0f GB", info.MemoryGB)
	t.Logf("  Power State: %s", info.PowerState)
}

// Test03_CloneVM tests VM cloning operation
func Test03_CloneVM(t *testing.T) {
	testdata.SkipIfNoConfig(t)
	t.Log("\n=== Test 03: Clone VM ===")

	ctx := context.Background()
	client, config := testdata.GetTestClient(t, ctx)
	defer client.Logout(ctx)

	vmName := testdata.GenerateTestVMName(config.TestResources.TestVMPrefix, "clone")
	t.Logf("Test VM name: %s", vmName)

	testFailed := false
	defer testdata.CleanupFunc(t, ctx, client, config, vmName, &testFailed)()

	// Clone VM
	t.Log("Cloning VM...")
	vm, err := vcenter.CloneVM(
		ctx,
		client.Client,
		config.TestResources.TemplateName,
		vmName,
		client.GetDatacenterName(),
		config.TestResources.Datastore,
		config.TestResources.ResourcePool,
		config.TestResources.Folder,
	)
	if err != nil {
		testFailed = true
		t.Fatalf("Failed to clone VM: %v", err)
	}
	t.Logf("✓ Cloned VM: %s", vm.Name())

	// Verify VM exists
	t.Log("Verifying VM exists...")
	info, err := vcenter.GetVMInfo(ctx, vm)
	if err != nil {
		testFailed = true
		t.Fatalf("Failed to get VM info: %v", err)
	}
	testdata.AssertEqual(t, vmName, info.Name, "VM name mismatch")
	t.Logf("✓ VM verified: %s", info.Name)
}

// Test04_PowerOperations tests VM power management
func Test04_PowerOperations(t *testing.T) {
	testdata.SkipIfNoConfig(t)
	t.Log("\n=== Test 04: Power Operations ===")

	ctx := context.Background()
	client, config := testdata.GetTestClient(t, ctx)
	defer client.Logout(ctx)

	vmName := testdata.GenerateTestVMName(config.TestResources.TestVMPrefix, "power")
	t.Logf("Test VM name: %s", vmName)

	testFailed := false
	defer testdata.CleanupFunc(t, ctx, client, config, vmName, &testFailed)()

	// Clone VM for testing
	t.Log("Creating test VM...")
	vm, err := vcenter.CloneVM(
		ctx,
		client.Client,
		config.TestResources.TemplateName,
		vmName,
		client.GetDatacenterName(),
		config.TestResources.Datastore,
		config.TestResources.ResourcePool,
		config.TestResources.Folder,
	)
	if err != nil {
		testFailed = true
		t.Fatalf("Failed to clone VM: %v", err)
	}
	t.Logf("✓ Created VM: %s", vm.Name())

	// Power on
	t.Log("Powering on VM...")
	if err := vcenter.PowerOnVM(ctx, vm); err != nil {
		testFailed = true
		t.Fatalf("Failed to power on: %v", err)
	}
	t.Log("✓ VM powered on")
	time.Sleep(3 * time.Second)

	// Verify power state
	info, err := vcenter.GetVMInfo(ctx, vm)
	testdata.AssertNoError(t, err, "Failed to get VM info")
	t.Logf("  Power state: %s", info.PowerState)

	// Power off
	t.Log("Powering off VM...")
	if err := vcenter.PowerOffVM(ctx, vm); err != nil {
		testFailed = true
		t.Fatalf("Failed to power off: %v", err)
	}
	t.Log("✓ VM powered off")
	time.Sleep(3 * time.Second)
}

// Test05_SnapshotOperations tests snapshot management
func Test05_SnapshotOperations(t *testing.T) {
	testdata.SkipIfNoConfig(t)
	t.Log("\n=== Test 05: Snapshot Operations ===")

	ctx := context.Background()
	client, config := testdata.GetTestClient(t, ctx)
	defer client.Logout(ctx)

	vmName := testdata.GenerateTestVMName(config.TestResources.TestVMPrefix, "snapshot")
	t.Logf("Test VM name: %s", vmName)

	testFailed := false
	defer testdata.CleanupFunc(t, ctx, client, config, vmName, &testFailed)()

	// Clone VM
	t.Log("Creating test VM...")
	vm, err := vcenter.CloneVM(
		ctx,
		client.Client,
		config.TestResources.TemplateName,
		vmName,
		client.GetDatacenterName(),
		config.TestResources.Datastore,
		config.TestResources.ResourcePool,
		config.TestResources.Folder,
	)
	if err != nil {
		testFailed = true
		t.Fatalf("Failed to clone VM: %v", err)
	}
	t.Logf("✓ Created VM: %s", vm.Name())

	snapshotName := "Integration Test Snapshot"

	// Create snapshot
	t.Log("Creating snapshot...")
	err = vcenter.CreateSnapshot(
		ctx,
		vm,
		snapshotName,
		"Created by integration test",
		false, // no memory
		false, // no quiesce
	)
	if err != nil {
		testFailed = true
		t.Fatalf("Failed to create snapshot: %v", err)
	}
	t.Logf("✓ Created snapshot: %s", snapshotName)

	// List snapshots
	t.Log("Listing snapshots...")
	snapshots, err := vcenter.ListSnapshots(ctx, vm)
	if err != nil {
		testFailed = true
		t.Fatalf("Failed to list snapshots: %v", err)
	}
	t.Logf("✓ Found %d snapshot(s)", len(snapshots))
	if len(snapshots) == 0 {
		testFailed = true
		t.Fatal("Expected at least one snapshot")
	}

	// Delete snapshot
	t.Log("Deleting snapshot...")
	if err := vcenter.DeleteSnapshot(ctx, vm, snapshotName, true, true); err != nil {
		testFailed = true
		t.Fatalf("Failed to delete snapshot: %v", err)
	}
	t.Log("✓ Deleted snapshot")

	// Verify deletion
	time.Sleep(2 * time.Second)
	snapshotsAfter, err := vcenter.ListSnapshots(ctx, vm)
	testdata.AssertNoError(t, err, "Failed to list snapshots after deletion")
	testdata.AssertEqual(t, 0, len(snapshotsAfter), "Snapshots not fully deleted")
	t.Log("✓ Verified snapshot deletion")
}

// Test06_DiskOperations tests disk management
func Test06_DiskOperations(t *testing.T) {
	testdata.SkipIfNoConfig(t)
	t.Log("\n=== Test 06: Disk Operations ===")

	ctx := context.Background()
	client, config := testdata.GetTestClient(t, ctx)
	defer client.Logout(ctx)

	vmName := testdata.GenerateTestVMName(config.TestResources.TestVMPrefix, "disk")
	t.Logf("Test VM name: %s", vmName)

	testFailed := false
	defer testdata.CleanupFunc(t, ctx, client, config, vmName, &testFailed)()

	// Clone VM
	t.Log("Creating test VM...")
	vm, err := vcenter.CloneVM(
		ctx,
		client.Client,
		config.TestResources.TemplateName,
		vmName,
		client.GetDatacenterName(),
		config.TestResources.Datastore,
		config.TestResources.ResourcePool,
		config.TestResources.Folder,
	)
	if err != nil {
		testFailed = true
		t.Fatalf("Failed to clone VM: %v", err)
	}
	t.Logf("✓ Created VM: %s", vm.Name())

	// Add disk (uses same datastore as VM's OS disk)
	t.Log("Adding new disk (5 GB, thin)...")
	err = vcenter.AddDisk(ctx, vm, 5, "thin")
	if err != nil {
		testFailed = true
		t.Fatalf("Failed to add disk: %v", err)
	}
	t.Log("✓ Added disk")

	// Note: We skip removal as we'll delete the entire VM anyway
}

// Test07_NetworkOperations tests network adapter management
func Test07_NetworkOperations(t *testing.T) {
	testdata.SkipIfNoConfig(t)
	t.Log("\n=== Test 07: Network Operations ===")

	ctx := context.Background()
	client, config := testdata.GetTestClient(t, ctx)
	defer client.Logout(ctx)

	vmName := testdata.GenerateTestVMName(config.TestResources.TestVMPrefix, "network")
	t.Logf("Test VM name: %s", vmName)

	testFailed := false
	defer testdata.CleanupFunc(t, ctx, client, config, vmName, &testFailed)()

	// Clone VM
	t.Log("Creating test VM...")
	vm, err := vcenter.CloneVM(
		ctx,
		client.Client,
		config.TestResources.TemplateName,
		vmName,
		client.GetDatacenterName(),
		config.TestResources.Datastore,
		config.TestResources.ResourcePool,
		config.TestResources.Folder,
	)
	if err != nil {
		testFailed = true
		t.Fatalf("Failed to clone VM: %v", err)
	}
	t.Logf("✓ Created VM: %s", vm.Name())

	// Get initial network info
	info, err := vcenter.GetVMInfo(ctx, vm)
	testdata.AssertNoError(t, err, "Failed to get VM info")
	initialNICCount := len(info.Networks)
	t.Logf("✓ Found %d network adapter(s)", initialNICCount)

	// Add network adapter
	t.Log("Adding network adapter...")
	err = vcenter.AddNetworkAdapter(ctx, vm, config.TestResources.Network)
	if err != nil {
		testFailed = true
		t.Fatalf("Failed to add network adapter: %v", err)
	}
	t.Log("✓ Added network adapter")

	// Verify adapter was added
	time.Sleep(2 * time.Second)
	infoAfter, err := vcenter.GetVMInfo(ctx, vm)
	testdata.AssertNoError(t, err, "Failed to get VM info after adding adapter")
	if len(infoAfter.Networks) != initialNICCount+1 {
		testFailed = true
		t.Fatalf("Expected %d adapters, got %d", initialNICCount+1, len(infoAfter.Networks))
	}
	t.Logf("✓ Verified adapter addition (%d -> %d adapters)", initialNICCount, len(infoAfter.Networks))
}

// // Test08_BatchOperations tests parallel VM cloning
// func Test08_BatchOperations(t *testing.T) {
// 	testdata.SkipIfNoConfig(t)
// 	t.Log("\n=== Test 08: Batch Operations ===")

// 	ctx := context.Background()
// 	client, config := testdata.GetTestClient(t, ctx)
// 	defer client.Logout(ctx)

// 	// Create ServerRequest array for batch cloning
// 	requests := []vcenter.ServerRequest{
// 		{
// 			Name:     testdata.GenerateTestVMName(config.TestResources.TestVMPrefix, "batch1"),
// 			Template: config.TestResources.TemplateName,
// 			CPUs:     2,
// 			MemoryGB: 4,
// 		},
// 		{
// 			Name:     testdata.GenerateTestVMName(config.TestResources.TestVMPrefix, "batch2"),
// 			Template: config.TestResources.TemplateName,
// 			CPUs:     2,
// 			MemoryGB: 4,
// 		},
// 		{
// 			Name:     testdata.GenerateTestVMName(config.TestResources.TestVMPrefix, "batch3"),
// 			Template: config.TestResources.TemplateName,
// 			CPUs:     2,
// 			MemoryGB: 4,
// 		},
// 	}

// 	t.Logf("Test VM names: %v", []string{requests[0].Name, requests[1].Name, requests[2].Name})

// 	defer func() {
// 		// Cleanup all VMs
// 		for _, req := range requests {
// 			if config.TestOptions.AutoCleanup {
// 				testdata.CleanupTestVM(t, ctx, client, req.Name)
// 			} else {
// 				t.Logf("  ⚠ Skipping cleanup (auto_cleanup=false), VM remains: %s", req.Name)
// 			}
// 		}
// 	}()

// 	// Clone multiple VMs in parallel
// 	t.Logf("Cloning %d VMs in parallel...", len(requests))
// 	vms, errors := vcenter.CloneMultiple(
// 		ctx,
// 		client.Client,
// 		requests,
// 		client.GetDatacenterName(),
// 		config.TestResources.Datastore,
// 		config.TestResources.ResourcePool,
// 		config.TestResources.Folder,
// 		"", "", "", // No domain credentials
// 		85, // Timezone (W. Europe Standard Time)
// 	)

// 	// Check results
// 	successCount := 0
// 	for i, vm := range vms {
// 		if errors[i] != nil {
// 			t.Logf("  ✗ %s -> Error: %v", requests[i].Name, errors[i])
// 		} else if vm != nil {
// 			t.Logf("  ✓ %s -> %s", requests[i].Name, vm.Name())
// 			successCount++
// 		}
// 	}

// 	if successCount == 0 {
// 		t.Fatal("No VMs were cloned successfully")
// 	}

// 	t.Logf("✓ Successfully cloned %d/%d VMs", successCount, len(requests))
// }

// Test09_CompleteLifecycle tests a complete VM lifecycle
func Test09_CompleteLifecycle(t *testing.T) {
	testdata.SkipIfNoConfig(t)
	t.Log("\n=== Test 09: Complete VM Lifecycle ===")

	ctx := context.Background()
	client, config := testdata.GetTestClient(t, ctx)
	defer client.Logout(ctx)

	vmName := testdata.GenerateTestVMName(config.TestResources.TestVMPrefix, "lifecycle")
	t.Logf("Test VM name: %s", vmName)

	snapshotName := "Lifecycle Test Snapshot"

	// 1. Clone
	t.Log("1. Cloning VM...")
	vm, err := vcenter.CloneVM(
		ctx,
		client.Client,
		config.TestResources.TemplateName,
		vmName,
		client.GetDatacenterName(),
		config.TestResources.Datastore,
		config.TestResources.ResourcePool,
		config.TestResources.Folder,
	)
	testdata.AssertNoError(t, err, "Failed to clone VM")
	t.Logf("  ✓ Cloned: %s", vm.Name())

	// 2. Create snapshot
	t.Log("2. Creating snapshot...")
	err = vcenter.CreateSnapshot(ctx, vm, snapshotName, "Pre-boot snapshot", false, false)
	testdata.AssertNoError(t, err, "Failed to create snapshot")
	t.Logf("  ✓ Snapshot created: %s", snapshotName)

	// 3. Power on
	t.Log("3. Powering on...")
	testdata.AssertNoError(t, vcenter.PowerOnVM(ctx, vm), "Failed to power on")
	t.Log("  ✓ Powered on")
	time.Sleep(3 * time.Second)

	// 4. Power off
	t.Log("4. Powering off...")
	testdata.AssertNoError(t, vcenter.PowerOffVM(ctx, vm), "Failed to power off")
	t.Log("  ✓ Powered off")
	time.Sleep(2 * time.Second)

	// 5. Delete snapshot
	t.Log("5. Deleting snapshot...")
	testdata.AssertNoError(t, vcenter.DeleteSnapshot(ctx, vm, snapshotName, true, true), "Failed to delete snapshot")
	t.Log("  ✓ Snapshot deleted")

	// 6. Delete VM
	t.Log("6. Deleting VM...")
	testdata.AssertNoError(t, vcenter.DeleteVM(ctx, vm, true, false), "Failed to delete VM")
	t.Log("  ✓ VM deleted")

	t.Log("\n✓ Complete lifecycle test passed")
}
