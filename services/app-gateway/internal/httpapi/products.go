package httpapi

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"emisell-app-platform/services/app-gateway/internal/adapters/emisell"
)

type productRateBucket struct {
	count int
	until time.Time
}
type productRateLimiter struct {
	sync.Mutex
	buckets map[string]productRateBucket
}

func (l *productRateLimiter) allow(key string) bool {
	l.Lock()
	defer l.Unlock()
	if l.buckets == nil {
		l.buckets = make(map[string]productRateBucket)
	}
	now := time.Now()
	for k, b := range l.buckets {
		if !now.Before(b.until) {
			delete(l.buckets, k)
		}
	}
	b, exists := l.buckets[key]
	if !exists {
		if len(l.buckets) >= 10000 {
			return false
		}
		b.until = now.Add(time.Minute)
	}
	if b.count >= 120 {
		return false
	}
	b.count++
	l.buckets[key] = b
	return true
}

func (h *handlers) getProducts(writer http.ResponseWriter, request *http.Request) {
	setInstallationResponseHeaders(writer)
	token, err := installationBearerToken(request)
	if err != nil {
		writeInstallationAccessError(writer, request, err, emisell.ReadProducts)
		return
	}
	access, err := h.installationAccess.Authenticate(request.Context(), token, emisell.ReadProducts)
	if err != nil {
		writeInstallationAccessError(writer, request, err, emisell.ReadProducts)
		return
	}
	if h.products == nil {
		writeProductError(writer, request, &emisell.Error{Status: 503, Code: "resource_disabled"})
		return
	}
	id := request.PathValue("productId")
	query, parseError := url.ParseQuery(request.URL.RawQuery)
	if parseError != nil {
		writeProductError(writer, request, &emisell.Error{Status: 400, Code: "invalid_request"})
		return
	}
	if err := emisell.ValidateQuery(query, id != ""); err != nil {
		writeProductError(writer, request, err)
		return
	}
	// Identity headers are deliberately ignored. Only the authenticated installation selects the tenant.
	route := "list"
	if id != "" {
		route = "detail"
	}
	if !h.productRate.allow(access.AppID + ":" + access.InstallationID + ":" + access.MerchantID + ":" + route) {
		writeProductError(writer, request, &emisell.Error{Status: 429, Code: "rate_limited", RetryAfter: 60})
		return
	}
	result, err := h.products.Read(request.Context(), access, id, query, requestIDFromContext(request.Context()))
	if err != nil {
		writeProductError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

func writeProductError(writer http.ResponseWriter, request *http.Request, err error) {
	var resourceError *emisell.Error
	if !errors.As(err, &resourceError) {
		writeInstallationAccessError(writer, request, err, emisell.ReadProducts)
		return
	}
	if resourceError.RetryAfter > 0 {
		writer.Header().Set("Retry-After", strconv.Itoa(resourceError.RetryAfter))
	}
	writeJSON(writer, resourceError.Status, errorEnvelope{Error: apiError{Code: resourceError.Code, Message: "Product resource request could not be completed.", RequestID: requestIDFromContext(request.Context())}})
}
