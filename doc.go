// Package vcenter tillhandahåller ett användarvänligt Go-API för VMware vCenter,
// inspirerat av PowerCLI. Paketet wrapprar govmomi och erbjuder:
//
//   - Enkel autentisering med både username/password och Windows SSPI
//   - Session caching för bättre prestanda
//   - PowerCLI-liknande funktioner för VM-hantering
//   - Batch-operationer för parallell VM-hantering
//   - Disk och nätverkshantering
//   - Windows VM customization med domain join
//
// # Installation
//
//	go get github.com/skabbio1976/vcenter
//
// # Exempel - Anslut med SSPI (Windows)
//
//	ctx := context.Background()
//	client, err := vcenter.ConnectWithSSPI(ctx, "vcenter.example.com", true, "Datacenter1")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Logout(ctx)
//
// # Exempel - Anslut med username/password
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
// # Exempel - Klona en VM
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
// # Exempel - Klona med Windows customization
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
