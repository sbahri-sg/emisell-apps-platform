package application

import "emisell-app-platform/services/app-gateway/internal/domain"

// This reviewed registry is the source of truth for discovery, not a runtime
// dispatch table. Adding an entry cannot activate a scope or execute a URL.
// Return fresh slices so callers cannot mutate the shared policy catalog.
func OfficialExtensionCatalog() domain.ExtensionCatalog {
	return domain.ExtensionCatalog{
		Version: "2026-09-04",
		Categories: []domain.AppCategoryDefinition{
			{ID: domain.CatalogCategoryPayment, Name: "Payment", Description: "Kategori listing untuk layanan pembayaran; bukan izin memproses transaksi.", Examples: []string{"Payment provider", "Rekonsiliasi pembayaran"}},
			{ID: domain.CatalogCategoryShipping, Name: "Shipping", Description: "Kategori listing untuk pengiriman; cek ongkir, booking dan tracking adalah kemampuan berbeda.", Examples: []string{"Shipping aggregator", "Tracking"}},
			{ID: domain.CatalogCategoryERP, Name: "ERP", Description: "Integrasi sistem bisnis; kebutuhan resource mengikuti use case yang disetujui.", Examples: []string{"Accounting", "Inventory sync"}},
			{ID: domain.CatalogCategoryMarketing, Name: "Marketing", Description: "Use case pemasaran; contoh kategori tidak menyatakan dukungan API atau widget.", Examples: []string{"Reviews", "Loyalty", "Campaigns"}},
			{ID: domain.CatalogCategoryOperations, Name: "Operations", Description: "Alur kerja operasional merchant.", Examples: []string{"Fulfillment", "Reporting"}},
			{ID: domain.CatalogCategoryCustom, Name: "Other apps", Description: "Kategori listing lainnya. Berbeda dari distribution=custom maupun extension type=custom.", Examples: []string{"Integrasi khusus merchant"}},
		},
		Families: []domain.ExtensionFamilyDefinition{
			{Type: domain.ExtensionTypePayment, Name: "Payment", Description: "Konfigurasi payment extension. Belum ada payment execution contract aktif.", ConfigurationSupported: true},
			{Type: domain.ExtensionTypeShipping, Name: "Shipping", Description: "Konfigurasi shipping extension. Menyimpan URL tidak mengaktifkan pengiriman request ongkir.", ConfigurationSupported: true},
			{Type: domain.ExtensionTypeCustom, Name: "General apps", Description: "Konfigurasi umum; nilai API tetap custom untuk kompatibilitas. Integrasi data dapat memakai Provider API tanpa extension UI.", ConfigurationSupported: true},
		},
		Capabilities: []domain.AppCapabilityDefinition{
			{ID: "merchant.profile.read", Name: "Read merchant identity", Type: domain.ExtensionTypeCustom, Availability: "available", ExecutionEnabled: true, Invocation: "provider_to_platform", RequiredScopes: []string{"read_merchant"}, Endpoints: []domain.ScopeEndpoint{{Method: "GET", Path: "/v1/merchant/profile"}}, Description: "Backend app membaca identitas merchant dari installation token.", Limitations: []string{"Tetap memerlukan installation token aktif dan merchant consent; tidak memberi akses seluruh profil/customer."}, GuideChapter: "merchant-data"},
			{ID: "products.read", Name: "Read base products", Type: domain.ExtensionTypeCustom, Availability: "pilot", ExecutionEnabled: false, Invocation: "provider_to_platform", RequiredScopes: []string{"read_products"}, Endpoints: []domain.ScopeEndpoint{{Method: "GET", Path: "/v1/products"}, {Method: "GET", Path: "/v1/products/{productId}"}}, Description: "Pilot produk dasar, default-off. Bukan variant, inventory siap jual, atau checkout.", Limitations: []string{"Aktivasi operator dan merchant allowlist diperlukan; read_products tetap planned untuk publikasi."}, GuideChapter: "merchant-data"},
			{ID: "shipping.rates.calculate", Name: "Calculate shipping rates", Type: domain.ExtensionTypeShipping, Availability: "pilot", ExecutionEnabled: false, Invocation: "platform_to_provider", RequiredScopes: []string{}, Endpoints: []domain.ScopeEndpoint{}, Description: "Bridge internal App Platform ke API Kurir sudah diimplementasikan tetapi nonaktif secara default. API Kurir menghitung dari rate card, snapshot/cache, atau optional provider quote pada miss/stale.", Limitations: []string{"Capability ini opsional dan tidak memaksa hit RajaOngkir/provider. API Kurir tetap memiliki quota ledger, credential, provider selection, location mapping dan cache policy.", "Hanya Emisell Backend tepercaya yang dapat memanggil bridge; endpoint tidak menjadi Partner API developer."}, GuideChapter: "shipping-rates"},
			plannedCapability("shipping.shipments.create", "Create shipment", domain.ExtensionTypeShipping, "Pembuatan pengiriman terpisah dari permintaan ongkir.", "app-surfaces", "Belum ada kontrak execution App Platform; quote ongkir tidak boleh digunakan sebagai jaminan booking."),
			plannedCapability("shipping.tracking.read", "Track shipment", domain.ExtensionTypeShipping, "Pelacakan pengiriman sebagai kemampuan tersendiri.", "app-surfaces", "Belum ada dispatch/tracking webhook App Platform."),
			plannedCapability("payment.session.create", "Start payment", domain.ExtensionTypePayment, "Memulai pembayaran membutuhkan kontrak transaksi khusus.", "app-surfaces", "Scope resource order tidak memberi izin charge/capture; kontrak dan approval payment belum tersedia."),
			plannedCapability("payment.refund.create", "Refund payment", domain.ExtensionTypePayment, "Refund adalah operasi terpisah dengan persetujuan dan idempotency sendiri.", "app-surfaces", "Belum tersedia; tidak diaktifkan oleh kategori Payment atau status app aktif."),
		},
		Surfaces: []domain.AppSurfaceDefinition{
			{ID: "server_only", Name: "Server-side integration", Availability: "available", Description: "Backend developer memakai API yang tersedia; di-host developer, bukan eksekusi kode otomatis di App Platform."},
			{ID: "app_home", Name: "App Home", Availability: "planned", Description: "Halaman app untuk merchant. Embedded host dan session bridge belum tersedia."},
			{ID: "admin", Name: "Merchant Admin UI", Availability: "planned", Description: "Action/block di halaman merchant, bukan dashboard operator /admin."},
			{ID: "checkout", Name: "Checkout", Availability: "planned", Description: "Target UI dan isolated renderer belum tersedia."},
			{ID: "online_store", Name: "Online Store", Availability: "planned", Description: "Widget/theme integration belum tersedia. Kategori Reviews tidak membuat widget otomatis."},
		},
		RuntimeContract: domain.ExtensionRuntimeContractDefinition{
			Version:          "v1-draft",
			Status:           "draft",
			ExecutionEnabled: false,
			Transport:        "https_json",
			Authentication:   "emisell_runtime_jwt",
			TokenTTLSeconds:  60,
			IdentityClaims:   []string{"merchant_id", "installation_id", "extension_id", "version_id", "environment", "operation"},
			Headers: []domain.RuntimeHeaderDefinition{
				{Name: "Authorization", Required: true, Description: "Short-lived Bearer runtime JWT issued by App Platform for one operation and one installation-extension binding."},
				{Name: "X-Emisell-Invocation-Id", Required: true, Description: "Unique invocation identifier generated by App Platform and echoed by the provider."},
				{Name: "X-Emisell-Idempotency-Key", Required: false, Description: "Required only by mutation operation policy. Rate calculation deduplication and provider quota control remain inside API Kurir."},
				{Name: "X-Emisell-Deadline", Required: true, Description: "Absolute RFC3339 deadline. Provider must stop work after it expires."},
			},
			Operations: []domain.RuntimeOperationDefinition{
				{ID: "shipping.rates.calculate", CapabilityID: "shipping.rates.calculate", Mode: "synchronous", Mutation: false, Idempotency: "not_required", TimeoutMS: 5000, MaximumAttempts: 1, Description: "Ask API Kurir to resolve rates for one validated checkout request. API Kurir checks local rate cards and exact snapshots/cache before an optional provider quote, so one calculation does not imply one upstream hit."},
				{ID: "shipping.shipments.create", CapabilityID: "shipping.shipments.create", Mode: "synchronous", Mutation: true, Idempotency: "required", TimeoutMS: 15000, MaximumAttempts: 1, Description: "Create one shipment after Emisell business validation; the provider result is not the source of truth for order state."},
				{ID: "shipping.tracking.read", CapabilityID: "shipping.tracking.read", Mode: "synchronous", Mutation: false, Idempotency: "not_required", TimeoutMS: 8000, MaximumAttempts: 1, Description: "Read normalized tracking for an existing provider shipment reference."},
				{ID: "payment.session.initialize", CapabilityID: "payment.session.create", Mode: "synchronous", Mutation: true, Idempotency: "required", TimeoutMS: 10000, MaximumAttempts: 1, Description: "Initialize a provider payment session while Emisell retains transaction ownership."},
				{ID: "payment.session.process", CapabilityID: "payment.session.create", Mode: "synchronous", Mutation: true, Idempotency: "required", TimeoutMS: 20000, MaximumAttempts: 1, Description: "Process an approved payment action without accepting merchant or amount overrides from the provider."},
				{ID: "payment.session.cancel", CapabilityID: "payment.session.create", Mode: "synchronous", Mutation: true, Idempotency: "required", TimeoutMS: 10000, MaximumAttempts: 1, Description: "Cancel a provider payment session using the same installation and transaction binding."},
				{ID: "payment.refund.create", CapabilityID: "payment.refund.create", Mode: "synchronous", Mutation: true, Idempotency: "required", TimeoutMS: 20000, MaximumAttempts: 1, Description: "Request a refund against an Emisell-authorized amount and transaction reference."},
			},
			Invariants: []string{
				"Merchant, installation, extension, installed version, environment and operation are derived by trusted services; providers cannot select another tenant.",
				"The dispatcher resolves only the reviewed runtime URL from the installed immutable snapshot and applies destination policy before network access.",
				"A valid token still requires active app, installation and extension state at dispatch time; uninstall or suspension fails closed.",
				"No automatic retry is allowed in v1. A caller may repeat only with the same idempotency key after checking the recorded result.",
				"Payment and shipping business state remains owned by Emisell Backend; provider responses are validated inputs, not authoritative state transitions.",
				"Rate calculation is optional per shipping extension. API Kurir owns cache, request coalescing, provider quota and fallback; App Platform never forces one RajaOngkir hit per checkout.",
			},
		},
	}
}

func plannedCapability(id, name string, family domain.ExtensionType, description, chapter, limitation string) domain.AppCapabilityDefinition {
	return domain.AppCapabilityDefinition{ID: id, Name: name, Type: family, Availability: "planned", ExecutionEnabled: false, Invocation: "platform_to_provider", RequiredScopes: []string{}, Endpoints: []domain.ScopeEndpoint{}, Description: description, Limitations: []string{limitation, "Capability ID bukan OAuth scope. Otorisasi execution belum ditetapkan; jangan memintanya pada consent."}, GuideChapter: chapter}
}
