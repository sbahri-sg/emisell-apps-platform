# ADR 0010 — Core grant/install intent, sebelum instalasi

Status: diterima untuk development lokal. Tanggal: 2026-09-05.

Amendemen ADR 0014: key platform full access memakai tenant assertion per request; binding credential di bawah tetap berlaku untuk key legacy. Batas consent-record/non-activation tidak berubah.

## Keputusan dan batas milestone

Tambahkan `emisell.installation.v1.InstallIntentService` pada internal ConnectRPC dan SDK Go. Tenant berasal dari service principal terautentikasi, **bukan** body/query/browser. Tidak ada halaman merchant dalam tiga frontend platform. Consent UI dan pemeriksaan hak staf berada di Emisell Core.

Milestone ini menyimpan intent serta keputusan consent/deny di PostgreSQL dengan audit atomik. **Consent recorded bukan permission grant aktif**: tidak membuat installation, token, OAuth connection, event `installed`, atau aktivasi. Semua respons menyatakan `execution_allowed=false`. Tidak ada RPC consume pada milestone ini.

Sumber intent hanya registry executable fixture lokal yang sudah diverifikasi, bukan `catalog_releases`, draft, maupun submission. Signature fixture memakai public test key sehingga hanya layak untuk development loopback. Listing publik tetap `installable:false`. Ini bukan pipeline executable release production.

## Trust boundary

- Service account first-party Core, terikat tepat satu tenant, memerlukan scope khusus `apps.install_intents.read`, `apps.install_intents.write`, atau `apps.install_intents.consent` sesuai operasi. Scope payment/shipping tidak memberikan akses ini. Kredensial Core yang ada **tidak** diperluas atau dirotasi otomatis.
- `core_actor_id` adalah ID opaque stabil dari sesi staf Core, bukan email. Hanya backend Core tepercaya yang boleh mengirimnya. Core WAJIB memeriksa sesi, CSRF, tenant membership, dan izin mengelola aplikasi pada create/read/decision; untuk consent harus ada klik persetujuan eksplisit. Platform mengaudit assertion Core ini, **bukan membuktikan sesi merchant secara independen**. DILARANG memberikan credential scope consent kepada app pihak ketiga, frontend, developer, atau browser.
- Intent terikat tenant + service ID + actor ID. Service lain pada tenant yang sama tetap tidak dapat membaca/memutuskan intent. Rotasi token dengan service ID sama boleh melanjutkan jika scope masih sesuai. Revokasi/expiry berlaku pada setiap request.
- Internal listener menolak Origin dan Cookie, tetap loopback. ID intent dan digest bukan bearer token. Tidak ada redirect URL, authorization code, atau token dalam URL.

## Snapshot dan lifecycle

- Create menerima app ID dan versi tepat, tanpa scope pilihan dari caller. Snapshot nama, developer, versi, scopes, capabilities dan profil eksekusi diambil dari registry.
- Manifest digest adalah SHA-256 JSON Go `appmanifest.Manifest` terverifikasi dengan array scopes/capabilities/subscriptions terurut (bukan hash file asli). Consent digest SHA-256 mengikat ID intent, owner, snapshot, created/expiry. Digest dihitung oleh server; caller hanya mengembalikan digest yang ditampilkan di Core.
- TTL tetap 10 menit memakai clock PostgreSQL setelah mendapat transaction lock. State tersimpan `pending`, `consented`, `denied`. Setelah TTL, `pending`/`consented` ditampilkan `expired`; read tidak menulis state/audit. Denial tetap terminal. Tidak ada perpanjangan TTL.
- Hanya satu keputusan dari `pending`, sebelum expiry, dengan actor dan digest cocok. Saat consent, registry diverifikasi ulang dan digest harus tetap sama. Deny tidak memerlukan registry tersedia. Manifest/versi/scope berubah memerlukan intent dan persetujuan baru.
- Idempotency key 16–128 karakter `[A-Za-z0-9_-]`, namespace tenant + service + actor, lintas operasi. Payload berbeda pada key sama ditolak. Retry identik menunjuk intent sama dan mengembalikan **state terkini**, termasuk `expired`, bukan response consent lama. Keputusan dengan key baru setelah terminal ditolak. Audit hanya sekali per mutation berhasil. Request gagal tidak dicache.
- Transaksi dan lock tenant menserialisasi keputusan konkuren, idempotency, snapshot, dan audit. SQL intent hanya menyentuh schema installation; pembacaan registry melalui contract app service. Tidak ada publish NATS karena belum terjadi instalasi/perubahan akses.

## Migration path

Migration additive `0009_install_intents.sql`; migration lama dan kontrak payment/shipping tidak berubah. Rollback aplikasi dapat memakai binary lama sambil mempertahankan tabel intent/audit; jangan drop data persetujuan. Intent lama tidak boleh otomatis menjadi grant aktif saat fitur consume ditambahkan.

Tahap berikutnya: executable release policy/trust root production, instalasi dengan konsumsi intent atomik dan sekali pakai, entitlement gratis, grant aktif, OAuth/setup, lalu activation. Scope expansion wajib reconsent. UI Core mengintegrasikan kontrak hanya sesudah pemeriksaan sesi/izin dan mapping penjelasan scope tersedia. Perlu rate/quota intent, retention audit, TLS identity, credential provisioning terkontrol, serta threat model sebelum production.

Legacy HTTP install lokal dipertahankan demi kompatibilitas fixture dan regression test. Endpoint itu belum memakai intent; DILARANG menggunakannya sebagai jalur install production atau fallback untuk melewati consent. Migrasi/deprecation endpoint dilakukan saat jalur consume siap, bukan klaim bahwa milestone ini sudah mengamankan semua instalasi lama.
