export type AssignmentStatus =
  | 'requested'
  | 'approved'
  | 'rejected'
  | 'revoked';
export type Assignment = {
  id: string;
  organizationId: string;
  releaseId: string;
  releaseKind?: 'managed_shipping' | 'ui';
  releaseSha256: string;
  merchantId: string;
  status: AssignmentStatus;
  revision: number;
  createdAt: string;
  updatedAt: string;
  app: {
    assignmentId: string;
    appId: string;
    appName: string;
    version: string;
    capability: string;
    readiness: {
      configurationReady: boolean;
      requiredScopesReady: boolean;
      installable: boolean;
      blockers: string[];
    };
  };
};
export type AssignmentPage = { assignments: Assignment[]; nextAfterId: string };
export type AssignmentDetail = {
  assignment: Assignment;
  history: {
    id: string;
    actorId: string;
    action: string;
    reason: string;
    occurredAt: string;
  }[];
};
export const assignmentStatus: Record<AssignmentStatus, string> = {
  requested: 'Menunggu persetujuan',
  approved: 'Pengujian disetujui',
  rejected: 'Ditolak',
  revoked: 'Dicabut',
};
export function assignmentActions(
  surface: string,
  role: string,
  status: AssignmentStatus,
): AssignmentStatus[] {
  if (surface !== 'admin' || role !== 'administrator') return [];
  return status === 'requested'
    ? ['approved', 'rejected']
    : status === 'approved'
      ? ['revoked']
      : [];
}
export const testingBlockers: Record<string, string> = {
  managed_installation_not_available:
    'Alur instalasi provider terkelola belum diaktifkan.',
  engine_grant_enforcement_not_available:
    'Pemeriksaan grant di engine API-Kurir belum tersambung.',
  managed_distribution_not_available:
    'Distribusi provider terkelola belum tersedia.',
  runtime_not_available: 'Runtime aplikasi developer belum tersedia.',
  oauth_not_available:
    'Alur instalasi dan token exchange OAuth belum tersedia.',
  endpoint_runtime_unverified:
    'Kompatibilitas runtime, health dan kebijakan egress belum diverifikasi.',
  release_not_ready:
    'Release tidak lagi signed atau pemeriksaan signature/konfigurasi gagal.',
  required_scopes_not_ready:
    'Scope wajib belum grantable; tidak dapat diberikan akses.',
};
