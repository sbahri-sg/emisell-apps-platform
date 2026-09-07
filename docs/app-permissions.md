# Fondasi permission aplikasi

`internal/apppermission` adalah evaluator bersama, tidak khusus RajaOngkir atau shipping. Tidak menyediakan login, menerbitkan token, atau membuka endpoint publik baru.

## Pembagian tanggung jawab

- Apps Platform menyimpan instalasi, consent dan grant merchant.
- Backend pemanggil harus diautentikasi dan dibatasi aplikasi yang boleh diwakilinya.
- Backend menentukan scope operasi dari kebijakan server, bukan scope yang diminta browser.
- Sumber grant harus dibaca ulang dari penyimpanan tepercaya. Identitas merchant, aplikasi, dan instalasi harus cocok seluruhnya.
- Semua scope yang dibutuhkan harus ada pada consent dan grant aktif. Instalasi nonaktif atau revoked selalu ditolak. Kebutuhan kosong dan wildcard tidak memberi akses.

Evaluator tidak membuat scope baru menjadi tersedia. Nama scope untuk operasi nyata tetap harus berasal dari katalog dan kontrak operasi yang didukung. Contoh domain lain pada unit test hanya membuktikan evaluator tidak terikat shipping.

## Pemakaian saat ini

Modul `providergrant` menggunakan evaluator ini untuk operasi bisnis shipping. Adapter PostgreSQL lebih dahulu mengambil irisan scope consent release dan grant saat ini. `binding.read` tetap operasi metadata internal terautentikasi agar pencabutan akses dapat disinkronkan; respons tersebut bukan izin menjalankan operasi bisnis.

Emisell Kurir adalah built-in backend Emisell dan tidak menggunakan fondasi ini. API-Kurir saat ini dikembalikan ke baseline; client/worker percobaan hanya tersimpan di branch arsip lokal. Tidak ada integrasi runtime API-Kurir yang diaktifkan oleh perubahan ini.

## Tahap berikutnya

Hubungkan adapter grant ke jalur aplikasi eksternal yang disetujui, uji instalasi/revokasi lintas merchant, lalu uji gateway di staging. Jangan mengaktifkan source instalasi provider sebelum enforcement gateway siap. Endpoint universal publik, SDK dan aktivasi production belum termasuk tahap ini.
