# @emisell/app-ui 0.1.0 — local preview

Paket presentasi tanpa dependency, JavaScript runtime, token, atau akses jaringan.
Belum diterbitkan ke npm; `private: true` mencegah publish tidak sengaja.

Build arsip dengan `npm pack` dari direktori ini. Consumer memasang arsip lokal
hasil pack menggunakan package manager proyeknya, lalu pada entry frontend:

```js
import '@emisell/app-ui/style.css';
```

Gunakan `class="eui"` (React: `className="eui"`) pada body/container aplikasi.
Compose native HTML memakai `eui-page`, `eui-stack`, `eui-header`, `eui-card`,
`eui-button`, `eui-badge`, `eui-banner`, dan `eui-description`.
Banner menerima `data-tone="critical"` atau `"success"`; teks status tetap wajib.
Tombol memakai native `disabled` dan `aria-busy` saat asynchronous.

Untuk HTML tanpa bundler, self-host `emisell-ui.css` dan muat memakai link stylesheet.
Jangan mengimpor stylesheet dari server localhost Emisell pada aplikasi production.
Go consumer memakai `appui.CSS` dari aset sumber yang sama.

Paket ini tidak menggantikan App Bridge, izin, atau review publikasi. Tidak menyediakan
form/table/dialog lengkap. Detail standar dan rollout: `docs/embedded-ui-kit.md`
di repository Emisell App Platform. Pertahankan versi yang dipin; 0.x belum menjanjikan
API stabil lintas minor. Perubahan ini additive terhadap 0.1.0 lokal yang belum dipublish.
