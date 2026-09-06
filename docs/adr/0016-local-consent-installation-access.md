# ADR 0016 — Consume consent, grant dan token installation lokal

Status: accepted, 5 September 2026. Memperluas ADR 0010 tanpa mengaktifkan gateway resource atau executable catalog self-service.

## Konteks dan keputusan

Consent tersimpan perlu mempunyai kelanjutan yang teruji. Registry yang tersedia hanya executable fixture terverifikasi; approval/publikasi katalog masih metadata dan `installable:false`. Karena itu lifecycle dibangun pada registry tersebut, tidak menganggap metadata sebagai executable atau signature fixture publik sebagai trust production.

- Tambahkan `InstallationService` internal ConnectRPC: Consume, GetInstallation, Activate, IssueToken, Uninstall. SDK `client.Installations`. Hanya key platform full-access, tenant wajib dan owner consent sama (tenant + service + actor). Key legacy mempertahankan consent-only authority.
- Prepare baru mengikat policy `local-reviewed-fixture/v1` dalam snapshot/digest. Consent historis tanpa policy tidak dapat dikonsumsi. Prepare/Decide/Get lama tetap consent record dan `executionAllowed:false`; status consent bukan lifecycle setelah consume.
- Consume mengunci lifecycle tenant, memeriksa consent/digest/TTL/policy/release, menulis receipt single-use, installation pending, grant pending kosong, audit dan outbox dalam satu transaksi. Tidak ada active grant atau token sampai Activate.
- Activate memverifikasi release lagi, memastikan scope fixture yang benar-benar didukung dan routing capability tidak bentrok. Simulator memakai profil/integrity fixture; remote memerlukan OAuth connection dan handshake yang ada. Grant active dan installation active atomik.
- Grant berada di aggregate installation agar consume/activation/revocation atomik tanpa SQL lintas modul. Package `oauth/apptoken` hanya mengelola material opaque dan audience; tidak mengimpor storage/transport. Tidak membuat abstraction permission/billing kosong.
- Token opaque `eat_` random 256-bit, hash-only, TTL 15 menit, audience khusus self-check installation lokal. Tidak ada refresh token. IssueToken baru mencabut token sebelumnya secara atomik; retry key/body sama mengembalikan metadata terbaru tanpa secret. Respons hilang: issuance dengan key baru setelah pemeriksaan kewenangan, bukan mengambil ulang plaintext.
- Token diambil backend Core dan diteruskan hanya lewat jalur backend app tepercaya. Ini bootstrap lokal, **bukan OAuth authorization-code/token exchange untuk developer production**. Tidak ada public token endpoint atau distribusi lewat browser, query, log, event ataupun webhook biasa.
- Endpoint REST GET self-check mengikat tenant/app/installation yang cocok, audience, expiry, grant aktif dan release terkini. Tidak memberi scope resource, delegasi reusable, akses Core RPC, portal, atau façade produk. Pemanggilan Core → capability tetap memakai key Core dan current active grant.
- Uninstall mencabut grant/token dan routing dalam transaksi yang sama. Trigger revocation melindungi setiap update disabling/uninstalled, termasuk cleanup worker. Remote tetap disabling selama cleanup gagal; akses lokal sudah dicabut. Retry worker yang ada menyelesaikan uninstall dan membatalkan delivery melalui gate existing.
- Tabel consumptions/grants mempertahankan sejarah ketika reinstall menghasilkan ID baru. Retry mengembalikan status terkini milik ID lama dan tidak boleh memodifikasi instalasi pengganti. Legacy mutation dilarang untuk installation intent-managed; legacy data tidak dimigrasi menjadi grant.

## Batas keamanan

Semua 108 scope Shopify resource tetap Plan, bukan namespace scope fixture. Required scope asing/unsupported gagal tertutup; manifest executable fixture tidak mempunyai optional scope selection. Draft metadata optional resource tidak diberikan karena metadata tidak dapat diinstal. Listing tetap gratis/non-installable. Tidak ada menu tenant/merchant, gateway Core baru, upload executable, app-code scanner, WASM/UI runtime atau biaya install baru.

Core tetap bertanggung jawab atas sesi staf, membership, CSRF dan eksplisit consent. Platform tidak membuktikan klik merchant secara independen. Actor lain/credential baru tidak otomatis mengambil alih installation; recovery ownership memerlukan desain serta persetujuan terpisah. Credential rotation service ID membutuhkan handoff yang dirancang, bukan migrasi otomatis.

## Migration, rollout, rollback

1. Jalankan seluruh tes pada `emisell_local_test`. Backup database lokal sebelum explicit migration 0012. Migration additive, tidak mengubah akun, key, consent ataupun status instalasi existing.
2. Apply migration memakai CLI, lalu jalankan binary baru. Pertahankan frontend localhost yang ada; kontrak UI dihasilkan dari sumber Protobuf/OpenAPI. Tidak melakukan auto-install pada startup atau live-data smoke test.
3. Rollout opt-in per consumer: fresh Prepare → Decide → Consume. Legacy callers tetap pada API lama untuk installation lama. Jangan mencoba legacy install sebagai fallback ketika Consume ditolak.
4. Sebelum ada consumption, rollback binary lama aman dengan tabel baru tetap disimpan. **Sesudah penggunaan lifecycle baru, jangan rollback ke binary pra-0012 yang tidak memahami grant/ownership.** Gunakan forward-fix; hentikan caller baru dan cabut akses melalui binary kompatibel jika perlu. Jangan drop tabel audit/token/grant atau merestore backup di atas data baru tanpa keputusan eksplisit.
5. Production memerlukan registry executable sungguhan (artifact/storage, review/scanning/signing trust), grant/resource policy, OAuth developer-client binding dan secure token delivery/exchange, TLS/server identity, rate/quota, retention, operational key rotation/recovery, serta gateway Core terverifikasi. Local self-check tidak memenuhi gate tersebut.

## Verifikasi

Pengujian meliputi pending/deny/expired/historical consent, digest/release mismatch, metadata non-executable, tenant/service/actor isolation, scope allowlist, concurrent consume/token issuance, request collision, capability routing conflict, transaksi gagal, token sekali tampil/hash-only/expiry/rotation/revocation, penggunaan lintas audience, reinstall, dan remote handshake/cleanup outage. Build, race suite, Buf lint/breaking, vet, generated-doc consistency serta verifikasi frontend wajib lulus. Hasil test fixture bukan bukti production readiness.
