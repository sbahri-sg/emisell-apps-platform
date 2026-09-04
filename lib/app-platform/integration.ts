export type IntegrationSection = "home" | "settings" | "versions" | "api-access" | "extensions" | "webhooks";

export interface IntegrationReadiness {
  appId: string;
  organizationId: string;
  appName: string;
  appRevision: number;
  activeVersionId: string | null;
  activeVersion: string | null;
  checkedAt: string;
  checks: Array<{
    code: string;
    title: string;
    status: "pass" | "attention" | "blocked" | "info";
    detail: string;
    section: IntegrationSection;
  }>;
  scopes: Array<{
    scope: string;
    access: string;
    availability: "available" | "planned";
    endpoints: Array<{ method: string; path: string }>;
  }>;
  installations: {
    sampled: number;
    hasMore: boolean;
    activeCurrentVersion: number;
    activeOtherVersion: number;
    inactive: number;
  };
  listingStatus: "draft" | "published" | "hidden";
  listingRevision: number;
  endToEndVerified: boolean;
}
