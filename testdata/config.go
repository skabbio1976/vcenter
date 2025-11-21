// Package testdata provides test configuration and utilities for integration tests.
package testdata

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/skabbio1976/vcenter"
)

// TestConfig holds all configuration for integration tests
type TestConfig struct {
	VCenter       VCenterConfig   `json:"vcenter"`
	TestResources TestResources   `json:"test_resources"`
	TestOptions   TestOptions     `json:"test_options"`
}

// VCenterConfig holds vCenter connection details
type VCenterConfig struct {
	Host       string `json:"host"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	Insecure   bool   `json:"insecure"`
	Datacenter string `json:"datacenter"`
}

// TestResources holds names of vCenter resources used for testing
type TestResources struct {
	TemplateName string `json:"template_name"`
	TestVMPrefix string `json:"test_vm_prefix"`
	Datastore    string `json:"datastore"`
	ResourcePool string `json:"resource_pool"`
	Network      string `json:"network"`
	Folder       string `json:"folder"`
}

// TestOptions holds test execution options
type TestOptions struct {
	AutoCleanup        bool `json:"auto_cleanup"`
	KeepFailedVMs      bool `json:"keep_failed_vms"`
	TestTimeoutSeconds int  `json:"test_timeout_seconds"`
}

// LoadTestConfig loads the test configuration from test-config.json
func LoadTestConfig() (*TestConfig, error) {
	return LoadTestConfigFromFile("test-config.json")
}

// LoadTestConfigFromFile loads test configuration from a specific file
func LoadTestConfigFromFile(filename string) (*TestConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config TestConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	// Validate required fields
	if config.VCenter.Host == "" {
		return nil, fmt.Errorf("vcenter.host is required")
	}
	if config.VCenter.Username == "" {
		return nil, fmt.Errorf("vcenter.username is required")
	}
	if config.VCenter.Datacenter == "" {
		return nil, fmt.Errorf("vcenter.datacenter is required")
	}
	if config.TestResources.TemplateName == "" {
		return nil, fmt.Errorf("test_resources.template_name is required")
	}
	if config.TestResources.Datastore == "" {
		return nil, fmt.Errorf("test_resources.datastore is required")
	}

	// Set defaults
	if config.TestOptions.TestTimeoutSeconds == 0 {
		config.TestOptions.TestTimeoutSeconds = 300
	}

	return &config, nil
}

// ToConnectConfig converts TestConfig to vcenter.ConnectConfig
func (c *TestConfig) ToConnectConfig() vcenter.ConnectConfig {
	return vcenter.ConnectConfig{
		Host:       c.VCenter.Host,
		Username:   c.VCenter.Username,
		Password:   c.VCenter.Password,
		Insecure:   c.VCenter.Insecure,
		Datacenter: c.VCenter.Datacenter,
	}
}

// ConfigExists checks if test-config.json exists
func ConfigExists() bool {
	_, err := os.Stat("test-config.json")
	return err == nil
}
