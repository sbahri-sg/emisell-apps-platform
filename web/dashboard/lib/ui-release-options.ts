import type { PortalAPI } from './portal.ts';
export type UIReleaseOption = {
  id: string;
  status: string;
  manifest: { name: string; version: string; mode: string };
};
export async function uiReleaseOptions(
  api: PortalAPI,
): Promise<UIReleaseOption[]> {
  const result: UIReleaseOption[] = [];
  let afterId = '';
  for (let i = 0; i < 10; i++) {
    const page = await api.request<{
      releases: UIReleaseOption[];
      nextAfterId: string;
    }>('/ui-releases', 'GET', undefined, false, { afterId });
    result.push(...page.releases);
    if (!page.nextAfterId) break;
    afterId = page.nextAfterId;
  }
  return result;
}
