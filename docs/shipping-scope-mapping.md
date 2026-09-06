# Pemetaan izin API-Kurir — pilot lokal

Pembaruan 6 September 2026. Dokumen ini menjelaskan implementasi, bukan health check atau bukti grant merchant saat ini.

Apps Platform mengelola release, assignment, consent, installation dan native grant. Emisell Backend meminta tarif langsung ke API-Kurir utama; API-Kurir tetap engine remote dengan aplikasi per-provider. Tidak perlu mengirim permintaan tarif melalui runtime simulator Platform.

| Identifier | Jenis | Batas saat ini |
|---|---|---|
| `shipping/v1` | Capability versioned | Deklarasi shipping, bukan izin semua operasi |
| `shipping.read` | Native scope installation | Scope pilot managed Emisell Kurir; bukan alias `read_shipping` |
| `rates.read` | Operasi EngineGrantService/Check | Pemeriksaan grant lokal sebelum permintaan/cache tarif pada Core dan API-Kurir utama yang dikonfigurasi untuk pilot |
| `settings.read` | Operasi kontrak grant lokal | Dikenali Platform/engine referensi; tidak membuktikan enforcement seluruh endpoint settings API-Kurir utama |
| `read_shipping`, `write_shipping` | Resource scope Gateway Emisell | Masih Planned, grantable false; endpoint carrier resource belum tersedia |

## Batas otorisasi

- Kontrak engine tetap `local-isolated`, provider exact `emisell`, merchant dari konteks server terverifikasi. Credential engine independen dari key full-access Core dan tidak diberikan ke developer/browser.
- Release/assignment harus tetap valid dan installation/native grant aktif. Revoke, suspend, uninstall, mismatch atau kegagalan pemeriksaan menolak permintaan baru; jangan fallback untuk melewati penolakan grant.
- Core dan API-Kurir utama menggunakan konfigurasi pilot lokal eksplisit. Tanpa konfigurasi tersebut, kompatibilitas legacy masih ada; ini bukan rollout enforcement produksi menyeluruh.
- Berhasil memilih tarif di Create order membuktikan alur tarif lokal, bukan shipment, tracking, perubahan credential, pengaturan provider atau revocation end-to-end produksi.
- Provider aktif di API-Kurir berbeda dari installation/grant aktif. Install tidak otomatis memilih provider.

## Katalog dan dokumentasi

Pemetaan teknis ini hanya berada di dokumentasi integrasi, tidak ditampilkan pada Katalog scope Admin/Developer. Katalog berfokus pada izin resource, kegunaan dan status dari `/access-scopes/verification`; dokumentasi endpoint tidak boleh menyimpan salinan status Active atau menaikkan readiness berdasarkan keberhasilan tarif. Lihat [handoff gateway](emisell-gateway-handoff.md) dan [ADR 0027](adr/0027-isolated-managed-engine-installation.md).

Perluasan operasi memerlukan kontrak, pemetaan scope, enforcement pada seluruh jalur terkait, isolasi merchant, pengujian deny/revoke/outage serta migration path. Jangan memperluas arti `shipping.read` atau memberikan `write_shipping` otomatis.
