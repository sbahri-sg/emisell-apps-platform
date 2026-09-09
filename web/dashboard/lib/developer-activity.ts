import type { Draft, Submission } from './portal.ts';

export type DeveloperEvent = {
  id: string;
  type: 'draft' | 'submission' | 'decision';
  title: string;
  at: string;
  status: string;
  submissionId?: string;
  feedback?: string;
};

// A bounded activity view of records already returned to this organization.
// These are lifecycle records, not HTTP requests, webhook deliveries or runtime logs.
export function developerActivity(
  draft: Draft,
  submissions: Submission[],
): DeveloperEvent[] {
  const events: DeveloperEvent[] = [
    {
      id: `draft:${draft.id}:${draft.revision}`,
      type: 'draft',
      title: `Draft r${draft.revision} tersimpan`,
      at: draft.updatedAt,
      status: 'draft',
    },
  ];
  for (const row of submissions.filter((item) => item.appId === draft.id)) {
    events.push({
      id: `submission:${row.id}`,
      type: 'submission',
      title: `Versi ${row.version} diajukan`,
      at: row.createdAt,
      status: 'submitted',
      submissionId: row.id,
    });
    if (row.decidedAt)
      events.push({
        id: `decision:${row.id}`,
        type: 'decision',
        title: `Keputusan review versi ${row.version}`,
        at: row.decidedAt,
        status: row.status,
        submissionId: row.id,
        feedback: row.feedback,
      });
  }
  return events
    .filter((event) => Number.isFinite(Date.parse(event.at)))
    .sort((a, b) => b.at.localeCompare(a.at));
}
