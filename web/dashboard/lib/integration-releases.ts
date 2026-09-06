import type { ScopeDeclaration } from './access-scopes.ts';

export type IntegrationStatus =
  | 'submitted'
  | 'approved'
  | 'rejected'
  | 'signed'
  | 'suspended';
export type IntegrationConfig = {
  protocol: string;
  endpoint: string;
  callbackUrl: string;
  healthUrl: string;
};
export type IntegrationManifest = {
  schema: string;
  policy: string;
  submissionId: string;
  metadata: {
    appId: string;
    name: string;
    version: string;
    capability: string;
    runtime: 'remote';
    scopes: string[];
    accessScopes?: ScopeDeclaration;
    installable: false;
  };
  config: IntegrationConfig;
};
export type IntegrationPackage = {
  manifest: IntegrationManifest;
  sha256: string;
  keyId: string;
  signature: string;
};
export type IntegrationRelease = {
  id: string;
  organizationId: string;
  manifest: IntegrationManifest;
  sha256: string;
  package?: IntegrationPackage;
  status: IntegrationStatus;
  revision: number;
  createdAt: string;
  updatedAt: string;
};
export type IntegrationReport = {
  valid: boolean;
  installable: false;
  checks: { code: string; passed: boolean; message: string }[];
  blockers: string[];
};
export type IntegrationDetail = {
  release: IntegrationRelease;
  validation: IntegrationReport;
  trustedPublicKey: string | null;
  history: {
    id: string;
    action: string;
    actorId: string;
    reason: string;
    occurredAt: string;
  }[];
};
export const integrationStatus: Record<IntegrationStatus, string> = {
  submitted: 'Menunggu review konfigurasi',
  approved: 'Konfigurasi disetujui',
  rejected: 'Ditolak',
  signed: 'Konfigurasi ditandatangani',
  suspended: 'Ditangguhkan',
};
export function integrationActions(
  surface: string,
  role: string,
  status: IntegrationStatus,
): IntegrationStatus[] {
  if (surface !== 'admin' || !['administrator', 'reviewer'].includes(role))
    return [];
  if (status === 'submitted') return ['approved', 'rejected'];
  if (role !== 'administrator') return [];
  if (status === 'approved') return ['signed', 'suspended'];
  return status === 'signed' ? ['suspended'] : [];
}
