package bootstrap

import (
	"context"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/pkg/appmanifest"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"time"
)

type LocalEngineConfig struct{ EngineURL, ServiceKey, EngineKey string }

func (c LocalEngineConfig) LocalManaged() (LocalManaged, error) {
	u, e := url.Parse(c.EngineURL)
	key := regexp.MustCompile(`^[a-f0-9]{64}$`)
	if e != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" || u.Path != "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || !key.MatchString(c.EngineKey) || !key.MatchString(c.ServiceKey) {
		return LocalManaged{}, errors.New("invalid isolated engine configuration")
	}
	return LocalManaged{EngineKey: c.EngineKey, Readiness: localEngineReadiness{c}}, nil
}

type localEngineReadiness struct{ config LocalEngineConfig }

func (s localEngineReadiness) ReadyProvider(ctx context.Context, _, _ string, binding appmanifest.ShippingProviderBinding) error {
	if binding.Engine != "api-kurir" || binding.ProviderCode != "emisell" {
		return fault.Forbidden
	}
	client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	req, e := http.NewRequestWithContext(ctx, "GET", s.config.EngineURL+"/internal/readiness", nil)
	if e != nil {
		return fault.Unavailable
	}
	req.Header.Set("key", s.config.ServiceKey)
	res, e := client.Do(req)
	if e != nil {
		return fault.Unavailable
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fault.Unavailable
	}
	raw, e := io.ReadAll(io.LimitReader(res.Body, 1025))
	if e != nil || len(raw) > 1024 {
		return fault.Unavailable
	}
	var v struct {
		Ready        bool   `json:"ready"`
		ProviderCode string `json:"providerCode"`
		Environment  string `json:"environment"`
	}
	if json.Unmarshal(raw, &v) != nil || !v.Ready || v.ProviderCode != "emisell" || v.Environment != "local-isolated" {
		return fault.Unavailable
	}
	return nil
}
