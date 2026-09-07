package providergrant

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

type sourceFunc func(context.Context, Request) (Snapshot, error)

func (f sourceFunc) Snapshot(c context.Context, r Request) (Snapshot, error) { return f(c, r) }
func TestGrantIdentityAndOperation(t *testing.T) {
	v := Snapshot{MerchantID: "m", AppID: "raja-app", InstallationID: "ins", ProviderCode: "rajaongkir", Revision: 1, Active: true, Scopes: []string{"shipping.read"}}
	s := Service{Enrolled: map[string]string{"raja-app": "rajaongkir"}, Source: sourceFunc(func(context.Context, Request) (Snapshot, error) { return v, nil })}
	r := Request{MerchantID: "m", AppID: "raja-app", InstallationID: "ins", ProviderCode: "rajaongkir", Operation: "rates.read"}
	if _, err := s.Check(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	for _, op := range []string{"settings.write", "shipments.create", "unknown"} {
		r.Operation = op
		if _, err := s.Check(context.Background(), r); !errors.Is(err, Denied) {
			t.Fatal("write granted with read scope")
		}
	}
	r.Operation = "rates.read"
	r.MerchantID = "other"
	if _, err := s.Check(context.Background(), r); !errors.Is(err, Denied) {
		t.Fatal("merchant bypass")
	}
	r.MerchantID = "m"
	v.Active = false
	v.Revoked = true
	v.Revision = 2
	r.Operation = "binding.read"
	got, err := s.Check(context.Background(), r)
	if err != nil || !got.Revoked || len(got.Scopes) != 0 {
		t.Fatal("revocation not synchronized", err)
	}
	r.Operation = "rates.read"
	if _, err := s.Check(context.Background(), r); !errors.Is(err, Denied) {
		t.Fatal("revocation bypass")
	}
}
func TestEngineOnlyHandler(t *testing.T) {
	calls := 0
	s := Service{Enrolled: map[string]string{"app": "rajaongkir"}, Source: sourceFunc(func(_ context.Context, r Request) (Snapshot, error) {
		calls++
		return Snapshot{MerchantID: r.MerchantID, AppID: r.AppID, InstallationID: r.InstallationID, ProviderCode: r.ProviderCode, Revision: 1}, nil
	})}
	key := strings.Repeat("a", 64)
	h, err := Handler(s, key)
	if err != nil {
		t.Fatal(err)
	}
	raw := `{"merchantId":"m","appId":"app","installationId":"ins","providerCode":"rajaongkir","operation":"binding.read"}`
	for _, tc := range []struct {
		auth, origin, body string
		want               int
	}{{"", "", raw, 403}, {"Bearer " + key, "https://seller.example", raw, 403}, {"Bearer " + key, "", raw + "{}", 400}, {"Bearer " + key, "", raw, 200}} {
		req := httptest.NewRequest("POST", Path, strings.NewReader(tc.body))
		req.Header.Set("Authorization", tc.auth)
		req.Header.Set("Origin", tc.origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != tc.want {
			t.Fatal(w.Code, tc.want)
		}
		if w.Code == 200 {
			var v map[string]any
			json.Unmarshal(w.Body.Bytes(), &v)
			if v["revision"] != "1" {
				t.Fatal("revision must be lossless string")
			}
		}
	}
	if calls != 1 {
		t.Fatal("unauthorized request reached source")
	}
}
