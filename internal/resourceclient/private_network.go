package resourceclient

import (
	"context"
	"errors"
	"net"
	"time"
)

func privateResourceIP(ip net.IP) bool {
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate())
}

// DNS is resolved and checked on every new connection, then a numeric address
// is dialed. Mixed public/private answers, metadata/link-local, unspecified and
// public addresses are denied. Redirects and environment proxies stay disabled.
func privateResourceDialer(expectedHost string) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		denied := errors.New("private resource destination unavailable")
		host, port, err := net.SplitHostPort(address)
		if err != nil || host != expectedHost {
			return nil, denied
		}
		lookup, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		ips, err := net.DefaultResolver.LookupIPAddr(lookup, host)
		if err != nil || len(ips) == 0 {
			return nil, denied
		}
		for _, ip := range ips {
			if !privateResourceIP(ip.IP) || ip.Zone != "" {
				return nil, denied
			}
		}
		dialer := net.Dialer{Timeout: 3 * time.Second}
		for _, ip := range ips {
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
			if err == nil {
				return conn, nil
			}
			if ctx.Err() != nil {
				break
			}
		}
		return nil, denied
	}
}
