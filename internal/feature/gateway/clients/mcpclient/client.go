package mcpclient

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	gatewayports "svc-registry/internal/feature/gateway/ports"
	gatewaytypes "svc-registry/internal/feature/gateway/types"
	"svc-registry/pkg/netguard"
	"svc-registry/pkg/outbound"
)

type Options struct {
	Outbound    outbound.Options
	Environment outbound.Options
	Policy      netguard.Policy
	Resolver    *net.Resolver
	UserAgent   string
	Version     string
	MaxTools    int
	Idle        time.Duration
}

type Client struct {
	opts       Options
	mu         sync.Mutex
	transports map[string]http.RoundTripper
	sessions   map[string]*entry
	wrap       func(http.RoundTripper) http.RoundTripper
}

func (c *Client) Instrument(wrap func(http.RoundTripper) http.RoundTripper) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.wrap = wrap
}

func New(o Options) *Client {
	if o.Resolver == nil {
		o.Resolver = net.DefaultResolver
	}
	if o.Idle <= 0 {
		o.Idle = 2 * time.Minute
	}
	if o.MaxTools <= 0 {
		o.MaxTools = 200
	}
	return &Client{opts: o, transports: map[string]http.RoundTripper{}, sessions: map[string]*entry{}}
}

func (c *Client) client() *sdk.Client {
	return sdk.NewClient(&sdk.Implementation{Name: "svc-registry", Version: c.opts.Version}, nil)
}

func (c *Client) connect(ctx context.Context, t gatewayports.Target) (*sdk.ClientSession, error) {
	var transport sdk.Transport
	if t.Transport == gatewaytypes.TransportSSE {
		transport = &sdk.SSEClientTransport{Endpoint: t.URL, HTTPClient: c.httpClient(t)}
	} else {
		transport = &sdk.StreamableClientTransport{Endpoint: t.URL, HTTPClient: c.httpClient(t), MaxRetries: -1, DisableStandaloneSSE: true}
	}
	cs, err := c.client().Connect(ctx, transport, nil)
	if err != nil {
		return nil, failure(ctx, err)
	}
	return cs, nil
}

func (c *Client) Check(ctx context.Context, t gatewayports.Target) (*gatewaytypes.Result, error) {
	ctx, cancel := context.WithTimeout(ctx, t.Timeout)
	defer cancel()
	cs, err := c.connect(ctx, t)
	if err != nil {
		return nil, err
	}
	defer func() { _ = cs.Close() }()
	res := &gatewaytypes.Result{Info: info(cs)}
	for tool, err := range cs.Tools(ctx, nil) {
		if err != nil {
			return nil, failure(ctx, err)
		}
		if len(res.Tools) == c.opts.MaxTools {
			res.Truncated = true
			break
		}
		res.Tools = append(res.Tools, toolOf(tool))
	}
	var declared []sdk.Icon
	if r := cs.InitializeResult(); r != nil && r.ServerInfo != nil {
		declared = r.ServerInfo.Icons
	}
	res.Icon = c.icon(ctx, t, declared)
	return res, nil
}

func (c *Client) Call(ctx context.Context, t gatewayports.Target, tool string, args json.RawMessage) (*gatewayports.CallResult, error) {
	ctx, cancel := context.WithTimeout(ctx, t.Timeout)
	defer cancel()
	if t.Transport == gatewaytypes.TransportSSE {
		cs, err := c.connect(ctx, t)
		if err != nil {
			return nil, err
		}
		defer func() { _ = cs.Close() }()
		return call(ctx, cs, tool, args, t.MaxBytes)
	}
	for attempt := 0; ; attempt++ {
		key, e, cs, err := c.session(ctx, t)
		if err != nil {
			return nil, err
		}
		res, err := call(ctx, cs, tool, args, t.MaxBytes)
		if err == nil {
			return res, nil
		}
		var f *gatewaytypes.Failure
		if !errors.As(err, &f) || f.Code != gatewaytypes.FailProtocol || sessionGone(f.Detail) {
			c.drop(key, e)
		}
		if attempt > 0 || !retryable(err) {
			return nil, err
		}
	}
}

func call(ctx context.Context, cs *sdk.ClientSession, tool string, args json.RawMessage, max int64) (*gatewayports.CallResult, error) {
	params := &sdk.CallToolParams{Name: tool}
	if len(args) > 0 && string(args) != "null" {
		params.Arguments = args
	}
	res, err := cs.CallTool(ctx, params)
	if err != nil {
		return nil, failure(ctx, err)
	}
	raw, err := json.Marshal(res)
	if err != nil {
		return nil, gatewaytypes.Fail(gatewaytypes.FailProtocol, err.Error())
	}
	if max > 0 && int64(len(raw)) > max {
		return nil, gatewaytypes.Fail(gatewaytypes.FailTooLarge, "the answer is larger than MCP_GATEWAY_MAX_RESULT_BYTES")
	}
	return &gatewayports.CallResult{Raw: raw, IsError: res.IsError}, nil
}

func info(cs *sdk.ClientSession) gatewaytypes.ServerInfo {
	r := cs.InitializeResult()
	if r == nil {
		return gatewaytypes.ServerInfo{}
	}
	out := gatewaytypes.ServerInfo{ProtocolVersion: r.ProtocolVersion, Instructions: r.Instructions}
	if r.ServerInfo != nil {
		out.Name, out.Title, out.Version = r.ServerInfo.Name, r.ServerInfo.Title, r.ServerInfo.Version
	}
	return out
}

func toolOf(t *sdk.Tool) gatewaytypes.Tool {
	out := gatewaytypes.Tool{Name: t.Name, Title: t.Title, Description: t.Description}
	out.InputSchema = marshal(t.InputSchema)
	out.OutputSchema = marshal(t.OutputSchema)
	if t.Annotations != nil {
		out.Annotations = marshal(t.Annotations)
	}
	return out
}

func marshal(v any) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil || string(raw) == "null" {
		return nil
	}
	return raw
}

func failure(ctx context.Context, err error) error {
	var f *gatewaytypes.Failure
	switch {
	case errors.As(err, &f):
		return f
	case errors.Is(err, sdk.ErrSessionMissing), errors.Is(err, sdk.ErrConnectionClosed):
		return gatewaytypes.Fail(gatewaytypes.FailUnreachable, "the server closed the session")
	case errors.Is(err, context.DeadlineExceeded), errors.Is(ctx.Err(), context.DeadlineExceeded):
		return gatewaytypes.Fail(gatewaytypes.FailTimeout, "the server did not answer in time")
	case errors.Is(err, netguard.ErrBlocked):
		return gatewaytypes.Fail(gatewaytypes.FailBlocked, err.Error())
	}
	var netErr net.Error
	var opErr *net.OpError
	var dnsErr *net.DNSError
	if errors.As(err, &opErr) || errors.As(err, &dnsErr) || errors.As(err, &netErr) {
		return gatewaytypes.Fail(gatewaytypes.FailUnreachable, err.Error())
	}
	return gatewaytypes.Fail(gatewaytypes.FailProtocol, err.Error())
}

func retryable(err error) bool {
	var f *gatewaytypes.Failure
	if !errors.As(err, &f) {
		return false
	}
	return f.Code == gatewaytypes.FailUnreachable || (f.Code == gatewaytypes.FailProtocol && sessionGone(f.Detail))
}
