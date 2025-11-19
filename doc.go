// Package vcenter provides a user-friendly Go API for VMware vCenter,
// inspired by PowerCLI. The package wraps govmomi and offers:
//
//   - Easy authentication with both username/password and Windows SSPI
//   - Session caching for better performance
//   - PowerCLI-like functions for VM management
//   - Batch operations for parallel VM management
//   - Disk and network management
//   - Windows VM customization with domain join
//
// # Installation
//
//	go get github.com/skabbio1976/vcenter
//
// # Example - Connect with SSPI (Windows)
//
//	ctx := context.Background()
//	client, err := vcenter.ConnectWithSSPI(ctx, "vcenter.example.com", true, "Datacenter1")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Logout(ctx)
//
// # Example - Connect with username/password
//
//	config := vcenter.ConnectConfig{
//	    Host:       "vcenter.example.com",
//	    Username:   "administrator@vsphere.local",
//	    Password:   "password",
//	    Insecure:   true,
//	    Datacenter: "Datacenter1",
//	}
//	client, err := vcenter.ConnectWithPassword(ctx, config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Logout(ctx)
//
// # Example - Clone a VM
//
//	vm, err := vcenter.CloneVM(
//	    ctx,
//	    client.Client,
//	    "Windows-Template",
//	    "NewVM",
//	    "Datacenter1",
//	    "datastore1",
//	    "Resources",
//	    "",
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// # Example - Clone with Windows customization
//
//	customization := vcenter.NewWindowsCustomization(
//	    "WebServer01",
//	    "example.com",
//	    "administrator@example.com",
//	    "domainpass",
//	    "adminpass",
//	    85, // W. Europe Standard Time
//	    []string{"192.168.1.1"},
//	    []string{"example.com"},
//	)
//
//	vm, err := vcenter.CloneVMWithCustomization(
//	    ctx, client.Client, "Windows-Template", "WebServer01",
//	    "Datacenter1", "datastore1", "Resources", "",
//	    customization,
//	)
package vcenter
