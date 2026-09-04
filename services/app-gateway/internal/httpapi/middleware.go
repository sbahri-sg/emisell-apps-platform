package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ids"
)

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._:-]{16,128}$`)

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestID := strings.TrimSpace(request.Header.Get("X-Request-Id"))
		if !validRequestID.MatchString(requestID) {
			requestID, _ = ids.NewUUIDv7()
		}
		writer.Header().Set("X-Request-Id", requestID)
		next.ServeHTTP(writer, request.WithContext(context.WithValue(request.Context(), requestContextKey, requestID)))
	})
}

func authenticationMiddleware(authenticator Authenticator, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		actor, err := authenticator.Authenticate(request)
		if err != nil {
			if errors.Is(err, domain.ErrForbidden) {
				writeError(writer, request, err)
				return
			}
			writeJSON(writer, http.StatusUnauthorized, errorEnvelope{Error: apiError{Code: "unauthorized", Message: "A valid session or bearer token is required.", RequestID: requestIDFromContext(request.Context())}})
			return
		}
		ctx := context.WithValue(request.Context(), actorContextKey, actor)
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

func requireCapability(request *http.Request, capability string) error {
	if !can(actorFromContext(request.Context()).Role, capability) {
		return domainForbidden(capability)
	}
	return nil
}

func requirePlatformOperator(request *http.Request) error {
	if !actorFromContext(request.Context()).PlatformOperator {
		return domainForbidden("platform.operator")
	}
	return nil
}

func domainForbidden(capability string) error {
	return fmtError(domain.ErrForbidden, "missing capability "+capability)
}

func fmtError(base error, message string) error {
	return &wrappedError{base: base, message: message}
}

type wrappedError struct {
	base    error
	message string
}

func (e *wrappedError) Error() string { return e.message }
func (e *wrappedError) Unwrap() error { return e.base }

func corsMiddleware(allowedOrigins []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		origin := request.Header.Get("Origin")
		if origin != "" && slices.Contains(allowedOrigins, origin) {
			writer.Header().Set("Access-Control-Allow-Origin", origin)
			writer.Header().Set("Access-Control-Allow-Credentials", "true")
			writer.Header().Set("Vary", "Origin")
			writer.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, X-CSRF-Token, X-Organization-Id, X-Request-Id")
			writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
		}
		if request.Method == http.MethodOptions {
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("Referrer-Policy", "no-referrer")
		writer.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(writer, request)
	})
}

func recoveryMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("request panic", "request_id", requestIDFromContext(request.Context()), "panic", recovered, "stack", string(debug.Stack()))
				writeError(writer, request, fmtError(errInternal, "request panic"))
			}
		}()
		next.ServeHTTP(writer, request)
	})
}

var errInternal = &wrappedError{message: "internal error"}

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (recorder *responseRecorder) WriteHeader(status int) {
	recorder.status = status
	recorder.ResponseWriter.WriteHeader(status)
}

func loggingMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		started := time.Now()
		recorder := &responseRecorder{ResponseWriter: writer, status: http.StatusOK}
		next.ServeHTTP(recorder, request)
		logger.Info("request", "method", request.Method, "path", request.URL.Path, "status", recorder.status, "duration_ms", time.Since(started).Milliseconds(), "request_id", requestIDFromContext(request.Context()))
	})
}
