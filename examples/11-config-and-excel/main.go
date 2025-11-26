package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/skabbio1976/vcenter"
)

func main() {
	tmpDir := os.TempDir()
	configPath := filepath.Join(tmpDir, "vcenter_config.json")
	created, err := vcenter.EnsureConfigFile(configPath)
	if err != nil {
		panic(err)
	}
	if created {
		fmt.Printf("Created template config at %s\n", configPath)
	} else {
		fmt.Printf("Config already exists at %s\n", configPath)
	}

	store := vcenter.NewCredentialStore()
	store.AddCredential("vcenter", vcenter.Credential{
		Server:   "vcenter.example.com",
		Username: "administrator@vsphere.local",
		Password: "SuperSecret",
		Insecure: true,
	})

	key, err := vcenter.GenerateEncryptionKey()
	if err != nil {
		panic(err)
	}
	encPath := filepath.Join(tmpDir, "credentials.json.enc")
	if err := store.SaveEncrypted(encPath, vcenter.KeySourceDirect(key)); err != nil {
		panic(err)
	}
	fmt.Printf("Encrypted credentials saved to %s (key: %s)\n", encPath, key)

	excelPath := filepath.Join(tmpDir, "vm_request_template.xlsx")
	if err := vcenter.CreateExcelTemplate(excelPath, nil); err != nil {
		panic(err)
	}
	fmt.Printf("Excel template written to %s\n", excelPath)

	ok, messages, err := vcenter.ValidateExcel(excelPath, nil, false)
	if err != nil {
		fmt.Printf("validation error: %v\n", err)
	} else {
		fmt.Printf("Template valid: %v\n", ok)
		if len(messages) > 0 {
			fmt.Println("Messages:")
			for _, msg := range messages {
				fmt.Printf("  - %s\n", msg)
			}
		}
	}

	// Use ExcelToJSON when you have a populated Excel file:
	// configs, err := vcenter.ExcelToJSON("vm_requests.xlsx", "vm_configs")
	// fmt.Printf("Converted %d VM definitions to JSON files\n", len(configs))
}
