package links

import (
	"net/netip"
	"strings"
)

var (
	alwaysDenied = []netip.Prefix{
		netip.MustParsePrefix("0.0.0.0/8"),
		netip.MustParsePrefix("169.254.0.0/16"),
		netip.MustParsePrefix("fe80::/10"),
		netip.MustParsePrefix("255.255.255.255/32"),
	}
	private = []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("172.16.0.0/12"),
		netip.MustParsePrefix("192.168.0.0/16"),
		netip.MustParsePrefix("100.64.0.0/10"),
		netip.MustParsePrefix("fc00::/7"),
	}
)

type Policy struct {
	AllowPrivate bool
	AllowHosts   []string
	DenyHosts    []string
}

func (p Policy) HostDenied(host string) bool { return matchHost(p.DenyHosts, host) }

func (p Policy) Allows(host string, ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || p.HostDenied(host) || ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() {
		return false
	}
	for _, n := range alwaysDenied {
		if n.Contains(ip) {
			return false
		}
	}
	for _, n := range private {
		if n.Contains(ip) {
			return p.AllowPrivate || matchHost(p.AllowHosts, host)
		}
	}
	return true
}

func matchHost(entries []string, host string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	for _, e := range entries {
		if domain, ok := strings.CutPrefix(e, "*."); ok {
			if strings.HasSuffix(host, "."+domain) {
				return true
			}
		} else if host == e {
			return true
		}
	}
	return false
}
