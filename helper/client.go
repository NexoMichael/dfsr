package helper

import (
	"strings"
	"sync"
	"time"

	"github.com/go-ole/go-ole"
	"gopkg.in/dfsr.v0/versionvector"
)

// Client provides a threadsafe and efficient means of querying DFSR backlog
// and report information. It maintains an expiring cache of version vectors
// and attempts to manage DFSR queries in such a way that they do not overburden
// the target servers.
type Client struct {
	m             sync.RWMutex
	caching       bool
	cacheDuration time.Duration
	servers       map[string]Reporter // Maps lower-case FQDNs to the Reporter inferface for each server
}

// NewClient creates a new Client that is capable of querying DFSR members via
// the DFSR Helper protocol. The returned Client will not cache version vectors.
func NewClient() (*Client, error) {
	return &Client{
		servers: make(map[string]Reporter),
	}, nil
}

// NewCachingClient creates a new Client that is capable of querying DFSR
// members via the DFSR Helper protocol. The returned Client will cache version
// vectors for the given cache duration.
func NewCachingClient(cacheDuration time.Duration) (*Client, error) {
	return &Client{
		caching:       true,
		cacheDuration: cacheDuration,
		servers:       make(map[string]Reporter),
	}, nil
}

// Close will release any resources consumed by the Client.
func (c *Client) Close() {
	c.m.Lock()
	defer c.m.Lock()
	for _, r := range c.servers {
		r.Close()
	}
	c.servers = make(map[string]Reporter)
}

// Backlog returns the outgoing backlog from one DSFR member to another. The
// backlog of each replicated folder within the requested group is returned.
// The members are identified by their fully qualified domain names.
func (c *Client) Backlog(from, to string, group ole.GUID) (backlog []int, err error) {
	f, err := c.server(from)
	if err != nil {
		return nil, err
	}

	t, err := c.server(to)
	if err != nil {
		return nil, err
	}

	v, err := t.Vector(group)
	if err != nil {
		return nil, err
	}
	defer v.Close()

	return f.Backlog(v)
}

// Vector returns the current referece version vector for the specified
// replication group on requested DFSR member. The member is identified by its
// fully qualified domain name.
func (c *Client) Vector(server string, group *ole.GUID) (vector *versionvector.Vector, err error) {
	s, err := c.server(server)
	if err != nil {
		return nil, err
	}

	return s.Vector(*group)
}

// Report generates a report for the requested replication group.
func (c *Client) Report(server string, group *ole.GUID, vector *versionvector.Vector, backlog, files bool) (data *ole.SafeArrayConversion, report string, err error) {
	s, err := c.server(server)
	if err != nil {
		return nil, "", err
	}

	return s.Report(group, vector, backlog, files)
}

func (c *Client) server(fqdn string) (r Reporter, err error) {
	fqdn = strings.ToLower(fqdn)
	c.m.RLock()
	r, found := c.servers[fqdn]
	c.m.RUnlock()
	if found {
		return r, nil
	}
	c.m.Lock()
	defer c.m.Unlock()
	r, err = c.create(fqdn)
	if err != nil {
		return
	}
	c.servers[fqdn] = r
	return
}

func (c *Client) create(fqdn string) (r Reporter, err error) {
	r, err = NewReporter(fqdn)
	if err != nil {
		return
	}
	r, err = NewLimiter(r, 1)
	if err != nil {
		return
	}
	if c.caching {
		r = NewCacher(r, c.cacheDuration)
	}
	return
}