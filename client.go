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

// Client är en wrapper runt govmomi.Client med cached datacenter och helper-funktioner
type Client struct {
	*govmomi.Client
	datacenter     *object.Datacenter
	datacenterName string
	finder         *find.Finder
	mu             sync.RWMutex
}

// ConnectConfig innehåller konfiguration för att ansluta till vCenter
type ConnectConfig struct {
	Host       string
	Username   string
	Password   string
	Insecure   bool
	Datacenter string
}

// ConnectWithPassword ansluter till vCenter med username/password
// Använder go-vcenter-auth för session caching och automatisk återanslutning
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

	// Använd go-vcenter-auth för att logga in med session caching
	authClient, err := vcauth.Login(ctx, config.Host, config.Username, config.Password, config.Insecure)
	if err != nil {
		return nil, &OperationError{Operation: "login", Err: err}
	}

	// Hämta vim25.Client från go-vcenter-auth
	vim25Client := authClient.GetVim()

	// Skapa govmomi.Client från vim25.Client
	govmomiClient := &govmomi.Client{
		Client:         vim25Client,
		SessionManager: session.NewManager(vim25Client),
	}

	// Skapa vår Client wrapper
	client := &Client{
		Client:         govmomiClient,
		datacenterName: config.Datacenter,
	}

	// Om datacenter är specificerat, cacha det
	if config.Datacenter != "" {
		err = client.SetDatacenter(ctx, config.Datacenter)
		if err != nil {
			return nil, err
		}
	}

	return client, nil
}

// ConnectWithSSPI ansluter till vCenter med Windows SSPI/Kerberos authentication
// Detta fungerar endast på Windows och använder den inloggade användarens credentials
// Returnerar ErrSSPINotSupported på icke-Windows plattformar
func ConnectWithSSPI(ctx context.Context, host string, insecure bool, datacenter string) (*Client, error) {
	if host == "" {
		return nil, &ValidationError{Field: "Host", Message: "host is required"}
	}

	// Använd go-vcenter-auth för SSPI login med session caching
	authClient, err := vcauth.LoginSSPI(ctx, host, insecure)
	if err != nil {
		return nil, &OperationError{Operation: "SSPI login", Err: err}
	}

	// Hämta vim25.Client från go-vcenter-auth
	vim25Client := authClient.GetVim()

	// Skapa govmomi.Client från vim25.Client
	govmomiClient := &govmomi.Client{
		Client:         vim25Client,
		SessionManager: session.NewManager(vim25Client),
	}

	// Skapa vår Client wrapper
	client := &Client{
		Client:         govmomiClient,
		datacenterName: datacenter,
	}

	// Om datacenter är specificerat, cacha det
	if datacenter != "" {
		err = client.SetDatacenter(ctx, datacenter)
		if err != nil {
			return nil, err
		}
	}

	return client, nil
}

// ConnectWithURL ansluter till vCenter med en komplett URL (inkl credentials)
// Använder standard govmomi utan go-vcenter-auth
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

// SetDatacenter sätter datacenter för klienten och cachar finder
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

// GetFinder returnerar en finder för det cachade datacentret
// Om inget datacenter är cachat, skapas en ny finder utan datacenter-context
func (c *Client) GetFinder() *find.Finder {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.finder != nil {
		return c.finder
	}

	return find.NewFinder(c.Client.Client, true)
}

// GetDatacenter returnerar det cachade datacentret
func (c *Client) GetDatacenter() *object.Datacenter {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.datacenter
}

// GetDatacenterName returnerar namnet på det cachade datacentret
func (c *Client) GetDatacenterName() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.datacenterName
}

// Logout loggar ut från vCenter och stänger klienten
func (c *Client) Logout(ctx context.Context) error {
	err := c.Client.Logout(ctx)
	if err != nil {
		return &OperationError{Operation: "logout", Err: err}
	}
	return nil
}
