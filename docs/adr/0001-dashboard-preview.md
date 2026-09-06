# ADR 0001 — Frontend dashboard terpisah dari engine Go

> Historis: scope merchant UI pada ADR ini telah dibatalkan dan source-nya dihapus. Keputusan frontend terbaru adalah tiga surface pada ADR 0007; jangan memakai dokumen ini untuk membangun kembali workspace merchant.

Tanggal: 5 September 2026. Status: diterapkan untuk increment dashboard.

## Konteks

Pengguna menyetujui implementasi dashboard modern sesuai `codex.md`. Saat pekerjaan dimulai, source aplikasi lama tercatat terhapus dalam working tree; yang tersisa adalah konfigurasi dan dokumentasi. Penghapusan tersebut dipertahankan. Frontend baru ditempatkan di `web/dashboard` agar pekerjaan pengguna tidak tertimpa dan backend Go dapat bertumbuh di struktur repository yang disepakati.

## Keputusan

Gunakan React, TypeScript, dan Vinext dari starter Sites, dengan komponen interaksi yang sudah disediakan dan tema Emisell tersendiri. Stack ini hanya membangun frontend; tidak menjadi engine backend atau runtime plugin pihak ketiga. Situs preview menggunakan data demo di memori dan dapat dipublish secara privat tanpa menyimpan data toko.

Scope increment adalah dashboard merchant: ringkasan, app catalog, installation UI, permissions UI, aktivitas, workspace selector, dan pengaturan demo. Dashboard developer/admin serta backend registry, installation, permission, ConnectRPC, PostgreSQL, NATS, dan WASM tetap merupakan pekerjaan berikutnya. Tidak ada klaim bahwa lifecycle produksi telah terimplementasi.

## Konsekuensi dan migration path

Model presentasi dipisahkan dari komponen React. Adapter demo dapat diganti dengan client backend melalui langkah: tetapkan contract API, implementasikan otorisasi backend, tambahkan adapter HTTP/ConnectRPC yang sesuai, lalu jalankan integration/contract tests sebelum mengaktifkan data produksi. Query workspace dari URL dan state browser tidak boleh menjadi sumber authorization. Mutation harus memperoleh idempotency key dari client dan enforcement dari backend saat integrasi tersebut dibuat.

Mode demo secara eksplisit menjelaskan bahwa data hilang saat reload, grants hanyalah simulasi, dan tidak ada provider atau transaksi eksternal. Jangan memperluas demo menjadi penyimpanan credential atau data merchant. Tidak diperlukan migration database dalam increment ini.

## Dependencies dan pemeriksaan

Starter membawa advisory pada sejumlah dependency. React/RSC, Vinext, Vite, plugin RSC, serta tooling Cloudflare diperbarui secara terarah ke versi kompatibel yang ditunjukkan audit; lockfile npm dipertahankan. Perubahan ini menangani dependency frontend tanpa mengubah pilihan backend Go. Hasil final build, typecheck, lint, test, audit, serta keterbatasan pemeriksaan browser dicatat pada laporan penyelesaian.

`components/ui` merupakan primitive vendored dari starter dan tidak diubah. Lint aplikasi mengecualikan folder generated tersebut karena aturan lint starter sendiri menghasilkan temuan pada wrapper generik yang tidak dipakai. Typecheck tetap mencakup primitive tersebut; komponen dashboard dan interaksi yang digunakan diperiksa melalui lint dan browser. Hook responsivitas menggunakan `useSyncExternalStore` untuk menyinkronkan viewport tanpa state update sinkron dalam effect.
