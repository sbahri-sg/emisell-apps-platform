# Emisell Portal Developer

## Tampilan developer

Portal tetap berada dalam project Apps Platform, termasuk ketika diakses melalui login terpadu. Tidak ada project atau domain `emisell.dev`. Tema gelap/mint hanya diterapkan pada sesi developer; administrator mempertahankan tampilannya.

- **Aplikasi:** pencarian nama/app ID, filter publikasi/draft, dan status dari API existing.
- **Buat aplikasi:** pilihan Emisell CLI atau membuat draft melalui endpoint `/api/v1/developer/apps`; setelah berhasil, langsung membuka Overview aplikasi. Editor revisi dan pengajuan review tersedia melalui Versions.
- **Menu aplikasi:** Overview, Monitoring, Logs, Versions, App settings. Credential berada di App settings dan pengajuan berada di Versions; tautan query lama tetap didukung. Monitoring membedakan status konfigurasi dari metrik runtime yang belum tersedia. Logs menampilkan aktivitas draft/pengajuan/keputusan, bukan log request atau seluruh audit historis. Lihat `docs/developer-portal-navigation.md` untuk batas implementasi.
- **Overview:** versi draft, pengajuan/feedback, jumlah toko pengujian yang disetujui, serta pintasan konfigurasi. Penugasan testing tidak dihitung sebagai instalasi.
- **Credential & pengajuan per aplikasi:** pilihan App ID dipertahankan saat berpindah antara ringkasan, credential, dan pengajuan; URL `?view=app-clients&app=...` atau `?view=reviews&app=...` dapat dimuat ulang. App ID hanya memilih data dari respons API organisasi, bukan memberikan hak akses. App ID yang tidak ada di daftar tidak membuka seluruh data sebagai fallback.
- **Pengajuan:** filter status dan pencarian nama/versi, feedback ringkas, urutan terbaru, serta detail review dari aktivitas aplikasi. Menu Semua aplikasi menghapus konteks aplikasi; alat organisasi seperti testing dan rilis tetap menampilkan cakupan organisasi secara terpisah.
- **Secret:** tersedia sejak create app, bisa ditampilkan/disalin/dirotasi di Settings. Plaintext hanya dimuat melalui tindakan eksplisit, hilang setelah 60 detik/tab disembunyikan/berganti aplikasi, dan tidak disimpan di URL/browser storage. Client rilis legacy tetap terpisah; lihat `docs/application-credentials.md`.
- Ringkasan menampilkan metadata aplikasi dan status pengajuan; panel metrik API yang belum tersedia tidak ditampilkan.
- Testing, app clients, rilis UI/integrasi, scope, katalog, dan developer tools tetap tersedia. Tidak ada perubahan auth, endpoint, grant, database, atau deployment untuk perubahan tampilan ini.

Implementasi bersama: `web/dashboard/components/developer-workspace.tsx` dan stylesheet terisolasi `web/dashboard/app/developer-redesign.css`. Kedua layout memuat stylesheet agar portal dedicated dan login terpadu konsisten.

Portal utama sekarang `/development` pada frontend `web/dashboard` (lokal `http://localhost:4317/development`). Dokumentasi publik berada di `/`, administrasi di `/admin`. Frontend `4319` di bawah dipertahankan sebagai entrypoint development dedicated/legacy, bukan domain publik terpisah.

Frontend lokal dedicated menyediakan dokumentasi pada `http://localhost:4319/` dan login developer pada `http://localhost:4319/development`. Satu backend Go `8087`; bukan microservice baru. Entrypoint ini tetap developer-only (tanpa `/admin`), sedangkan portal lengkap berada pada 4317. Acuan: `codex.md` bagian 1.2–1.3 dan ADR 0008.

Tersedia: login developer, ownership organisasi, draft Aplikasi Integrasi `shipping/v1`, revisi, submit snapshot, feedback/history/resubmit, validasi/export metadata dan akses paket katalog bertanda tangan milik organisasi. Menu **Rilis integrasi** menyediakan validasi dan pengajuan konfigurasi endpoint/callback/health untuk review terpisah (ADR 0017, codex §1.12). Nama teknis tetap `remote`. Signing/publish hanya melalui Administrator. Tidak ada merchant workspace, instalasi, signup publik, undangan tim atau eksekusi endpoint. App clients dan distribusi Testing tersedia sebagai persiapan terpisah; konfigurasi signed belum executable, runtime umum dan OAuth token exchange belum tersedia.

## Menjalankan

Siapkan akun dengan `go run ./cmd/cli init-portals` dari root setelah meninjau migration/rollout dalam `docs/local-development.md`. Password privat ada di `.local/portals.json`. Akun owner toko tidak berlaku di sini.

```sh
npm ci
npm run dev
```

Versi React/Vinext/tooling memakai versi existing Admin. Primitive/presentasi shared tetap di `web/dashboard`, diimpor melalui alias; kedua entrypoint, build, proses, session dan request audience terpisah. Tidak ada config/deploy Sites baru. Jangan mengganti origin atau menggabungkan proxy Admin ke Developer.

## Verifikasi

```sh
npm run typecheck
npm run lint
npm test
npm run build
```

Tests API client shared ada di `web/dashboard/lib/portal.test.ts`. Jalankan verifikasi kedua frontend bila mengubah komponen/tema shared. Tailwind source eksplisit dibutuhkan agar kelas primitive luar root ikut terkompilasi. Jika perubahan konfigurasi menyebabkan Vinext dev gagal reload, restart proses Developer yang tepat; jangan pindah port secara otomatis.

Payment gateway merupakan integrasi internal Emisell (ADR 0022). Record payment lama hanya untuk history/verifikasi; bukan pengajuan/distribusi baru. Runtime umum dan CLI developer belum tersedia.
