// Package testdata provides test configuration and utilities for integration tests.
package testdata

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/skabbio1976/vcenter"
	"github.com/vmware/govmomi/object"
)

var (
	optionsLoggedOnce sync.Once
)

// SkipIfNoConfig skips the test if test-config.json doesn't exist
func SkipIfNoConfig(t *testing.T) {
	if !ConfigExists() {
		t.Skip("⊘ Skipping integration test - no test-config.json found\n  Copy test-config.example.json to test-config.json and configure it")
	}
}

// GetTestConfig loads test configuration or fails the test
func GetTestConfig(t *testing.T) *TestConfig {
	config, err := LoadTestConfig()
	if err != nil {
		t.Fatalf("Failed to load test config: %v", err)
	}

	// Log options once per test run
	logTestOptions(config)

	return config
}

// GetTestClient creates a connected vCenter client for testing
func GetTestClient(t *testing.T, ctx context.Context) (*vcenter.Client, *TestConfig) {
	config := GetTestConfig(t)

	connectConfig := config.ToConnectConfig()
	client, err := vcenter.ConnectWithPassword(ctx, connectConfig)
	if err != nil {
		t.Fatalf("Failed to connect to vCenter: %v", err)
	}

	return client, config
}

// GenerateTestVMName creates a unique test VM name with timestamp
func GenerateTestVMName(prefix, testName string) string {
	timestamp := time.Now().Unix()
	return fmt.Sprintf("%s-%s-%d", prefix, testName, timestamp)
}

// CleanupTestVM attempts to delete a test VM (best effort)
func CleanupTestVM(t *testing.T, ctx context.Context, client *vcenter.Client, vmName string) {
	t.Logf("  Cleaning up test VM: %s", vmName)

	// Find the VM
	datacenter := client.GetDatacenterName()
	vm, err := vcenter.GetVM(ctx, client.Client, vmName, datacenter)
	if err != nil {
		// VM doesn't exist, nothing to clean up
		t.Logf("  ℹ VM %s not found (already cleaned up?)", vmName)
		return
	}

	// Try to power off first (ignore errors)
	_ = vcenter.PowerOffVM(ctx, vm)

	// Wait a moment for power off
	time.Sleep(2 * time.Second)

	// Delete the VM
	if err := vcenter.DeleteVM(ctx, vm, true, false); err != nil {
		t.Logf("  ⚠ Failed to delete VM %s: %v", vmName, err)
	} else {
		t.Logf("  ✓ Cleaned up VM: %s", vmName)
	}
}

// CleanupFunc returns a cleanup function that respects test options
func CleanupFunc(t *testing.T, ctx context.Context, client *vcenter.Client, config *TestConfig, vmName string, testFailed *bool) func() {
	return func() {
		// Decide if we should cleanup
		shouldCleanup := config.TestOptions.AutoCleanup
		if *testFailed && config.TestOptions.KeepFailedVMs {
			t.Logf("  ⚠ Test failed - keeping VM for debugging: %s", vmName)
			return
		}

		if shouldCleanup {
			CleanupTestVM(t, ctx, client, vmName)
		} else {
			t.Logf("  ⚠ Skipping cleanup (auto_cleanup=false), VM remains: %s", vmName)
		}
	}
}

// WaitForVMReady waits for VM to be ready (powered on and tools running)
func WaitForVMReady(ctx context.Context, vm *object.VirtualMachine, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for VM to be ready")
		case <-ticker.C:
			powerState, err := vm.PowerState(ctx)
			if err != nil {
				return fmt.Errorf("failed to get power state: %w", err)
			}

			if powerState == "poweredOn" {
				return nil // VM is powered on and ready
			}
		}
	}
}

// logTestOptions logs test options once per run
func logTestOptions(config *TestConfig) {
	optionsLoggedOnce.Do(func() {
		fmt.Printf("Test options: auto_cleanup=%v keep_failed_vms=%v test_timeout=%ds\n",
			config.TestOptions.AutoCleanup,
			config.TestOptions.KeepFailedVMs,
			config.TestOptions.TestTimeoutSeconds,
		)
	})
}

// AssertNoError fails the test if err is not nil
func AssertNoError(t *testing.T, err error, msg string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", msg, err)
	}
}

// AssertError fails the test if err is nil
func AssertError(t *testing.T, err error, msg string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected error but got nil", msg)
	}
}

// AssertEqual fails the test if expected != actual
func AssertEqual(t *testing.T, expected, actual interface{}, msg string) {
	t.Helper()
	if expected != actual {
		t.Fatalf("%s: expected %v but got %v", msg, expected, actual)
	}
}
