export type Overview = {
  publishedApps: number;
  developers: number;
  activeInstallations: number;
  pendingReviews: number;
  history: { date: string; count: number }[];
  webhookPending: number;
  webhookDead: number;
  portalSessions: number;
  checkedAt: string;
};
