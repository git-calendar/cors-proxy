package main

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"testing"
)

func TestIsForbiddenUpstreamIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		address   string
		forbidden bool
	}{
		{name: "current network", address: "0.42.42.42", forbidden: true},
		{name: "private 10/8", address: "10.0.0.1", forbidden: true},
		{name: "private 172.16/12 start", address: "172.16.0.1", forbidden: true},
		{name: "private 172.16/12 end", address: "172.31.255.255", forbidden: true},
		{name: "private 192.168/16", address: "192.168.1.1", forbidden: true},
		{name: "loopback", address: "127.0.0.1", forbidden: true},
		{name: "loopback subnet", address: "127.255.255.255", forbidden: true},
		{name: "link local metadata", address: "169.254.169.254", forbidden: true},
		{name: "IPv6 unspecified", address: "::", forbidden: true},
		{name: "IPv6 loopback", address: "::1", forbidden: true},
		{name: "IPv6 private", address: "fd00:ec2::254", forbidden: true},
		{name: "IPv6 link local", address: "fe80::1", forbidden: true},
		{name: "public IPv4", address: "8.8.8.8", forbidden: false},
		{name: "below private 172 range", address: "172.15.255.255", forbidden: false},
		{name: "above private 172 range", address: "172.32.0.0", forbidden: false},
		{name: "public IPv6", address: "2606:4700:4700::1111", forbidden: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			address := netip.MustParseAddr(test.address)
			if got := isForbiddenUpstreamIP(address); got != test.forbidden {
				t.Fatalf("isForbiddenUpstreamIP(%s) = %t, want %t", address, got, test.forbidden)
			}
		})
	}
}

func TestSSRFSafeDialerRejectsMixedDNSResponse(t *testing.T) {
	t.Parallel()

	dialCalled := false
	dialer := &ssrfSafeDialer{
		lookupNetIP: func(context.Context, string, string) ([]netip.Addr, error) {
			return []netip.Addr{
				netip.MustParseAddr("93.184.216.34"),
				netip.MustParseAddr("169.254.169.254"),
			}, nil
		},
		dialContext: func(context.Context, string, string) (net.Conn, error) {
			dialCalled = true
			return nil, errors.New("unexpected dial")
		},
	}

	_, err := dialer.DialContext(context.Background(), "tcp", "example.com:443")
	if err == nil {
		t.Fatal("DialContext() succeeded, want forbidden address error")
	}
	if dialCalled {
		t.Fatal("DialContext() attempted a connection after resolving a forbidden address")
	}
}

func TestSSRFSafeDialerPinsConnectionToResolvedIP(t *testing.T) {
	t.Parallel()

	errDial := errors.New("dial stopped by test")
	var dialedAddress string
	dialer := &ssrfSafeDialer{
		lookupNetIP: func(_ context.Context, network, host string) ([]netip.Addr, error) {
			if network != "ip" || host != "example.com" {
				t.Fatalf("resolved network/host = %q/%q, want ip/example.com", network, host)
			}
			return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
		},
		dialContext: func(_ context.Context, network, address string) (net.Conn, error) {
			if network != "tcp" {
				t.Fatalf("dial network = %q, want tcp", network)
			}
			dialedAddress = address
			return nil, errDial
		},
	}

	_, err := dialer.DialContext(context.Background(), "tcp", "example.com:443")
	if !errors.Is(err, errDial) {
		t.Fatalf("DialContext() error = %v, want %v", err, errDial)
	}
	if dialedAddress != "93.184.216.34:443" {
		t.Fatalf("dialed address = %q, want 93.184.216.34:443", dialedAddress)
	}
}

func TestSSRFSafeDialerRejectsPrivateLiteralIP(t *testing.T) {
	t.Parallel()

	lookupCalled := false
	dialCalled := false
	dialer := &ssrfSafeDialer{
		lookupNetIP: func(context.Context, string, string) ([]netip.Addr, error) {
			lookupCalled = true
			return nil, errors.New("unexpected lookup")
		},
		dialContext: func(context.Context, string, string) (net.Conn, error) {
			dialCalled = true
			return nil, errors.New("unexpected dial")
		},
	}

	_, err := dialer.DialContext(context.Background(), "tcp", "127.0.0.1:80")
	if err == nil {
		t.Fatal("DialContext() succeeded, want forbidden address error")
	}
	if lookupCalled || dialCalled {
		t.Fatalf("literal private IP caused lookup=%t dial=%t, want both false", lookupCalled, dialCalled)
	}
}
