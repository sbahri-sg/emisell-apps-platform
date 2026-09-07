package webhook

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"emisell.app/platform/pkg/appapi"
	"emisell.app/platform/pkg/webhookconfig"
)

func TestWebhookPublicIPPolicy(t *testing.T) {
	for _, ip := range []string{"127.0.0.1", "10.1.1.1", "169.254.169.254", "172.16.0.1", "192.168.0.50", "100.100.100.200", "0.0.0.0", "192.0.2.1", "198.18.0.1", "224.0.0.1", "255.255.255.255", "::1", "::ffff:127.0.0.1", "fc00::1", "fe80::1", "64:ff9b::a00:1", "2002:a00:1::1", "2001:db8::1"} {
		if publicWebhookIP(net.ParseIP(ip)) {
			t.Error("accepted", ip)
		}
	}
	for _, ip := range []string{"8.8.8.8", "1.1.1.1", "2606:4700:4700::1111"} {
		if !publicWebhookIP(net.ParseIP(ip)) {
			t.Error("rejected", ip)
		}
	}
}

func TestHTTPSInvalidDeliveryNeverResolvesOrDials(t *testing.T) {
	sender := httpsSender{
		lookup: func(context.Context, string, string) ([]net.IP, error) {
			t.Fatal("invalid delivery resolved DNS")
			return nil, nil
		},
		dial: func(context.Context, string, string) (net.Conn, error) {
			t.Fatal("invalid delivery dialled")
			return nil, nil
		},
	}
	base := HTTPSDelivery{Delivery: Delivery{ID: "delivery_1", EventID: "event_1", Tenant: "merchant_1", Installation: "ins_1", Body: []byte(`{}`)}, Endpoint: "https://example.com/events", Topic: "products.created", APIVersion: webhookconfig.APIVersion}
	for _, mutate := range []func(*HTTPSDelivery){
		func(d *HTTPSDelivery) { d.Endpoint = "https://user:secret@example.com/events" },
		func(d *HTTPSDelivery) { d.Endpoint = "https://127.0.0.1/events" },
		func(d *HTTPSDelivery) { d.Endpoint = "https://example.com/events?secret=x" },
		func(d *HTTPSDelivery) { d.Topic = "*" },
		func(d *HTTPSDelivery) { d.Tenant = "merchant\r\nInjected: x" },
		func(d *HTTPSDelivery) { d.EventID = "" },
		func(d *HTTPSDelivery) { d.APIVersion = "unknown" },
		func(d *HTTPSDelivery) { d.Body = make([]byte, (1<<20)+1) },
	} {
		d := base
		mutate(&d)
		if status, reason := sender.send(context.Background(), d, "test-only"); status != "dead" || reason != "invalid_delivery" {
			t.Fatal(status, reason)
		}
	}
	if status, _ := sender.send(context.Background(), base, ""); status != "dead" {
		t.Fatal("empty secret accepted")
	}
}

func TestHTTPSWebhookPinnedTLSAndOutcomes(t *testing.T) {
	var calls atomic.Int32
	var status atomic.Int32
	status.Store(204)
	const secret = "test-only-receiver-secret"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		body, _ := io.ReadAll(r.Body)
		if !appapi.VerifyWebhook(secret, r.Header.Get("X-Emisell-Tenant"), r.Header.Get("X-Emisell-Installation"), r.Header.Get("X-Emisell-Delivery"), r.Header.Get("X-Emisell-Timestamp"), r.Header.Get("X-Emisell-Signature"), body, time.Now()) {
			t.Error("signature invalid")
		}
		if r.Host != "example.com" || r.Header.Get("X-Emisell-Event") != "event_1" {
			t.Error("wrong host/event")
		}
		w.Header().Set("Location", "https://127.0.0.1/private")
		w.WriteHeader(int(status.Load()))
	}))
	defer server.Close()
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	var lookups, dials atomic.Int32
	sender := httpsSender{
		lookup: func(context.Context, string, string) ([]net.IP, error) {
			lookups.Add(1)
			return []net.IP{net.ParseIP("8.8.8.8")}, nil
		},
		dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			dials.Add(1)
			if address != "8.8.8.8:443" {
				t.Errorf("not pinned: %s", address)
			}
			return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
		},
		tlsConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12},
	}
	d := HTTPSDelivery{Delivery: Delivery{ID: "delivery_1", EventID: "event_1", Tenant: "merchant_1", Installation: "ins_1", Body: []byte(`{"id":"product_1"}`)}, Endpoint: "https://example.com/events", Topic: "products.created", APIVersion: webhookconfig.APIVersion}
	for _, tc := range []struct {
		code int32
		want string
	}{{204, "delivered"}, {503, "pending"}, {429, "pending"}, {302, "dead"}} {
		status.Store(tc.code)
		got, reason := sender.send(context.Background(), d, secret)
		if got != tc.want {
			t.Fatalf("status %d: %s %s", tc.code, got, reason)
		}
	}
	if calls.Load() != 4 || lookups.Load() != 4 || dials.Load() != 4 {
		t.Fatal("redirect or unexpected lookup", calls.Load(), lookups.Load(), dials.Load())
	}
	sender.lookup = func(context.Context, string, string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("8.8.8.8"), net.ParseIP("127.0.0.1")}, nil
	}
	got, reason := sender.send(context.Background(), d, secret)
	if got != "dead" || reason != "endpoint_not_public" || dials.Load() != 4 {
		t.Fatal("mixed DNS answers not blocked")
	}
	sender.lookup = func(context.Context, string, string) ([]net.IP, error) { return []net.IP{net.ParseIP("8.8.8.8")}, nil }
	sender.tlsConfig = nil
	got, _ = sender.send(context.Background(), d, secret)
	if got != "pending" || calls.Load() != 4 {
		t.Fatal("untrusted TLS certificate accepted")
	}
}
