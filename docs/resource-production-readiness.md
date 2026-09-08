# Resource access: batas rilis setelah uji produk lokal

Uji lokal telah melewati consent → install → buka → baca/cari produk → pagination. CLI `@emisell/cli@0.3.1` telah dipublikasikan ke npm, termasuk wizard dan pemeriksaan project lokal; template produk tetap membutuhkan runtime lokal yang dikonfigurasi operator. Publikasi CLI tidak menyambungkan atau men-deploy backend produksi. Integrasi bekerja dengan grant `read_products` saja; jangan mengklaim akses order/write tersedia.

API-service kini memiliki modul `embedded-sessions`, PostgreSQL encrypted session store, dan router HTTPS yang dapat dikomposisikan untuk tahap berikutnya. Identitas tidak otomatis menjadi resource grant: private Platform boundary tetap mengautentikasi confidential client dan memeriksa release, assignment, instalasi, consent serta scope pada setiap read.

**Belum production-ready sebagai alur lengkap.** Router baru belum di-mount, konfigurasi key/sertifikat production belum disediakan, dan source reviewed-resource pada `cmd/server` masih khusus lokal. Batas ini sengaja dipertahankan; tidak ada perubahan yang membuat local simulator atau credential development dapat diaktifkan dengan mengganti domain saja.

Urutan berikutnya: komposisi production resource runtime dengan private Core transport → konfigurasi origin dan identitas persisten → adaptor backend aplikasi HTTPS → staging end-to-end → rollout. Pilihan jaringan, domain, secret manager dan database target harus dikonfirmasi sebelum deployment resource runtime. Portal/control plane sudah tersedia di `https://apps-platform.emisell.com`; pembaruan portal tidak mengaktifkan alur akses produk produksi. Konfigurasi demo `.local` tidak boleh disalin ke server.

Lihat `src/modules/app-platform/EMBEDDED_AUTH.md` di repository API-service untuk kontrak dan gate konfigurasi. Pengujian adapter HTTPS/persistence tidak menggantikan pengujian staging dengan seluruh dependency asli. Publish CLI 0.3.1 telah selesai; push/merge remote dan deploy tetap langkah terpisah.
