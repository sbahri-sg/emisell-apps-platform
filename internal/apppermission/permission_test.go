package apppermission

import "testing"

func TestAllows(t *testing.T) {
	identity := Identity{"merchant", "app", "installation"}
	for _, scope := range []string{"shipping.read", "orders.read", "products.read"} {
		grant := Grant{Identity: identity, Active: true, ConsentedScopes: []string{scope}, GrantedScopes: []string{scope}}
		if !Allows(identity, grant, []string{scope}) {
			t.Fatal("valid grant rejected", scope)
		}
		for _, field := range []int{0, 1, 2} {
			other := identity
			switch field {
			case 0:
				other.MerchantID = "other"
			case 1:
				other.AppID = "other"
			case 2:
				other.InstallationID = "other"
			}
			if Allows(other, grant, []string{scope}) {
				t.Fatal("identity mismatch allowed")
			}
		}
		for _, required := range [][]string{nil, {""}, {"*"}, {scope, "missing"}} {
			if Allows(identity, grant, required) {
				t.Fatal("invalid requirements allowed")
			}
		}
		grant.Revoked = true
		if Allows(identity, grant, []string{scope}) {
			t.Fatal("revoked allowed")
		}
		grant.Revoked = false
		grant.Active = false
		if Allows(identity, grant, []string{scope}) {
			t.Fatal("inactive allowed")
		}
		grant.Active = true
		grant.ConsentedScopes = nil
		if Allows(identity, grant, []string{scope}) {
			t.Fatal("unconsented scope allowed")
		}
		grant.ConsentedScopes = []string{scope}
		grant.GrantedScopes = nil
		if Allows(identity, grant, []string{scope}) {
			t.Fatal("missing grant allowed")
		}
	}
}
