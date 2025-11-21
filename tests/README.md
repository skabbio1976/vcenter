# Integration Tests for vcenter (Go)

Integration tests for the vcenter Go package that run against a real vCenter environment. These tests are designed to work in **airgapped environments** where you build on an internet-connected machine and deploy to an isolated environment.

## Quick Start

### Setup

1. **Copy the example config:**
   ```bash
   cp test-config.example.json test-config.json
   ```

2. **Edit `test-config.json` with your vCenter details:**
   ```json
   {
     "vcenter": {
       "host": "vcenter.yourcompany.local",
       "username": "administrator@vsphere.local",
       "password": "YourPassword",
       "insecure": true,
       "datacenter": "Datacenter1"
     },
     "test_resources": {
       "template_name": "Ubuntu-20.04-Template",
       "test_vm_prefix": "go-test",
       "datastore": "datastore1",
       "resource_pool": "Resources",
       "network": "VM Network"
     },
     "test_options": {
       "auto_cleanup": true,
       "keep_failed_vms": true
     }
   }
   ```

3. **Run the tests:**
   ```bash
   go test -v -timeout 30m
   ```

## Test Coverage

The integration test suite includes:

- ✅ **Test 01: Authentication** - Connect and datacenter verification
- ✅ **Test 02: List and Get VM** - VM listing and info retrieval
- ✅ **Test 03: Clone VM** - Clone a VM from template
- ✅ **Test 04: Power Operations** - Power on/off VMs
- ✅ **Test 05: Snapshot Operations** - Create, list, delete snapshots
- ✅ **Test 06: Disk Operations** - Add and remove disks
- ✅ **Test 07: Network Operations** - Add network adapters
- ✅ **Test 08: Batch Operations** - Parallel VM cloning
- ✅ **Test 09: Complete Lifecycle** - Full VM lifecycle test

## Airgapped Environment Workflow

For customers with airgapped (isolated) environments, follow this workflow:

### Phase 1: Build on Internet-Connected Machine

**Windows:**
```powershell
# Build the test binary
go test -c -o vcenter-integration-test.exe

# Verify the binary was created
dir vcenter-integration-test.exe
```

**Linux:**
```bash
# Build the test binary
go test -c -o vcenter-integration-test

# Make it executable
chmod +x vcenter-integration-test

# Verify
ls -lh vcenter-integration-test
```

### Phase 2: Prepare for Transfer

1. **Create a transfer directory:**
   ```bash
   mkdir vcenter-go-tests
   ```

2. **Copy the test binary:**

   **Windows:**
   ```powershell
   copy vcenter-integration-test.exe vcenter-go-tests\
   ```

   **Linux:**
   ```bash
   cp vcenter-integration-test vcenter-go-tests/
   ```

3. **Create config file in transfer directory:**
   ```bash
   # Copy example as starting point
   cp test-config.example.json vcenter-go-tests/test-config.json

   # Edit vcenter-go-tests/test-config.json with customer's vCenter details
   ```

4. **Create a run script for convenience:**

   **Windows (run-tests.bat):**
   ```batch
   @echo off
   echo Running vcenter Go integration tests...
   echo.

   if not exist vcenter-integration-test.exe (
       echo ERROR: Test binary not found!
       exit /b 1
   )

   vcenter-integration-test.exe -test.v

   echo.
   echo Tests completed!
   pause
   ```

   **Linux (run-tests.sh):**
   ```bash
   #!/bin/bash
   echo "Running vcenter Go integration tests..."
   echo

   if [ ! -f ./vcenter-integration-test ]; then
       echo "ERROR: Test binary not found!"
       exit 1
   fi

   chmod +x ./vcenter-integration-test
   ./vcenter-integration-test -test.v

   echo
   echo "Tests completed!"
   ```

### Phase 3: Transfer to Airgapped Environment

Transfer the entire `vcenter-go-tests` directory to the airgapped environment using approved methods:
- USB drive
- Secure file transfer
- Media approved by security team

### Phase 4: Run on Airgapped Machine

**Windows:**
```powershell
cd vcenter-go-tests
.\run-tests.bat
```

**Linux:**
```bash
cd vcenter-go-tests
chmod +x run-tests.sh
./run-tests.sh
```

Or run the binary directly:

**Windows:**
```powershell
.\vcenter-integration-test.exe -test.v
```

**Linux:**
```bash
./vcenter-integration-test -test.v
```

## Configuration Options

### `test-config.json` Format

```json
{
  "vcenter": {
    "host": "vcenter.example.com",         // vCenter hostname or IP
    "username": "admin@vsphere.local",     // Username
    "password": "password",                // Password
    "insecure": true,                      // Skip SSL verification (true/false)
    "datacenter": "Datacenter1"            // Datacenter name
  },
  "test_resources": {
    "template_name": "Ubuntu-Template",    // VM template to clone from
    "test_vm_prefix": "go-test",           // Prefix for test VMs
    "datastore": "datastore1",             // Datastore name
    "resource_pool": "Resources",          // Resource pool name
    "network": "VM Network",               // Network name
    "folder": ""                           // Optional: VM folder (empty for default)
  },
  "test_options": {
    "auto_cleanup": true,                  // Auto-delete test VMs (true/false)
    "keep_failed_vms": true,               // Keep VMs if test fails (true/false)
    "test_timeout_seconds": 300            // Timeout per test
  }
}
```

### Cleanup Behavior

- **`auto_cleanup: true`** - Test VMs are deleted after each test
- **`auto_cleanup: false`** - Test VMs remain for manual inspection
- **`keep_failed_vms: true`** - If a test fails, the VM is kept for debugging even if auto_cleanup is true

## Running Individual Tests

Run a specific test:
```bash
# Run only authentication test
go test -v -run Test01_Authentication

# Run only snapshot tests
go test -v -run Test05_Snapshot
```

Or with the binary (airgapped):
```bash
./vcenter-integration-test -test.v -test.run Test01_Authentication
```

## Troubleshooting

### Tests are skipped

**Problem:** Tests print "Skipping integration test - no test-config.json found"

**Solution:**
1. Ensure `test-config.json` exists in the same directory as the test binary
2. Check the file is named exactly `test-config.json`

### Authentication fails

**Problem:** Connection errors or authentication failures

**Solution:**
1. Verify vCenter hostname/IP is correct
2. Check username and password
3. If using self-signed certificates, ensure `"insecure": true`
4. Verify network connectivity to vCenter
5. Check vCenter is running and accessible

### Template not found

**Problem:** "VM not found: Template" error

**Solution:**
1. Log into vCenter UI
2. Find the correct template name (case-sensitive!)
3. Update `template_name` in test-config.json
4. Ensure template is in the correct datacenter

### Insufficient permissions

**Problem:** "Permission denied" or "Access denied" errors

**Solution:**
1. Verify the user has necessary permissions:
   - Create/delete VMs
   - Create/delete snapshots
   - Modify VM hardware
   - Access datastore
   - Access resource pool

### Tests hang or timeout

**Problem:** Tests seem to hang

**Solution:**
1. Increase timeout: `go test -v -timeout 60m`
2. Check vCenter is not overloaded
3. Verify datastore has sufficient space
4. Check resource pool quotas

### Cleanup issues

**Problem:** Test VMs are not being cleaned up

**Solution:**
1. Check `auto_cleanup` setting in config
2. Manually delete VMs with prefix matching `test_vm_prefix`
3. Check for permission issues on delete operations

## Security Notes

**IMPORTANT:**

1. **Never commit `test-config.json` to version control** - It contains credentials
2. The file is already in `.gitignore` but double-check
3. Use a dedicated service account with minimum required permissions
4. In airgapped environments, ensure test-config.json is transferred securely

## Development Workflow

For active development:

```bash
# Run tests with verbose output
go test -v -timeout 30m

# Run specific test
go test -v -run Test03_CloneVM

# Run with race detector
go test -v -race -timeout 30m

# Show test coverage (though integration tests won't show much)
go test -v -cover
```

## CI/CD Integration

These tests are designed for **manual execution** in airgapped environments. For CI/CD in connected environments, you would need:

1. GitHub Actions secret variables for vCenter credentials
2. Access to a test vCenter environment from CI
3. Modification of tests to use environment variables instead of JSON

This is **not the primary use case** but can be added if needed for community contributions.

## Building for Different Platforms

Build test binaries for different platforms:

```bash
# Windows AMD64
GOOS=windows GOARCH=amd64 go test -c -o vcenter-test-windows-amd64.exe

# Linux AMD64
GOOS=linux GOARCH=amd64 go test -c -o vcenter-test-linux-amd64

# macOS ARM64 (M1/M2)
GOOS=darwin GOARCH=arm64 go test -c -o vcenter-test-darwin-arm64
```

## Test Execution Time

Approximate execution times (depends on vCenter performance):

- **Test 01 (Auth):** 2-5 seconds
- **Test 02 (List/Get):** 3-5 seconds
- **Test 03 (Clone):** 30-90 seconds
- **Test 04 (Power):** 60-120 seconds
- **Test 05 (Snapshots):** 40-90 seconds
- **Test 06 (Disks):** 40-90 seconds
- **Test 07 (Network):** 40-90 seconds
- **Test 08 (Batch):** 120-300 seconds (3 parallel clones)
- **Test 09 (Lifecycle):** 90-180 seconds

**Total:** ~10-20 minutes for complete test suite

Use `-timeout 30m` to ensure tests don't timeout prematurely.
