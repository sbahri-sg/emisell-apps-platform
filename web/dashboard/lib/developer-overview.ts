import type { AppLifecycle } from './app-lifecycle.ts';
import type { Draft, Submission, PortalAPI } from './portal.ts';

// Credential readiness is not a release approval or a merchant installation.
export async function loadApplicationIdentity(
  api: Pick<PortalAPI, 'request'>,
  appId: string,
) {
  const result = await api.request<{
    credential: { appId: string; clientId: string };
  }>(`/apps/${encodeURIComponent(appId)}/credentials`);
  if (
    result?.credential?.appId !== appId ||
    typeof result.credential.clientId !== 'string' ||
    !result.credential.clientId.trim()
  )
    throw new Error('Identitas aplikasi belum dapat diverifikasi.');
}

// A testing assignment is not an installation. Only summarize what existing APIs prove.
export function developerOverview(
  draft: Draft,
  data: AppLifecycle | null,
  submissions: Submission[],
) {
  const reviews = submissions
    .filter((row) => row.appId === draft.id)
    .sort((a, b) => b.createdAt.localeCompare(a.createdAt));
  const stores = data
    ? new Set(
        data.assignments
          .filter(
            (row) => row.app.appId === draft.id && row.status === 'approved',
          )
          .map((row) => row.merchantId),
      ).size
    : null;
  return { reviews, stores, latest: reviews[0] ?? null };
}
