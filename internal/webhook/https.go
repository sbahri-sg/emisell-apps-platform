package webhook

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"time"

	"emisell.app/platform/pkg/appapi"
	"emisell.app/platform/pkg/sdk/events"
	"emisell.app/platform/pkg/webhookconfig"
)

// HTTPSDelivery is resolved from trusted configuration by the caller, never from
// the producer's event payload. Sending does not authorize an installation.
type HTTPSDelivery struct {
	Delivery
	Endpoint, Topic, APIVersion string
}

type httpsSender struct {
	lookup    func(context.Context, string, string) ([]net.IP, error)
	dial      func(context.Context, string, string) (net.Conn, error)
	tlsConfig *tls.Config
}

// SendHTTPS must run inside the installation lifecycle gate after current grant
// and source release checks. It does not reuse the local fixture's HTTP client.
func SendHTTPS(ctx context.Context, d HTTPSDelivery, secret string) (string, string) {
	dialer := &net.Dialer{Timeout: time.Second}
	return (httpsSender{lookup: net.DefaultResolver.LookupIP, dial: dialer.DialContext}).send(ctx, d, secret)
}

// Exclude special-purpose networks in addition to RFC1918 and link-local IPs.
var blockedWebhookNetworks = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"), netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"), netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"), netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"), netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("3fff::/20"),
}

func publicWebhookIP(ip net.IP) bool {
	a, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	a = a.Unmap()
	if !a.IsGlobalUnicast() || a.IsPrivate() || a.IsLoopback() || a.IsLinkLocalUnicast() {
		return false
	}
	// Conservatively allow only the currently allocated global-unicast IPv6 range.
	if a.Is6() && !netip.MustParsePrefix("2000::/3").Contains(a) {
		return false
	}
	for _, p := range blockedWebhookNetworks {
		if p.Contains(a) {
			return false
		}
	}
	return true
}

func (s httpsSender) send(ctx context.Context, d HTTPSDelivery, secret string) (string, string) {
	_, topicErr := webhookconfig.RequiredScope(d.Topic)
	if !webhookconfig.ValidEndpoint(d.Endpoint) || len(d.Body) == 0 || len(d.Body) > 1<<20 || secret == "" || !events.Token.MatchString(d.ID) || !events.Token.MatchString(d.Tenant) || !events.Token.MatchString(d.Installation) || !events.Token.MatchString(d.EventID) || topicErr != nil || d.APIVersion != webhookconfig.APIVersion {
		return "dead", "invalid_delivery"
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.Endpoint, bytes.NewReader(d.Body))
	if err != nil {
		return "dead", "invalid_delivery"
	}
	host := req.URL.Hostname()
	ips, err := s.lookup(ctx, "ip", host)
	if err != nil || len(ips) == 0 {
		return "pending", "dns_unavailable"
	}
	// Reject mixed public/private answers; never silently select just the public one.
	for _, ip := range ips {
		if !publicWebhookIP(ip) {
			return "dead", "endpoint_not_public"
		}
	}
	tlsConfig := s.tlsConfig
	if tlsConfig == nil {
		tlsConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	} else {
		tlsConfig = tlsConfig.Clone()
	}
	tlsConfig.ServerName = host
	transport := &http.Transport{Proxy: nil, DisableKeepAlives: true, TLSClientConfig: tlsConfig, TLSHandshakeTimeout: time.Second, MaxResponseHeaderBytes: 32 << 10,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			// Dial numeric IPs from this validated resolution only. No second DNS lookup.
			expected := net.JoinHostPort(host, "443")
			if address != expected {
				return nil, errors.New("unexpected webhook host")
			}
			var err error
			for _, ip := range ips {
				var conn net.Conn
				conn, err = s.dial(ctx, "tcp", net.JoinHostPort(ip.String(), "443"))
				if err == nil {
					return conn, nil
				}
			}
			return nil, err
		},
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	stamp := strconv.FormatInt(time.Now().Unix(), 10)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Emisell-Tenant", d.Tenant)
	req.Header.Set("X-Emisell-Installation", d.Installation)
	req.Header.Set("X-Emisell-Delivery", d.ID)
	req.Header.Set("X-Emisell-Event", d.EventID)
	req.Header.Set("X-Emisell-Topic", d.Topic)
	req.Header.Set("X-Emisell-API-Version", d.APIVersion)
	req.Header.Set("X-Emisell-Timestamp", stamp)
	req.Header.Set("X-Emisell-Signature", appapi.SignWebhook(secret, d.Tenant, d.Installation, d.ID, stamp, d.Body))
	resp, err := client.Do(req)
	if err != nil {
		return "pending", "receiver_unavailable"
	}
	resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return "delivered", ""
	}
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return "dead", "redirect_forbidden"
	}
	return "pending", "receiver_rejected"
}
