package merchantid_test

import (
	"testing"

	"emisell.app/platform/pkg/merchantid"
	intent "emisell.app/platform/pkg/sdk/gen/emisell/installation/v1"
	integration "emisell.app/platform/pkg/sdk/gen/emisell/integration/v1"
	pay "emisell.app/platform/pkg/sdk/gen/emisell/payment/v1"
	product "emisell.app/platform/pkg/sdk/gen/emisell/resource/product/v1"
	ship "emisell.app/platform/pkg/sdk/gen/emisell/shipping/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func TestSingleIdentityForEveryRPCMessage(t *testing.T) {
	for _, sample := range []proto.Message{
		&intent.PrepareRequest{}, &intent.GetRequest{}, &intent.DecideRequest{}, &intent.InstallIntent{},
		&intent.ConsumeRequest{}, &intent.InstallationRequest{}, &intent.InstallationAccess{}, &integration.CheckResponse{},
		&pay.CreateRequest{}, &pay.CaptureRequest{}, &pay.RefundRequest{}, &pay.StatusRequest{},
		&ship.CreateRequest{}, &ship.GetRatesRequest{}, &ship.TrackRequest{}, &product.AccessContext{},
	} {
		t.Run(string(sample.ProtoReflect().Descriptor().FullName()), func(t *testing.T) {
			for _, input := range []string{`{"merchantId":"merchant_a"}`, `{"merchant_id":"merchant_a"}`, `{"tenantId":"merchant_a"}`, `{"tenant_id":"merchant_a"}`, `{"merchantId":"merchant_a","tenantId":"merchant_a"}`} {
				msg := proto.Clone(sample)
				if err := protojson.Unmarshal([]byte(input), msg); err != nil {
					t.Fatal(err)
				}
				if err := merchantid.Normalize(msg); err != nil {
					t.Fatal(err)
				}
				m := msg.ProtoReflect()
				for _, f := range []string{"merchant_id", "tenant_id"} {
					var value string
					for i := 0; i < m.Descriptor().Fields().Len(); i++ {
						field := m.Descriptor().Fields().Get(i)
						if string(field.Name()) == f {
							value = m.Get(field).String()
						}
					}
					if value != "merchant_a" {
						t.Fatal("identity not propagated", f)
					}
				}
			}
			conflict := proto.Clone(sample)
			if err := protojson.Unmarshal([]byte(`{"merchantId":"merchant_a","tenantId":"merchant_b"}`), conflict); err != nil {
				t.Fatal(err)
			}
			if merchantid.Normalize(conflict) == nil {
				t.Fatal("conflicting identity accepted")
			}
		})
	}
}

func TestNestedIdentityAndBlankNeverInventID(t *testing.T) {
	r := &intent.ActivateRequest{Target: &intent.InstallationRequest{MerchantId: "merchant_a"}}
	if merchantid.Normalize(r) != nil || r.Target.TenantId != "merchant_a" {
		t.Fatal("nested request")
	}
	if merchantid.Normalize(&intent.ActivateRequest{}) != nil {
		t.Fatal("absent target")
	}
	v := &intent.PrepareRequest{}
	if merchantid.Normalize(v) != nil || v.MerchantId != "" || v.TenantId != "" {
		t.Fatal("invented identity")
	}
}
