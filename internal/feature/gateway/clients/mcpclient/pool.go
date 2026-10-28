package mcpclient

import (
	"context"
	"strings"
	"sync"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	gatewayports "svc-registry/internal/feature/gateway/ports"
)

type entry struct {
	mu   sync.Mutex
	cs   *sdk.ClientSession
	used time.Time
}

func (c *Client) session(ctx context.Context, t gatewayports.Target) (string, *entry, *sdk.ClientSession, error) {
	key := t.ServerID.String() + "|" + t.Version
	c.mu.Lock()
	c.sweep(time.Now())
	e := c.sessions[key]
	if e == nil {
		e = &entry{}
		c.sessions[key] = e
	}
	c.mu.Unlock()
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cs == nil {
		cs, err := c.connect(ctx, t)
		if err != nil {
			c.forget(key, e)
			return "", nil, nil, err
		}
		e.cs = cs
	}
	e.used = time.Now()
	return key, e, e.cs, nil
}

func (c *Client) drop(key string, e *entry) {
	c.forget(key, e)
	e.mu.Lock()
	cs := e.cs
	e.cs = nil
	e.mu.Unlock()
	if cs != nil {
		_ = cs.Close()
	}
}

func (c *Client) forget(key string, e *entry) {
	c.mu.Lock()
	if c.sessions[key] == e {
		delete(c.sessions, key)
	}
	c.mu.Unlock()
}

func (c *Client) sweep(now time.Time) {
	for key, e := range c.sessions {
		if !e.mu.TryLock() {
			continue
		}
		if e.cs != nil && now.Sub(e.used) > c.opts.Idle {
			_ = e.cs.Close()
			e.cs = nil
			delete(c.sessions, key)
		}
		e.mu.Unlock()
	}
}

func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key, e := range c.sessions {
		e.mu.Lock()
		if e.cs != nil {
			_ = e.cs.Close()
		}
		e.mu.Unlock()
		delete(c.sessions, key)
	}
}

func sessionGone(detail string) bool {
	return strings.Contains(detail, "session") && (strings.Contains(detail, "not found") || strings.Contains(detail, "missing") ||
		strings.Contains(detail, "closed"))
}
