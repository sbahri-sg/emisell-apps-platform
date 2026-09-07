# Izin pengiriman: built-in dan aplikasi eksternal

Pembaruan 7 September 2026. Dokumentasi ini bukan bukti aktivasi atau grant merchant.

**Emisell Kurir adalah built-in:** aktivasi melalui backend Emisell dan API-Kurir,
tanpa instalasi atau grant Apps Platform. Kontrak pilot lokal lama hanya referensi
teknis dengan environment `local-isolated`, bukan alur built-in production yang harus diaktifkan.

Untuk aplikasi provider eksternal seperti RajaOngkir, Platform mengelola consent
dan grant instalasi; API-Kurir tetap menyediakan engine tarif/pengiriman.

| Identifier | Kegunaan | Status/batas |
|---|---|---|
| `shipping.read` | Izin native membaca layanan shipping | Tarif eksternal opt-in di API-Kurir; belum rollout production |
| `shipping.write` | Izin native perubahan konfigurasi provider | Pengaitan credential oleh operator; bukan bukti shipment/pickup sudah dilindungi |
| `rates.read` | Operasi engine membutuhkan `shipping.read` | Pemeriksaan sebelum cache tarif bila diaktifkan |
| `settings.write` | Operasi engine membutuhkan `shipping.write` | Alat pengaitan credential, bukan endpoint seller publik |
| `settings.read`, `tracking.read`, `shipments.create` | Operasi kontrak provider | Deklarasi bukan bukti enforcement seluruh jalur runtime |
| `read_shipping`, `write_shipping` | Resource carrier service Gateway | Tetap Planned/tidak grantable; bukan alias scope native |

`shipping/v1` adalah versi capability internal, bukan scope atau izin semua operasi.

## Batas keamanan dan aktivasi

- Aktifkan hanya pasangan merchant/provider yang disetujui secara eksplisit.
- Identitas merchant/aplikasi/instalasi harus cocok; scope harus ada pada consent
  dan grant aktif. Credential tetap milik merchant.
- Integrasi yang diaktifkan harus menolak saat grant ditolak/tidak tersedia,
  tanpa fallback legacy. Uninstall tidak menghapus riwayat pengiriman.
- Emisell Kurir dan merchant existing tidak dimigrasikan otomatis.
- API-Kurir memuat implementasi opt-in, tetapi konfigurasi production nonaktif.
  Integrasi instalasi provider Platform dan uji end-to-end belum selesai.
- Jangan menganggap `shipping.write` membuka shipment, pickup, tracking atau order.

## Katalog

Katalog Admin/Developer menampilkan resource scope berdasarkan verification
gateway. Status tidak dinaikkan karena tes tarif atau deployment kode berhasil.
Lihat [ADR 0027](adr/0027-isolated-managed-engine-installation.md) untuk sejarah
pilot; kontrak pilot tidak mewajibkan Emisell Kurir production memakai Platform.
