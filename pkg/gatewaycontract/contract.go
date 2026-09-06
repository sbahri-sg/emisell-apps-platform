// Package gatewaycontract is the non-executable handoff contract for Core owners.
// Coverage metadata never authorizes an app, merchant, installation or request.
package gatewaycontract

import (
	"crypto/sha256"
	"emisell.app/platform/pkg/accessscope"
	product "emisell.app/platform/pkg/sdk/gen/emisell/resource/product/v1"
	productconnect "emisell.app/platform/pkg/sdk/gen/emisell/resource/product/v1/productv1connect"
	"encoding/hex"
	"encoding/json"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"slices"
)

const Version = "emisell.resource.product/v1"

type Operation struct {
	Procedure      string   `json:"procedure"`
	Title          string   `json:"title"`
	Version        string   `json:"version"`
	RequiredScope  string   `json:"requiredScope"`
	AcceptedScopes []string `json:"acceptedScopes"`
	Implementation string   `json:"implementation"`
	Owner          string   `json:"owner"`
	Description    string   `json:"description"`
}
type Coverage struct {
	Scope           string   `json:"scope"`
	Resource        string   `json:"resource"`
	ReferenceStatus string   `json:"referenceStatus"`
	ContractStatus  string   `json:"contractStatus"`
	Implementation  string   `json:"implementation"`
	Grantable       bool     `json:"grantable"`
	Operations      []string `json:"operations"`
	NextAction      string   `json:"nextAction"`
}
type Handoff struct {
	ContractRevision string      `json:"contractRevision,omitempty"`
	Version          string      `json:"version"`
	ScopeProfile     string      `json:"scopeProfile"`
	Live             bool        `json:"live"`
	Operations       []Operation `json:"operations"`
	Coverage         []Coverage  `json:"coverage"`
	Checks           []string    `json:"checks"`
}

func Reference() Handoff {
	h := Handoff{Version: Version, ScopeProfile: accessscope.Profile, Operations: []Operation{
		{Procedure: productconnect.ProductServiceListProcedure, Title: "ProductService · List", Description: "Daftar proyeksi produk dasar, cursor pagination, urut ID. Filter status dan awalan judul. Bukan varian, inventory, koleksi atau operasi tulis."},
		{Procedure: productconnect.ProductServiceGetProcedure, Title: "ProductService · Get", Description: "Baca produk dasar menurut ID. Produk milik merchant lain dan ID tidak ada sama-sama not_found tanpa membocorkan keberadaan data."},
	}, Coverage: []Coverage{}, Checks: []string{
		"Implementasikan handler Core dan query merchant-scoped sesuai Protobuf, bukan endpoint provider-specific.",
		"Verifikasi caller/delegation dan cocokkan merchant, app, installation, scope profile serta grant revision; request body bukan sumber otorisasi.",
		"Grant/installation harus aktif dan fresh; deny untuk revoked, suspended, stale, scope asing atau service credential yang salah audience.",
		"Jalankan conformance suite pada fixture dua merchant milik tim backend, bukan data produksi.",
		"Verifikasi trust/crypto, revocation, deadline, batas payload, audit dan telemetry secara terpisah; suite ini bukan sertifikasi keamanan.",
		"Lampirkan revision implementasi, hasil test, target environment, owner, serta approval sebelum supported-operation registry/adapter diaktifkan.",
		"Aktivasi membutuhkan consent, consume intent dan grant/token lifecycle. Mengubah status dokumen tidak pernah memberikan grant.",
	}}
	for i := range h.Operations {
		o := &h.Operations[i]
		o.Version, o.RequiredScope, o.AcceptedScopes = Version, "read_products", []string{"read_products", "write_products"}
		o.Implementation, o.Owner = "planned", "Tim backend Emisell Core"
	}
	for _, s := range accessscope.Reference().Scopes {
		c := Coverage{Scope: s.Handle, Resource: s.Resource, ReferenceStatus: s.Status, ContractStatus: "missing", Implementation: "planned", Operations: []string{}, NextAction: "Tetapkan operasi resource dan schema bersama tim Platform, lalu implementasikan gateway + contract tests."}
		if s.Status != "planned" {
			c.ContractStatus, c.NextAction = "reference_only", "Evaluasi relevansi/versi terlebih dahulu. Tidak wajib membuat padanan fitur khusus Shopify."
		}
		for _, o := range h.Operations {
			if slices.Contains(o.AcceptedScopes, s.Handle) {
				c.Operations = append(c.Operations, o.Procedure)
			}
		}
		if len(c.Operations) > 0 {
			c.ContractStatus = "partial"
			c.NextAction = "Implementasikan List/Get produk dasar. Varian, koleksi, selling plan dan operasi tulis belum memiliki kontrak; jangan menganggap seluruh scope sudah tercakup."
		}
		h.Coverage = append(h.Coverage, c)
	}
	// Include the generated wire schema as well as scope mapping. Changing a
	// request/response shape must invalidate old docs even within the same version.
	schema, _ := (proto.MarshalOptions{Deterministic: true}).Marshal(protodesc.ToFileDescriptorProto(product.File_emisell_resource_product_v1_product_proto))
	body, _ := json.Marshal(struct {
		Mapping       Handoff
		ProductSchema []byte
	}{h, schema})
	sum := sha256.Sum256(body)
	h.ContractRevision = hex.EncodeToString(sum[:])
	return h
}
