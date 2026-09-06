package bootstrap_test

import (
	"emisell.app/platform/pkg/accessscope"
	"emisell.app/platform/pkg/gatewaycontract"
	"reflect"
	"testing"
)

func TestAccessScopeCatalogPortalBoundaries(t *testing.T) {
	f := setup(t)
	dev := portalAccount(t, f, "developer", "developer")
	admin := portalAccount(t, f, "admin", "operator")
	want := pexpect(t, admin, "GET", "/api/v1/admin/access-scopes", nil, "", 200)
	got := pexpect(t, dev, "GET", "/api/v1/developer/access-scopes", nil, "", 200)
	if !reflect.DeepEqual(got, want) || got["profile"] != accessscope.Profile || got["grantable"] != false || len(got["scopes"].([]any)) != len(accessscope.Reference().Scopes) {
		t.Fatal("inconsistent scope catalog")
	}
	for _, s := range got["scopes"].([]any) {
		if s.(map[string]any)["grantable"] != false {
			t.Fatal("unimplemented scope advertised active")
		}
	}
	for _, p := range []*fixture{admin, dev} {
		surface := "admin"
		if p == dev {
			surface = "developer"
		}
		v := pexpect(t, p, "GET", "/api/v1/"+surface+"/access-scopes/verification", nil, "", 200)
		if v["environment"] != "local" || v["contractRevision"] != gatewaycontract.Reference().ContractRevision || len(v["operations"].([]any)) != 2 {
			t.Fatal("shared gateway read model missing")
		}
		for _, op := range v["operations"].([]any) {
			if op.(map[string]any)["status"] != "planned" {
				t.Fatal("endpoint falsely active")
			}
		}
		if v["verification"] != "platform_build_inventory" || v["coreChecked"] != false || len(v["scopes"].([]any)) != 108 {
			t.Fatal("invalid verification")
		}
		for _, row := range v["scopes"].([]any) {
			r := row.(map[string]any)
			if r["status"] != "planned" || r["grantable"] != false {
				t.Fatal("unverified active scope")
			}
		}
	}
	pexpect(t, f, "GET", "/api/v1/admin/access-scopes/verification", nil, "", 401)
	pexpect(t, f, "GET", "/api/v1/admin/access-scopes", nil, "", 401)
	unauth := *f
	unauth.cookie = nil
	pexpect(t, &unauth, "GET", "/api/v1/admin/access-scopes", nil, "", 401)
	status, _, _, err := admin.call("GET", "/api/v1/admin/access-scopes", nil, "", developerOrigin)
	if err != nil || status != 403 {
		t.Fatal("cross origin read", status, err)
	}
	wrong := *dev
	cookie := *dev.cookie
	cookie.Name = "emisell_admin_session"
	wrong.cookie = &cookie
	pexpect(t, &wrong, "GET", "/api/v1/admin/access-scopes", nil, "", 401)
	resp, err := f.server.Client().Get(f.server.URL + "/api/v1/store/access-scopes")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatal("portal scope catalog exposed on Store", resp.StatusCode)
	}
}
