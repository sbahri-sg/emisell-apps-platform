# ADR 0011 — Katalog scope resource dan batas grant

Status: diterima, 2026-09-05.

## Keputusan

- Gunakan handle persis dari tabel **Authenticated access scopes** Shopify sebagai referensi, snapshot `shopify-authenticated-2026-09-05`. Ini kesamaan nama/konsep izin, bukan kompatibilitas API, token, atau persetujuan Shopify.
- Storefront (`unauthenticated_*`) dan Customer Account (`customer_*`) memiliki principal/token berbeda dan tidak dicampur dalam profil ini. Snapshot dapat memuat scope versi mendatang; tandai `future_reference`. Scope khusus Shopify Payments diberi `reference_only`.
- Sumber kebenaran ada di `pkg/accessscope`, dipakai validasi backend dan endpoint katalog Admin/Developer. Tidak ada salinan allowlist di frontend. Pembaruan sumber harus membuat profil baru, melalui review; jangan mengubah arti profil yang sudah ditandatangani.
- Semua scope resource saat ini **belum grantable**. Status `planned` berarti dapat dideklarasikan untuk perencanaan, bukan API resource telah tersedia. Katalog tidak mengeluarkan token/grant. Persetujuan metadata oleh Admin tidak menggantikan consent merchant atau restricted-data review.
- `AppDocument.accessScopes` opsional memuat `profile`, `required`, `optional`. Disimpan server-side dalam JSONB draft dan snapshot pengajuan immutable. Validasi menolak scope asing, duplikat, profil asing, dependensi tidak terpenuhi, dan optional yang sudah tercakup required (termasuk implikasi write → read yang tercantum).
- `AppDocument.scopes` lama tetap izin fixture capability dot-style. Tidak diterjemahkan otomatis: `shipping.read` bukan sinonim `read_shipping` (carrier service). Scope service Core `apps.install_intents.*` juga terpisah.
- Draft dengan deklarasi resource menghasilkan **`emisell.catalog/v2` / `catalog-metadata/v2`**, tetap metadata non-executable, gratis, `installable:false`. Required/optional beserta profil terikat signature. Store menampilkan rencana izin, bukan grant aktif. Tanpa deklarasi tetap v1; canonical bytes/signature v1 tidak berubah.
- Endpoint merchant/workspace compatibility dikeluarkan dari daftar, pencarian, contoh, dan unduhan Dokumentasi API Admin. Backend legacy tidak dihapus pada milestone ini.

## Kontrak integrasi Core berikutnya (belum endpoint yang tersedia)

1. Developer menyatakan kebutuhan → Admin mereview release immutable → Store menampilkan kebutuhan tersebut → Emisell Core menampilkan consent menggunakan konteks tenant/staff terautentikasi.
2. Gateway harus mempunyai allowlist mapping `profile + scope + resource operation + contract version`, availability per tenant, restricted approval, batas data/field dan implied scopes. Tidak boleh routing berdasarkan prefix atau nama scope saja.
3. Grant efektif = kebutuhan release ∩ consent merchant ∩ kebijakan review ∩ operasi gateway yang benar-benar tersedia. Required yang belum didukung menggagalkan install; optional belum didukung tidak diberikan. Izin yang implied tetap tunduk pada dukungan/policy yang sama.
4. Consume intent, creation installation, grant/version, audit dan outbox harus atomik/idempotent sebelum token app diterbitkan. Token terikat tenant, installation, audience, scope profile dan grant revision. Core memvalidasi ulang actor/tenant serta policy operasi; network internal bukan bypass authorization.
5. Gateway resource untuk apps → Core menggunakan ConnectRPC/Protobuf/Buf internal. REST/JSON tetap external. Capability payment/v1 dan shipping/v1 (Core → provider melalui platform) tetap jalur terpisah.
6. Tambahan required scope membutuhkan release + consent baru; optional butuh persetujuan eksplisit. Revocation/uninstall menonaktifkan token/grant, invalidasi cache, audit dan event. Default deny untuk profil, scope, tenant atau operasi yang tidak dikenal.

Tidak menambahkan issuer OAuth umum, resource endpoint palsu, atau pemetaan otomatis ke fixture. Adapter pertama yang disarankan: `read_products`; dukungan baru dapat ditandai aktif setelah contract test, tenant isolation, grant enforcement dan revocation lolos.

## Migration / rollback

Tidak ada perubahan tabel atau rewrite release lama. Field JSON opsional mempertahankan hash draft lama saat tidak ada deklarasi. Rollout reader/validator v2 sebelum authoring UI; reader lama boleh menolak v2 dengan jelas, tidak downgrade diam-diam. Setelah v2 tersimpan, rollback harus memakai reader yang memahami v2 atau menonaktifkan penulisan baru dan menangguhkan listing v2 melalui lifecycle terotorisasi; jangan menjalankan binary lama yang bisa membuang field draft atau menolak katalog publik secara keseluruhan. Jangan resign/mengubah release v1.

## Sumber

- [Shopify access scopes](https://shopify.dev/docs/api/usage/access-scopes), tabel authenticated, diperiksa 2026-09-05.
- [Manage access scopes](https://shopify.dev/docs/apps/build/authentication-authorization/manage-access-scopes), required/optional dan consent.

Review flags di Emisell adalah kebijakan konservatif tersendiri, bukan bukti bahwa Shopify mewajibkan review yang identik untuk setiap scope. Data sensitif, akses historis, pembayaran, script/theme dan staff membutuhkan penilaian tambahan sebelum implementasi aktif.
