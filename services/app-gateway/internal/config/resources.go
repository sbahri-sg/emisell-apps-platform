package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"emisell-app-platform/services/app-gateway/internal/adapters/emisell"
)

// Disabled by default. Enabling code locally is not an App Store availability decision.
func LoadProducts(environment string) (*emisell.Products, error) {
	enabled, err := strconv.ParseBool(value("EMISELL_RESOURCE_ENABLED", "false"))
	if err != nil {
		return nil, fmt.Errorf("invalid EMISELL_RESOURCE_ENABLED")
	}
	if !enabled {
		return nil, nil
	}
	key, err := os.ReadFile(strings.TrimSpace(os.Getenv("EMISELL_RESOURCE_PRIVATE_KEY_FILE")))
	if err != nil {
		return nil, fmt.Errorf("cannot read EMISELL_RESOURCE_PRIVATE_KEY_FILE")
	}
	return emisell.NewProducts(emisell.Options{
		Origin:        strings.TrimSpace(os.Getenv("EMISELL_RESOURCE_BASE_URL")),
		KeyID:         strings.TrimSpace(os.Getenv("EMISELL_RESOURCE_KEY_ID")),
		PrivateKeyPEM: key, AllowHTTP: environment == "development",
	})
}
