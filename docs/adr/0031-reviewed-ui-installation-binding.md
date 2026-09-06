# ADR 0031 — Binding consent aplikasi dengan UI

Status: lifecycle backend opt-in diimplementasikan dan diuji; distribusi/launch umum belum aktif.

## Keputusan

Tambahkan policy `reviewed-ui/v1` khusus instalasi UI tanpa scope bisnis atau capability. `IntentRelease.uiBinding` mengikat release ID, signed launch (app/client/release digest/URL/parent/mode) dan signature ke consent digest existing. Field opsional tidak mengubah encoding snapshot lama saat kosong. Deep copy mencegah alias mutable pada binding consent.

Lifecycle yang sama menangani Prepare, Decide, Consume, Activate, List dan Uninstall; policy baru tidak membuat endpoint merchant kedua. `WithReviewedUIAccess` memeriksa current source dan installation aktif di bawah lock; signature diverifikasi dengan public key eksplisit. Tidak ada key berarti deny. IssueToken bisnis ditolak pada policy ini. External tetap hanya mode URL, bukan SSO atau resource grant.

## Boundary source dan rollout

`ManagedReleases.WithRelease` adalah port existing untuk snapshot authoritative, bukan DTO browser. Komposisi untuk policy baru kelak WAJIB menahan current release/assignment/app-client/approved-launch locks selama callback. Lock order: source lebih dahulu, installation kemudian; uninstall tidak membutuhkan source agar pencabutan tetap tersedia saat suspend/outage.

Default server BELUM menyediakan adapter current source policy ini. Pengujian memakai source review terkontrol dan signature nyata dengan PostgreSQL isolated. Ini bukan bukti review portal telah tersambung ke instalasi, bukan public runtime, dan bukan izin memasang app developer di toko pengguna.

Tahap berikutnya: adapter persisten lintas modul dengan current release/client/review checks, authoring UI-only dan assignment, lalu RPC/Core/session issuer serta DTO sidebar. Jangan mengisi source dari katalog atau mengganti demo dengan app ID developer agar terlihat aktif.

## Compatibility dan verifikasi

Tidak ada migration DDL, backfill, perubahan instalasi/key atau restart server pengguna. Binding memakai snapshot JSON existing. Policy baru tidak dipasang pada listener default; reader Core lama belum menerima profil ini, sehingga jangan melakukan rollout hanya pada Platform. Forward rollout server/consumer terkoordinasi; uninstall tetap tersedia pada binary yang memahami policy ini, jangan downgrade setelah ada data policy baru di environment bersama.

Unit tests memeriksa dua mode, signature/key/context, perubahan URL/client/digest dan larangan scope/capability. Test PostgreSQL `TestReviewedUIPersistentLifecycle` menguji consent wajib, consume retry, aktivasi/list, isolasi actor/merchant, business token ditolak, current source denial, uninstall saat source unavailable serta penolakan setelah uninstall. Regression pilot existing tetap lulus. Belum menguji concurrent source revoke pada adapter production karena adapter tersebut belum tersedia.

## Increment: current client dan review persisten

`embedded.CurrentBinding.WithBinding` kini menggabungkan app-client persisten dan launch persisten di bawah lock. `appclient.WithBoundReady` mencocokkan seluruh binding release, ID client, status verified, TTL bukti endpoint dan keberadaan secret; `postgres.WithApproved` menahan shared row lock sampai callback selesai dan memverifikasi signature, digest serta status approved. Origin endpoint dan parent juga harus cocok. Urutan tetap release/assignment → client → launch → installation. Callback harus singkat, tanpa request remote; nilai hasil callback bukan cache otorisasi.

Port ini internal, bukan endpoint dan bukan pengganti grant merchant. Pemanggil WAJIB sudah memegang lock release/assignment authoritative, bukan menerima binding dari request. `WithReady` portal tetap memeriksa ownership dan release sebelum memakai helper yang sama. Default listener belum memasang adapter policy UI umum. Release shipping tidak boleh dikonversi dengan menghapus capability/scopes; authoring serta assignment UI-only dan komposisi source lengkap masih diperlukan sebelum RPC/Core diaktifkan.

Verifikasi tambahan: PostgreSQL menguji callback ditolak setelah revoke, invalid key, callback failure propagation, serta `FOR UPDATE NOWAIT` menghasilkan lock-not-available selama callback dan lock dilepas setelah failure. Uji portal memakai client/review persisten melalui gate baru, memeriksa parent mismatch dan revoke. Ini belum merupakan uji end-to-end general UI installation atau concurrency seluruh release/assignment.
