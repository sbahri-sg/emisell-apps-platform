package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

func decodeJSON(request *http.Request, destination any) error {
	decoder := json.NewDecoder(io.LimitReader(request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("%w: invalid JSON body: %v", domain.ErrValidation, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("%w: request body must contain one JSON object", domain.ErrValidation)
	}
	return nil
}

func decodeOptionalJSON(request *http.Request, destination any) error {
	body, err := io.ReadAll(io.LimitReader(request.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("%w: read request body", domain.ErrValidation)
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	request.Body = io.NopCloser(bytes.NewReader(body))
	return decodeJSON(request, destination)
}

func pageFilter(request *http.Request) (ports.AppFilter, error) {
	filter := ports.AppFilter{
		Cursor: request.URL.Query().Get("cursor"),
		Search: request.URL.Query().Get("search"),
		Status: domain.AppStatus(request.URL.Query().Get("status")),
		Limit:  25,
	}
	if raw := request.URL.Query().Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 100 {
			return ports.AppFilter{}, fmt.Errorf("%w: limit must be between 1 and 100", domain.ErrValidation)
		}
		filter.Limit = limit
	}
	return filter, nil
}

func idempotencyKey(request *http.Request) (string, error) {
	key := strings.TrimSpace(request.Header.Get("Idempotency-Key"))
	if len(key) < 16 || len(key) > 128 {
		return "", fmt.Errorf("%w: Idempotency-Key must contain 16 to 128 characters", domain.ErrValidation)
	}
	return key, nil
}

type optionalString struct {
	Set   bool
	Value *string
}

func (value *optionalString) UnmarshalJSON(encoded []byte) error {
	value.Set = true
	if bytes.Equal(bytes.TrimSpace(encoded), []byte("null")) {
		value.Value = nil
		return nil
	}
	var decoded string
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return err
	}
	value.Value = &decoded
	return nil
}
