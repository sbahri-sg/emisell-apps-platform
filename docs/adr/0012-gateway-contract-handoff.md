# ADR 0012 — Handoff kontrak resource gateway Emisell

Status: diterima untuk handoff, 2026-09-05. Implementasi gateway Core belum tersedia/diverifikasi dari repository ini.

## Keputusan

- Tambahkan kontrak additive `emisell.resource.product.v1.ProductService` dengan `List` dan `Get`, Protobuf dan generated Connect client/server interfaces. Ini arah **App Platform → Core**, berbeda dengan capability payment/shipping yang dipanggil Core → Platform. Tidak menambah listener, service deploy, atau route executable di Platform.
- Kontrak pertama hanya proyeksi produk dasar (ID, judul, handle, status, waktu perubahan). `read_products`/implied `write_products` hanya dipetakan ke dua operasi baca tersebut. Tidak mengklaim API Shopify parity, semua operasi tulis, varian, koleksi, inventory, atau selling plan telah tersedia.
- `pkg/gatewaycontract` mendefinisikan mapping, coverage seluruh 108 scope dari katalog immutable, validasi kontrak produk, dan checklist penerimaan. CLI `gateway-contract` mengekspor metadata tanpa membaca credential/database. Matriks generated dan referensi API Admin memakai sumber ini; perubahan kontrak tidak diedit pada file generated.
- Pisahkan contract coverage (`missing`, `partial`, `reference_only`) dari implementation status dan grantability. Semua implementation masih `planned`, seluruh `grantable:false`. Matriks adalah snapshot handoff, BUKAN discovery/health endpoint live dan tidak boleh dipakai sebagai authorization store.
- `AccessContext` mengandung assertion tenant, installation, app, scope profile, grant revision, dan request ID. Core hanya boleh memercayainya setelah mengikat semua field pada delegasi terverifikasi dari App Platform dan memeriksa state grant saat ini. Payload tidak membawa scope yang bisa diberikan sendiri oleh caller.
- Detail crypto/trust provisioning dan protokol sinkronisasi/revalidasi grant lintas repository masih gate integrasi, bukan issuer token yang dibuat pada milestone ini. SDK tidak menyisipkan credential atau menganggap jaringan internal sebagai permission. Jangan menggunakan token capability Core, cookie portal, ataupun katalog signing key untuk resource delegation.
- Sediakan conformance suite reusable yang menerima client untuk skenario auth/state dan dataset dua tenant terisolasi. Suite dapat dijalankan oleh tim Core melalui client Connect miliknya; di repository ini diverifikasi terhadap implementation test-only. Lulus test-only bukan verifikasi backend Core nyata atau security certification.
- Endpoint planned tampil dalam kelompok **Gateway Emisell · handoff** dengan base URL placeholder, tidak mengaku tersedia di :8088. Kontrak dapat diunduh, tidak ada tombol mengeksekusi request.

## Pembagian tanggung jawab dan rollout

1. Platform menetapkan scope/operation contract dan menyediakan generated bindings, validasi, suite dan handoff.
2. Tim Core mengimplementasikan queries dan enforcement pada modular backend yang sudah ada, tanpa membaca database Platform langsung. Kedua tim menyepakati identity/trust, freshness/revocation, domain errors, target URL dan deployment evidence.
3. Jalankan suite terhadap backend Core terisolasi dan uji grant lifecycle/crypto/egress/deadline/audit. Tambahkan bukti hasil + revision implementasi pada review integrasi. Jangan mengubah snapshot scope profile yang sudah signed.
4. Baru setelah approval, tambahkan adapter Platform, supported-operation registry, consume intent/grant/token pipeline serta rollout per tenant; scope belum supported tetap deny-by-default. Discovery atau toggle dokumentasi bukan jalur aktivasi.

Additive contract tidak mengubah baseline Protobuf lama, manifest/signature, schema DB, sesi, atau instalasi. Rollback tahap handoff cukup menarik referensi baru; jangan downgrade atau menghapus kontrak setelah digunakan oleh consumer. Breaking perubahan protokol/data harus versi package baru dan migration window.
