package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"emisell-app-platform/services/app-gateway/internal/domain"
)

type envelope[T any] struct {
	Data T `json:"data"`
}

type pageEnvelope[T any] struct {
	Data T   `json:"data"`
	Meta any `json:"meta"`
}

type errorEnvelope struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"requestId"`
	Details   map[string]any `json:"details,omitempty"`
}

func writeJSON(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	if err := json.NewEncoder(writer).Encode(payload); err != nil {
		panic(fmt.Errorf("encode response: %w", err))
	}
}

func writeError(writer http.ResponseWriter, request *http.Request, err error) {
	status := http.StatusInternalServerError
	code := "internal_error"
	message := "An unexpected error occurred."

	switch {
	case errors.Is(err, domain.ErrValidation):
		status, code, message = http.StatusUnprocessableEntity, "validation_error", err.Error()
	case errors.Is(err, domain.ErrNotFound):
		status, code, message = http.StatusNotFound, "not_found", "Resource not found."
	case errors.Is(err, domain.ErrConflict):
		status, code, message = http.StatusConflict, "conflict", err.Error()
	case errors.Is(err, domain.ErrForbidden):
		status, code, message = http.StatusForbidden, "forbidden", "You do not have permission to perform this action."
	case errors.Is(err, domain.ErrUnauthorized):
		status, code, message = http.StatusUnauthorized, "unauthorized", "Authentication failed."
	case errors.Is(err, domain.ErrInvalidGrant):
		status, code, message = http.StatusBadRequest, "invalid_grant", "The authorization grant is invalid."
	}

	writeJSON(writer, status, errorEnvelope{Error: apiError{Code: code, Message: message, RequestID: requestIDFromContext(request.Context())}})
}
