## Apa yang perlu diverifikasi

**Autentikasi** membuktikan identitas pemanggil. **Otorisasi** menentukan data dan tindakan yang diizinkan. Keduanya diperlukan: identitas yang valid belum tentu memiliki izin membaca produk atau pesanan.

Untuk aplikasi embedded, backend harus memastikan identitas terikat pada aplikasi, client, toko, instalasi, dan staf yang benar. Jangan mempercayai merchant ID dari query browser sebagai bukti kepemilikan toko.

## Cara kerja

1. Seller memasang aplikasi setelah meninjau izin yang diminta.
2. Aplikasi embedded meminta identitas melalui bridge dari Dashboard seller yang diizinkan.
3. Backend aplikasi memverifikasi identitas dan binding yang diterima.
4. Setiap permintaan resource diperiksa terhadap scope dan akses instalasi terkini.
5. Data dikembalikan hanya jika semua pemeriksaan berhasil.

Grant dapat berubah setelah identitas diterbitkan. Permintaan berikutnya harus ditolak bila akses dicabut atau instalasi tidak lagi berlaku.

## Istilah utama

### App ID dan client ID

App ID mengidentifikasi aplikasi terdaftar. Client ID mengidentifikasi client yang dikonfigurasi untuk aplikasi. Keduanya bukan pengganti secret maupun persetujuan seller.

### Client secret

Secret membuktikan identitas confidential client pada backend. Simpan privat, bukan pada source React, URL, browser storage, atau log. Lihat [pengelolaan credential](/docs/credentials).

### Identitas embedded

Identitas dari Dashboard harus diverifikasi sebelum dipakai. Verifikasi mencakup signature, issuer, audience/client, waktu berlaku, dan binding aplikasi/instalasi/toko/staf sesuai kontrak backend. Token tidak boleh menentukan sendiri endpoint verifikasi yang dipercaya.

### Scope dan grant

Scope menyatakan jenis akses, seperti `read_products`. Grant adalah akses yang berlaku untuk instalasi dan toko tersebut. Deklarasi scope di metadata tidak otomatis menciptakan grant.

## Pilih jalur implementasi

| Kebutuhan | Jalur yang tersedia |
| --- | --- |
| Pengujian embedded lokal | Template CLI, adapter Core lokal, serta alur review dan seller install |
| Hosting aplikasi | Runtime produksi dan adapter backend nyata yang dikonfigurasi operator |
| Mengakses resource lain | Deklarasi scope yang didukung, review versi, dan persetujuan seller |

> Jangan menganggap Emisell memakai alur OAuth, token exchange, atau header platform lain. Ikuti kontrak dan adapter Emisell yang benar-benar diaktifkan di lingkungan Anda.

## Saat akses ditolak

Bedakan kegagalan identitas, penolakan izin, dan dependency backend yang tidak tersedia. Jangan mengatasi penolakan dengan menonaktifkan pemeriksaan atau meneruskan cookie seller ke aplikasi. Gunakan [troubleshooting](/docs/troubleshooting) untuk urutan pemeriksaan.
