package clientmeta

import (
	"context"
	"crypto/tls"
	"crypto/x509"
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
		return accesstypes.FetchedMetadata{}, failed(accesstypes.ReasonFetchFailed, err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return accesstypes.FetchedMetadata{}, failed(reasonOf(err), err)
	}
	defer func() { _ = resp.Body.Close() }()
	switch {
	case resp.StatusCode >= 300 && resp.StatusCode < 400:
		return accesstypes.FetchedMetadata{}, unavailable(accesstypes.ReasonRedirect, fmt.Sprintf("status %d", resp.StatusCode))
	case resp.StatusCode != http.StatusOK:
		return accesstypes.FetchedMetadata{}, unavailable(accesstypes.ReasonStatus(resp.StatusCode), fmt.Sprintf("status %d", resp.StatusCode))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, accesstypes.ClientMetadataMaxBytes+1))
	if err != nil {
		return accesstypes.FetchedMetadata{}, failed(reasonOf(err), err)
	}
	if len(body) > accesstypes.ClientMetadataMaxBytes {
		return accesstypes.FetchedMetadata{}, unavailable(accesstypes.ReasonTooLarge,
			fmt.Sprintf("larger than %d bytes", accesstypes.ClientMetadataMaxBytes))
	}
	return accesstypes.FetchedMetadata{Body: body, CacheControl: resp.Header.Get("Cache-Control")}, nil
}

func unavailable(reason, detail string) error {
	return fmt.Errorf("%w: %w", ErrUnavailable, accesstypes.MetadataUnavailable(reason, detail))
}

func failed(reason string, cause error) error {
	return fmt.Errorf("%w: %w: %w", ErrUnavailable, accesstypes.MetadataUnavailable(reason, ""), cause)
}

func reasonOf(err error) string {
	var dnsErr *net.DNSError
	var netErr net.Error
	var certErr *tls.CertificateVerificationError
	var alert tls.AlertError
	var record tls.RecordHeaderError
	var unknownCA x509.UnknownAuthorityError
	var hostErr x509.HostnameError
	var invalidCert x509.CertificateInvalidError
	switch {
	case errors.Is(err, netguard.ErrBlocked):
		return accesstypes.ReasonBlockedAddress
	case errors.As(err, &dnsErr):
		return accesstypes.ReasonDNS
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &netErr) && netErr.Timeout():
		return accesstypes.ReasonTimeout
	case errors.As(err, &certErr), errors.As(err, &alert), errors.As(err, &record), errors.As(err, &unknownCA),
		errors.As(err, &hostErr), errors.As(err, &invalidCert):
		return accesstypes.ReasonTLS
	default:
		return accesstypes.ReasonConnection
	}
}
