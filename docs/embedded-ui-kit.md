# Emisell UI Kit — standar embedded apps

Status: fondasi lokal **0.1.0**, bukan SDK publik lengkap. Aplikasi tetap dibuat dan di-host developer; Emisell menyediakan bahasa desain dan Bridge. Tidak ada renderer JSON-to-UI atau injeksi CSS dari Dashboard.

## Aturan produk

- Embedded app WAJIB memakai UI Kit Emisell untuk komponen yang tersedia. Komponen khusus boleh dibuat dengan token dan pola interaksi yang sama, lalu direview.
- Dashboard seller memiliki sidebar/header global. Isi iframe tidak boleh menduplikasi shell, store selector, atau form login Emisell. Navigasi aplikasi harus melalui kontrak Bridge yang didukung; jangan mengakses DOM parent.
- Aplikasi external boleh memakai desain sendiri. Extension tanpa UI seperti konfigurasi Emisell Kurir tidak masuk sidebar Apps; konfigurasi tetap di Settings existing. Mode tampilan tidak menambah scope.
- Konsistensi desain adalah syarat review publikasi, BUKAN kemampuan iframe memblokir CSS lain atau pengganti current-access check.

## Memakai fondasi lokal

Sumber stylesheet: `pkg/appui/emisell-ui.css`. Developer menyalin/bundle versi yang dipin ke origin aplikasinya; tidak memuat aset dari localhost Emisell pada produksi. Go consumer lokal dapat memakai `appui.CSS`. Paket lokal `@emisell/app-ui` dapat dibuat dengan `npm pack` dari `pkg/appui`, dipasang dari tarball, lalu diimpor sebagai `@emisell/app-ui/style.css`. Paket bersifat private; publikasi npm/CDN belum dilakukan. Tidak perlu React atau framework baru.

```html
<link rel="stylesheet" href="/assets/emisell-ui.css">
<body class="eui">
  <main class="eui-page eui-stack">
    <div class="eui-header">
      <h1 class="eui-title">Nama aplikasi</h1>
      <button class="eui-button" type="button">Simpan</button>
    </div>
    <section class="eui-card" aria-labelledby="section-title">
      <h2 class="eui-heading" id="section-title">Pengaturan</h2>
    </section>
  </main>
</body>
```

Tombol contoh harus dihubungkan developer ke use case nyata, bukan dianggap otomatis menyimpan. UI Kit tidak mempunyai akses API atau identitas.

## Komponen dan token awal

| Komponen | Contract CSS | Penggunaan |
|---|---|---|
| Page / Stack | `eui-page`, `eui-stack` | Lebar konten adaptif dan spacing 4px |
| Header | `eui-header`, `eui-title` | Judul dan satu aksi utama; wrap di mobile |
| Card | `eui-card`, `eui-heading` | Kelompok konten dengan judul semantik |
| Button | `eui-button` pada `button` | Keyboard, focus visible, native disabled |
| Badge | `eui-badge`, `connected`, `blocked` | Label tekstual, bukan warna saja |
| Banner | `eui-banner` | Pesan status; beri `role=status` untuk pembaruan |
| Description list | `eui-description` pada `dl` | Label/nilai; aman untuk identifier panjang |

Token `--eui-*` mencakup background netral, surface putih, teks, border, aksi gelap, focus, radius dan unit spacing. Arah visual mengikuti Dashboard seller existing, bukan aksen ungu Portal Developer. Belum satu sumber token lintas repository; sinkronisasi otomatis Dashboard adalah pekerjaan terpisah. Jangan override token sembarang per aplikasi.

## Checklist review wajib

- [ ] Catat versi kit, release aplikasi, reviewer, tanggal, dan bukti screenshot desktop/mobile.
- [ ] Komponen tersedia memakai kit; komponen khusus dicatat dan mengikuti token.
- [ ] Tidak ada duplikasi sidebar/header global atau login Emisell di iframe.
- [ ] Keyboard, focus, label, kontras, zoom 200%, teks panjang dan ukuran 360px/desktop diperiksa.
- [ ] Loading, empty, denied, error dan success nyata; tidak ada statistik/tombol palsu.
- [ ] Navigasi, origin, CSP, sesi expired/revoked dan isolation diuji terpisah dari desain.

Checklist ini **manual**. Endpoint review saat ini belum menyimpan evidence UI terstruktur dan belum memblokir publikasi otomatis berdasarkan versi kit. Jangan mengklaim enforcement tersebut sudah aktif. Tidak mengubah signature atau menandai release lama sebagai compliant otomatis.

## Rollout

1. Fondasi kit dan Embedded Demo sebagai consumer pertama (increment ini).
2. QA visual desktop/mobile bersama seller, lalu tambah komponen form/table/dialog sesuai kebutuhan nyata.
3. Hubungkan bukti review UI versioned ke pipeline publikasi melalui ADR/schema additive; keputusan lama tetap immutable.
4. Sinkronkan token Dashboard, distribusikan SDK versioned, baru migrasikan app developer bertahap.

Rollback tampilan dengan versi aset sebelumnya; jangan rollback grant/auth. Minor pre-1.0 yang breaking perlu panduan migrasi. Perubahan ini tidak memasang aplikasi, membuka scope, mengganti jalur API-Kurir, atau membangun aplikasi RajaOngkir.

## Hasil verifikasi lokal — 6 September 2026

- Browser Chromium: desktop 1280px, mobile 360px dan pembesaran teks iframe 200% pada viewport 720px; tidak ditemukan overflow horizontal. Screenshot diperiksa, scroll vertikal iframe tetap diperlukan pada mobile/teks besar.
- Keyboard focus terlihat; Enter memperbarui sesi. Tombol mobile minimal 44px. Revocation sintetis menghapus identitas dan menampilkan banner critical, bukan informasi biru.
- `pkg/appui/browser-qa.cjs` menyediakan regression test yang dapat dijalankan ulang. Memerlukan Playwright terpasang (`PLAYWRIGHT_MODULE`), opsional `PLAYWRIGHT_EXECUTABLE`, folder `UI_QA_OUTPUT`, serta proses demo sintetis baru di 4320/4321. Test mencabut akses **demo sintetis**, bukan installation seller; restart demo setelah test agar dapat dicoba lagi.
- Go race tests, vet, build dan tiga Bridge tests lulus. `npm pack` hanya mengemas CSS, README dan package.json, tanpa credential atau kode server.
- Belum merupakan audit aksesibilitas menyeluruh, uji semua browser, SDK lengkap atau gate publikasi otomatis. Tidak menambah form/table/dialog yang belum memiliki use case nyata pada demo identitas.
