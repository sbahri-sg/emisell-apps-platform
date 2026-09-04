# Handoff Admin → Developer → Emisell

Tahap ini fokus pada kedua dashboard dan pemeriksaan konfigurasi untuk integrasi. Tidak membangun App Store publik, mengaktifkan billing, mengubah Payment/Shipping Gateway, atau menandai deployment production siap.

## Yang dapat digunakan

- **Developer: Apps → pilih app → Integration** (`/apps/<app-slug>/integration`). Melihat konfigurasi yang masih kurang, perbedaan draft dengan versi aktif, scopes yang benar-benar tersedia, status metadata credential, dan bukti instalasi. Tombol **Open** mengarah ke konfigurasi yang relevan.
- **Admin: App catalog → Review app**. Membaca pemeriksaan app milik organisasi yang dipilih tanpa berpindah menjadi developer organisasi tersebut. **Publish reviewed version** mengirim versi dan revisi yang diperiksa; keputusan yang sudah kedaluwarsa ditolak. Pencarian, refresh, dan paginasi memakai API, bukan data contoh.
- **Admin: Documentation → API reference**. Kontrak Admin dan Developer Management memiliki endpoint pemeriksaan masing-masing. Provider Runtime tidak mendapatkan endpoint internal ini.

Tampilan mempertahankan design system dashboard yang ada, dengan panel pemeriksaan, loading/error/retry, dan layout responsif. Tidak ada score persentase atau indikator “integration connected” yang dibuat dari konfigurasi saja.

## Makna hasil pemeriksaan

| Hasil | Makna |
| --- | --- |
| Observed / `pass` | Kondisi spesifik ditemukan dalam data backend; bukan sertifikasi integrasi |
| Needs review / `attention` | Ada pekerjaan/pemeriksaan lanjutan atau akses belum diaktifkan |
| Missing / `blocked` | Prasyarat konfigurasi tersebut belum ada |
| Information / `info` | Ringkasan dengan keterbatasan yang dijelaskan |

Pemeriksaan meliputi status organisasi/app, launch URL, versi aktif dan callback allowlist, perubahan konfigurasi yang belum diaktifkan, scopes dari katalog resmi, metadata credential aktif/belum kedaluwarsa, serta ringkasan extensions dan webhook pada versi aktif.

- `read_products` tetap pilot yang nonaktif secara default, bukan general availability. Laporan tidak membuka pilot atau mengubah allowlist merchant. Scope yang tidak dikenal juga tidak dianggap tersedia.
- Tidak semua app membutuhkan extension atau webhook. Keduanya tidak diwajibkan hanya agar checklist terlihat lengkap.
- Credential yang tercatat bukan bukti token exchange berhasil. Tidak ada secret, client ID, fingerprint, konfigurasi extension, URL mentah, data merchant, atau payload webhook dalam laporan.
- Instalasi aktif versi lama dan instalasi nonaktif tidak dihitung sebagai bukti instalasi aktif versi sekarang. Test invitation yang masih pending bukan instalasi.
- Maksimal 100 instalasi pada halaman pertama diperiksa. Jika `installations.hasMore=true`, semua hitungan adalah sampel, bukan total. Buka pengelolaan instalasi untuk pemeriksaan lebih lanjut.
- `checkedAt` adalah waktu pembacaan konfigurasi, bukan waktu tes integrasi. Laporan berasal dari beberapa pembacaan, bukan snapshot transaksi; refresh sebelum keputusan penting. Perubahan revisi app/versi saat pembacaan menghasilkan `409`.
- `endToEndVerified` selalu `false`: endpoint ini tidak menjalankan atau menyimpan hasil tes end-to-end. Bukti harus berasal dari tes terpisah. Kegagalan membaca database menghasilkan error, bukan fallback “ready”.

## Kontrak pemeriksaan

### Developer

`GET /v1/apps/{appId}/integration-readiness`

Memerlukan session developer atau control-plane bearer yang valid dan capability `app.read`. Organisasi berasal dari identitas terverifikasi; tidak bisa diganti lewat query. Owner/admin/developer/analyst dapat membaca ringkasan aman untuk organisasinya sendiri.

### Admin

`GET /v1/internal/organizations/{organizationId}/apps/{appId}/integration-readiness`

Memerlukan `platform_operator=true`. Role owner organisasi tidak cukup. Pasangan organisasi/app yang salah menghasilkan `404`. Kedua endpoint menolak semua query parameter, tidak melakukan mutasi atau external request, dan menggunakan `Cache-Control: private, no-store`.

Respons tetap mengikuti `{ "data": ... }` serta error gateway: `401` identitas tidak valid, `403` kewenangan tidak cukup, `404` app tidak ditemukan dalam lingkup tersebut, `409` app berubah selama pemeriksaan, `422` input tidak valid, `500` pembacaan dependensi gagal. Detail schema disediakan di OpenAPI, bukan kontrak terpisah.

## Publikasi setelah pemeriksaan

Pada endpoint `PUT /v1/internal/organizations/{organizationId}/apps/{appId}/catalog-listing`, UI Admin kini mengirim:

```json
{
  "category": "custom",
  "status": "published",
  "featured": false,
  "revision": 0,
  "expectedAppRevision": 2,
  "expectedActiveVersionId": "<uuid-versi-yang-diperiksa>"
}
```

Nilai di atas **ilustrasi**. Ambil `revision` dari `listingRevision`, `expectedAppRevision` dari `appRevision`, dan versi dari `activeVersionId` hasil terbaru. `revision=0` hanya untuk listing yang belum ada. Jika mendapat `409`, refresh dan periksa ulang; jangan mengganti angka lalu otomatis mengulangi publikasi.

Kedua field `expected*` harus diberikan bersama. Keduanya opsional untuk kompatibilitas klien lama; klien lama tidak memiliki perlindungan terhadap layar review yang kedaluwarsa. Namun semua pemanggil tetap mendapat pemeriksaan app/version di dalam transaksi sehingga versi tidak bisa berganti antara validasi service dan commit publikasi.

**Batas yang tetap berlaku:** ini bukan workflow baru pengajuan/review per versi, bukan production approval, dan bukan version pinning. Katalog yang sudah published masih mengikuti versi aktif sesuai perilaku sebelumnya. Pemeriksaan tidak mengganti aturan eligibility publikasi dan tidak mewajibkan semua baris menjadi `pass`. Admin tetap bertanggung jawab memeriksa bukti operasional secara terpisah.

## Checklist uji nyata sebelum integrasi digunakan

1. Admin memasukkan developer terpilih, menyetujui, dan menyerahkan undangan sekali pakai melalui jalur aman.
2. Developer menerima undangan dengan identitas yang tepat; data app organisasi lain tidak dapat diakses.
3. Developer mengonfigurasi app, membuat versi, dan mengirim test-install request ke merchant yang diizinkan. Merchant ID tetap bukan otorisasi.
4. Merchant masuk memakai identitas Emisell terverifikasi dan menyetujui scopes. Provider menukar code dengan PKCE dari backend.
5. Resource yang diizinkan dapat dibaca; scope yang tidak diberikan serta merchant berbeda ditolak.
6. Webhook yang diperlukan diterima dan signature diperiksa oleh provider. Jangan menggunakan sekadar adanya URL sebagai bukti.
7. Setelah deactivate/uninstall, resource/token tidak lagi memberi akses sesuai lifecycle. Instalasi merchant lain tetap terisolasi.
8. Admin meninjau hasil sebelum publikasi. Kunci production, routing/cookie, allowlist pilot, observability, dan rate limit deployment harus diperiksa terpisah.

Gunakan [panduan test install](./development-test-installation.md), [contoh Product Reader](../examples/product-reader/README.md), [integrasi backend](./emisell-backend-integration.md), dan [resource pilot](./resource-pilot.md). Jangan menjalankan skenario pada merchant nyata hanya untuk mengubah indikator dashboard.

## Verifikasi perubahan ini

- `npm run check`: kontrak API, pemisahan audience dokumentasi, frontend type/lint/build, Go vet dan race tests.
- `npm run test:integration-readiness`: PostgreSQL disposable milik test; verifikasi data persisten, isolasi organisasi, laporan tidak menulis audit, penolakan revisi/versi stale di transaksi, pembacaan setelah repository dibuat ulang, dan organisasi suspended.
- Test HTTP mencakup anonim, owner non-operator, tenant lain, input/query invalid, redaksi detail sensitif, perubahan draft, credential kedaluwarsa, instalasi/uninstall, dan publikasi dengan hasil pemeriksaan kedaluwarsa.

Tidak ada migration baru untuk fitur pemeriksaan. Test tidak memakai database development utama dan tidak mengaktifkan fitur billing. Status lulus test kode bukan verifikasi deployment atau seluruh integrasi api-service.

## Pekerjaan yang masih terpisah

Dashboard ini belum menyediakan approval production, perubahan entitlement/suspend organisasi, atau pencarian audit khusus Admin. Penyambungan deployment api-service, pengujian provider nyata, rate limit operasional, dan integrasi renewal billing juga belum diselesaikan oleh perubahan ini. App Store publik tetap ditunda.
