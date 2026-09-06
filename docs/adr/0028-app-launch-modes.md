# ADR 0028 — Embedded dan Dashboard eksternal

Keputusan pengguna: kedua mode pembukaan harus didukung, dengan lifecycle instalasi dan grant yang tetap sama. Mode tampilan bukan jenis runtime atau permission baru.

## Implementasi incremental

Metadata launch mempunyai `mode: embedded | external`. Field kosong pada data historis tetap berarti embedded dan dihilangkan saat canonical JSON serialization sehingga signature lama tetap valid. Field explicit ikut signature; mengganti mode atau URL memerlukan rilis/client baru dan review, bukan update metadata immutable.

Pengajuan developer dan approval administrator menggunakan gate current signed release, ownership, endpoint proof dan secret existing. Kedua mode hanya menerima HTTPS pada origin endpoint terverifikasi, tanpa credential/query/fragment. External tidak berarti mengizinkan arbitrary redirect URL atau otomatis memberi akses data.

`Launcher.Open` hanya menerima embedded dan menerbitkan token identitas melalui current authorization. `Launcher.External` hanya menerima external, memeriksa signature/current authorization dan mengembalikan metadata tanpa token. Metode ini bukan OAuth atau SSO. Bridge browser menolak mode external.

Portal menempatkan pengajuan dan keputusan pada detail App clients, menggunakan endpoint review opt-in existing dan lookup baru `/api/v1/{developer|admin}/app-clients/{id}/launch`. Null berarti belum diajukan; error tidak dianggap konfigurasi kosong. Persetujuan tidak ditampilkan sebagai instalasi launchable.

## Compatibility dan rollout

Tidak ada migration atau perubahan signed release/instalasi lama. Metadata mode disimpan dalam JSONB existing. Rollout reader/server yang memahami mode sebelum writer/frontend mengirim mode baru. Setelah external disimpan, jangan rollback ke binary lama yang tidak mengenal mode: JSON decoder lama dapat mengabaikan mode lalu canonical verification akan gagal; tidak boleh fallback ke iframe. Revoke tetap tindakan terminal.

## Gate yang masih terbuka

Listener utama belum memasang komposisi review dengan dedicated signing key/parent origin. Binding installation managed release ke generic integration app-client masih perlu policy dan implementasi persisten; tidak bisa ditebak dari nama. Real Core session, URL handoff, identity issuance dan Dashboard Open app belum tersambung. Jangan menggunakan callback demo sintetis sebagai auth seller. Implementasi mode dan review ini belum menyelesaikan keseluruhan alur seller yang diminta.

Tidak ada perubahan API-Kurir, tarif, merchant grant, database development atau credential pengguna dalam increment ini.
