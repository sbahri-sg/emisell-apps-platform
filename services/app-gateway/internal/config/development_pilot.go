package config

import (
	"fmt"
	"os"
	"strings"

	"emisell-app-platform/services/app-gateway/internal/adapters/emisell"
	"emisell-app-platform/services/app-gateway/internal/application"
)

func LoadDevelopmentPilot(environment string, resourcesEnabled bool) (application.DevelopmentResourcePilot, bool, error) {
	pilot := application.DevelopmentResourcePilot{ReadProductsMerchants: map[string]bool{}}
	localHTTP := os.Getenv("DEVELOPMENT_LOOPBACK_APP_HTTP")
	merchants := strings.TrimSpace(os.Getenv("EMISELL_RESOURCE_TEST_MERCHANT_IDS"))
	if localHTTP != "" && localHTTP != "false" && localHTTP != "true" {
		return pilot, false, fmt.Errorf("invalid DEVELOPMENT_LOOPBACK_APP_HTTP")
	}
	if environment != "development" && (localHTTP == "true" || merchants != "") {
		return pilot, false, fmt.Errorf("local app pilot settings are development-only")
	}
	if merchants != "" {
		if !resourcesEnabled {
			return pilot, false, fmt.Errorf("test-install resource pilot requires the configured resource adapter")
		}
		for _, id := range strings.Split(merchants, ",") {
			id = strings.TrimSpace(id)
			if !emisell.Identifier.MatchString(id) {
				return pilot, false, fmt.Errorf("invalid resource test merchant ID")
			}
			pilot.ReadProductsMerchants[id] = true
		}
		if len(pilot.ReadProductsMerchants) > 1000 {
			return pilot, false, fmt.Errorf("too many resource test merchants")
		}
	}
	return pilot, localHTTP == "true", nil
}
