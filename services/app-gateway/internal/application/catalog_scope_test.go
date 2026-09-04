package application

import (
	"testing"

	"emisell-app-platform/services/app-gateway/internal/domain"
)

func TestCatalogPublicationScopeAvailability(t *testing.T) {
	t.Parallel()

	if scope := firstUnavailableVersionScope([]domain.SnapshotScope{{Scope: "read_merchant", Access: string(domain.ScopeAccessRequired)}}); scope != "" {
		t.Fatalf("available scope was blocked: %s", scope)
	}
	if scope := firstUnavailableVersionScope([]domain.SnapshotScope{{Scope: "read_orders", Access: string(domain.ScopeAccessRequired)}}); scope != "read_orders" {
		t.Fatalf("planned scope blocker = %q, want read_orders", scope)
	}
	if scope := firstUnavailableVersionScope([]domain.SnapshotScope{{Scope: "read_unregistered", Access: string(domain.ScopeAccessOptional)}}); scope != "read_unregistered" {
		t.Fatalf("unregistered scope blocker = %q, want read_unregistered", scope)
	}
}
