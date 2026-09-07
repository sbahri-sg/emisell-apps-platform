# Verifikasi lifecycle remote lokal

Status 7 September 2026: pengujian backend lokal lulus; **bukan sertifikasi
webhook universal atau kesiapan produksi**. Tidak ada perubahan runtime,
deployment, credential produksi, maupun jalur built-in Emisell Kurir.

## Alur yang diverifikasi

`TestConsentInstallationRemoteHandshakeAndCleanup` kini menguji satu rantai:

1. Prepare/consume consent melalui SDK Core dan buat instalasi pending.
2. Tolak aktivasi sebelum koneksi remote tersedia.
3. Jalankan OAuth reference lokal, aktivasi, dan penerbitan app token.
4. Panggil capability simulasi dan ambil event hasil pemanggilan sebenarnya.
5. Ingest event dua kali: tetap hanya satu delivery.
6. Receiver menerima webhook tetapi respons dibuat hilang: delivery menjadi pending.
7. Retry delivery: berhasil, receipt receiver tetap satu.
8. Antrekan event lain, lalu uninstall saat endpoint cleanup remote tidak tersedia.
9. Token langsung ditolak, API tidak bisa dipanggil, webhook antrean dibatalkan.
10. Pulihkan receiver dan selesaikan cleanup menjadi uninstalled.

Tes pendamping memeriksa isolasi tenant/session, replay OAuth, refresh-token
rotation, signature/body/timestamp webhook, replay setelah uninstall, serta
persistensi queue dan ACK setelah broker NATS sementara direstart.

## Menjalankan ulang

Gunakan database terisolasi bernama `emisell_local_test` pada loopback. Jangan
menggunakan database development merchant atau produksi. Fixture membuat data
simulasi unik, menjalankan migration pada database test, dan menyimpan audit test.
Broker dan HTTP receiver memakai port loopback sementara; broker pengguna tidak
direstart. Reference payment hanyalah fixture tanpa transaksi finansial/provider.

```sh
export EMISELL_TEST_DATABASE_URL='<URL PostgreSQL loopback untuk emisell_local_test>'
export EMISELL_NATS_SERVER='/absolute/path/to/pinned/nats-server'
make test-remote-lifecycle
```

Target menolak konfigurasi kosong agar hasil tidak tampak lulus akibat seluruh
tes integrasi di-skip. Hasil saat pengerjaan: lima tes integrasi utama lulus
dengan race detector, termasuk dua subtes consent simulator.

## Sebelum webhook produksi

- Implementasikan subscription event yang direview dan terikat installation/scope;
  runtime yang diuji saat ini hanya event `emisell.capability.invoked.v1` untuk
  profil `local-remote`, bukan event order/shipment universal.
- Siapkan resolver endpoint HTTPS tepercaya, perlindungan egress/SSRF, dan pengelolaan
  serta rotasi secret per instalasi. Jangan mengaktifkan konfigurasi loopback di produksi.
- Siapkan worker/broker produksi terpisah dengan health check, alert backlog/dead
  delivery, recovery, dan uji outage. Compose dashboard saat ini tidak menyediakan worker.
- Uji receiver aplikasi sebenarnya, scope yang dibutuhkan, throttling, duplikasi,
  event terlambat/tidak berurutan, dan revoke pada staging sebelum rollout terbatas.
- Jalankan pengujian Dashboard → api-service → Platform dengan aplikasi yang dituju;
  tes ini memanggil SDK backend, bukan menguji antarmuka seller atau RajaOngkir.

Jangan menyimpulkan worker sehat dari antrean kosong. Jangan menjanjikan exactly-once
atau urutan global: receiver harus melakukan deduplikasi secara transaksional.
