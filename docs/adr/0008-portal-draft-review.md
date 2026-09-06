# ADR 0008 — Identitas portal dan alur draft–review

Tanggal: 5 September 2026. Status: diimplementasikan untuk development lokal; bukan publikasi/production readiness.

## Scope milestone

Admin di `4317` dan Developer di `4319` adalah frontend terpisah, memakai satu engine/API Go `8087`. App Store `4318`, signing, publish, suspend, registrasi publik, undangan tim, credential aplikasi, SDK/CLI developer, runtime aplikasi unggahan, dan billing belum termasuk milestone ini. Semua app tetap gratis. Merchant workspace tidak dibuat ulang.

## Identitas dan authorization

- `identity` memiliki `portal_accounts`, `portal_sessions`, dan audit login/logout yang terpisah dari user/session owner merchant dan service account Core. Password menggunakan mekanisme hashing existing; token acak hanya disimpan dalam bentuk hash, expiry 8 jam. Relogin mengganti sesi sebelumnya pada surface yang sama. Disable akun diperiksa setiap request.
- Admin: `administrator` dan `reviewer` dapat melihat submission dan memutuskan review. `operator` hanya membaca. Tidak ada endpoint browser untuk membuat atau meningkatkan privilege admin. Role management UI belum tersedia.
- `developer` memiliki organisasi dan membership. Increment ini satu owner/organisasi per akun; CLI menyediakan akun lokal secara eksplisit. Membership tidak sama dengan tenant merchant. Organisasi diturunkan dari principal, bukan body/query. Tidak ada akses draft developer lain; Admin hanya melihat snapshot yang sudah diajukan, bukan draft privat.
- Cookie `emisell_admin_session` memakai Path `/api/v1/admin`; cookie `emisell_developer_session` memakai Path `/api/v1/developer`. Keduanya HttpOnly/SameSite Strict. Cookie berbeda, audience database, authorization server-side, dan Origin/Referer exact bersama-sama membatasi akses; **port/cookie Path saja bukan security boundary**.
- Origin/Referer diperiksa untuk pembacaan portal; mutation wajib Origin exact dan JSON. Tidak ada wildcard CORS, browser token storage, tenant selector, atau password di bundle. Login dibatasi 30 percobaan/menit/surface pada proses lokal. Profil ini hanya loopback HTTP. Production memerlukan HTTPS/Secure cookies, strategi host/session production, rate limit terdistribusi, recovery/rotasi kredensial, MFA dan onboarding terverifikasi.
- Provisioning `cli init-portals` menyimpan credential acak pada `.local/portals.json` mode 0600. Rerun memverifikasi identitas/role/password dan tidak me-reset akun, tidak melakukan privilege upgrade, serta tidak mengubah akun merchant.

## Draft dan submission

- Modul `app` memiliki draft, nomor revisi optimistic concurrency, request deduplication, dan audit draft. ID aplikasi ditentukan server, tidak menimpa ID/release fixture existing.
- `AppDocument` adalah **kontrak authoring**, bukan executable manifest `emisell.app/v1`. Draft mencakup nama, ringkasan, deskripsi, versi stabil `major.minor.patch`, capability reference, baseline scopes, dan endpoint HTTPS. Prerelease/build metadata belum diterima pada authoring v1. Runtime hanya Remote App pada milestone ini.
- Draft boleh belum lengkap. Submission wajib ringkasan/deskripsi/endpoint dan seluruh baseline scopes yang sesuai capability. Endpoint hanya disimpan sebagai teks; tidak dites/fetch/diikuti link oleh platform. Dukungan credential di URL dilarang. Egress/SSRF, artifact, runtime handshake, dan manifest lengkap harus diperiksa sebelum implementasi eksekusi/publikasi.
- `review` mengambil draft melalui port/service publik, kemudian menyimpan snapshot immutable dalam schema sendiri. Tidak ada pembacaan/penulisan tabel app/developer oleh repository review. Edit draft setelah submit tidak mengubah snapshot. Submitted revision adalah revisi yang dimuat ketika submit; edit concurrent dapat membuat revisi baru tanpa mengubah snapshot tersebut.
- Satu submission pending per app. Revisi yang sama tidak bisa diajukan ulang dengan key baru. `changes_requested` membolehkan perbaikan dalam revisi baru pada versi yang sama. `approved`/`rejected` menutup versi tersebut; pengajuan berikutnya memerlukan nomor versi baru.
- Hanya transisi `submitted → changes_requested | approved | rejected`. Catatan nonblank wajib untuk semua keputusan. Keputusan dan audit atomik, row lock mencegah dua reviewer sama-sama memutuskan. Submitter tidak boleh memutuskan submission-nya sendiri. Tidak ada auto-publish atau signing sebagai efek approve.
- Save, submit, decision memakai `Idempotency-Key`; key/input sama mengembalikan response pertama, input berubah menghasilkan 409. Audit/request/result commit atomik. Hak akses diperiksa sebelum replay. Client menyimpan key retry dalam memori per instance/surface, mempertahankannya setelah kegagalan jaringan; tidak otomatis mengulang operasi dengan key baru.
- List mengembalikan maksimum 200 record terbaru dengan urutan deterministik. UI menyatakan batas tersebut; search/filter bekerja pada kumpulan ini, bukan klaim pencarian seluruh platform. Pagination server menjadi increment saat volumenya membutuhkan.
- Database adalah sumber data; reload/login ulang mempertahankan draft, submission, feedback, dan history. Tidak ada angka/grafik palsu.

## Frontend

`web/dashboard` tetap memakai tooling existing. `web/developer` memakai versi React/Vinext/tooling yang sama; tidak menambah framework. Kedua build memakai primitive, tema, komponen presentasi dan API client terparameterisasi surface di `web/dashboard`; state/sesi tetap terpisah di masing-masing instance. Entry point memilih surface secara statis, bukan dari URL, role selector, atau localStorage. Alias developer mengarah pada primitive existing dan React dideduplikasi. Tailwind source eksplisit menjaga kelas shared terkompilasi pada kedua build.

Ini reuse incremental, bukan izin mencampurkan hak akses. Jika reuse bertambah, ekstraksi package UI bersama dilakukan melalui migration path tersendiri. Keduanya lokal; hosting config existing tidak diubah dan tidak ada deploy Sites.

## Migration dan rollout

1. Migration `0007_portal_identity_drafts_review.sql` additive dan ber-checksum. Tidak mengubah migration existing, membership merchant, installation, resource pembayaran, atau release terpasang. Rollback aplikasi mempertahankan tabel/audit; jangan DROP schema untuk rollback UI.
2. Database dev sebelumnya baru sampai 0005. CLI existing menerapkan seluruh migration urut, sehingga rollout API ini juga memerlukan migration additive 0006 dan reader event compatible dengan source payment yang sudah ada. Full backend integration/race tests dijalankan sebelum rollout.
3. Snapshot database dan binary lama dicadangkan privat. API/worker/Core consumer lama dihentikan, migration/provisioning dijalankan; schema Core disiapkan tanpa rotasi token, lalu Core consumer baru dinyalakan sebelum worker/API baru. Schema reference remote additive disiapkan dengan credential existing dipertahankan; tidak ada `--simulate`, replay bisnis, transaksi asli, maupun reinstall.
4. Bagian backend source tahap 5 ikut termuat pada runtime lokal baru. Ini bukan penyelesaian tahap 5 secara keseluruhan: dokumentasi kontrak payment/reconciliation dan kesiapan production masih memerlukan milestone tersendiri. Panel payment merchant tetap dihapus. Jangan menjalankan consumer whitelist lama terhadap event type baru ketika rollback.
5. API legacy owner tetap tersedia untuk compatibility, tetapi tidak dikonsumsi portal. Deprecation API lama menunggu integrasi consent/Core pengganti.

## Verifikasi

- Unit validation document/versi/scopes dan browser API client retry-key.
- PostgreSQL integration: cookie audience/merchant isolation, cross-developer list/detail/edit/submit/history denial, exact Origin, operator read-only, optimistic conflict, immutable snapshot, request replay setelah draft berubah, resubmission/version policy, concurrent submit/decisions, relogin/logout, audit, serta tidak ada release publik setelah approve.
- Migration repeatable/checksum, Go race tests/build/vet dan Buf lint/breaking baseline; frontend typecheck/lint/test/build pada kedua surface.
- Browser: login dua peran, buat draft shipping, submit, request changes, edit/resubmit, approve, persistence, filter, desktop/mobile, drawer dan overflow. Satu app berlabel `QA Portal Shipping` milik akun local developer kedua dipertahankan sebagai data QA/audit, tidak diterbitkan atau dipasang.
