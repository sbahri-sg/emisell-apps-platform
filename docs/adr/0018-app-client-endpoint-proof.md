# ADR 0018 — App-client dan bukti kendali origin

Status: diterima untuk development lokal, 2026-09-05. Melanjutkan ADR 0017; tidak mengaktifkan runtime/installation umum.

## Konteks

Rilis konfigurasi signed menjamin integritas snapshot terhadap trusted key, bukan kendali server developer. Sebelum OAuth dan runtime umum dapat dirancang, platform memerlukan identitas confidential client, bukti kendali origin, credential lifecycle serta pemeriksaan yang nyata dan terbatas. Identitas client, persetujuan merchant dan access token adalah fakta berbeda.

## Keputusan

1. `oauth/appclient` memiliki aggregate client/request/audit sendiri di `platform_oauth`. Module app tetap memiliki rilis dan menyediakan port gate snapshot signed; bootstrap memetakan binding immutable tanpa cyclic import atau query schema app dari repository OAuth. HTTP hanya adapter; tidak menambah framework/service/dependency.
2. Developer hanya mendaftarkan release signed yang valid dan milik organisasinya. Satu client per release, binding app/version/digest/endpoint/redirect URI immutable. Ini identity reference per-release, bukan janji stable client ID production. Stabilitas app-level lintas versi dan migrasi redirect/grant harus diputuskan eksplisit sebelum authorization flow production dibuat.
3. Admin membaca semua client; hanya administrator boleh revoke. Developer mengelola client sendiri; operator/reviewer hanya membaca. Revoke terminal dan tetap berfungsi saat release/key tidak tersedia. Mutasi memakai revision, actor-scoped idempotency key, ownership, alasan dan audit atomik; receipt tidak menyimpan secret.
4. Challenge 256-bit berumur 10 menit pada path well-known di origin signed. Response JSON exact mengikat schema, client ID, release digest dan nonce. Proof 24 jam; perbarui challenge menghapus proof/secret lama. Status `verified` tersimpan bukan otorisasi: setiap readiness/auth memeriksa expiry dan release terkini.
5. Adapter HTTPS membatasi DNS A publik, memeriksa seluruh jawaban dan pin satu IPv4 saat dial. IPv6/NAT64, alamat private/link-local/metadata/reserved, IP literal, non-443 dan redirect ditolak. TLS CA/hostname diverifikasi, proxy environment/kompresi/keep-alive/cookies tidak digunakan. Nonce yang diharapkan tidak dikirim. Limit empat probe/process, cooldown 1 menit/client, 5 detik total dan body 4096 byte. Tidak ada dev flag untuk melewati aturan; injection DNS/dial/CA hanya unexported test seam.
6. Reservasi probe tersimpan dahulu; tidak ada lock DB selama network I/O. Gate release + revision diperiksa lagi saat completion. Revoke/renew/suspend bersamaan mengalahkan probe lama. Crash setelah reserve tidak menyebabkan retry key lama menjalankan request lagi; setelah cooldown, caller dapat retry dengan key baru.
7. Gate release memakai short shared lock sampai commit client; client memakai pool koneksi tersendiri, terpisah dari pool gate. Ini menghindari deadlock lease antar pool, tanpa memisahkan proses/database. Dengan shared release lock, suspension menunggu operasi pendek atau menang sebelum pemeriksaan ulang; outbound request tidak memperpanjang lock itu.
8. Secret random 256-bit `eacs_`, hash-only SHA-256 dan constant-time comparison. Server mengembalikan secret hanya pada mutasi penerbitan/rotasi pertama; retry hanya metadata terkini. UI menyimpan sementara di state, auto-clear 5 menit, tanpa browser storage. Rotasi/revoke membatalkan secret lama. Hash cepat dipakai untuk credential acak berentropi tinggi, bukan password manusia.
9. POST `/api/v1/app/client-check` memakai HTTP Basic + JSON kosong, menolak Origin/Cookie/query. Ini self-check server-to-server yang **tidak** menerbitkan token, mengalihkan callback, membaca resource atau memberi grant. Key Core dan token installation tidak diterima. OAuth token exchange, PKCE, authorization code, consent binding dan general runtime belum diimplementasikan; `oauthEnabled/installable/resourceGatewayAllowed` tetap false.
10. Bukti hanya origin control pada saat pemeriksaan, bukan health bisnis/callback/conformance/security scan. Kontrak konfigurasi signed v1 tidak diubah menjadi executable. Seluruh 108 resource scope tetap Plan; UI merchant, billing berbayar, resource gateway dan fixture install tetap tidak berubah.

## Dasar keamanan

Pembatasan destination, validasi resolved IP, dan penolakan redirect mengikuti pertimbangan mitigasi SSRF pada [OWASP SSRF Prevention Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Server_Side_Request_Forgery_Prevention_Cheat_Sheet.html). Mekanisme pin/limit IPv4 adalah keputusan implementasi tahap ini; production tetap memerlukan kontrol egress jaringan dan review threat model, bukan mengandalkan validasi URL saja.

Konfigurasi redirect exact dan pemisahan identitas client dari grant mempertimbangkan [OAuth 2.0 Security Best Current Practice (RFC 9700)](https://www.rfc-editor.org/rfc/rfc9700.html). Self-check ini bukan flow OAuth standar atau klaim implementasi RFC lengkap. Sebelum token exchange dibuat, wajib merancang authorization code single-use, PKCE, redirect binding, consent/tenant/app/audience, token lifetime/revocation dan replay protection bersama tim Core.

## Migration / rollback

Migration 0014 additive membuat tabel client/request/audit dan trigger binding immutable/revision/revocation. Tidak mengubah migration terdahulu, akun/key/installation atau grant. Backup privat sebelum apply eksplisit; restart backend dengan pool client dan verifier, lalu frontend. Key integrasi existing tidak dirotasi/ditimpa. Jangan membuat live client/proof/secret sebagai demo QA.

Rollback menyembunyikan menu/rute baru, mempertahankan data/audit/schema dan binary grant-aware ADR 0016. Client revoked tidak dibuka kembali. Untuk identitas lintas release/production, tambahkan kontrak/migration pemetaan yang eksplisit; jangan mutate binding lama atau membawa consent/token ke versi baru diam-diam.

## Bukti verifikasi

- PostgreSQL integration/race tests: ownership, per-role, idempotency, immutable binding, challenge/proof expiry, cooldown, secret once-only/rotasi, suspension, revoke saat probe dan authentication audience.
- Controlled TLS tests: pin IP/TLS hostname, sertifikat tidak dipercaya, DNS mixed/private, redirect tanpa request kedua, nonce salah, duplicate/unknown/trailing JSON, batas ukuran dan konkurensi/timeout. Tidak melakukan probe live developer.
- `make verify`, typecheck/test/lint/build Admin/Developer/Store, generated OpenAPI drift. Browser visual QA hanya ketika diminta pengguna; HTTP smoke test bukan bukti layout/interaksi visual telah diuji.

Panduan operator/developer: [App clients](../app-clients.md).
