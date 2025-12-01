package vcenter

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

const (
	vmRequestSheet      = "VM Requests"
	validValuesSheet    = "Valid Values"
	instructionSheetENG = "Instructions-EN"
	instructionSheetSWE = "Instructions-SWE"
)

var requiredColumns = []string{"vm_name", "template", "port_group_1", "num_cpus", "memory_mb"}

// ExcelValidValues defines dropdown entries for the template.
type ExcelValidValues struct {
	Templates        []string
	Datacenters      []string
	ComputeClusters  []string
	StorageClusters  []string
	Datastores       []string
	PortGroups       []string
	SubnetMasks      []string
	Domains          []string
	Timezones        []string
	CPUOptions       []int
	MemoryOptions    []int
	DiskOptions      []int
	ServerRoles      []string
	DiskProvisioning []string
}

// DefaultExcelValidValues mirrors the defaults from py-vcenter.
func DefaultExcelValidValues() ExcelValidValues {
	return ExcelValidValues{
		Templates:        []string{"Windows2022-Template", "Windows2019-Template", "Ubuntu2204-Template", "RHEL9-Template"},
		Datacenters:      []string{"DC-Stockholm", "DC-Göteborg", "DC-Malmö"},
		ComputeClusters:  []string{"Prod-Cluster", "Dev-Cluster", "Test-Cluster"},
		StorageClusters:  []string{"VSAN-Cluster-01", "VSAN-Cluster-02", "NFS-Cluster-01", "Backup-Cluster-01"},
		Datastores:       []string{"VSAN-Datastore-01", "VSAN-Datastore-02", "NFS-Backup-01"},
		PortGroups:       []string{"VLAN-100-Prod", "VLAN-200-Dev", "VLAN-300-Test", "VLAN-400-Mgmt"},
		SubnetMasks:      []string{"255.255.254.0", "255.255.255.0", "255.255.255.128", "255.255.255.192", "255.255.255.224", "255.255.255.240"},
		Domains:          []string{"corp.example.com", "dev.example.com", "test.example.com"},
		Timezones:        []string{"W. Europe Standard Time", "Central European Standard Time", "UTC", "GMT Standard Time"},
		CPUOptions:       []int{2, 4, 8, 16},
		MemoryOptions:    []int{4096, 8192, 16384, 32768, 65536},
		DiskOptions:      []int{50, 100, 200, 500, 1000},
		ServerRoles:      []string{"DC", "CA", "CRL", "Member", "T0-RDS", "T1-RDS", "File", "SQL", "Web", "Standalone"},
		DiskProvisioning: []string{"thin", "thick"},
	}
}

// CreateExcelTemplate writes an Excel workbook replicating the Python helper.
func CreateExcelTemplate(path string, values *ExcelValidValues) error {
	if values == nil {
		defaults := DefaultExcelValidValues()
		values = &defaults
	}

	f := excelize.NewFile()
	defer f.Close()

	if err := f.SetSheetName("Sheet1", vmRequestSheet); err != nil {
		return err
	}

	headers := []string{
		"vm_name", "template", "datacenter", "compute_cluster", "storage_cluster", "datastore", "folder",
		"port_group_1", "ip_1", "subnet_mask_1", "gateway_1", "dns_servers_1",
		"port_group_2", "ip_2", "subnet_mask_2", "gateway_2", "dns_servers_2",
		"port_group_3", "ip_3", "subnet_mask_3", "gateway_3", "dns_servers_3",
		"num_cpus", "memory_mb", "disk_gb", "disk_provisioning", "server_role",
		"hostname", "domain", "domain_join_user", "ou_path",
		"autologon_count", "timezone", "run_once_commands",
	}

	for colIdx, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
		f.SetCellValue(vmRequestSheet, cell, header)
		style, _ := f.NewStyle(&excelize.Style{
			Font:      &excelize.Font{Bold: true, Color: "#FFFFFF"},
			Fill:      excelize.Fill{Type: "pattern", Color: []string{"#4472C4"}, Pattern: 1},
			Alignment: &excelize.Alignment{Horizontal: "center"},
		})
		f.SetCellStyle(vmRequestSheet, cell, cell, style)
	}

	columnWidths := map[string]float64{
		"A": 18, "B": 25, "C": 15, "D": 18, "E": 18, "F": 20, "G": 25,
		"H": 18, "I": 15, "J": 15, "K": 15, "L": 25,
		"M": 18, "N": 15, "O": 15, "P": 15, "Q": 25,
		"R": 18, "S": 15, "T": 15, "U": 15, "V": 25,
		"W": 10, "X": 12, "Y": 15, "Z": 15, "AA": 12,
		"AB": 18, "AC": 20, "AD": 18, "AE": 50,
		"AF": 15, "AG": 25, "AH": 50,
	}
	for col, width := range columnWidths {
		f.SetColWidth(vmRequestSheet, col, col, width)
	}

	for rowIdx, row := range exampleRows() {
		for colIdx, value := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			f.SetCellValue(vmRequestSheet, cell, value)
		}
	}

	if _, err := f.NewSheet(validValuesSheet); err != nil {
		return err
	}

	writeColumn := func(col string, header string, list []string) {
		head := fmt.Sprintf("%s1", col)
		f.SetCellValue(validValuesSheet, head, header)
		style, _ := f.NewStyle(&excelize.Style{
			Font: &excelize.Font{Bold: true, Color: "#FFFFFF"},
			Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#4472C4"}},
		})
		f.SetCellStyle(validValuesSheet, head, head, style)
		for i, val := range list {
			f.SetCellValue(validValuesSheet, fmt.Sprintf("%s%d", col, i+2), val)
		}
	}

	intsToStrings := func(values []int) []string {
		res := make([]string, len(values))
		for i, v := range values {
			res[i] = strconv.Itoa(v)
		}
		return res
	}

	writeColumn("A", "Templates", values.Templates)
	writeColumn("B", "Datacenters", values.Datacenters)
	writeColumn("C", "Compute Clusters", values.ComputeClusters)
	writeColumn("D", "Storage Clusters", values.StorageClusters)
	writeColumn("E", "Datastores", values.Datastores)
	writeColumn("F", "Port Groups", values.PortGroups)
	writeColumn("G", "Domains", values.Domains)
	writeColumn("H", "Timezones", values.Timezones)
	writeColumn("I", "CPU Options", intsToStrings(values.CPUOptions))
	writeColumn("J", "Memory Options (MB)", intsToStrings(values.MemoryOptions))
	writeColumn("K", "Disk Options (GB)", intsToStrings(values.DiskOptions))
	writeColumn("L", "Server Roles", values.ServerRoles)
	writeColumn("M", "Disk Provisioning", values.DiskProvisioning)
	writeColumn("N", "Subnet Masks", values.SubnetMasks)

	// addValidationRef creates a dropdown that references the Valid Values sheet
	// Users can edit the Valid Values sheet to add more options
	addValidationRef := func(col string, validValuesCol string, maxRows int) {
		dvRange := fmt.Sprintf("%s2:%s200", col, col)
		dv := excelize.NewDataValidation(true)
		dv.Sqref = dvRange
		// Reference the Valid Values sheet - use dynamic range up to maxRows
		formula := fmt.Sprintf("'%s'!$%s$2:$%s$%d", validValuesSheet, validValuesCol, validValuesCol, maxRows+1)
		dv.SetSqrefDropList(formula)
		_ = f.AddDataValidation(vmRequestSheet, dv)
	}

	// Map validation columns in VM Requests to columns in Valid Values sheet
	// Using 100 rows allows users to add more values in Valid Values sheet
	addValidationRef("B", "A", 100)  // template -> Templates
	addValidationRef("C", "B", 100)  // datacenter -> Datacenters
	addValidationRef("D", "C", 100)  // compute_cluster -> Compute Clusters
	addValidationRef("E", "D", 100)  // storage_cluster -> Storage Clusters
	addValidationRef("F", "E", 100)  // datastore -> Datastores
	addValidationRef("H", "F", 100)  // port_group_1 -> Port Groups
	addValidationRef("M", "F", 100)  // port_group_2 -> Port Groups
	addValidationRef("R", "F", 100)  // port_group_3 -> Port Groups
	addValidationRef("J", "N", 100)  // subnet_mask_1 -> Subnet Masks
	addValidationRef("O", "N", 100)  // subnet_mask_2 -> Subnet Masks
	addValidationRef("T", "N", 100)  // subnet_mask_3 -> Subnet Masks
	addValidationRef("W", "I", 100)  // num_cpus -> CPU Options
	addValidationRef("X", "J", 100)  // memory_mb -> Memory Options
	addValidationRef("Z", "M", 100)  // disk_provisioning -> Disk Provisioning
	addValidationRef("AA", "L", 100) // server_role -> Server Roles
	addValidationRef("AC", "G", 100) // domain -> Domains
	addValidationRef("AG", "H", 100) // timezone -> Timezones

	instructionWidths := map[string]float64{
		"A": 32,
		"B": 75,
		"C": 22,
		"D": 18,
	}

	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:  true,
			Size:  16,
			Color: "#FFFFFF",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Pattern: 1,
			Color:   []string{"#2F5597"},
		},
		Alignment: &excelize.Alignment{
			Horizontal: "left",
			Vertical:   "center",
		},
	})

	subHeaderStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:  true,
			Size:  14,
			Color: "#FFFFFF",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Pattern: 1,
			Color:   []string{"#3C78D8"},
		},
		Alignment: &excelize.Alignment{
			Horizontal: "left",
			Vertical:   "center",
		},
	})

	tableHeaderStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:  true,
			Color: "#FFFFFF",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Pattern: 1,
			Color:   []string{"#1F4E78"},
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	})

	tableBodyStyle, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "left",
			Vertical:   "center",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Pattern: 1,
			Color:   []string{"#EEF2FA"},
		},
	})

	bulletStyle, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "left",
			Vertical:   "center",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Pattern: 1,
			Color:   []string{"#FFFFFF"},
		},
	})

	subHeaderTitles := map[string]struct{}{
		"Så här använder du mallen:":             {},
		"CIDR-tabell för subnet_mask-fälten":     {},
		"How to use the template:":               {},
		"CIDR reference for subnet_mask columns": {},
	}

	instructionsSwe := [][]string{
		{"Instruktioner (SV)"},
		{""},
		{"Så här använder du mallen:"},
		{"1. Fyll i en rad per VM på fliken 'VM Requests'."},
		{"2. Utgå från exempelraderna 2-9 och ersätt med dina värden."},
		{"3. Använd dropdown-menyerna för kolumner med fördefinierade val."},
		{"4. Lista flera värden med semikolon, t.ex. dns_servers '10.0.0.10;10.0.0.11'."},
		{"5. Lämna IP-fält tomma eller skriv 'DHCP' för dynamisk adressering."},
		{""},
		{"Kolumn", "Beskrivning"},
		{"vm_name", "Unikt namn på VM:n. Måste vara unikt per arbetsbok."},
		{"template", "VMware-mall från listan 'Templates' på fliken 'Valid Values'."},
		{"datacenter", "Datacenter i vCenter. Lämna tomt om global konfiguration används."},
		{"compute_cluster", "Compute-kluster där VM:n ska köras."},
		{"storage_cluster", "Storage-kluster att använda (valfritt om datastore anges direkt)."},
		{"datastore", "Specifik datastore för VM:n. Lämna blankt för att låta policyn välja."},
		{"folder", "Målmapp i inventariet, t.ex. 'Production/Servers'."},
		{"port_group_1", "Obligatorisk port group för NIC1. Måste finnas i listan 'Port Groups'."},
		{"ip_1", "IP-adress för NIC1 eller 'DHCP' för dynamisk konfiguration."},
		{"subnet_mask_1", "Subnet mask för NIC1 vid statisk IP (se tabellen nedan)."},
		{"gateway_1", "Gateway för NIC1 vid statisk IP."},
		{"dns_servers_1", "Semikolonseparerad lista med DNS-servrar för NIC1 (t.ex. '10.0.0.10;10.0.0.11')."},
		{"port_group_2", "Valfritt: port group för andra nätverkskortet."},
		{"ip_2", "IP-adress eller 'DHCP' för NIC2. Lämna tomt om adaptern saknar IP."},
		{"subnet_mask_2", "Subnet mask för NIC2 vid statisk IP."},
		{"gateway_2", "Lämnas normalt tom. Extra gateways kan orsaka routingproblem."},
		{"dns_servers_2", "Semikolonseparerad lista med DNS-servrar för NIC2."},
		{"port_group_3", "Valfritt: port group för tredje nätverkskortet."},
		{"ip_3", "IP-adress eller 'DHCP' för NIC3."},
		{"subnet_mask_3", "Subnet mask för NIC3 vid statisk IP."},
		{"gateway_3", "Lämnas normalt tom."},
		{"dns_servers_3", "Semikolonseparerad lista med DNS-servrar för NIC3."},
		{"num_cpus", "Antal vCPU:er. Värdet måste finnas i listan 'CPU Options'."},
		{"memory_mb", "RAM i MB. Välj från listan 'Memory Options'."},
		{"disk_gb", "Semikolonseparerad lista över extra diskar i GB, t.ex. '50;100'. Lämna tomt för inga extra diskar."},
		{"disk_provisioning", "Välj 'thin' eller 'thick'."},
		{"server_role", "Serverroll från listan 'Server Roles'."},
		{"hostname", "Hostname som sätts i gäst-OS."},
		{"domain", "AD-domän som VM:n ska gå med i. Krävs om domänjoin används."},
		{"domain_join_user", "Konto med rättigheter att gå med i domänen (valfritt vid manuell join)."},
		{"ou_path", "LDAP-sökväg, t.ex. 'OU=Servers,DC=corp,DC=example,DC=com'."},
		{"autologon_count", "Antal gånger administratörskontot ska logga in automatiskt."},
		{"timezone", "Tidszon enligt listan 'Timezones'."},
		{"run_once_commands", "Semikolonseparerade kommandon som körs efter första inloggningen."},
		{""},
		{"CIDR-tabell för subnet_mask-fälten:"},
		{"CIDR", "Nätmask", "Totalt antal IP-adresser", "Max värdar"},
		{"/23", "255.255.254.0", "512", "510"},
		{"/24", "255.255.255.0", "256", "254"},
		{"/25", "255.255.255.128", "128", "126"},
		{"/26", "255.255.255.192", "64", "62"},
		{"/27", "255.255.255.224", "32", "30"},
		{"/28", "255.255.255.240", "16", "14"},
		{""},
		{"Se README.md för fullständig dokumentation."},
	}

	instructionsEng := [][]string{
		{"Instructions (EN)"},
		{""},
		{"How to use the template:"},
		{"1. Enter one row per VM on the 'VM Requests' sheet."},
		{"2. Start from the sample rows (2-9) and overwrite them with your values."},
		{"3. Use the dropdowns wherever predefined options exist."},
		{"4. Separate multiple values with semicolons, e.g. dns_servers '10.0.0.10;10.0.0.11'."},
		{"5. Leave IP fields blank or type 'DHCP' for dynamic addressing."},
		{""},
		{"Column", "Description"},
		{"vm_name", "Unique name for the VM. Must be unique within this workbook."},
		{"template", "VMware template from the 'Templates' list on the 'Valid Values' sheet."},
		{"datacenter", "vCenter datacenter for the VM. Leave blank if provided by global configuration."},
		{"compute_cluster", "Compute cluster where the VM will run."},
		{"storage_cluster", "Storage cluster to use (optional when a datastore is specified)."},
		{"datastore", "Specific datastore for the VM. Leave blank to let policy pick one."},
		{"folder", "Target inventory folder, e.g. 'Production/Servers'."},
		{"port_group_1", "Required port group for NIC1. Must exist in the 'Port Groups' list."},
		{"ip_1", "Static IP for NIC1 or 'DHCP' for dynamic configuration."},
		{"subnet_mask_1", "Subnet mask for NIC1 when using a static IP (see table below)."},
		{"gateway_1", "Default gateway for NIC1 when a static IP is set."},
		{"dns_servers_1", "Semicolon-separated DNS servers for NIC1 (e.g. '10.0.0.10;10.0.0.11')."},
		{"port_group_2", "Optional port group for the second NIC."},
		{"ip_2", "Static IP or 'DHCP' for NIC2. Leave blank if the adapter has no IP."},
		{"subnet_mask_2", "Subnet mask for NIC2 when using a static IP."},
		{"gateway_2", "Should normally be left blank; additional gateways can cause routing issues."},
		{"dns_servers_2", "Semicolon-separated DNS servers for NIC2."},
		{"port_group_3", "Optional port group for the third NIC."},
		{"ip_3", "Static IP or 'DHCP' for NIC3."},
		{"subnet_mask_3", "Subnet mask for NIC3 when using a static IP."},
		{"gateway_3", "Should normally be left blank."},
		{"dns_servers_3", "Semicolon-separated DNS servers for NIC3."},
		{"num_cpus", "Number of vCPUs. Must match a value in the 'CPU Options' list."},
		{"memory_mb", "RAM in MB. Choose a value from the 'Memory Options' list."},
		{"disk_gb", "Semicolon-separated list of additional disks in GB, e.g. '50;100'. Leave blank for none."},
		{"disk_provisioning", "Choose 'thin' or 'thick'."},
		{"server_role", "Server role from the 'Server Roles' list."},
		{"hostname", "Hostname assigned inside the guest OS."},
		{"domain", "Active Directory domain to join. Required when performing a domain join."},
		{"domain_join_user", "Account with permissions to join the domain (optional for manual join)."},
		{"ou_path", "LDAP path, e.g. 'OU=Servers,DC=corp,DC=example,DC=com'."},
		{"autologon_count", "Number of times the Administrator account should log on automatically."},
		{"timezone", "Time zone value from the 'Timezones' list."},
		{"run_once_commands", "Semicolon-separated commands that run after the first logon."},
		{""},
		{"CIDR reference for subnet_mask columns:"},
		{"CIDR", "Netmask", "Total IP addresses", "Max hosts"},
		{"/23", "255.255.254.0", "512", "510"},
		{"/24", "255.255.255.0", "256", "254"},
		{"/25", "255.255.255.128", "128", "126"},
		{"/26", "255.255.255.192", "64", "62"},
		{"/27", "255.255.255.224", "32", "30"},
		{"/28", "255.255.255.240", "16", "14"},
		{""},
		{"See README.md for full documentation."},
	}

	writeInstructionSheet := func(sheet string, data [][]string) error {
		if _, err := f.NewSheet(sheet); err != nil {
			return err
		}
		inTable := false
		for rowIdx, row := range data {
			for colIdx, value := range row {
				cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+1)
				f.SetCellValue(sheet, cell, value)
			}

			style := bulletStyle
			switch {
			case rowIdx == 0:
				style = headerStyle
				if err := f.SetRowHeight(sheet, rowIdx+1, 28); err != nil {
					return err
				}
			case len(row) == 1:
				trimmed := strings.TrimSpace(row[0])
				if trimmed != "" {
					if _, ok := subHeaderTitles[trimmed]; ok {
						style = subHeaderStyle
						if err := f.SetRowHeight(sheet, rowIdx+1, 22); err != nil {
							return err
						}
					} else {
						style = bulletStyle
					}
				}
			case len(row) > 1:
				if !inTable {
					style = tableHeaderStyle
					inTable = true
					if err := f.SetRowHeight(sheet, rowIdx+1, 20); err != nil {
						return err
					}
				} else {
					style = tableBodyStyle
				}
			default:
				style = bulletStyle
			}

			startCol := 1
			endCol := len(row)
			if endCol == 0 {
				endCol = 1
			}
			switch {
			case style == headerStyle || style == subHeaderStyle:
				if endCol < 4 {
					endCol = 4
				}
			case style == tableHeaderStyle || style == tableBodyStyle:
				if endCol < 4 {
					endCol = 4
				}
			}
			startCell, _ := excelize.CoordinatesToCellName(startCol, rowIdx+1)
			endCell, _ := excelize.CoordinatesToCellName(endCol, rowIdx+1)
			f.SetCellStyle(sheet, startCell, endCell, style)

			if len(row) == 1 && row[0] == "" {
				inTable = false
			}
		}
		for col, width := range instructionWidths {
			f.SetColWidth(sheet, col, col, width)
		}
		showGridLines := false
		if err := f.SetSheetView(sheet, 0, &excelize.ViewOptions{ShowGridLines: &showGridLines}); err != nil {
			return err
		}
		return nil
	}

	if err := writeInstructionSheet(instructionSheetSWE, instructionsSwe); err != nil {
		return err
	}
	if err := writeInstructionSheet(instructionSheetENG, instructionsEng); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return f.SaveAs(path)
}

func exampleRows() [][]any {
	return [][]any{
		{"DC-PROD-01", "Windows2022-Template", "DC-Stockholm", "Prod-Cluster", "VSAN-Cluster-01", "", "Production/Infrastructure",
			"VLAN-100-Prod", "10.20.30.10", "255.255.255.0", "10.20.30.1", "10.20.1.10;10.20.1.11",
			"", "", "", "", "",
			"", "", "", "", "",
			4, 8192, "20", "thick", "DC",
			"DC-PROD-01", "corp.example.com", "svc_domainjoin", "OU=Domain Controllers,DC=corp,DC=example,DC=com",
			3, "W. Europe Standard Time", ""},
		{"SQL-PROD-01", "Windows2022-Template", "DC-Stockholm", "Prod-Cluster", "VSAN-Cluster-01", "", "Production/Database",
			"VLAN-100-Prod", "10.20.30.20", "255.255.255.0", "10.20.30.1", "10.20.1.10;10.20.1.11",
			"VLAN-400-Mgmt", "10.20.40.20", "255.255.255.0", "10.20.40.1", "",
			"", "", "", "", "",
			8, 32768, "50;100;200", "thin", "SQL",
			"SQL-PROD-01", "corp.example.com", "svc_domainjoin", "OU=SQL,OU=Servers,DC=corp,DC=example,DC=com",
			3, "W. Europe Standard Time", "powershell.exe -File C:\\Scripts\\sql-setup.ps1"},
		{"APP-PROD-01", "Windows2022-Template", "DC-Stockholm", "Prod-Cluster", "VSAN-Cluster-01", "", "Production/Applications",
			"VLAN-100-Prod", "DHCP", "", "", "10.20.1.10;10.20.1.11",
			"", "", "", "", "",
			"", "", "", "", "",
			2, 4096, "", "thin", "Member",
			"APP-PROD-01", "corp.example.com", "svc_domainjoin", "OU=Servers,OU=Production,DC=corp,DC=example,DC=com",
			3, "W. Europe Standard Time", ""},
	}
}

// ExcelNetworkAdapter mirrors JSON output from the Python module.
type ExcelNetworkAdapter struct {
	PortGroup  string   `json:"port_group"`
	IPAddress  string   `json:"ip_address,omitempty"`
	SubnetMask string   `json:"subnet_mask,omitempty"`
	Gateway    string   `json:"gateway,omitempty"`
	DNSServers []string `json:"dns_servers,omitempty"`
}

// ExcelHardware describes VM hardware overrides.
type ExcelHardware struct {
	NumCPUs          int    `json:"num_cpus"`
	MemoryMB         int    `json:"memory_mb"`
	DiskGB           []int  `json:"disk_gb,omitempty"`
	DiskProvisioning string `json:"disk_provisioning"`
	ServerRole       string `json:"server_role,omitempty"`
}

// ExcelDomainJoin describes domain join configuration.
type ExcelDomainJoin struct {
	Username       string `json:"username,omitempty"`
	PasswordSecret string `json:"password_secret,omitempty"`
	OUPath         string `json:"ou_path,omitempty"`
}

// ExcelCustomization matches the JSON structure from py-vcenter.
type ExcelCustomization struct {
	Hostname            string          `json:"hostname,omitempty"`
	Domain              string          `json:"domain,omitempty"`
	DomainJoin          ExcelDomainJoin `json:"domain_join"`
	AdminPasswordSecret string          `json:"admin_password_secret"`
	Autologon           struct {
		Enabled  bool   `json:"enabled"`
		Count    int    `json:"count"`
		Username string `json:"username"`
	} `json:"autologon"`
	RunOnceCommands []string `json:"run_once_commands,omitempty"`
	Timezone        string   `json:"timezone,omitempty"`
}

// ExcelVMConfig encapsulates one VM definition parsed from Excel.
type ExcelVMConfig struct {
	Template        string                `json:"template"`
	VMName          string                `json:"vm_name"`
	Datacenter      string                `json:"datacenter,omitempty"`
	Cluster         string                `json:"cluster,omitempty"`
	StorageCluster  string                `json:"storage_cluster,omitempty"`
	Datastore       string                `json:"datastore,omitempty"`
	ResourcePool    string                `json:"resource_pool,omitempty"`
	Folder          string                `json:"folder,omitempty"`
	NetworkAdapters []ExcelNetworkAdapter `json:"network_adapters"`
	Hardware        ExcelHardware         `json:"hardware"`
	Customization   ExcelCustomization    `json:"customization"`
}

// ExcelToJSON converts Excel rows into ExcelVMConfig list and optionally writes JSON files.
func ExcelToJSON(path string, outputDir string) ([]ExcelVMConfig, error) {
	rows, headerMap, err := readExcelRows(path)
	if err != nil {
		return nil, err
	}

	configs := make([]ExcelVMConfig, 0, len(rows))
	for _, row := range rows {
		cfg, err := rowToConfig(row, headerMap)
		if err != nil {
			return nil, err
		}
		configs = append(configs, cfg)
	}

	if outputDir != "" {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return nil, err
		}
		for _, cfg := range configs {
			data, err := json.MarshalIndent(cfg, "", "  ")
			if err != nil {
				return nil, err
			}
			filename := filepath.Join(outputDir, fmt.Sprintf("%s.json", cfg.VMName))
			if err := os.WriteFile(filename, data, 0o600); err != nil {
				return nil, err
			}
		}
		allData, err := json.MarshalIndent(configs, "", "  ")
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(outputDir, "_all_vms.json"), allData, 0o600); err != nil {
			return nil, err
		}
	}

	return configs, nil
}

// ValidateExcel performs similar validation checks as the Python implementation.
func ValidateExcel(path string, cfg *VCenterConfig, strict bool) (bool, []string, error) {
	rows, headerMap, err := readExcelRows(path)
	if err != nil {
		return false, nil, err
	}

	var errorsList []string
	var warnings []string
	nameSeen := map[string]int{}

	for _, row := range rows {
		vmName := getCell(row, headerMap, "vm_name")
		rowNum := toRowNumber(row, headerMap)

		if idx, ok := nameSeen[vmName]; ok {
			errorsList = append(errorsList, fmt.Sprintf("Duplicate VM name '%s' found in rows %d and %d", vmName, idx, rowNum))
		} else {
			nameSeen[vmName] = rowNum
		}

		template := strings.TrimSpace(row[headerMap["template"]])
		if template == "" {
			errorsList = append(errorsList, fmt.Sprintf("%s: template is empty", vmLabel(row, headerMap)))
		}

		if err := ensurePositiveInt(row, headerMap, "num_cpus", 1, nil); err != nil {
			errorsList = append(errorsList, fmt.Sprintf("%s: %v", vmLabel(row, headerMap), err))
		}
		if err := ensurePositiveInt(row, headerMap, "memory_mb", 512, nil); err != nil {
			errorsList = append(errorsList, fmt.Sprintf("%s: %v", vmLabel(row, headerMap), err))
		}

		if err := validateDiskProvisioning(row, headerMap); err != nil {
			errorsList = append(errorsList, fmt.Sprintf("%s: %v", vmLabel(row, headerMap), err))
		}

		if result := validateNetwork(row, headerMap, strict); result != nil {
			if result.warning {
				warnings = append(warnings, result.message)
			} else {
				errorsList = append(errorsList, result.message)
			}
		}

		if cfg == nil && getCell(row, headerMap, "datacenter") == "" {
			errorsList = append(errorsList, fmt.Sprintf("%s: datacenter missing and no global config provided", vmLabel(row, headerMap)))
		}
	}

	allMessages := append([]string{}, errorsList...)
	if strict {
		allMessages = append(allMessages, warnings...)
	} else {
		for _, w := range warnings {
			allMessages = append(allMessages, "WARNING: "+w)
		}
	}

	isValid := len(errorsList) == 0
	if strict && len(warnings) > 0 {
		isValid = false
	}
	return isValid, allMessages, nil
}

func readExcelRows(path string) ([][]string, map[string]int, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, nil, err
	}
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	rows, err := f.GetRows(vmRequestSheet)
	if err != nil {
		return nil, nil, err
	}
	if len(rows) == 0 {
		return nil, nil, errors.New("Excel sheet has no rows")
	}

	headerRow := rows[0]
	headerMap := map[string]int{}
	for idx, header := range headerRow {
		headerMap[strings.TrimSpace(header)] = idx
	}
	rowNumberIndex := len(headerRow)
	headerMap["_row"] = rowNumberIndex

	for _, required := range requiredColumns {
		if _, ok := headerMap[required]; !ok {
			return nil, nil, fmt.Errorf("missing required column: %s", required)
		}
	}

	var parsed [][]string
	for rowIdx := 1; rowIdx < len(rows); rowIdx++ {
		row := rows[rowIdx]
		vmName := strings.TrimSpace(row[headerMap["vm_name"]])
		if strings.TrimSpace(vmName) == "" {
			continue
		}
		rowCopy := make([]string, rowNumberIndex+1)
		copy(rowCopy, row)
		rowCopy[rowNumberIndex] = strconv.Itoa(rowIdx + 1)
		parsed = append(parsed, rowCopy)
	}

	if len(parsed) == 0 {
		return nil, nil, errors.New("no VM rows found")
	}

	return parsed, headerMap, nil
}

func getCell(row []string, header map[string]int, key string) string {
	idx, ok := header[key]
	if !ok || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

func rowToConfig(row []string, header map[string]int) (ExcelVMConfig, error) {
	vmName := getCell(row, header, "vm_name")
	template := getCell(row, header, "template")

	numCPUs, err := strconv.Atoi(getCell(row, header, "num_cpus"))
	if err != nil {
		return ExcelVMConfig{}, fmt.Errorf("%s: invalid num_cpus", vmName)
	}
	memoryMB, err := strconv.Atoi(getCell(row, header, "memory_mb"))
	if err != nil {
		return ExcelVMConfig{}, fmt.Errorf("%s: invalid memory_mb", vmName)
	}
	diskList, err := parseDiskList(getCell(row, header, "disk_gb"))
	if err != nil {
		return ExcelVMConfig{}, fmt.Errorf("%s: %w", vmName, err)
	}

	adapters := parseAdapters(row, header)
	customization := ExcelCustomization{
		Hostname: getCell(row, header, "hostname"),
		Domain:   getCell(row, header, "domain"),
		DomainJoin: ExcelDomainJoin{
			Username:       getCell(row, header, "domain_join_user"),
			PasswordSecret: "vault:domainjoin/prod",
			OUPath:         getCell(row, header, "ou_path"),
		},
		AdminPasswordSecret: "vault:localadmin/prod",
		RunOnceCommands:     parseSemicolonList(getCell(row, header, "run_once_commands")),
		Timezone:            getCell(row, header, "timezone"),
	}
	autologonCount, _ := strconv.Atoi(defaultString(getCell(row, header, "autologon_count"), "0"))
	customization.Autologon.Enabled = autologonCount > 0
	customization.Autologon.Count = autologonCount
	customization.Autologon.Username = "Administrator"

	config := ExcelVMConfig{
		Template:        template,
		VMName:          vmName,
		Datacenter:      getCell(row, header, "datacenter"),
		Cluster:         getCell(row, header, "compute_cluster"),
		StorageCluster:  getCell(row, header, "storage_cluster"),
		Datastore:       getCell(row, header, "datastore"),
		ResourcePool:    getCell(row, header, "resource_pool"),
		Folder:          getCell(row, header, "folder"),
		NetworkAdapters: adapters,
		Hardware: ExcelHardware{
			NumCPUs:          numCPUs,
			MemoryMB:         memoryMB,
			DiskGB:           diskList,
			DiskProvisioning: strings.ToLower(defaultString(getCell(row, header, "disk_provisioning"), "thin")),
			ServerRole:       getCell(row, header, "server_role"),
		},
		Customization: customization,
	}

	return config, nil
}

func parseAdapters(row []string, header map[string]int) []ExcelNetworkAdapter {
	adapters := []ExcelNetworkAdapter{}
	for i := 1; i <= 3; i++ {
		pg := getCell(row, header, fmt.Sprintf("port_group_%d", i))
		if pg == "" {
			continue
		}
		adapter := ExcelNetworkAdapter{
			PortGroup:  pg,
			IPAddress:  normalizeIP(getCell(row, header, fmt.Sprintf("ip_%d", i))),
			SubnetMask: getCell(row, header, fmt.Sprintf("subnet_mask_%d", i)),
			Gateway:    normalizeIP(getCell(row, header, fmt.Sprintf("gateway_%d", i))),
			DNSServers: parseSemicolonList(getCell(row, header, fmt.Sprintf("dns_servers_%d", i))),
		}
		adapters = append(adapters, adapter)
	}
	return adapters
}

func normalizeIP(value string) string {
	if strings.EqualFold(value, "dhcp") {
		return ""
	}
	return value
}

func parseSemicolonList(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ";")
	var result []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func parseDiskList(raw string) ([]int, error) {
	if raw == "" {
		return nil, nil
	}
	parts := parseSemicolonList(raw)
	result := make([]int, len(parts))
	for i, part := range parts {
		size, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid disk size '%s'", part)
		}
		result[i] = size
	}
	return result, nil
}

func validateDiskProvisioning(row []string, header map[string]int) error {
	value := strings.ToLower(getCell(row, header, "disk_provisioning"))
	if value == "" {
		return nil
	}
	if value != "thin" && value != "thick" {
		return fmt.Errorf("disk_provisioning must be 'thin' or 'thick'")
	}
	return nil
}

func ensurePositiveInt(row []string, header map[string]int, key string, min int, max *int) error {
	valStr := getCell(row, header, key)
	value, err := strconv.Atoi(valStr)
	if err != nil {
		return fmt.Errorf("%s is not a valid integer", key)
	}
	if value < min {
		return fmt.Errorf("%s must be >= %d", key, min)
	}
	if max != nil && value > *max {
		return fmt.Errorf("%s must be <= %d", key, *max)
	}
	return nil
}

type validationError struct {
	warning bool
	message string
}

func validateNetwork(row []string, header map[string]int, strict bool) *validationError {
	vm := vmLabel(row, header)
	pg := getCell(row, header, "port_group_1")
	if pg == "" {
		return &validationError{message: fmt.Sprintf("%s: port_group_1 is required", vm)}
	}

	ip := getCell(row, header, "ip_1")
	if ip != "" && !strings.EqualFold(ip, "DHCP") {
		if !isValidIPv4(ip) {
			return &validationError{message: fmt.Sprintf("%s: ip_1 must be a valid IPv4 address", vm)}
		}
		if getCell(row, header, "subnet_mask_1") == "" {
			return &validationError{message: fmt.Sprintf("%s: subnet_mask_1 required when ip_1 is static", vm)}
		}
		if getCell(row, header, "gateway_1") == "" {
			return &validationError{message: fmt.Sprintf("%s: gateway_1 required when ip_1 is static", vm)}
		}
	}

	for i := 2; i <= 3; i++ {
		pg := getCell(row, header, fmt.Sprintf("port_group_%d", i))
		if pg == "" {
			continue
		}
		gw := getCell(row, header, fmt.Sprintf("gateway_%d", i))
		if gw != "" && !strings.EqualFold(gw, "DHCP") {
			if strict {
				return &validationError{message: fmt.Sprintf("%s: NIC %d should not define gateway", vm, i)}
			}
			return &validationError{warning: true, message: fmt.Sprintf("NIC %d defines gateway (may cause routing conflicts)", i)}
		}
	}
	return nil
}

func vmLabel(row []string, header map[string]int) string {
	return fmt.Sprintf("VM '%s' (row %d)", getCell(row, header, "vm_name"), toRowNumber(row, header))
}

func toRowNumber(row []string, header map[string]int) int {
	value := getCell(row, header, "_row")
	if value == "" {
		return 0
	}
	num, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return num
}

var ipv4Regex = regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$`)

func isValidIPv4(ip string) bool {
	if !ipv4Regex.MatchString(ip) {
		return false
	}
	parts := strings.Split(ip, ".")
	for _, part := range parts {
		val, err := strconv.Atoi(part)
		if err != nil || val < 0 || val > 255 {
			return false
		}
	}
	return true
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
