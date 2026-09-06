import type { ScopeDeclaration } from '@/lib/access-scopes';

export function ScopeSummary({ value }: { value?: ScopeDeclaration }) {
  if (!value) return null;
  return (
    <section
      className="access-summary"
      aria-label="Rencana akses data aplikasi"
    >
      <h3>Rencana akses data</h3>
      <p>
        Belum aktif. Deklarasi ini bukan grant, token, atau izin mengakses data
        toko.
      </p>
      <small>Profil: {value.profile}</small>
      {(['required', 'optional'] as const).map((kind) => (
        <div key={kind}>
          <strong>
            {kind === 'required' ? 'Wajib (required)' : 'Opsional (optional)'}
          </strong>
          {value[kind].length ? (
            <ul className="scope-list">
              {value[kind].map((h) => (
                <li key={h}>
                  <code>{h}</code>
                </li>
              ))}
            </ul>
          ) : (
            <p>Tidak ada.</p>
          )}
        </div>
      ))}
      <p>
        Review metadata tidak menggantikan review data sensitif, dukungan
        gateway, atau persetujuan merchant di Emisell Core.
      </p>
    </section>
  );
}
