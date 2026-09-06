# Emisell Portal Developer

Frontend lokal terpisah pada `http://localhost:4319/`. Satu backend Go `8087`; bukan microservice baru. Acuan: `codex.md` bagian 1.2–1.3 dan ADR 0008.

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
