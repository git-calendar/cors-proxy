package proxy

import (
	"context"
	"fmt"
	"net"
	"net/netip"
)

type ssrfSafeDialer struct {
	lookupNetIP func(context.Context, string, string) ([]netip.Addr, error)
	dialContext func(context.Context, string, string) (net.Conn, error)
}

func newSSRFSafeDialer(resolver *net.Resolver, dialer *net.Dialer) *ssrfSafeDialer {
	return &ssrfSafeDialer{resolver.LookupNetIP, dialer.DialContext}
}

func (d *ssrfSafeDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	ips := []netip.Addr{}
	if ip, parseErr := netip.ParseAddr(host); parseErr == nil {
		ips = append(ips, ip)
	} else {
		ips, err = d.lookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, err
		}
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("upstream host %q resolved to no addresses", host)
	}

	// Reject the complete DNS answer if any address is unsafe. This prevents a
	// resolver or attacker from hiding a private address in a mixed response.
	for i, ip := range ips {
		ips[i] = ip.Unmap()
		if isForbiddenUpstreamIP(ips[i]) {
			return nil, fmt.Errorf("upstream address %s is not publicly routable", ip)
		}
	}

	// Dial the validated IP directly so DNS cannot change the destination.
	var connection net.Conn
	for _, ip := range ips {
		connection, err = d.dialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return connection, nil
		}
	}
	return nil, err
}

func isForbiddenUpstreamIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	return !ip.IsValid() || !ip.IsGlobalUnicast() || ip.IsPrivate() ||
		(ip.Is4() && ip.As4()[0] == 0)
}
