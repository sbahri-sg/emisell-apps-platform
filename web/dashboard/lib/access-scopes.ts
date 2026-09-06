export type ScopeDeclaration = {
  profile: string;
  required: string[];
  optional: string[];
};
export type AccessScope = {
  handle: string;
  resource: string;
  action: 'read' | 'write';
  status: 'planned' | 'reference_only' | 'future_reference';
  grantable: false;
  implies: string[];
  requiresAny: string[];
  review: 'standard' | 'restricted';
  notes: string;
  availableFrom?: string;
};
export type ScopeCatalog = {
  profile: string;
  source: string;
  checkedAt: string;
  grantable: false;
  scopes: AccessScope[];
};
export const scopeStatus = {
  planned: 'Direncanakan',
  reference_only: 'Khusus Shopify',
  future_reference: 'Versi mendatang',
};

export type ScopeVerification = {
  profile: string;
  checkedAt: string;
  verification: 'platform_build_inventory';
  coreChecked: boolean;
  contractRevision?: string;
  environment?: string;
  operations?: GatewayOperationReadiness[];
  scopes: {
    handle: string;
    status: 'active' | 'planned';
    grantable: boolean;
    contractStatus: string;
    operations: string[];
    blockers: string[];
  }[];
};
export type GatewayOperationReadiness = {
  procedure: string;
  version: string;
  requiredScope: string;
  acceptedScopes: string[];
  status: 'active' | 'planned';
  blockers: string[];
};
export const readinessLabel = {
  active: 'Active',
  planned: 'Plan',
  unknown: 'Belum terverifikasi',
};
export function verifiedOperationState(
  report: ScopeVerification | null,
  procedure: string,
  expectedRevision?: string,
): 'active' | 'planned' | 'unknown' {
  if (
    !report ||
    report.verification !== 'platform_build_inventory' ||
    report.environment !== 'local' ||
    !/^[a-f0-9]{64}$/.test(report.contractRevision ?? '') ||
    (expectedRevision && report.contractRevision !== expectedRevision)
  )
    return 'unknown';
  const rows =
    report.operations?.filter((o) => o.procedure === procedure) ?? [];
  if (rows.length !== 1) return 'unknown';
  const o = rows[0];
  if (o.status === 'planned') return 'planned';
  return o.status === 'active' &&
    report.coreChecked === true &&
    o.blockers.length === 0
    ? 'active'
    : 'unknown';
}
export function verifiedScopeState(
  catalog: ScopeCatalog,
  report: ScopeVerification | null,
  handle: string,
): 'active' | 'planned' | 'unknown' {
  if (
    !report ||
    report.profile !== catalog.profile ||
    report.verification !== 'platform_build_inventory'
  )
    return 'unknown';
  if (!catalog.scopes.some((s) => s.handle === handle)) return 'unknown';
  const rows = report.scopes.filter((s) => s.handle === handle);
  if (rows.length !== 1) return 'unknown';
  const s = rows[0];
  if (s.status === 'planned' && s.grantable === false) return 'planned';
  if (
    s.status === 'active' &&
    s.grantable === true &&
    report.coreChecked === true &&
    s.contractStatus === 'complete' &&
    s.operations.length > 0 &&
    s.operations.every(
      (procedure) =>
        verifiedOperationState(report, procedure) === 'active' &&
        report.operations
          ?.find((o) => o.procedure === procedure)
          ?.acceptedScopes.includes(handle),
    ) &&
    s.blockers.length === 0
  )
    return 'active';
  return 'unknown';
}

export function filterScopes(
  catalog: ScopeCatalog,
  query: string,
  filter: string,
  value?: ScopeDeclaration,
  verification: ScopeVerification | null = null,
) {
  const selected = new Set([
    ...(value?.required ?? []),
    ...(value?.optional ?? []),
  ]);
  const term = query.trim().toLowerCase();
  return catalog.scopes.filter(
    (s) =>
      `${s.handle} ${s.resource} ${s.notes}`.toLowerCase().includes(term) &&
      (filter === 'all' ||
        (['active', 'planned', 'unknown'].includes(filter)
          ? verifiedScopeState(catalog, verification, s.handle) === filter
          : filter === 'selected'
            ? selected.has(s.handle)
            : filter === 'restricted'
              ? s.review === 'restricted'
              : s.status === filter)),
  );
}
export function selectScope(
  catalog: ScopeCatalog,
  value: ScopeDeclaration | undefined,
  handle: string,
  mode: string,
): ScopeDeclaration | undefined {
  if (
    !catalog.scopes.some((s) => s.handle === handle) ||
    !['none', 'required', 'optional'].includes(mode)
  )
    throw new Error('Pilihan scope tidak valid.');
  if (value && value.profile !== catalog.profile)
    throw new Error('Profil berbeda; jangan migrasi izin otomatis.');
  const next = {
    profile: catalog.profile,
    required: (value?.required ?? []).filter((s) => s !== handle),
    optional: (value?.optional ?? []).filter((s) => s !== handle),
  };
  if (mode === 'required') next.required.push(handle);
  if (mode === 'optional') next.optional.push(handle);
  next.required.sort();
  next.optional.sort();
  return next.required.length + next.optional.length ? next : undefined;
}
export function declarationError(
  catalog: ScopeCatalog,
  value?: ScopeDeclaration,
): string {
  if (!value) return '';
  if (value.profile !== catalog.profile)
    return 'Profil scope berbeda. Perlu migration path, bukan perubahan otomatis.';
  const scopes = new Map(catalog.scopes.map((s) => [s.handle, s]));
  const all = [...value.required, ...value.optional];
  if (
    !all.length ||
    new Set(all).size !== all.length ||
    all.some((h) => !scopes.has(h))
  )
    return 'Scope kosong, duplikat, atau tidak dikenal.';
  const expand = (handles: string[]) =>
    new Set(handles.flatMap((h) => [h, ...scopes.get(h)!.implies]));
  const required = expand(value.required);
  if (value.optional.some((h) => required.has(h)))
    return 'Scope optional sudah tercakup required. Hapus pilihan optional tersebut; write dapat mencakup read.';
  for (const [handles, effective] of [
    [value.required, required],
    [value.optional, expand(all)],
  ] as const) {
    for (const h of handles) {
      const deps = scopes.get(h)!.requiresAny;
      if (deps.length && !deps.some((d) => effective.has(d)))
        return `${h} memerlukan ${deps.join(' atau ')}. Dependensi required juga harus required.`;
    }
  }
  return '';
}
