export type ClientView = {
  client: {
    id: string;
    binding: {
      releaseId: string;
      organizationId: string;
      appId: string;
      version: string;
      name: string;
      digest: string;
      endpoint: string;
      redirectUri: string;
    };
    status: 'pending' | 'verified' | 'revoked';
    revision: number;
    challengeId: string;
    challenge: string;
    challengeExpiresAt: string;
    verifiedUntil: string | null;
    lastAttemptAt: string | null;
    lastResult: string;
    secretVersion: number;
    createdAt: string;
    updatedAt: string;
  };
  proofUrl: string;
  expected: {
    schema: string;
    clientId: string;
    releaseSha256: string;
    challenge: string;
  };
  clientReady: boolean;
  oauthEnabled: false;
  installable: false;
  blockers: string[];
};
export type ClientDetail = {
  view: ClientView;
  history: {
    id: string;
    actorId: string;
    action: string;
    reason: string;
    occurredAt: string;
  }[];
};
export type ClientAction = 'challenge' | 'verify' | 'rotate_secret' | 'revoke';
export function clientActions(
  surface: string,
  role: string,
  v: ClientView,
  now = Date.now(),
): ClientAction[] {
  const c = v.client;
  if (c.status === 'revoked') return [];
  if (surface === 'admin') return role === 'administrator' ? ['revoke'] : [];
  if (surface !== 'developer' || role !== 'developer') return [];
  const out: ClientAction[] = ['challenge'];
  if (
    c.status === 'pending' &&
    Date.parse(c.challengeExpiresAt) > now &&
    (!c.lastAttemptAt || now - Date.parse(c.lastAttemptAt) >= 60000)
  )
    out.push('verify');
  if (
    c.status === 'verified' &&
    c.verifiedUntil &&
    Date.parse(c.verifiedUntil) > now
  )
    out.push('rotate_secret');
  out.push('revoke');
  return out;
}
export const clientActionLabels: Record<ClientAction, string> = {
  challenge: 'Buat challenge baru',
  verify: 'Verifikasi endpoint',
  rotate_secret: 'Terbitkan / rotasi secret',
  revoke: 'Cabut client',
};
export const proofResult: Record<string, string> = {
  not_checked: 'Belum diperiksa',
  checking: 'Pemeriksaan dimulai',
  verified: 'Bukti cocok',
  failed: 'Pemeriksaan gagal',
  interrupted: 'Pemeriksaan terputus — coba lagi',
};
