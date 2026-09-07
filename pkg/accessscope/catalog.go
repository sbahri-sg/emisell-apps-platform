// Package accessscope owns the versioned app-resource permission contract.
// It is separate from Core service credentials and executable fixture scopes.
package accessscope

import (
	"errors"
	"slices"
	"strings"
)

const Profile = "shopify-authenticated-2026-09-05"
const Source = "https://shopify.dev/docs/api/usage/access-scopes"

type Scope struct {
	Handle        string   `json:"handle"`
	Resource      string   `json:"resource"`
	Action        string   `json:"action"`
	Status        string   `json:"status"`
	Grantable     bool     `json:"grantable"`
	Implies       []string `json:"implies"`
	RequiresAny   []string `json:"requiresAny"`
	Review        string   `json:"review"`
	Notes         string   `json:"notes"`
	AvailableFrom string   `json:"availableFrom,omitempty"`
}
type Catalog struct {
	Profile   string  `json:"profile"`
	Source    string  `json:"source"`
	CheckedAt string  `json:"checkedAt"`
	Grantable bool    `json:"grantable"`
	Scopes    []Scope `json:"scopes"`
}
type Declaration struct {
	Profile  string   `json:"profile"`
	Required []string `json:"required"`
	Optional []string `json:"optional"`
}

// These handles are a pinned reference, not an automatically synced allowlist.
// Descriptions are Emisell labels; API semantics must be mapped explicitly later.
const rows = `read_all_orders|Riwayat pesanan penuh
read_analytics_annotations,write_analytics_annotations|Anotasi analitik
write_app_proxy|Proxy aplikasi
read_assigned_fulfillment_orders,write_assigned_fulfillment_orders,read_merchant_managed_fulfillment_orders,write_merchant_managed_fulfillment_orders,read_third_party_fulfillment_orders,write_third_party_fulfillment_orders,read_marketplace_fulfillment_orders|Penugasan fulfillment
read_cart_transforms,write_cart_transforms|Transformasi keranjang
read_checkout_branding_settings,write_checkout_branding_settings|Tampilan checkout
read_checkout_and_accounts_configurations,write_checkout_and_accounts_configurations|Konfigurasi checkout dan akun
read_content,write_content,read_online_store_pages|Konten toko
read_customer_events,write_pixels|Pixel dan aktivitas pelanggan
read_customer_merge,write_customer_merge|Penggabungan pelanggan
read_customer_payment_methods|Metode pembayaran pelanggan
read_customers,write_customers|Pelanggan, segmen dan perusahaan
read_delivery_customizations,write_delivery_customizations|Kustomisasi pengiriman
read_discounts,write_discounts|Diskon
read_draft_orders,write_draft_orders|Draft pesanan
read_files,write_files|File
read_fulfillments,write_fulfillments|Layanan fulfillment
read_gift_cards,write_gift_cards|Kartu hadiah
read_inventory,write_inventory|Item dan stok inventaris
read_inventory_shipments,write_inventory_shipments|Pengiriman inventaris
read_inventory_shipments_received_items,write_inventory_shipments_received_items|Penerimaan inventaris
read_inventory_transfers,write_inventory_transfers|Transfer inventaris
read_legal_policies|Kebijakan hukum toko
read_locales,write_locales|Bahasa toko
read_locations,write_locations|Lokasi
read_markets,write_markets|Pasar
read_marketing_events,write_marketing_events|Aktivitas pemasaran
read_merchant_approval_signals|Sinyal persetujuan toko
read_metaobject_definitions,write_metaobject_definitions|Definisi metaobject
read_metaobjects,write_metaobjects|Data metaobject
read_online_store_navigation,write_online_store_navigation|Navigasi dan redirect toko
read_order_edits,write_order_edits|Perubahan pesanan
read_orders,write_orders|Pesanan dan transaksi
read_own_subscription_contracts,write_own_subscription_contracts|Kontrak subscription milik aplikasi
read_payment_customizations,write_payment_customizations|Kustomisasi pembayaran
read_payment_gateways,write_payment_gateways|Konfigurasi aplikasi pembayaran
read_payment_mandate,write_payment_mandate|Mandat pembayaran
write_payment_sessions|Sesi, capture, refund dan void pembayaran
read_payment_terms,write_payment_terms|Jadwal dan termin pembayaran
read_price_rules,write_price_rules|Aturan harga
write_privacy_settings,read_privacy_settings|Pengaturan privasi
read_products,write_products|Produk, varian dan koleksi
read_reports,write_reports|Laporan analitik
read_returns,write_returns|Retur
read_script_tags,write_script_tags|Script toko
read_shipping,write_shipping|Layanan carrier pengiriman
read_shopify_payments_disputes|Sengketa Shopify Payments
read_shopify_payments_dispute_evidences,write_shopify_payments_dispute_evidences|Bukti sengketa Shopify Payments
read_shopify_payments_dispute_file_uploads,write_shopify_payments_dispute_file_uploads|File sengketa Shopify Payments
read_shopify_payments_payouts|Payout Shopify Payments
read_store_credit_accounts|Akun kredit toko
read_store_credit_account_transactions,write_store_credit_account_transactions|Transaksi kredit toko
read_themes,write_themes|Tema toko
read_translations,write_translations|Terjemahan
read_users|Staf toko
read_validations,write_validations|Validasi checkout`

func Reference() Catalog {
	c := Catalog{Profile: Profile, Source: Source, CheckedAt: "2026-09-05", Scopes: []Scope{}}
	known := map[string]bool{}
	for _, row := range strings.Split(rows, "\n") {
		parts := strings.SplitN(row, "|", 2)
		for _, handle := range strings.Split(parts[0], ",") {
			known[handle] = true
			action, _, _ := strings.Cut(handle, "_")
			s := Scope{Handle: handle, Resource: parts[1], Action: action, Status: "planned", Implies: []string{}, RequiresAny: []string{}, Review: "standard", Notes: "Belum tersedia melalui gateway Emisell; deklarasi bukan grant."}
			if strings.Contains(handle, "customer") || strings.Contains(handle, "orders") || strings.Contains(handle, "payment") || strings.Contains(handle, "subscription") || strings.Contains(handle, "store_credit") || strings.Contains(handle, "gift_card") || strings.Contains(handle, "pixels") || strings.Contains(handle, "script_tags") || strings.Contains(handle, "themes") || handle == "read_users" {
				s.Review = "restricted"
			}
			if strings.Contains(handle, "shopify_payments") {
				s.Status, s.Notes = "reference_only", "Khusus Shopify Payments; tidak memiliki implementasi ekuivalen Emisell."
			}
			if strings.Contains(handle, "analytics_annotations") {
				s.Status, s.AvailableFrom = "future_reference", "2026-10"
				s.Notes = "Referensi Shopify versi mendatang; bukan bagian dukungan Emisell aktif."
			}
			switch handle {
			case "read_all_orders":
				s.RequiresAny = []string{"read_orders", "write_orders"}
				s.Notes = "Referensi akses historis di luar 60 hari; perlu review khusus dan scope pesanan. Batas data Emisell belum diimplementasikan."
			case "read_shipping", "write_shipping":
				s.Notes = "Resource carrier service belum tersedia. Berbeda dari izin native aplikasi provider eksternal; tidak diperlukan untuk Emisell Kurir built-in."
			case "read_reports", "write_reports":
				s.Notes = "Scope laporan; resource AnalyticsTarget pada referensi Shopify baru tersedia 2026-10."
			case "read_users":
				s.Notes = "Pada Shopify terkait paket Plus; tidak menyatakan Emisell memiliki fitur atau paket yang sama."
			case "read_products", "write_products":
				s.Notes = "Produk/varian/koleksi. Selling plan membutuhkan izin tambahan sesuai operasi; pemetaan field gateway belum tersedia."
			}
			c.Scopes = append(c.Scopes, s)
		}
	}
	for i := range c.Scopes {
		s := &c.Scopes[i]
		if s.Action == "write" {
			read := "read_" + strings.TrimPrefix(s.Handle, "write_")
			if known[read] {
				s.Implies = []string{read}
			}
		}
	}
	slices.SortFunc(c.Scopes, func(a, b Scope) int { return strings.Compare(a.Handle, b.Handle) })
	return c
}

func index() map[string]Scope {
	out := map[string]Scope{}
	for _, s := range Reference().Scopes {
		out[s.Handle] = s
	}
	return out
}

// Effective expands only explicit relationships in this profile, never a prefix wildcard.
func effective(handles []string, catalog map[string]Scope) map[string]bool {
	out := map[string]bool{}
	for _, h := range handles {
		out[h] = true
		for _, implied := range catalog[h].Implies {
			out[implied] = true
		}
	}
	return out
}

func (d Declaration) Validate() error {
	invalid := errors.New("invalid resource access scope declaration")
	catalog := index()
	if d.Profile != Profile || d.Required == nil || d.Optional == nil || len(d.Required)+len(d.Optional) > len(catalog) || len(d.Required)+len(d.Optional) == 0 {
		return invalid
	}
	seen := map[string]bool{}
	for _, list := range [][]string{d.Required, d.Optional} {
		for _, h := range list {
			if _, ok := catalog[h]; !ok || seen[h] {
				return invalid
			}
			seen[h] = true
		}
	}
	required := effective(d.Required, catalog)
	for _, h := range d.Optional {
		if required[h] {
			return invalid
		}
	}
	all := effective(append(slices.Clone(d.Required), d.Optional...), catalog)
	for _, group := range []struct {
		handles   []string
		available map[string]bool
	}{{d.Required, required}, {d.Optional, all}} {
		for _, h := range group.handles {
			deps := catalog[h].RequiresAny
			if len(deps) == 0 {
				continue
			}
			if !slices.ContainsFunc(deps, func(dep string) bool { return group.available[dep] }) {
				return invalid
			}
		}
	}
	return nil
}

func (d Declaration) Canonical() Declaration {
	d.Required, d.Optional = slices.Clone(d.Required), slices.Clone(d.Optional)
	slices.Sort(d.Required)
	slices.Sort(d.Optional)
	return d
}
