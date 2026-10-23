package clientmeta

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/pkg/netguard"
	"svc-registry/pkg/outbound"
)

var ErrUnavailable = errors.New("client metadata document unavailable")

type FetcherClient struct{ client *http.Client }

func New(o outbound.Options) *FetcherClient {
	return NewWithPolicy(o, netguard.Policy{})
}

func NewWithPolicy(o outbound.Options, allow netguard.Allower) *FetcherClient {
	return newClient(netguard.NewTransport(o, allow, net.DefaultResolver))
}

func NewProxied(o outbound.Options) *FetcherClient {
	return newClient(netguard.NewProxyTransport(o, netguard.Policy{}, net.DefaultResolver))
}

func newClient(rt http.RoundTripper) *FetcherClient {
	return &FetcherClient{client: &http.Client{
		Transport:     rt,
		Timeout:       accesstypes.ClientMetadataTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

func (c *FetcherClient) Fetch(ctx context.Context, url string) (accesstypes.FetchedMetadata, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return accesstypes.FetchedMetadata{}, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return accesstypes.FetchedMetadata{}, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return accesstypes.FetchedMetadata{}, fmt.Errorf("%w: status %d", ErrUnavailable, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, accesstypes.ClientMetadataMaxBytes+1))
	if err != nil {
		return accesstypes.FetchedMetadata{}, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	if len(body) > accesstypes.ClientMetadataMaxBytes {
		return accesstypes.FetchedMetadata{}, fmt.Errorf("%w: larger than %d bytes", ErrUnavailable, accesstypes.ClientMetadataMaxBytes)
	}
	return accesstypes.FetchedMetadata{Body: body, CacheControl: resp.Header.Get("Cache-Control")}, nil
}
