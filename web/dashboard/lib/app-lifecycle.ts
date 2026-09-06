import type { PortalAPI, Draft, Submission } from './portal.ts';
import type { ManagedShippingRelease } from './managed-shipping.ts';
import type { IntegrationRelease } from './integration-releases.ts';
import type { Assignment, AssignmentPage } from './testing.ts';

type Catalog = {
  status: string;
  package: { manifest: { appId: string; version: string } };
};
export type AppLifecycle = {
  managed: ManagedShippingRelease[];
  integrations: IntegrationRelease[];
  catalog: Catalog[];
  submissions: Submission[];
  assignments: Assignment[];
};
type ReadAPI = Pick<PortalAPI, 'request'>;

// These endpoints are owner-scoped by the Developer API. No merchant identity,
// mutation, grant or runtime probe is introduced by this display read model.
export async function loadAppLifecycle(api: ReadAPI): Promise<AppLifecycle> {
  async function list<T>(path: string, field: string): Promise<T[]> {
    const result = await api.request<Record<string, T[]>>(path);
    const rows = result?.[field];
    // Existing non-paginated lists cap at 200; absence after that cap cannot
    // prove an app is a draft. Never silently summarize incomplete history.
    if (!Array.isArray(rows) || rows.length >= 200 || rows.some((row) => !row))
      throw new Error('Status aplikasi belum dapat diverifikasi lengkap.');
    return rows;
  }
  async function assignments() {
    const rows: Assignment[] = [];
    const cursors = new Set<string>();
    let afterId = '';
    for (let pageNumber = 0; pageNumber < 25; pageNumber++) {
      const page = await api.request<AssignmentPage>(
        '/test-assignments',
        'GET',
        undefined,
        false,
        { afterId },
      );
      if (
        !Array.isArray(page?.assignments) ||
        typeof page.nextAfterId !== 'string' ||
        page.assignments.some((row) => !row)
      )
        throw new Error('Status testing belum dapat diverifikasi.');
      rows.push(...page.assignments);
      if (!page.nextAfterId) return rows;
      if (cursors.has(page.nextAfterId))
        throw new Error('Halaman testing tidak valid.');
      cursors.add(page.nextAfterId);
      afterId = page.nextAfterId;
    }
    throw new Error('Status testing belum dapat diverifikasi lengkap.');
  }
  const [managed, integrations, catalog, submissions, testing] =
    await Promise.all([
      list<ManagedShippingRelease>('/managed-shipping-releases', 'releases'),
      list<IntegrationRelease>('/integration-releases', 'releases'),
      list<Catalog>('/catalog', 'releases'),
      list<Submission>('/submissions', 'submissions'),
      assignments(),
    ]);
  return { managed, integrations, catalog, submissions, assignments: testing };
}

export function appLifecycleStatus(draft: Draft, data: AppLifecycle) {
  const version = draft.document.version;
  const matches = (appId: string, releaseVersion: string) =>
    appId === draft.id && releaseVersion === version;
  const status = (label: string, tone = 'draft') => ({ label, tone, version });
  const catalog = data.catalog.find((r) =>
    matches(r.package.manifest.appId, r.package.manifest.version),
  );
  if (catalog?.status === 'published')
    return status('Dipublikasikan', 'approved');

  const releases = [
    ...data.managed
      .filter((r) => matches(r.manifest.appId, r.manifest.version))
      .map((r) => ({ ...r, kind: 'managed_shipping' })),
    ...data.integrations
      .filter((r) =>
        matches(r.manifest.metadata.appId, r.manifest.metadata.version),
      )
      .map((r) => ({ ...r, kind: 'integration' })),
  ];
  const testing = data.assignments.filter((a) =>
    matches(a.app.appId, a.app.version),
  );
  const approved = testing.filter((a) => a.status === 'approved');
  const sourceFor = (a: Assignment) =>
    releases.find(
      (r) =>
        r.id === a.releaseId && r.kind === (a.releaseKind || 'integration'),
    );
  const ready = approved.some(
    (a) =>
      sourceFor(a)?.status === 'signed' &&
      a.app.readiness.configurationReady &&
      a.app.readiness.requiredScopesReady &&
      a.app.readiness.installable &&
      a.app.readiness.blockers.length === 0,
  );
  if (ready) return status('Testing', 'approved');
  if (approved.some((a) => sourceFor(a)?.status === 'suspended'))
    return status('Rilis ditangguhkan', 'rejected');
  if (approved.length) return status('Testing belum siap', 'changes_requested');
  if (testing.some((a) => a.status === 'requested'))
    return status('Menunggu izin testing', 'submitted');
  if (testing.some((a) => a.status === 'revoked'))
    return status('Testing dicabut', 'rejected');
  if (testing.some((a) => a.status === 'rejected'))
    return status('Testing ditolak', 'rejected');
  if (releases.some((r) => r.status === 'signed'))
    return status('Rilis ditandatangani', 'approved');
  if (
    releases.some((r) => r.status === 'suspended') ||
    catalog?.status === 'suspended'
  )
    return status('Rilis ditangguhkan', 'rejected');
  if (releases.some((r) => r.status === 'approved'))
    return status('Rilis disetujui', 'approved');
  if (releases.some((r) => r.status === 'submitted'))
    return status('Review rilis', 'submitted');
  if (releases.some((r) => r.status === 'rejected'))
    return status('Rilis ditolak', 'rejected');
  if (catalog?.status === 'signed')
    return status('Katalog ditandatangani', 'approved');
  const review = data.submissions.find(
    (s) => matches(s.appId, s.version) && s.draftRevision === draft.revision,
  );
  const reviews: Record<string, [string, string]> = {
    approved: ['Metadata disetujui', 'approved'],
    submitted: ['Dalam review', 'submitted'],
    changes_requested: ['Perlu perbaikan', 'changes_requested'],
    rejected: ['Review ditolak', 'rejected'],
  };
  if (review)
    return reviews[review.status]
      ? status(...reviews[review.status])
      : status('Belum terverifikasi');
  if (catalog || releases.length || testing.length)
    return status('Belum terverifikasi');
  return status('Draft');
}
