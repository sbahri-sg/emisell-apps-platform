# Kontrak internal v1

Boundary: Emisell Core → App Platform, bukan RPC antar-modul monolith.

- Package `emisell.payment.v1`: `Create`, `Capture`, `Refund`, `Status`.
- Package `emisell.shipping.v1`: `GetRates`, `Create`, `Track`.
- Request payment/shipping wajib tenant terotorisasi dan idempotency key 16–128 karakter `[A-Za-z0-9_-]`.
- Header `Authorization: Bearer <service-token>`; browser session tidak diterima.
- Amount integer minor units. Shipping weight integer gram. Rilis ini IDR, simulator, full capture/refund, zona fixture saja.
- Reference 1–128 karakter `[A-Za-z0-9_-]`; payment amount 1–1.000.000.000.000; shipping weight 1–30.000 gram.
- Respons tidak mengandung tipe/nama provider; `installation_id` hanya identifier routing untuk audit.
- Deadline client delapan detik, handler sepuluh detik. Setelah timeout, hasil bisa ambigu: ulangi request dengan key/body yang sama.
- Key baru untuk operasi bisnis berbeda. Mengganti provider tidak memindahkan resource lama; retry key lama setelah pergantian routing bisa ditolak conflict, bukan dijalankan diam-diam pada provider baru.

`make generate` menggunakan plugin yang dipin di `go.mod`. `make contracts` memeriksa format, lint STANDARD, dan compatibility terhadap `api/proto-baseline.binpb`. Baseline initial v1 tidak boleh diperbarui untuk menyembunyikan breaking change; gunakan versi baru.

Detail error, authentication, event compatibility, dan migration path: `docs/adr/0003-core-rpc-and-event-delivery.md`.

## Install intent (additive)

- Package `emisell.installation.v1`: `InstallIntentService.Prepare`, `Get`, `Decide`; SDK `client.InstallIntents`.
- Request tidak mempunyai field tenant. Tenant berasal dari authenticated Core service principal; `core_actor_id` adalah assertion backend Core setelah memeriksa sesi dan izin staf.
- Scope operasi terpisah: `apps.install_intents.write`, `.read`, `.consent` pada prefix `apps.install_intents`.
- Create/decision wajib idempotency key. Get tidak memutasi state. TTL 10 menit; retry mengembalikan state terkini, termasuk expired.
- Consent tidak membuat installation/permission/token. `execution_allowed=false`; local fixture-only. Katalog metadata tidak diterima.
- Internal listener menolak Cookie dan Origin. Consent UI di Core, bukan tiga frontend platform.

Kontrak rinci dan migration path: `docs/core-install-intents.md` dan ADR 0010. Baseline payment/shipping tetap dipertahankan.
