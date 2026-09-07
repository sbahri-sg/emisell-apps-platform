package accessscope

import (
	"slices"
	"strings"
	"testing"
)

func TestShippingScopesRemainUnavailable(t *testing.T) {
	for _, s := range Reference().Scopes {
		if s.Handle == "read_shipping" || s.Handle == "write_shipping" {
			if s.Grantable || s.Status != "planned" || !strings.Contains(s.Notes, "built-in") {
				t.Fatal("shipping availability or built-in distinction changed", s.Handle)
			}
		}
	}
}

func TestReferenceIsIsolatedAndNeverGrantable(t *testing.T) {
	c := Reference()
	seen := map[string]bool{}
	if c.Profile != Profile || c.Grantable || len(c.Scopes) != 108 {
		t.Fatal("incomplete reference")
	}
	for _, s := range c.Scopes {
		if seen[s.Handle] || s.Grantable || s.Action == "" || s.Resource == "" || s.Review == "" {
			t.Fatal("invalid scope", s.Handle)
		}
		seen[s.Handle] = true
		if s.Status != "planned" && s.Status != "future_reference" && s.Status != "reference_only" {
			t.Fatal("unexpected availability")
		}
	}
	idx := index()
	if !slices.Equal(idx["write_products"].Implies, []string{"read_products"}) {
		t.Fatal("write implication missing")
	}
	if idx["read_shopify_payments_payouts"].Status != "reference_only" || idx["read_analytics_annotations"].AvailableFrom != "2026-10" {
		t.Fatal("reference distinction lost")
	}
	for _, h := range []string{"shipping.read", "payments.write", "apps.install_intents.consent", "unauthenticated_read_checkouts", "customer_read_orders"} {
		if seen[h] {
			t.Fatal("mixed permission profiles", h)
		}
	}
	c.Scopes[0].Implies = append(c.Scopes[0].Implies, "admin.write")
	if slices.Contains(Reference().Scopes[0].Implies, "admin.write") {
		t.Fatal("mutable global catalog")
	}
	t.Logf("%d authenticated reference handles", len(c.Scopes))
}

func TestDeclarationRules(t *testing.T) {
	for _, tc := range []struct {
		name               string
		required, optional []string
		valid              bool
	}{
		{"minimal", []string{"read_products"}, []string{}, true},
		{"optional only", []string{}, []string{"read_products"}, true},
		{"planned restricted", []string{"read_all_orders", "write_orders"}, []string{"read_customers"}, true},
		{"reference planning", []string{}, []string{"read_analytics_annotations", "read_shopify_payments_payouts"}, true},
		{"missing lists", nil, nil, false},
		{"empty", []string{}, []string{}, false},
		{"duplicate", []string{"read_products", "read_products"}, []string{}, false},
		{"cross duplicate", []string{"read_products"}, []string{"read_products"}, false},
		{"implied optional", []string{"write_products"}, []string{"read_products"}, false},
		{"optional write expansion", []string{"read_products"}, []string{"write_products"}, true},
		{"all orders missing dependency", []string{"read_all_orders"}, []string{}, false},
		{"required dependent on optional", []string{"read_all_orders"}, []string{"read_orders"}, false},
		{"optional dependencies", []string{}, []string{"read_all_orders", "write_orders"}, true},
		{"internal scope", []string{"orders.read"}, []string{}, false},
		{"unknown", []string{"read_everything"}, []string{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := Declaration{Profile: Profile, Required: tc.required, Optional: tc.optional}
			if (d.Validate() == nil) != tc.valid {
				t.Fatalf("valid=%v", tc.valid)
			}
		})
	}
	d := Declaration{Profile: "foreign", Required: []string{"read_products"}, Optional: []string{}}
	if d.Validate() == nil {
		t.Fatal("foreign profile accepted")
	}
	d.Profile = Profile
	d.Required = []string{"write_products", "read_customers"}
	canonical := d.Canonical()
	if canonical.Validate() != nil || !slices.IsSorted(canonical.Required) || d.Required[0] != "write_products" {
		t.Fatal("canonical mutated caller")
	}
}
