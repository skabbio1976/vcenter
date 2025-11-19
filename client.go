package vcenter

import (
	"context"
	"fmt"
	"net/url"
	"sync"

	vcauth "github.com/skabbio1976/go-vcenter-auth"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/session"
)

// Client is a wrapper around govmomi.Client with cached datacenter and helper functions
type Client struct {
	*govmomi.Client
	datacenter     *object.Datacenter
	datacenterName string
	finder         *find.Finder
	mu             sync.RWMutex
}

// ConnectConfig contains configuration for connecting to vCenter
type ConnectConfig struct {
	Host       string
	Username   string
	Password   string
	Insecure   bool
	Datacenter string
}

// ConnectWithPassword connects to vCenter with username/password
// Uses go-vcenter-auth for session caching and automatic reconnection
func ConnectWithPassword(ctx context.Context, config ConnectConfig) (*Client, error) {
	if config.Host == "" {
		return nil, &ValidationError{Field: "Host", Message: "host is required"}
	}
	if config.Username == "" {
		return nil, &ValidationError{Field: "Username", Message: "username is required"}
	}
	if config.Password == "" {
		return nil, &ValidationError{Field: "Password", Message: "password is required"}
	}

	// Use go-vcenter-auth to log in with session caching
	authClient, err := vcauth.Login(ctx, config.Host, config.Username, config.Password, config.Insecure)
	if err != nil {
		return nil, &OperationError{Operation: "login", Err: err}
	}

	// Retrieve vim25.Client from go-vcenter-auth
	vim25Client := authClient.GetVim()

	// Create govmomi.Client from vim25.Client
	govmomiClient := &govmomi.Client{
		Client:         vim25Client,
		SessionManager: session.NewManager(vim25Client),
	}

	// Create our Client wrapper
	client := &Client{
		Client:         govmomiClient,
		datacenterName: config.Datacenter,
	}

	// If datacenter is specified, cache it
	if config.Datacenter != "" {
		err = client.SetDatacenter(ctx, config.Datacenter)
		if err != nil {
			return nil, err
		}
	}

	return client, nil
}

// ConnectWithSSPI connects to vCenter with Windows SSPI/Kerberos authentication
// This only works on Windows and uses the logged-in user's credentials
// Returns ErrSSPINotSupported on non-Windows platforms
func ConnectWithSSPI(ctx context.Context, host string, insecure bool, datacenter string) (*Client, error) {
	if host == "" {
		return nil, &ValidationError{Field: "Host", Message: "host is required"}
	}

	// Use go-vcenter-auth for SSPI login with session caching
	authClient, err := vcauth.LoginSSPI(ctx, host, insecure)
	if err != nil {
		return nil, &OperationError{Operation: "SSPI login", Err: err}
	}

	// Retrieve vim25.Client from go-vcenter-auth
	vim25Client := authClient.GetVim()

	// Create govmomi.Client from vim25.Client
	govmomiClient := &govmomi.Client{
		Client:         vim25Client,
		SessionManager: session.NewManager(vim25Client),
	}

	// Create our Client wrapper
	client := &Client{
		Client:         govmomiClient,
		datacenterName: datacenter,
	}

	// If datacenter is specified, cache it
	if datacenter != "" {
		err = client.SetDatacenter(ctx, datacenter)
		if err != nil {
			return nil, err
		}
	}

	return client, nil
}

// ConnectWithURL connects to vCenter with a complete URL (including credentials)
// Uses standard govmomi without go-vcenter-auth
func ConnectWithURL(ctx context.Context, urlStr string, insecure bool, datacenter string) (*Client, error) {
	if urlStr == "" {
		return nil, &ValidationError{Field: "URL", Message: "URL is required"}
	}

	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, &ValidationError{Field: "URL", Message: fmt.Sprintf("invalid URL: %v", err)}
	}

	govmomiClient, err := govmomi.NewClient(ctx, u, insecure)
	if err != nil {
		return nil, &OperationError{Operation: "connect", Err: err}
	}

	client := &Client{
		Client:         govmomiClient,
		datacenterName: datacenter,
	}

	if datacenter != "" {
		err = client.SetDatacenter(ctx, datacenter)
		if err != nil {
			return nil, err
		}
	}

	return client, nil
}

// SetDatacenter sets the datacenter for the client and caches the finder
func (c *Client) SetDatacenter(ctx context.Context, datacenterName string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	finder := find.NewFinder(c.Client.Client, true)
	dc, err := finder.Datacenter(ctx, datacenterName)
	if err != nil {
		return &NotFoundError{ResourceType: "Datacenter", Name: datacenterName}
	}

	finder.SetDatacenter(dc)
	c.datacenter = dc
	c.datacenterName = datacenterName
	c.finder = finder

	return nil
}

// GetFinder returns a finder for the cached datacenter
// If no datacenter is cached, a new finder is created without datacenter context
func (c *Client) GetFinder() *find.Finder {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.finder != nil {
		return c.finder
	}

	return find.NewFinder(c.Client.Client, true)
}

// GetDatacenter returns the cached datacenter
func (c *Client) GetDatacenter() *object.Datacenter {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.datacenter
}

// GetDatacenterName returns the name of the cached datacenter
func (c *Client) GetDatacenterName() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.datacenterName
}

// Logout logs out from vCenter and closes the client
func (c *Client) Logout(ctx context.Context) error {
	err := c.Client.Logout(ctx)
	if err != nil {
		return &OperationError{Operation: "logout", Err: err}
	}
	return nil
}
