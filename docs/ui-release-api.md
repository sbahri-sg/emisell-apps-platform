# API release UI — komposisi opt-in

Status: penyimpanan PostgreSQL dan API tersedia melalui `bootstrap.UIReleaseHandler`, dirangkai ke server melalui `AddUIReleaseRoutes` tanpa mengganti rute/pilot existing. Menu **Aplikasi dengan UI** tersedia pada Admin/Developer (`?view=ui-releases`). App clients dan Testing existing menerima release UI signed. Belum ada instalasi atau launch otomatis; readiness UI tetap `installable:false`.

Signing key lokal khusus diprovision eksplisit melalui `go run ./cmd/cli init-ui-release-signing`; file privat `.local/ui-release-signing.json` tidak boleh dibagikan/di-commit. Key existing tidak dirotasi. Key tidak tersedia membuat signing ditolak, tanpa menutup review/suspend.

## Authentication

Gunakan sesi portal existing, cookie dan trusted Origin sesuai surface Admin/Developer. Guard portal/CSRF existing tetap berlaku. Semua POST memakai `Idempotency-Key` valid (8–128 karakter alfanumerik, underscore atau minus). Jangan mengirim merchant, role, developerId, scope atau credential dalam payload authoring.

## Endpoint

| Metode | Path | Hak |
|---|---|---|
| GET | `/api/v1/developer/ui-releases?afterId=...` | Release organisasi sendiri, maksimal 20 |
| GET | `/api/v1/developer/ui-releases/{id}` | Detail organisasi sendiri |
| POST | `/api/v1/developer/ui-releases` | Mengajukan snapshot immutable |
| GET | `/api/v1/admin/ui-releases?afterId=...` | Daftar admin, maksimal 20 |
| GET | `/api/v1/admin/ui-releases/{id}` | Detail admin |
| POST | `/api/v1/admin/ui-releases/{id}/status` | Review/sign/suspend sesuai role |

Pagination memakai `nextAfterId`; cursor terakhir dapat menghasilkan halaman kosong. Ini bukan total-count dashboard. Detail/mutation mengembalikan `{release, installable:false}`. Daftar mengembalikan `{releases,nextAfterId,installable:false}`.

## Pengajuan

```json
{
  "version": "1.0.0",
  "name": "Example UI",
  "summary": "Aplikasi embedded tanpa akses data bisnis",
  "mode": "embedded",
  "url": "https://app.example.com/dashboard",
  "reason": "Pengajuan pertama"
}
```

App baru mendapat ID server-side, ownership berasal dari sesi developer, pricing selalu free. Versi berikutnya menyertakan `appId` existing milik organisasi sendiri. Satu app/version hanya boleh satu snapshot; perubahan sesudah submit memerlukan versi baru. Mode `external` juga tersedia; tanpa scope bisnis dan bukan SSO.

## Review

POST status menerima `{ "revision":1, "status":"approved", "reason":"Hasil review" }`.

- submitted → approved/rejected: reviewer atau administrator.
- approved → signed/suspended dan signed → suspended: administrator.
- Signing membutuhkan key privat UI eksplisit; tidak diprovision otomatis.
- Request identik memakai key sama membaca **state terkini**, bukan mengembalikan snapshot approval lama; key sama dengan payload berbeda conflict.
- Audit actor/action/reason tercatat atomik dengan perubahan dan receipt.

Signature hanya attestation release. Review URL statis belum membuktikan DNS, kendali endpoint, kualitas UI atau availability. Assignment/consent/launch tetap gate berikutnya.

## Migration dan rollout

Migration `0019_ui_releases.sql` additive, tanpa backfill dan tidak mengubah instalasi lama. Diuji pada database `emisell_local_test`, lalu diterapkan ke database lokal sesudah backup privat `/tmp/emisell-before-ui-release.PZvp8g/database.dump`. Server lokal dimuat ulang dengan konfigurasi pilot existing. Untuk environment lain, backup dan apply eksplisit sebelum menjalankan binary terbaru: startup verification menolak schema lama. API UI belum tampil di generator dokumentasi dashboard; dokumen ini referensi developer sementara.

## App clients dan Testing

Gunakan endpoint App clients existing dengan `releaseId` UI signed. Ownership, signature dan status sumber diperiksa kembali; verifikasi kendali origin tetap memakai jalur App clients existing. Suspend release menolak operasi client yang memerlukan sumber aktif.

Pada endpoint Testing existing, kirim `releaseKind: "ui"`, `releaseId`, `merchantId` dan `reason`. Approval admin membutuhkan merchant terdaftar dan release signed terkini. Migration `0020_ui_assignments.sql` menambah FK UI tersendiri tanpa mengonversi assignment lama. Revoke tetap tersedia saat release suspended.

Assignment bukan consent atau izin install. Tanpa konfigurasi runtime UI, blocker `ui_installation_not_available` tetap berlaku dan UI tidak dikirim ke Core. Komposisi opt-in di bawah membuka distribusi hanya setelah seluruh gate terkini lulus. Tidak ada menu baru untuk App clients/Testing.

## Adapter sumber instalasi terverifikasi

Update runtime lokal: `EnableReviewedUI` merangkai source ke lifecycle dan distribusi
merchant dengan profil explicit `reviewed-ui/v1`. Sumber shipping tetap dipertahankan;
UI yang ditolak tidak fallback ke shipping. Listener dapat mengaktifkan komposisi
melalui file privat `.local/reviewed-ui.json` berisi `environment: development`,
`parentOrigin` HTTPS, dan `launchSeed` acak Ed25519/base64 (32 byte) khusus signing
launch. Key release UI tetap terpisah. Tidak memprovision atau mengaktifkan file
tersebut otomatis; production ditolak. Sumber/runtime dan review portal memakai
key serta parent yang sama. Tidak ada migrasi baru atau rewrite snapshot.

Kontrak RPC `InstallationAccess.reviewed_ui_launch` kini additive. Hanya GetInstallation
yang mengisinya setelah current-source check; snapshot persisten dan replay mutation
tidak mengeluarkan URL launch otomatis. Core client memiliki parser opt-in
`reviewedUI`, default off. Core menyediakan route sesi lokal dan Dashboard consumer;
rollout memerlukan konfigurasi eksplisit di kedua backend dan URL HTTPS terverifikasi.
Testing hanya installable setelah semua gate sumber terkini lulus, bukan dari DTO saja.

`bootstrap.ReviewedUIInstallSource` tersedia untuk komposisi eksplisit. Adapter menahan lock release → assignment → client → review launch selama callback lifecycle. Signature release dan launch diperiksa dengan public key terpisah; ownership, checksum, URL/mode exact, parent origin, proof client dan secret aktif wajib cocok. Lookup client tidak memberikan authorization tanpa pemeriksaan ulang di bawah lock.

Snapshot `uiBinding.assignmentId` additive mengikat assignment asal; pencabutan lalu pembuatan assignment baru tidak menghidupkan consent/installation lama. Field kosong mempertahankan snapshot historis. Tidak memerlukan migration database karena snapshot JSON; data lama tidak ditulis ulang.

Uji PostgreSQL terisolasi mencakup embedded dan external: review → consent → consume → activate → pemeriksaan akses; penolakan sebelum consent/review, merchant lain, suspend dan assignment replacement; uninstall tetap tersedia ketika sumber ditolak. Adapter dirangkai hanya ketika konfigurasi runtime tersedia. Token sesi diterbitkan Core development secara terpisah dan token demo tidak dipakai ulang. Pengujian terisolasi bukan klaim rollout release UI umum pada toko pengguna.
