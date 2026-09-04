package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"emisell-app-platform/services/app-gateway/internal/adapters/apikurir"
)

// LoadShippingRates keeps the bridge disabled unless every server-only API
// Kurir setting is explicit. It never supplies a development provider key.
func LoadShippingRates(environment string) (*apikurir.Rates, error) {
	enabled, err := strconv.ParseBool(value("API_KURIR_RATES_ENABLED", "false"))
	if err != nil {
		return nil, fmt.Errorf("API_KURIR_RATES_ENABLED must be true or false")
	}
	if !enabled {
		return nil, nil
	}
	timeout, err := time.ParseDuration(value("API_KURIR_RATES_TIMEOUT", "5s"))
	if err != nil {
		return nil, fmt.Errorf("parse API_KURIR_RATES_TIMEOUT: %w", err)
	}
	return apikurir.NewRates(apikurir.Options{
		Origin:     strings.TrimSpace(os.Getenv("API_KURIR_BASE_URL")),
		ServiceKey: strings.TrimSpace(os.Getenv("API_KURIR_SERVICE_KEY")),
		AllowHTTP:  environment == "development",
		Timeout:    timeout,
	})
}
