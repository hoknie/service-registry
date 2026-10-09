package linkcheck

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"syscall"
	"time"

	"svc-registry/internal/config"
)

type Policy interface {
	Allows(host string, ip netip.Addr) bool
}

var errBlocked = errors.New("address forbidden by the link check policy")

func newTransport(out config.OutboundConfig, policy Policy, resolver *net.Resolver) *http.Transport {
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	if len(out.CAPEM) > 0 {
		roots.AppendCertsFromPEM(out.CAPEM)
	}
	return &http.Transport{
		Proxy:                 nil,
		DialContext:           dialer(policy, resolver),
		TLSClientConfig:       &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout:   10 * time.Second,
		IdleConnTimeout:       30 * time.Second,
		MaxIdleConnsPerHost:   2,
		ForceAttemptHTTP2:     true,
		DisableCompression:    true,
		ResponseHeaderTimeout: 60 * time.Second,
	}
}

func dialer(policy Policy, resolver *net.Resolver) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		var ips []netip.Addr
		if ip, err := netip.ParseAddr(host); err == nil {
			ips = []netip.Addr{ip}
		} else if ips, err = resolver.LookupNetIP(ctx, "ip", host); err != nil {
			return nil, err
		}
		for _, ip := range ips {
			if !policy.Allows(host, ip) {
				return nil, fmt.Errorf("%s (%s): %w", host, ip, errBlocked)
			}
		}
		d := net.Dialer{
			Timeout: 10 * time.Second,
			Control: func(_, address string, _ syscall.RawConn) error {
				ap, err := netip.ParseAddrPort(address)
				if err != nil || !policy.Allows(host, ap.Addr()) {
					return errBlocked
				}
				return nil
			},
		}
		var last error
		for _, ip := range ips {
			conn, err := d.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if err == nil {
				return conn, nil
			}
			last = err
		}
		if last == nil {
			last = &net.DNSError{Err: "no addresses", Name: host, IsNotFound: true}
		}
		return nil, last
	}
}
