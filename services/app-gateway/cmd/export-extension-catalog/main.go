// Export reviewed, non-secret metadata for contract generation and drift tests.
package main

import (
	"encoding/json"
	"os"

	"emisell-app-platform/services/app-gateway/internal/application"
)

func main() {
	if err := json.NewEncoder(os.Stdout).Encode(application.OfficialExtensionCatalog()); err != nil {
		panic(err)
	}
}
