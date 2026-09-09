# Mengelola dokumentasi publik

Artikel ditulis sebagai Markdown, bukan JSX di komponen. Tidak ada CMS atau database baru.

1. Edit artikel `.md` yang sudah ada, atau tambahkan file baru dengan ID lowercase-dash.
2. Untuk artikel baru, tambahkan `id`, `title`, `summary`, dan `category` di `navigation.json`. Urutan array menentukan sidebar dan tautan sebelumnya/berikutnya.
3. Gunakan heading `##` / `###`. Judul utama diambil dari metadata. Didukung: paragraf, daftar, tabel, kutipan, link, bold/italic, inline code, dan fenced code.
4. Tautan internal memakai `/docs/article-id#heading-id`; halaman aplikasi boleh ditautkan melalui `/development`. HTML/JSX, gambar, serta skema URL berbahaya ditolak. Contoh HTML di dalam fenced code tetap ditampilkan sebagai teks.
5. Jalankan `npm run docs:content` di `web/dashboard`, lalu `npm test` dan build kedua frontend yang memakai komponen bersama.

Dev server utama `web/dashboard` memantau folder ini: perubahan artikel memperbarui preview setelah validasi. Jika validasi gagal, perbaiki kesalahan yang tampil; artikel sebelumnya tidak ditimpa. Build utama selalu menghasilkan konten terbaru, sementara test `docs:content:check` menolak hasil generator yang belum diperbarui. Frontend dedicated `web/developer` memakai JSON generated yang sama tanpa memerlukan parser tambahan; jalankan generator di `web/dashboard` sebelum membangun frontend dedicated saja. Commit file Markdown, metadata, dan `lib/documentation.generated.json` bersama-sama. Jangan edit file generated langsung.

Pencarian, daftar isi, urutan halaman, dan estimasi waktu baca berasal dari artikel yang sama. Renderer memakai elemen React yang diizinkan, bukan HTML mentah atau evaluasi MDX. Parser `marked` hanya dipakai pada tooling build; tidak ditambahkan sebagai runtime aplikasi.

## Pemeriksaan editorial

- Cocokkan perintah dan fitur dengan `packages/developer-cli/README.md`, `CHANGELOG.md`, template `README.md`, `TESTING.md`, dan `DEPLOYMENT.md` di repository.
- Status lokal, rilis, dan deployment produksi harus dibedakan. Jangan mengklaim scope aktif hanya karena ada dalam katalog.
- Jangan memasukkan secret, cookie, token nyata, data seller, kontrak admin/Core, atau contoh ID milik pengguna.
- Saat API berubah, review dokumentasi pada perubahan yang sama. Referensi endpoint publik saat ini dikurasi; generator ini tidak mengekspor seluruh kontrak internal sebagai API publik.
- Verifikasi konten teknis melalui review tim. Tidak diperlukan halaman admin untuk menerbitkan artikel; konten mengikuti release frontend yang biasa.

Folder ini hanya memuat artikel yang disebut dalam `navigation.json`. README pengelola ini tidak dipublikasikan atau dimasukkan ke indeks pencarian.
