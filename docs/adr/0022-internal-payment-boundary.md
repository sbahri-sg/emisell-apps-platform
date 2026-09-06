# ADR 0022 — Payment gateway internal, shipping sebagai reference aplikasi publik

Tanggal: 6 September 2026. Status: diterapkan sebagai kebijakan authoring/distribusi lokal; bukan implementasi payment processor atau CLI developer.

## Konteks dan keputusan

Pengguna menetapkan payment gateway checkout di modul internal Emisell, dikonfigurasi melalui Settings → Payments, bukan App Store/Portal Developer umum. Shipping/API-Kurir tetap integrasi eksternal melalui contract provider-neutral `shipping/v1`. CRM, marketing dan resource/webhook apps tetap arah platform, tetapi authoring/runtime generiknya belum tersedia.

Shopify menjadi pembanding distribusi, bukan batas kemampuan teknis: [payments extensions](https://shopify.dev/docs/apps/build/payments) memakai jalur partner khusus, dan [tidak terlihat atau dapat di-install lewat App Store](https://shopify.dev/docs/apps/build/payments/requirements). Keputusan Emisell tidak berarti seluruh pembayaran Shopify internal.

## Enforcement dan compatibility

- Allowlist aplikasi umum saat ini hanya `shipping/v1`, didefinisikan di application module app, bukan serializer manifest. UI mengikuti kebijakan tersebut; backend tetap authoritative.
- Save/update payment draft dan konversi identitas draft payment lama ditolak (403). Submit/approve metadata dan signing/publish katalog payment ditolak. Reject/changes_requested, suspend, history/download tetap tersedia sesuai role.
- Validasi konfigurasi melaporkan check `public_distribution` gagal untuk payment; submit/approve/sign ditolak. WithSigned tidak menerima konfigurasi tersebut sehingga app-client registration, proof/secret/self-check dan request/approval Testing tidak memperoleh kesiapan baru. Revoke tetap tersedia; credential tidak dihapus atau dirotasi.
- Public catalog list/count/detail dan merchant Testing list tidak menawarkan payment, termasuk release historis berstatus published/approved. Filter dilakukan sebelum pagination. List/history portal mempertahankan data asli; status published historis tidak berarti masih didistribusikan.
- Read-only canonicalization dan verifikasi signature tetap menerima schema payment lama tanpa perubahan signed bytes. Scope resource Shopify-reference tidak dihapus, diubah nama, atau diaktifkan; izin data bukan izin menjadi payment processor.
- RPC/SDK/manifest payment fixture, installation/grant/token/routing/audit lama tetap kompatibel. Tidak menghapus Emisell Pay, menonaktifkan instalasi live, mengubah seed fixture, atau membangun modul pembayaran baru di repo Core.
- CLI developer selanjutnya: generator generik dan contoh shipping, local validation/dev/test; MCP opsional memakai library bersama. Ini rencana, bukan executable baru. Tidak ada template payment gateway publik atau penggunaan full-access Core key oleh developer tools.
- Billing app install/subscription berbeda dari pembayaran checkout; semua aplikasi tetap gratis.

## Retry dan rollout

Tidak ada migration database. Rollout backend dengan allowlist lebih dahulu, lalu frontend; frontend lama yang masih mengirim payment tetap ditolak server. Source schema/historical packages tetap valid; policy distribusi terpisah dapat menggagalkan retry payment yang dahulu diterima. Receipt tidak dihapus/ditulis ulang. Retry yang hanya membaca receipt lama tidak boleh membuat side effect atau mengaktifkan ulang release/assignment. Baca detail/history untuk hasil historis; jangan mengulang dengan key baru agar melewati policy.

Jangan rollback binary lama yang membuka distribusi payment. Gunakan forward-fix. Data tetap tersedia untuk audit dan migration payment internal yang kelak diotorisasi terpisah. Sebelum menghentikan fixture sepenuhnya, inventarisasi consumer, tambah jalur pengganti internal, uji parity, migrasikan konfigurasi/consumer secara eksplisit dan tetapkan deprecation window; tidak ada auto-migration merchant atau credential.

## Verifikasi

Uji allowlist fail-closed, bypass direct API, read-only payment history/signature, suspend/revoke, public pagination tanpa payment, shipping flow, isolasi organisasi/merchant dan compatibility lifecycle payment fixture. Tes database hanya pada database pengujian terisolasi. Jangan memakai instalasi merchant nyata sebagai smoke-test mutation.
