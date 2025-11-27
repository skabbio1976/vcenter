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
	vmRequestSheet   = "VM Requests"
	validValuesSheet = "Valid Values"
	instructionSheet = "Instruktioner"
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
			Font:      &excelize.Font{Bold: true, Color: "FFFFFFFF"},
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
			Font: &excelize.Font{Bold: true, Color: "FFFFFFFF"},
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

	addValidation := func(col string, options []string) {
		dvRange := fmt.Sprintf("%s2:%s200", col, col)
		dv := excelize.NewDataValidation(true)
		dv.Sqref = dvRange
		if len(options) == 0 {
			return
		}
		if err := dv.SetDropList(options); err != nil {
			return
		}
		_ = f.AddDataValidation(vmRequestSheet, dv)
	}

	addValidation("B", values.Templates)
	addValidation("C", values.Datacenters)
	addValidation("D", values.ComputeClusters)
	addValidation("E", values.StorageClusters)
	addValidation("F", values.Datastores)
	addValidation("H", values.PortGroups)
	addValidation("M", values.PortGroups)
	addValidation("R", values.PortGroups)
	addValidation("W", intsToStrings(values.CPUOptions))
	addValidation("X", intsToStrings(values.MemoryOptions))
	addValidation("Z", values.DiskProvisioning)
	addValidation("AA", values.ServerRoles)
	addValidation("AC", values.Domains)
	addValidation("AG", values.Timezones)

	instructions := [][]string{
		{"VM Request Mall - Instruktioner"},
		{""},
		{"Så här fyller du i mallen:"},
		{""},
		{"1. Fyll i en rad per VM i fliken 'VM Requests'"},
		{"2. Rad 2-9 (gröna) är exempel – skriv över dem"},
		{"3. Använd dropdown-menyer där de finns"},
		{"4. Listor (dns_servers, disk_gb) separeras med semikolon"},
		{""},
		{"Nyckelkolumner:"},
		{"• disk_gb - EXTRA diskar, format: '50;100;200'"},
		{"• port_group_1 - Obligatorisk, övriga NIC:ar valfria"},
		{"• Skriv 'DHCP' i IP-fältet om du vill använda DHCP"},
		{"• disk_provisioning - 'thin' eller 'thick'"},
		{"• server_role - DC, CA, CRL, Member, T0-RDS, T1-RDS, File, SQL, Web, Standalone"},
		{""},
		{"Se README.md för fullständig dokumentation."},
	}
	if _, err := f.NewSheet(instructionSheet); err != nil {
		return err
	}
	for rowIdx, row := range instructions {
		f.SetCellValue(instructionSheet, fmt.Sprintf("A%d", rowIdx+1), row[0])
	}
	f.SetColWidth(instructionSheet, "A", "A", 80)

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
