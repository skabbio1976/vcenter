package main

import (
	"context"
	"log"
	"time"

	"github.com/skabbio1976/vcenter"
)

// This example demonstrates guest operations: file/directory transfer and script execution.
// Requires VMware Tools running on the guest.
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

	// Get the VM
	vm, err := vcenter.GetVM(ctx, client.Client, "WebServer01", "Datacenter1")
	if err != nil {
		log.Fatal(err)
	}

	// Guest credentials
	guestUser := "Administrator"
	guestPass := "GuestPassword123!"
	isWindows := true

	// =========================================================================
	// File Operations
	// =========================================================================

	// Upload a single file
	log.Println("Uploading configuration file...")
	err = vcenter.UploadFileToVM(ctx, vm, guestUser, guestPass,
		"configs/app.config",      // local path
		"C:\\App\\app.config",     // remote path
		true,                      // overwrite
	)
	if err != nil {
		log.Printf("Upload failed: %v", err)
	}

	// Download a single file
	log.Println("Downloading log file...")
	err = vcenter.DownloadFileFromVM(ctx, vm, guestUser, guestPass,
		"C:\\App\\logs\\app.log",  // remote path
		"logs/app.log",            // local path
	)
	if err != nil {
		log.Printf("Download failed: %v", err)
	}

	// =========================================================================
	// Directory Operations (much faster than file-by-file!)
	// =========================================================================

	// Upload entire directory
	log.Println("Uploading config directory...")
	err = vcenter.UploadDirectoryToVM(ctx, vm, guestUser, guestPass,
		"configs/",                // local directory
		"C:\\App\\configs",        // remote directory
		isWindows,
	)
	if err != nil {
		log.Printf("Directory upload failed: %v", err)
	}

	// Download entire directory
	log.Println("Downloading logs directory...")
	err = vcenter.DownloadDirectoryFromVM(ctx, vm, guestUser, guestPass,
		"C:\\App\\logs",           // remote directory
		"collected-logs/",         // local directory
		isWindows,
	)
	if err != nil {
		log.Printf("Directory download failed: %v", err)
	}

	// =========================================================================
	// Script Execution
	// =========================================================================

	// Run a command without waiting
	log.Println("Starting background process...")
	pid, err := vcenter.RunScriptOnVM(ctx, vm, guestUser, guestPass,
		"C:\\Windows\\System32\\cmd.exe",
		[]string{"/c", "echo", "Hello from VM"},
		"",    // working directory
		false, // don't wait
	)
	if err != nil {
		log.Printf("Script failed: %v", err)
	} else {
		log.Printf("Started process with PID: %d", pid)
	}

	// Run PowerShell script and wait for completion
	log.Println("Running PowerShell script...")
	pid, err = vcenter.RunScriptOnVM(ctx, vm, guestUser, guestPass,
		"C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe",
		[]string{"-ExecutionPolicy", "Bypass", "-Command", "Get-Process | Out-File C:\\temp\\processes.txt"},
		"C:\\temp",
		true, // wait for completion
	)
	if err != nil {
		log.Printf("PowerShell script failed: %v", err)
	} else {
		log.Printf("Script completed (PID: %d)", pid)
	}

	// Upload and run script in one operation
	log.Println("Uploading and running setup script...")
	pid, err = vcenter.UploadAndRunScript(ctx, vm, guestUser, guestPass,
		"scripts/setup.ps1",       // local script
		"C:\\temp\\setup.ps1",     // remote destination
		"C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe",
		[]string{"-ExecutionPolicy", "Bypass", "-File", "C:\\temp\\setup.ps1"},
		true, // wait for completion
	)
	if err != nil {
		log.Printf("Setup script failed: %v", err)
	} else {
		log.Printf("Setup completed (PID: %d)", pid)
	}

	// =========================================================================
	// Complete Post-Deployment Workflow
	// =========================================================================

	log.Println("\n=== Full Post-Deployment Example ===")

	// 1. Wait for customization to complete first
	log.Println("Waiting for customization...")
	err = vcenter.WaitForCustomization(ctx, vm, 15*time.Minute, vcenter.CustomizationExpected{
		Hostname: "TestServer01",
		IP:       "dhcp",
	})
	if err != nil {
		log.Printf("Warning: %v", err)
	}

	// 2. Upload application files
	log.Println("Uploading application...")
	err = vcenter.UploadDirectoryToVM(ctx, vm, guestUser, guestPass,
		"app-package/", "C:\\App", isWindows)
	if err != nil {
		log.Fatalf("Failed to upload app: %v", err)
	}

	// 3. Run installation script
	log.Println("Running installer...")
	_, err = vcenter.UploadAndRunScript(ctx, vm, guestUser, guestPass,
		"scripts/install.ps1", "C:\\temp\\install.ps1",
		"powershell.exe",
		[]string{"-ExecutionPolicy", "Bypass", "-File", "C:\\temp\\install.ps1"},
		true,
	)
	if err != nil {
		log.Fatalf("Installation failed: %v", err)
	}

	// 4. Download installation logs
	log.Println("Collecting logs...")
	err = vcenter.DownloadDirectoryFromVM(ctx, vm, guestUser, guestPass,
		"C:\\temp\\logs", "deployment-logs/", isWindows)
	if err != nil {
		log.Printf("Warning: Could not collect logs: %v", err)
	}

	log.Println("Done!")
}
