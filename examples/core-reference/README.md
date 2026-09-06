# Client referensi Emisell Core

Client ini membuktikan kontrak capability dan delivery event, bukan implementasi bisnis Emisell Core produksi. Jalankan dari root repository setelah setup pada `docs/local-development.md`.

- `cmd/core-reference` mengirim tujuh operasi payment/shipping melalui generated Connect client; tidak memiliki konfigurasi provider.
- `pkg/sdk` berisi client dan kontrak publik. Package domain/installation/runtime platform tidak menjadi dependency client.
- `inbox.go` merupakan consumer contoh. Efeknya hanya menyimpan receipt, **bukan** memperbarui order/payment sungguhan.
- `inbox.sql` dimiliki contoh Core dan diterapkan eksplisit oleh `init-core`. Engine tidak membaca schema ini.

Untuk integrasi repository Core sesungguhnya, bawa client/contract version yang dipin, service identity yang disetujui, dan implementasi inbox ke database Core. Ganti penghitung receipt dengan use case Core pada transaksi yang sama atau durable workflow yang sesuai. Jangan memindahkan token broker operator, database owner, atau file `.local` ke frontend/hosting.

Delivery adalah at-least-once. Simpan event ID dalam inbox dan ACK setelah commit; jangan mengandalkan deduplication broker sebagai pengganti transaksi. Raw event/version/tenant/subject harus divalidasi sebelum side effect.
