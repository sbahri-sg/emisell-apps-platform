# ADR 0030 — Konsistensi UI aplikasi embedded

Status: diterima untuk arah produk; fondasi lokal tersedia.

## Konteks dan keputusan

Pengguna memilih pengalaman seperti Shopify: shell Dashboard tetap milik Emisell, UI aplikasi dibuat developer memakai kit resmi. Iframe tidak mewarisi CSS parent. Kita memakai CSS namespaced + HTML semantik sebagai fondasi framework-neutral, terpisah dari Bridge dan otorisasi. Tidak menambah framework atau menyalin brand Shopify.

Embedded apps wajib menggunakan komponen kit yang tersedia dan mengikuti review desain untuk komponen khusus. External app tidak diwajibkan meniru shell. Extension konfigurasi tanpa UI tetap Settings, bukan sidebar Apps.

## Konsekuensi dan batas

Demo memakai aset shared `pkg/appui` versi 0.1.0. Ini bukan library lengkap, npm release, automatic CSS enforcement, atau pembuktian semua app telah memenuhi review. Token mengikuti arah visual seller namun belum tersinkron otomatis lintas repo. Review UI masih checklist manual; perlu evidence versioned dan gate backend terpisah sebelum enforcement otomatis.

## Migrasi dan verifikasi

Tidak mengubah manifest/signature, launch URL, installation, scope, token, DB, atau API-Kurir. Adopt demo dahulu, QA, lalu tambah komponen dan pipeline evidence secara additive. Release historis tidak ditulis ulang. Rollback aset saja. Panduan dan checklist: [Embedded UI Kit](../embedded-ui-kit.md).
