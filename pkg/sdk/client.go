// Package sdk is the provider-neutral Emisell Core client boundary.
package sdk

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"connectrpc.com/connect"
	intent "emisell.app/platform/pkg/sdk/gen/emisell/installation/v1/installationv1connect"
	integration "emisell.app/platform/pkg/sdk/gen/emisell/integration/v1/integrationv1connect"
	pay "emisell.app/platform/pkg/sdk/gen/emisell/payment/v1/paymentv1connect"
	ship "emisell.app/platform/pkg/sdk/gen/emisell/shipping/v1/shippingv1connect"
	testing "emisell.app/platform/pkg/sdk/gen/emisell/testing/v1/testingv1connect"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

type Client struct {
	TestDistribution testing.TestDistributionServiceClient
	Connection       integration.ConnectionServiceClient
	Payment          pay.PaymentServiceClient
	Shipping         ship.ShippingServiceClient
	InstallIntents   intent.InstallIntentServiceClient
	Installations    intent.InstallationServiceClient
}

// NewLocalClient permits only the development loopback listener. No redirects or
// environment proxy may forward its bearer token. Production needs TLS identity.
func NewLocalClient(address, token string) (*Client, error) {
	u, err := url.Parse(address)
	validToken := len(token) == 43 || (len(token) == 47 && strings.HasPrefix(token, "epk_"))
	if err != nil || u.Scheme != "http" || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" || !validToken {
		return nil, errors.New("invalid local RPC configuration")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	client := &http.Client{Timeout: 8 * time.Second, Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	auth := connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, r connect.AnyRequest) (connect.AnyResponse, error) {
			r.Header().Set("Authorization", "Bearer "+token)
			otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(r.Header()))
			return next(ctx, r)
		}
	})
	// No hidden retries: the caller owns the persistent business idempotency key.
	return &Client{
		Connection:       integration.NewConnectionServiceClient(client, address, connect.WithInterceptors(auth)),
		TestDistribution: testing.NewTestDistributionServiceClient(client, address, connect.WithInterceptors(auth)),
		Payment:          pay.NewPaymentServiceClient(client, address, connect.WithInterceptors(auth)),
		Shipping:         ship.NewShippingServiceClient(client, address, connect.WithInterceptors(auth)),
		InstallIntents:   intent.NewInstallIntentServiceClient(client, address, connect.WithInterceptors(auth)),
		Installations:    intent.NewInstallationServiceClient(client, address, connect.WithInterceptors(auth)),
	}, nil
}
