// Package endpointproof verifies a bounded HTTPS origin-control challenge.
// No app credentials, tenant identifiers or expected nonce are sent outbound.
package endpointproof

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"emisell.app/platform/internal/oauth/appclient"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"time"
)

var denied = errors.New("endpoint proof could not be verified")
var challengePath = regexp.MustCompile(`^/\.well-known/emisell-app-verification/proof_[A-Z2-7]{26}$`)
var hostname = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9.-]{0,251}[a-z0-9])?$`)
var blocked = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"), netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("169.254.0.0/16"), netip.MustParsePrefix("172.16.0.0/12"), netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"), netip.MustParsePrefix("192.88.99.0/24"), netip.MustParsePrefix("192.168.0.0/16"), netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("198.51.100.0/24"), netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("224.0.0.0/4"), netip.MustParsePrefix("240.0.0.0/4"), netip.MustParsePrefix("168.63.129.16/32"),
}

func public(ip netip.Addr) bool {
	if !ip.Is4() || !ip.IsGlobalUnicast() {
		return false
	}
	for _, p := range blocked {
		if p.Contains(ip) {
			return false
		}
	}
	return true
}

type Verifier struct {
	slots  chan struct{}
	lookup func(context.Context, string, string) ([]netip.Addr, error)
	dial   func(context.Context, string, string) (net.Conn, error)
	roots  *x509.CertPool // nil in production; never configurable from app input.
}

func New() *Verifier {
	d := &net.Dialer{Timeout: 2 * time.Second}
	return &Verifier{slots: make(chan struct{}, 4), lookup: net.DefaultResolver.LookupNetIP, dial: d.DialContext}
}
func (v *Verifier) Verify(ctx context.Context, raw string, expected appclient.Proof) error {
	select {
	case v.slots <- struct{}{}:
		defer func() { <-v.slots }()
	default:
		return denied
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	u, err := url.Parse(raw)
	if err != nil || len(raw) > 2048 || u.Scheme != "https" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" || u.RawPath != "" || (u.Port() != "" && u.Port() != "443") || !challengePath.MatchString(u.Path) || !hostname.MatchString(u.Hostname()) || net.ParseIP(u.Hostname()) != nil {
		return denied
	}
	// IPv4-only v1. Validate every A answer, then dial ONE pinned numeric IP.
	// Proxy env, IPv6/NAT64, redirects and a second DNS lookup cannot bypass it.
	ips, err := v.lookup(ctx, "ip4", u.Hostname())
	if err != nil || len(ips) == 0 || len(ips) > 16 {
		return denied
	}
	for _, ip := range ips {
		if !public(ip) {
			return denied
		}
	}
	target := net.JoinHostPort(ips[0].String(), "443")
	t := &http.Transport{Proxy: nil, DisableCompression: true, DisableKeepAlives: true, MaxResponseHeaderBytes: 16 << 10, ResponseHeaderTimeout: 2 * time.Second, TLSHandshakeTimeout: 2 * time.Second, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, ServerName: u.Hostname(), RootCAs: v.roots}, DialContext: func(c context.Context, network, address string) (net.Conn, error) {
		if address != net.JoinHostPort(u.Hostname(), "443") {
			return nil, denied
		}
		return v.dial(c, "tcp4", target)
	}}
	defer t.CloseIdleConnections()
	c := &http.Client{Transport: t, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, err := http.NewRequestWithContext(ctx, "GET", raw, nil)
	if err != nil {
		return denied
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Emisell-Endpoint-Proof/1")
	r, err := c.Do(req)
	if err != nil {
		return denied
	}
	defer r.Body.Close()
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" || r.StatusCode != 200 || r.Header.Get("Content-Encoding") != "" {
		return denied
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 4097))
	if err != nil || len(body) > 4096 {
		return denied
	}
	if !matches(body, expected) {
		return denied
	}
	return nil
}
func matches(raw []byte, p appclient.Proof) bool {
	d := json.NewDecoder(bytes.NewReader(raw))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return false
	}
	values := map[string]string{}
	for d.More() {
		k, e := d.Token()
		if e != nil {
			return false
		}
		key, ok := k.(string)
		if !ok {
			return false
		}
		if _, seen := values[key]; seen {
			return false
		}
		var value string
		if d.Decode(&value) != nil {
			return false
		}
		values[key] = value
	}
	if token, err = d.Token(); err != nil || token != json.Delim('}') {
		return false
	}
	if _, err = d.Token(); err != io.EOF {
		return false
	}
	return len(values) == 4 && values["schema"] == p.Schema && values["clientId"] == p.ClientID && values["releaseSha256"] == p.ReleaseSHA256 && values["challenge"] == p.Challenge
}
