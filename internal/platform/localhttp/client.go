// Package localhttp is a narrow, explicitly development-only egress policy.
package localhttp

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"time"
)

type bounded struct {
	transport *http.Transport
	origin    string
}

func (b bounded) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Scheme+"://"+r.URL.Host != b.origin || r.URL.User != nil {
		return nil, errors.New("egress target denied")
	}
	resp, err := b.transport.RoundTrip(r)
	if err == nil {
		resp.Body = http.MaxBytesReader(nil, resp.Body, 32<<10)
	}
	return resp, err
}
func Client(origin string) (*http.Client, error) {
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("local remote app requires an explicit loopback origin")
	}
	dialer := net.Dialer{Timeout: time.Second}
	transport := &http.Transport{Proxy: nil, DisableCompression: true, MaxIdleConnsPerHost: 8, MaxConnsPerHost: 8, IdleConnTimeout: 30 * time.Second, ResponseHeaderTimeout: 2 * time.Second, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != u.Host {
			return nil, errors.New("egress target denied")
		}
		return dialer.DialContext(ctx, "tcp", u.Host)
	}}
	return &http.Client{Transport: bounded{transport, origin}, Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, nil
}
