// Package connectapi adapts internal RPC to the existing capability use case.
package connectapi

import (
	"context"
	"crypto/subtle"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"connectrpc.com/connect"
	appservice "emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/capability"
	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/installation/domain"
	installservice "emisell.app/platform/internal/installation/service"
	"emisell.app/platform/internal/oauth/appclient"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/ids"
	"emisell.app/platform/internal/resourceclient"
	"emisell.app/platform/pkg/merchantid"
	engine "emisell.app/platform/pkg/sdk/gen/emisell/engine/v1"
	engineconnect "emisell.app/platform/pkg/sdk/gen/emisell/engine/v1/enginev1connect"
	intentconnect "emisell.app/platform/pkg/sdk/gen/emisell/installation/v1/installationv1connect"
	integration "emisell.app/platform/pkg/sdk/gen/emisell/integration/v1"
	integrationconnect "emisell.app/platform/pkg/sdk/gen/emisell/integration/v1/integrationv1connect"
	pay "emisell.app/platform/pkg/sdk/gen/emisell/payment/v1"
	payconnect "emisell.app/platform/pkg/sdk/gen/emisell/payment/v1/paymentv1connect"
	ship "emisell.app/platform/pkg/sdk/gen/emisell/shipping/v1"
	shipconnect "emisell.app/platform/pkg/sdk/gen/emisell/shipping/v1/shippingv1connect"
	testconnect "emisell.app/platform/pkg/sdk/gen/emisell/testing/v1/testingv1connect"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type caller struct {
	identity.ServicePrincipal
	correlation string
}
type callerKey struct{}
type Server struct {
	ResourceProducts *resourceclient.Products
	ResourceClients  appclient.Service
	EngineKey        string
	EngineCheck      func(context.Context, string, string, string) (domain.Access, error)
	Testing          appservice.Testing
	Accounts         identity.ServiceAccounts
	Capabilities     capability.Service
	Intents          installservice.Intents
	Lifecycle        installservice.Lifecycle
	Logger           *slog.Logger
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	code := connect.CodeInternal
	switch {
	case errors.Is(err, fault.Invalid):
		code = connect.CodeInvalidArgument
	case errors.Is(err, fault.Unauthenticated):
		code = connect.CodeUnauthenticated
	case errors.Is(err, fault.Forbidden):
		code = connect.CodePermissionDenied
	case errors.Is(err, fault.NotFound):
		code = connect.CodeNotFound
	case errors.Is(err, fault.Conflict):
		code = connect.CodeAlreadyExists
	case errors.Is(err, fault.Unavailable):
		code = connect.CodeUnavailable
	case errors.Is(err, context.DeadlineExceeded):
		code = connect.CodeDeadlineExceeded
	case errors.Is(err, context.Canceled):
		code = connect.CodeCanceled
	}
	return connect.NewError(code, errors.New(code.String()))
}
func (s Server) Handler() http.Handler {
	mux := http.NewServeMux()
	if s.ResourceProducts != nil {
		mux.Handle("/internal/resources/products", s.resourceProductsHandler())
	}
	metrics := prometheus.NewRegistry()
	calls := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "emisell_rpc_requests_total", Help: "Internal capability RPC results."}, []string{"procedure", "code"})
	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "emisell_rpc_request_duration_seconds", Help: "Internal RPC latency."}, []string{"procedure"})
	metrics.MustRegister(calls, duration)
	interceptor := connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (res connect.AnyResponse, err error) {
			ctx, cancel := context.WithTimeout(otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(req.Header())), 10*time.Second)
			defer cancel()
			ctx, span := otel.Tracer("emisell/rpc").Start(ctx, req.Spec().Procedure)
			defer span.End()
			requestID := ids.New("rpc")
			start := time.Now()
			defer func() {
				if recover() != nil {
					res = nil
					err = mapError(errors.New("panic"))
				}
				code := "ok"
				if err != nil {
					code = connect.CodeOf(err).String()
				}
				calls.WithLabelValues(req.Spec().Procedure, code).Inc()
				duration.WithLabelValues(req.Spec().Procedure).Observe(time.Since(start).Seconds())
				s.Logger.Info("internal rpc", "request_id", requestID, "procedure", req.Spec().Procedure, "code", code, "trace_id", span.SpanContext().TraceID().String())
			}()
			header := req.Header().Get("Authorization")
			if req.Spec().Procedure == engineconnect.EngineGrantServiceCheckProcedure {
				if s.EngineCheck == nil || len(s.EngineKey) != 64 || len(req.Header().Values("Authorization")) != 1 || subtle.ConstantTimeCompare([]byte(header), []byte("Bearer "+s.EngineKey)) != 1 {
					return nil, mapError(fault.Unauthenticated)
				}
				return next(ctx, req)
			}
			if !strings.HasPrefix(header, "Bearer ") {
				return nil, mapError(fault.Unauthenticated)
			}
			principal, e := s.Accounts.Authenticate(ctx, strings.TrimPrefix(header, "Bearer "))
			if e != nil {
				return nil, mapError(e)
			}
			if public, _ := ctx.Value(coreHTTPKey{}).(bool); public && !principal.PlatformFull {
				return nil, mapError(fault.Forbidden)
			}
			ctx = context.WithValue(ctx, callerKey{}, caller{principal, requestID})
			if merchantid.Normalize(req.Any()) != nil {
				return nil, mapError(fault.Invalid)
			}
			res, err = next(ctx, req)
			if res != nil {
				if merchantid.Normalize(res.Any()) != nil {
					return nil, mapError(errors.New("invalid response identity"))
				}
				res.Header().Set("X-Request-ID", requestID)
			}
			return res, err
		}
	})
	opts := []connect.HandlerOption{connect.WithInterceptors(interceptor), connect.WithReadMaxBytes(32 << 10), connect.WithSendMaxBytes(32 << 10)}
	if s.EngineCheck != nil {
		p, h := engineconnect.NewEngineGrantServiceHandler(engineServer{s}, opts...)
		mux.Handle(p, h)
	}
	connectionPath, connectionHandler := integrationconnect.NewConnectionServiceHandler(connectionServer{Accounts: s.Accounts}, opts...)
	mux.Handle(connectionPath, connectionHandler)
	path, h := payconnect.NewPaymentServiceHandler(paymentServer{s}, opts...)
	mux.Handle(path, h)
	path, h = shipconnect.NewShippingServiceHandler(shippingServer{s}, opts...)
	mux.Handle(path, h)
	path, h = intentconnect.NewInstallIntentServiceHandler(intentServer{s}, opts...)
	mux.Handle(path, h)
	path, h = intentconnect.NewInstallationServiceHandler(lifecycleServer{s}, opts...)
	mux.Handle(path, h)
	if s.Testing.Repo != nil {
		path, h = testconnect.NewTestDistributionServiceHandler(testingServer{s}, opts...)
		mux.Handle(path, h)
	}
	mux.Handle("/metrics", promhttp.HandlerFor(metrics, promhttp.HandlerOpts{}))
	// This listener is internal, loopback-only: never accepts browser cookies/origins.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := strings.Split(r.Host, ":")[0]
		w.Header().Set("Cache-Control", "no-store")
		if (host != "127.0.0.1" && host != "localhost") || r.Header.Get("Origin") != "" || r.Header.Get("Cookie") != "" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

type engineServer struct{ Server }

func (s engineServer) Check(ctx context.Context, r *connect.Request[engine.CheckRequest]) (*connect.Response[engine.CheckResponse], error) {
	if !testingActor.MatchString(r.Msg.MerchantId) || r.Msg.Environment != "local-isolated" {
		return nil, mapError(fault.Invalid)
	}
	a, err := s.EngineCheck(ctx, r.Msg.MerchantId, r.Msg.ProviderCode, r.Msg.Operation)
	if err != nil {
		return nil, mapError(err)
	}
	return connect.NewResponse(&engine.CheckResponse{MerchantId: r.Msg.MerchantId, ProviderCode: r.Msg.ProviderCode, InstallationId: a.Installation.ID, AppId: a.Release.AppID, Environment: "local-isolated", Allowed: true}), nil
}
func (s Server) invoke(ctx context.Context, tenant, key, capabilityID, scope string, req capability.Request) (capability.Response, error) {
	c, ok := ctx.Value(callerKey{}).(caller)
	if !ok {
		return capability.Response{}, mapError(fault.Unauthenticated)
	}
	if tenant == "" {
		return capability.Response{}, mapError(fault.Invalid)
	}
	principal, err := c.BindTenant(tenant)
	if err != nil {
		return capability.Response{}, mapError(err)
	}
	if !principal.AllowsServiceScope(scope) {
		return capability.Response{}, mapError(fault.Forbidden)
	}
	v, err := s.Capabilities.Invoke(ctx, c.ID, tenant, capabilityID, key, c.correlation, req)
	return v, mapError(err)
}

type paymentServer struct{ Server }
type shippingServer struct{ Server }

type connectionServer struct{ Accounts identity.ServiceAccounts }

func (s connectionServer) EnsureMerchant(ctx context.Context, r *connect.Request[integration.EnsureMerchantRequest]) (*connect.Response[integration.EnsureMerchantResponse], error) {
	c, ok := ctx.Value(callerKey{}).(caller)
	if !ok {
		return nil, mapError(fault.Unauthenticated)
	}
	created, err := s.Accounts.EnsureMerchant(ctx, c.ServicePrincipal, r.Msg.MerchantId, r.Msg.CoreActorId)
	if err != nil {
		return nil, mapError(err)
	}
	return connect.NewResponse(&integration.EnsureMerchantResponse{MerchantId: r.Msg.MerchantId, Registered: true, Created: created}), nil
}

func (connectionServer) Check(ctx context.Context, _ *connect.Request[integration.CheckRequest]) (*connect.Response[integration.CheckResponse], error) {
	c, ok := ctx.Value(callerKey{}).(caller)
	if !ok {
		return nil, mapError(fault.Unauthenticated)
	}
	v := &integration.CheckResponse{ServiceId: c.ID, TenantId: c.TenantID, Scopes: slices.Clone(c.Scopes), PlatformFullAccess: c.PlatformFull}
	if !c.PlatformFull {
		v.ExpiresAt = timestamppb.New(c.ExpiresAt)
	}
	return connect.NewResponse(v), nil
}

func payment(v capability.Response) *pay.Payment {
	if v.Resource == nil {
		return nil
	}
	r := v.Resource
	return &pay.Payment{Id: r.ID, Reference: r.Reference, Status: r.Status, AmountMinor: r.AmountMinor, Currency: r.Currency}
}
func shipment(v capability.Response) *ship.Shipment {
	if v.Resource == nil {
		return nil
	}
	r := v.Resource
	return &ship.Shipment{Id: r.ID, Reference: r.Reference, Status: r.Status, AmountMinor: r.AmountMinor, Currency: r.Currency}
}
func (s paymentServer) Create(ctx context.Context, r *connect.Request[pay.CreateRequest]) (*connect.Response[pay.CreateResponse], error) {
	v, e := s.invoke(ctx, r.Msg.TenantId, r.Msg.IdempotencyKey, "payment/v1", "payments.write", capability.Request{Operation: "create", Reference: r.Msg.Reference, AmountMinor: r.Msg.AmountMinor, Currency: r.Msg.Currency})
	if e != nil {
		return nil, e
	}
	return connect.NewResponse(&pay.CreateResponse{Payment: payment(v), InstallationId: v.InstallationID, Simulation: v.Simulation}), nil
}
func (s paymentServer) Capture(ctx context.Context, r *connect.Request[pay.CaptureRequest]) (*connect.Response[pay.CaptureResponse], error) {
	v, e := s.invoke(ctx, r.Msg.TenantId, r.Msg.IdempotencyKey, "payment/v1", "payments.write", capability.Request{Operation: "capture", ResourceID: r.Msg.ResourceId})
	if e != nil {
		return nil, e
	}
	return connect.NewResponse(&pay.CaptureResponse{Payment: payment(v), InstallationId: v.InstallationID, Simulation: v.Simulation}), nil
}
func (s paymentServer) Refund(ctx context.Context, r *connect.Request[pay.RefundRequest]) (*connect.Response[pay.RefundResponse], error) {
	v, e := s.invoke(ctx, r.Msg.TenantId, r.Msg.IdempotencyKey, "payment/v1", "payments.write", capability.Request{Operation: "refund", ResourceID: r.Msg.ResourceId})
	if e != nil {
		return nil, e
	}
	return connect.NewResponse(&pay.RefundResponse{Payment: payment(v), InstallationId: v.InstallationID, Simulation: v.Simulation}), nil
}
func (s paymentServer) Status(ctx context.Context, r *connect.Request[pay.StatusRequest]) (*connect.Response[pay.StatusResponse], error) {
	v, e := s.invoke(ctx, r.Msg.TenantId, r.Msg.IdempotencyKey, "payment/v1", "payments.read", capability.Request{Operation: "status", ResourceID: r.Msg.ResourceId})
	if e != nil {
		return nil, e
	}
	return connect.NewResponse(&pay.StatusResponse{Payment: payment(v), InstallationId: v.InstallationID, Simulation: v.Simulation}), nil
}
func (s shippingServer) Create(ctx context.Context, r *connect.Request[ship.CreateRequest]) (*connect.Response[ship.CreateResponse], error) {
	v, e := s.invoke(ctx, r.Msg.TenantId, r.Msg.IdempotencyKey, "shipping/v1", "shipping.write", capability.Request{Operation: "create", Reference: r.Msg.Reference, WeightGrams: int(r.Msg.WeightGrams), DestinationZone: r.Msg.DestinationZone})
	if e != nil {
		return nil, e
	}
	return connect.NewResponse(&ship.CreateResponse{Shipment: shipment(v), InstallationId: v.InstallationID, Simulation: v.Simulation}), nil
}
func (s shippingServer) GetRates(ctx context.Context, r *connect.Request[ship.GetRatesRequest]) (*connect.Response[ship.GetRatesResponse], error) {
	v, e := s.invoke(ctx, r.Msg.TenantId, r.Msg.IdempotencyKey, "shipping/v1", "shipping.read", capability.Request{Operation: "get_rates", WeightGrams: int(r.Msg.WeightGrams), DestinationZone: r.Msg.DestinationZone, OriginZone: r.Msg.OriginZone})
	if e != nil {
		return nil, e
	}
	rates := make([]*ship.Rate, 0, len(v.Rates))
	for _, r := range v.Rates {
		rates = append(rates, &ship.Rate{Service: r.Service, AmountMinor: r.AmountMinor, Currency: r.Currency})
	}
	return connect.NewResponse(&ship.GetRatesResponse{Rates: rates, InstallationId: v.InstallationID, Simulation: v.Simulation}), nil
}
func (s shippingServer) Track(ctx context.Context, r *connect.Request[ship.TrackRequest]) (*connect.Response[ship.TrackResponse], error) {
	v, e := s.invoke(ctx, r.Msg.TenantId, r.Msg.IdempotencyKey, "shipping/v1", "shipping.read", capability.Request{Operation: "track", ResourceID: r.Msg.ResourceId})
	if e != nil {
		return nil, e
	}
	return connect.NewResponse(&ship.TrackResponse{Shipment: shipment(v), InstallationId: v.InstallationID, Simulation: v.Simulation}), nil
}
