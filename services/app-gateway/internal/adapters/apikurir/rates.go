// Package apikurir implements the private App Platform -> API Kurir boundary.
package apikurir

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"emisell-app-platform/services/app-gateway/internal/application"
	"emisell-app-platform/services/app-gateway/internal/domain"
)

const calculatePath = "/api/v1/calculate/district/domestic-cost"

var externalID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
var correlationID = regexp.MustCompile(`^[A-Za-z0-9._:-]{16,128}$`)
var providerCode = regexp.MustCompile(`^[A-Za-z0-9_.:>-]{1,128}$`)

type Options struct {
	Origin     string
	ServiceKey string
	AllowHTTP  bool
	Timeout    time.Duration
}

type Rates struct {
	origin     string
	serviceKey string
	allowHTTP  bool
	client     *http.Client
}

func NewRates(options Options) (*Rates, error) {
	origin := strings.TrimRight(strings.TrimSpace(options.Origin), "/")
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" ||
		(parsed.Scheme != "https" && !(options.AllowHTTP && parsed.Scheme == "http")) {
		return nil, errors.New("invalid API Kurir origin")
	}
	key := strings.TrimSpace(options.ServiceKey)
	if len(key) < 32 || len(key) > 512 || strings.ContainsAny(key, "\r\n") {
		return nil, errors.New("API Kurir service key must contain 32 to 512 characters")
	}
	if options.Timeout == 0 {
		options.Timeout = 5 * time.Second
	}
	if options.Timeout < 100*time.Millisecond || options.Timeout > 5*time.Second {
		return nil, errors.New("API Kurir timeout must be between 100ms and 5s")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.ResponseHeaderTimeout = options.Timeout
	return &Rates{
		origin: origin, serviceKey: key, allowHTTP: options.AllowHTTP,
		client: &http.Client{Transport: transport, Timeout: options.Timeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }},
	}, nil
}

type upstreamMeta struct {
	Message *string `json:"message"`
	Code    *int    `json:"code"`
	Status  *string `json:"status"`
}

type upstreamRate struct {
	Name             *string `json:"name"`
	Code             *string `json:"code"`
	Logo             *string `json:"logo"`
	Service          *string `json:"service"`
	CanonicalService *string `json:"canonical_service"`
	ServiceGroup     *string `json:"service_group"`
	ServiceType      *string `json:"service_type"`
	Description      *string `json:"description"`
	Cost             *int64  `json:"cost"`
	ETD              *string `json:"etd"`
}

type upstreamResponse struct {
	Meta *upstreamMeta  `json:"meta"`
	Data []upstreamRate `json:"data"`
}

func (r *Rates) Calculate(ctx context.Context, merchantID string, environment domain.Environment, requestID string, input application.ShippingRateRequest) (application.ShippingRateResponse, error) {
	if r == nil || r.client == nil || !externalID.MatchString(merchantID) || !correlationID.MatchString(requestID) {
		return application.ShippingRateResponse{}, runtimeError(503, "shipping_runtime_unavailable")
	}
	mode := "sandbox"
	if environment == domain.EnvironmentProduction {
		mode = "live"
	} else if environment != domain.EnvironmentSandbox {
		return application.ShippingRateResponse{}, runtimeError(400, "invalid_merchant_context")
	}
	form := url.Values{
		"origin":        {input.Origin},
		"destination":   {input.Destination},
		"weight":        {strconv.FormatInt(input.Weight, 10)},
		"include_group": {"true"},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, r.origin+calculatePath, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return application.ShippingRateResponse{}, runtimeError(503, "shipping_runtime_unavailable")
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("key", r.serviceKey)
	request.Header.Set("X-Emisell-Merchant-ID", merchantID)
	request.Header.Set("X-Emisell-Execution-Mode", mode)
	request.Header.Set("X-Request-ID", requestID)
	response, err := r.client.Do(request)
	if err != nil {
		return application.ShippingRateResponse{}, runtimeError(503, "shipping_runtime_unavailable")
	}
	defer response.Body.Close()
	const maxBody = 1 << 20
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBody+1))
	if err != nil || len(body) > maxBody {
		return application.ShippingRateResponse{}, runtimeError(503, "shipping_runtime_unavailable")
	}
	if response.StatusCode != http.StatusOK {
		return application.ShippingRateResponse{}, mapUpstreamError(response.StatusCode, response.Header.Get("Retry-After"), body)
	}
	var payload upstreamResponse
	if json.Unmarshal(body, &payload) != nil || payload.Meta == nil || payload.Meta.Message == nil || payload.Meta.Code == nil || payload.Meta.Status == nil ||
		*payload.Meta.Code != http.StatusOK || *payload.Meta.Status != "success" || strings.TrimSpace(*payload.Meta.Message) == "" || payload.Data == nil || len(payload.Data) > 200 {
		return application.ShippingRateResponse{}, runtimeError(503, "shipping_runtime_unavailable")
	}
	result := application.ShippingRateResponse{
		Meta: application.ShippingRateMeta{Message: *payload.Meta.Message, Code: *payload.Meta.Code, Status: *payload.Meta.Status},
		Data: make([]application.ShippingRateOption, 0, len(payload.Data)),
	}
	seen := make(map[string]struct{}, len(payload.Data))
	for _, item := range payload.Data {
		if !validRate(item, r.allowHTTP) {
			return application.ShippingRateResponse{}, runtimeError(503, "shipping_runtime_unavailable")
		}
		identity := *item.Code + ":" + *item.Service
		if _, duplicate := seen[identity]; duplicate {
			return application.ShippingRateResponse{}, runtimeError(503, "shipping_runtime_unavailable")
		}
		seen[identity] = struct{}{}
		result.Data = append(result.Data, application.ShippingRateOption{
			Name: *item.Name, Code: *item.Code, Logo: *item.Logo, Service: *item.Service,
			CanonicalService: *item.CanonicalService, ServiceGroup: *item.ServiceGroup, ServiceType: *item.ServiceType,
			Description: *item.Description, Cost: *item.Cost, ETD: *item.ETD,
		})
	}
	return result, nil
}

func validRate(item upstreamRate, allowHTTP bool) bool {
	if item.Name == nil || item.Code == nil || item.Logo == nil || item.Service == nil || item.CanonicalService == nil ||
		item.ServiceGroup == nil || item.ServiceType == nil || item.Description == nil || item.Cost == nil || item.ETD == nil ||
		strings.TrimSpace(*item.Name) == "" || !providerCode.MatchString(*item.Code) || !providerCode.MatchString(*item.Service) ||
		!providerCode.MatchString(*item.CanonicalService) || !providerCode.MatchString(*item.ServiceGroup) || !providerCode.MatchString(*item.ServiceType) ||
		*item.Cost < 0 || strings.TrimSpace(*item.ETD) == "" {
		return false
	}
	logo, err := url.Parse(*item.Logo)
	return err == nil && logo.Hostname() != "" && logo.User == nil &&
		(logo.Scheme == "https" || (allowHTTP && logo.Scheme == "http"))
}

func mapUpstreamError(status int, retryHeader string, body []byte) error {
	var upstream struct {
		Meta *struct {
			Status *string `json:"status"`
			Code   *int    `json:"code"`
		} `json:"meta"`
	}
	// Parse only shape/status consistency. Upstream messages and private error
	// details are never forwarded to Emisell Backend.
	if json.Unmarshal(body, &upstream) != nil || upstream.Meta == nil || upstream.Meta.Code == nil || *upstream.Meta.Code != status {
		return runtimeError(503, "shipping_runtime_unavailable")
	}
	switch status {
	case http.StatusBadRequest, http.StatusUnsupportedMediaType:
		return runtimeError(400, "invalid_rate_request")
	case http.StatusConflict:
		return runtimeError(409, "shipping_disabled")
	case http.StatusUnprocessableEntity:
		return runtimeError(422, "rate_not_available")
	case http.StatusTooManyRequests:
		retry, _ := strconv.Atoi(strings.TrimSpace(retryHeader))
		if retry < 1 || retry > 60 {
			retry = 60
		}
		return &application.ShippingRateError{Status: 429, Code: "shipping_rate_limited", RetryAfter: retry}
	case http.StatusServiceUnavailable:
		return runtimeError(503, "provider_quota_exhausted")
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusBadGateway:
		return runtimeError(502, "shipping_provider_unavailable")
	default:
		return runtimeError(503, "shipping_runtime_unavailable")
	}
}

func runtimeError(status int, code string) error {
	return &application.ShippingRateError{Status: status, Code: code}
}

var _ application.ShippingRateClient = (*Rates)(nil)
