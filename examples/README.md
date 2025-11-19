# vcenter - Code Examples

This directory contains practical examples demonstrating how to use the vcenter package for various vCenter operations.

## Overview

The examples are organized in numerical order from simple to advanced usage:

### Connection

- **01-connect-sspi** - Connect with Windows SSPI/Kerberos (single sign-on)
- **02-connect-password** - Connect with username and password

### VM Cloning

- **03-simple-clone** - Simple VM cloning without customization
- **04-clone-with-customization** - Clone Windows VM with domain join (DHCP)
- **05-clone-static-ip** - Clone Windows VM with static IP address
- **10-server-request** - Use ServerRequest for structured configuration

### Batch Operations

- **06-batch-clone** - Clone multiple VMs in parallel
- **07-bulk-power** - Power operations on multiple VMs simultaneously

### VM Management

- **08-disk-operations** - Add, extend, and remove disks
- **09-network-operations** - Add and modify network adapters

## How to Run the Examples

### Prerequisites

1. Go 1.21 or later installed
2. Access to a vCenter Server
3. A template to clone from

### Configure Examples

Each example contains placeholders that need to be updated:

```go
// Update these values
Host:       "vcenter.example.com",      // Your vCenter hostname
Username:   "administrator@vsphere.local",
Password:   "YourPassword",
Datacenter: "Datacenter1",              // Your datacenter
```

### Run an Example

```bash
# Navigate to the examples directory
cd examples/03-simple-clone

# Edit main.go and update the configuration
vim main.go

# Run the example
go run main.go
```

## Example Descriptions

### 01-connect-sspi

Demonstrates how to connect using Windows integrated authentication. Perfect for Windows environments where users are already authenticated against Active Directory.

**Requires:** Windows operating system

```bash
cd examples/01-connect-sspi
go run main.go
```

### 02-connect-password

Shows standard connection with username and password. Works on all platforms and uses session caching for better performance.

```bash
cd examples/02-connect-password
go run main.go
```

### 03-simple-clone

Basic VM cloning without Windows customization. Good for:
- Linux VMs
- Templates without sysprep
- Quick cloning for testing

```bash
cd examples/03-simple-clone
go run main.go
```

### 04-clone-with-customization

Clone Windows VM with:
- Domain join
- DHCP IP configuration
- DNS settings
- Timezone
- Automatic waiting for VMware Tools and IP

```bash
cd examples/04-clone-with-customization
go run main.go
```

### 05-clone-static-ip

Same as the example above but with static IP address instead of DHCP. Perfect for servers that need fixed IP addresses.

```bash
cd examples/05-clone-static-ip
go run main.go
```

### 06-batch-clone

Demonstrates how to clone multiple VMs in parallel with goroutines. Dramatically faster than sequential cloning:

- 4 VMs cloned simultaneously
- Shows success/failure for each VM
- Uses ServerRequest for configuration

```bash
cd examples/06-batch-clone
go run main.go
```

**Performance:** Clones 4 VMs in the same time it takes to clone 1 VM sequentially.

### 07-bulk-power

Parallel power operations on multiple VMs:
- Power on
- Power off
- Restart

Perfect for starting/stopping entire environments simultaneously.

```bash
cd examples/07-bulk-power
go run main.go
```

### 08-disk-operations

Complete example of disk management:
- Add new disks (thin provisioned)
- Extend existing disks
- Remove disks

**NOTE:** Remember to extend partitions in the guest system after disk extension.

```bash
cd examples/08-disk-operations
go run main.go
```

### 09-network-operations

Network adapter management:
- Add VMXNET3 adapters
- Change network/port group
- Multi-NIC configuration

```bash
cd examples/09-network-operations
go run main.go
```

### 10-server-request

Advanced example showing ServerRequest struct for:
- Structured server configuration
- Input validation
- Complete server provisioning with one function
- Automatic verification of IP configuration

```bash
cd examples/10-server-request
go run main.go
```

## Best Practices

### 1. Error Handling

Always handle errors and use type assertions for specific error types:

```go
vm, err := vcenter.GetVM(ctx, client.Client, "NonExistent", "DC1")
if err != nil {
    var notFoundErr *vcenter.NotFoundError
    if errors.As(err, &notFoundErr) {
        log.Printf("VM not found: %s", notFoundErr)
    }
}
```

### 2. Context and Timeout

Always use context with timeout for long-running operations:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
defer cancel()

vm, err := vcenter.CloneVM(ctx, ...)
```

### 3. Resource Cleanup

Don't forget to logout from vCenter:

```go
client, err := vcenter.ConnectWithPassword(ctx, config)
if err != nil {
    log.Fatal(err)
}
defer client.Logout(ctx)  // IMPORTANT!
```

### 4. Parallel Operations

For batch operations, use the built-in parallel functions:

```go
// GOOD - Parallel cloning
vms, errors := vcenter.CloneMultiple(ctx, client, requests, ...)

// AVOID - Sequential cloning
for _, req := range requests {
    vm, err := vcenter.CloneFromRequest(ctx, client, req, ...)
}
```

### 5. VMware Tools

Always wait for VMware Tools before IP operations:

```go
err = vcenter.WaitForTools(ctx, vm)
if err == nil {
    ip, err := vcenter.WaitForIP(ctx, vm, 10*time.Minute)
}
```

## Common Problems

### Problem: "VM not found"

**Solution:** Check that the VM name and datacenter are correct:

```go
// Search in all datacenters
vm, err := vcenter.GetVM(ctx, client.Client, "VMName", "")

// Search in specific datacenter
vm, err := vcenter.GetVM(ctx, client.Client, "VMName", "DC1")
```

### Problem: Timeout at WaitForIP

**Solution:**
1. Check that VMware Tools is installed
2. Increase the timeout value
3. Check network configuration

```go
// Increase timeout to 15 minutes
ip, err := vcenter.WaitForIP(ctx, vm, 15*time.Minute)
```

### Problem: "disk with label X not found"

**Solution:** Check the exact label of the disk in vCenter:

```go
// Use exact label from vCenter
err := vcenter.ExtendDisk(ctx, vm, "Hard disk 2", 200)
```

### Problem: Customization doesn't work

**Solution:**
1. Check that the template has sysprep prepared
2. Verify domain credentials
3. Check DNS configuration
4. Make sure PowerOn is true in CloneSpec

## Additional Resources

- [Main documentation](../README.md)
- [godoc](https://pkg.go.dev/github.com/skabbio1976/vcenter)
- [govmomi documentation](https://github.com/vmware/govmomi)
- [VMware vSphere API](https://developer.vmware.com/apis/968/vsphere)

## Contributing Examples

Have a useful example? Send a pull request with:

1. Well-organized code with comments
2. Update to this README
3. Testing against an actual vCenter

## Support

If you find bugs or have questions:
- Open an issue on GitHub
- Check existing examples first
- Include error messages and Go version
