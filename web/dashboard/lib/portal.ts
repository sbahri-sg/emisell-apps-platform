import type { ScopeDeclaration } from './access-scopes.ts';
export type Surface = 'admin' | 'developer';
export type User = {
  id: string;
  email: string;
  surface: Surface;
  role: string;
};
export type Session = {
  user: User;
  organization?: { id: string; name: string; role: string };
};
export type AppDocument = {
  name: string;
  summary: string;
  description: string;
  version: string;
  capability: string;
  scopes: string[];
  endpoint: string;
  accessScopes?: ScopeDeclaration;
};
export type Draft = {
  id: string;
  organizationId: string;
  revision: number;
  document: AppDocument;
  updatedAt: string;
};
export type Submission = {
  id: string;
  appId: string;
  organizationId: string;
  submitterId: string;
  draftRevision: number;
  version: string;
  snapshot: AppDocument;
  status: string;
  createdAt: string;
  decidedAt: string | null;
  reviewerId: string;
  feedback: string;
};
export type Audit = {
  id: string;
  actorId: string;
  action: string;
  feedback: string;
  occurredAt: string;
};
export const statuses: Record<string, string> = {
  submitted: 'Diajukan',
  changes_requested: 'Perlu perbaikan',
  approved: 'Disetujui',
  rejected: 'Ditolak',
};
export const scopesFor = (capability: string) =>
  capability === 'shipping/v1'
    ? ['orders.read', 'shipping.read', 'shipping.write']
    : capability === 'payment/v1'
      ? ['orders.read', 'payments.read', 'payments.write'] // Historical display only.
      : [];
export const blankDocument = (): AppDocument => ({
  name: '',
  summary: '',
  description: '',
  version: '1.0.0',
  capability: 'shipping/v1',
  scopes: scopesFor('shipping/v1'),
  endpoint: '',
});
export class PortalError extends Error {
  status: number;
  constructor(status: number, code: string) {
    const messages: Record<string, string> = {
      unauthenticated: 'Email atau kata sandi salah, atau sesi telah berakhir.',
      forbidden: 'Akun ini tidak memiliki akses untuk tindakan tersebut.',
      forbidden_origin: 'Buka portal melalui alamat localhost yang sesuai.',
      invalid_argument: 'Periksa kelengkapan dan format data.',
      invalid: 'Periksa kelengkapan dan format data.',
      invalid_request: 'Periksa kelengkapan dan format data.',
      not_found: 'Data tidak ditemukan atau tidak dapat diakses.',
      conflict:
        'Data telah berubah atau versi ini sudah diajukan. Muat ulang sebelum melanjutkan.',
      too_many_attempts: 'Terlalu banyak percobaan. Tunggu satu menit.',
    };
    super(
      messages[code] ??
        'Permintaan belum berhasil. Coba lagi; jika berulang, periksa layanan platform.',
    );
    this.status = status;
  }
}
// Per-mount, per-surface memory only; no tokens or tenant context in browser storage.
export class PortalAPI {
  readonly surface: Surface;
  private keys = new Map<string, string>();
  constructor(surface: Surface) {
    this.surface = surface;
  }
  async request<T>(
    path: string,
    method = 'GET',
    body?: unknown,
    idempotent = false,
    query?: { afterId: string },
  ): Promise<T> {
    if (!path.startsWith('/') || path.includes('..') || path.includes('?'))
      throw new Error('Invalid portal path');
    if (
      query &&
      (!(
        ['/test-assignments', '/ui-releases'].includes(path) ||
        (this.surface === 'admin' && ['/staff', '/developers'].includes(path))
      ) ||
        method !== 'GET' ||
        (query.afterId !== '' && !/^[A-Za-z0-9_-]{1,100}$/.test(query.afterId)))
    )
      throw new Error('Invalid page cursor');
    const fingerprint = JSON.stringify([method, path, body]);
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    };
    if (idempotent) {
      if (!this.keys.has(fingerprint))
        this.keys.set(fingerprint, crypto.randomUUID());
      headers['Idempotency-Key'] = this.keys.get(fingerprint)!;
    }
    const options: RequestInit = {
      method,
      headers,
      credentials: 'same-origin',
      cache: 'no-store',
      signal: AbortSignal.timeout(12000),
    };
    if (method !== 'GET' && body !== undefined)
      options.body = JSON.stringify(body);
    let response: Response;
    try {
      response = await fetch(
        `/api/v1/${this.surface}${path}${query?.afterId ? `?afterId=${encodeURIComponent(query.afterId)}` : ''}`,
        options,
      );
    } catch {
      throw new Error(
        'Koneksi ke platform terputus. Coba lagi untuk melanjutkan permintaan yang sama.',
      );
    }
    let data: unknown;
    try {
      data = await response.json();
    } catch {
      throw new Error(
        'Layanan portal belum siap. Periksa layanan platform, lalu coba lagi.',
      );
    }
    if (!response.ok) {
      if (response.status >= 400 && response.status < 500)
        this.keys.delete(fingerprint);
      throw new PortalError(
        response.status,
        typeof data === 'object' &&
          data !== null &&
          'error' in data &&
          typeof data.error === 'string'
          ? data.error
          : 'unknown',
      );
    }
    this.keys.delete(fingerprint);
    return data as T;
  }
}
