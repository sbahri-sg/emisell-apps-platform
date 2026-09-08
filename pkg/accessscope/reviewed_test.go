package accessscope

import "testing"

func TestEmisellReviewedPermissionsDoNotChangeReferenceOrOldProfiles(t *testing.T) {
	if ReviewedProfile([]string{"read_products"}) != Profile || ReviewedProfile([]string{"read_orders", "read_shipping"}) != Profile {
		t.Fatal("existing profile changed")
	}
	for _, scopes := range [][]string{{"read_catalogs"}, {"read_collections"}, {"read_catalogs", "read_collections", "read_inventory", "read_locations", "read_orders", "read_products", "read_shipping"}} {
		d := Declaration{Profile: ReviewedProfile(scopes), Required: scopes, Optional: []string{}}
		if d.Profile != ReviewedReadProfile || d.Validate() != nil {
			t.Fatal("reviewed permission rejected")
		}
		d.Profile = Profile
		if d.Validate() == nil {
			t.Fatal("Emisell-only permissions entered reference profile")
		}
	}
	for _, scopes := range [][]string{nil, {}, {"write_catalogs"}, {"read_catalogs", "read_catalogs"}, {"read_collections", "read_catalogs"}, {"read_customers"}} {
		if (Declaration{Profile: ReviewedReadProfile, Required: scopes, Optional: []string{}}).Validate() == nil {
			t.Fatal("invalid reviewed permission accepted", scopes)
		}
	}
	if (Declaration{Profile: ReviewedReadProfile, Required: []string{"read_catalogs"}, Optional: []string{"read_products"}}).Validate() == nil {
		t.Fatal("optional grant accepted")
	}
}
