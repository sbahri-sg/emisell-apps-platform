import type { PortalAPI } from './portal.ts';
import type { ScopeCatalog, ScopeVerification } from './access-scopes.ts';

export type ScopeReadinessSnapshot = {
  catalog: ScopeCatalog | null;
  verification: ScopeVerification | null;
  error: string;
  verificationError: string;
};
export const emptyReadiness: ScopeReadinessSnapshot = {
  catalog: null,
  verification: null,
  error: '',
  verificationError: '',
};

// Both portal views use this read-only path. No token storage, shared cross-user
// cache, resource invocation, or fallback to a generated readiness snapshot.
export async function loadScopeReadiness(
  api: Pick<PortalAPI, 'request'>,
): Promise<ScopeReadinessSnapshot> {
  const [catalog, report] = await Promise.allSettled([
    api.request<ScopeCatalog>('/access-scopes'),
    api.request<ScopeVerification>('/access-scopes/verification'),
  ]);
  const c = catalog.status === 'fulfilled' ? catalog.value : null;
  const r = report.status === 'fulfilled' ? report.value : null;
  const stringList = (v: unknown): v is string[] =>
    Array.isArray(v) && v.every((s) => typeof s === 'string');
  const validCatalog =
    c &&
    typeof c.profile === 'string' &&
    Array.isArray(c.scopes) &&
    c.scopes.every(
      (s) =>
        s &&
        typeof s.handle === 'string' &&
        typeof s.notes === 'string' &&
        typeof s.resource === 'string' &&
        stringList(s.implies) &&
        stringList(s.requiresAny),
    );
  const validReport =
    validCatalog &&
    r &&
    r.profile === c.profile &&
    r.verification === 'platform_build_inventory' &&
    typeof r.coreChecked === 'boolean' &&
    Number.isFinite(Date.parse(r.checkedAt)) &&
    Array.isArray(r.scopes) &&
    r.scopes.every(
      (s) =>
        s &&
        typeof s.handle === 'string' &&
        typeof s.grantable === 'boolean' &&
        stringList(s.operations) &&
        stringList(s.blockers),
    ) &&
    r.environment === 'local' &&
    /^[a-f0-9]{64}$/.test(r.contractRevision ?? '') &&
    Array.isArray(r.operations) &&
    r.operations.every(
      (o) =>
        o &&
        typeof o.procedure === 'string' &&
        typeof o.version === 'string' &&
        typeof o.requiredScope === 'string' &&
        stringList(o.acceptedScopes) &&
        o.acceptedScopes.includes(o.requiredScope) &&
        stringList(o.blockers),
    );
  return {
    catalog: validCatalog ? c : null,
    verification: validReport ? r : null,
    error: validCatalog
      ? ''
      : 'Katalog scope belum dapat dimuat. Muat ulang untuk mencoba kembali.',
    verificationError: validReport
      ? ''
      : 'Status belum terverifikasi: layanan tidak tersedia atau kontrak tidak cocok. Tidak ada scope/endpoint yang dianggap aktif.',
  };
}

export function scopeCatalogURL(scope = ''): string {
  const query = new URLSearchParams({ view: 'scopes' });
  if (scope) query.set('scope', scope);
  return '/?' + query.toString();
}
export function gatewayOperationURL(procedure: string): string {
  return (
    '/?' +
    new URLSearchParams({
      view: 'api-docs',
      api_group: 'gateway',
      api_operation: procedure,
    }).toString()
  );
}
