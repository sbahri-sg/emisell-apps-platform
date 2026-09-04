import type { ExtensionType } from './domain';

export type ExtensionCatalog = {
  version: string;
  categories: { id: string; name: string; description: string; examples: string[] }[];
  families: { type: ExtensionType; name: string; description: string; configurationSupported: boolean }[];
  capabilities: {
    id: string; name: string; type: ExtensionType;
    availability: 'available' | 'pilot' | 'planned'; executionEnabled: boolean;
    invocation: 'provider_to_platform' | 'platform_to_provider';
    requiredScopes: string[]; endpoints: { method: string; path: string }[];
    description: string; limitations: string[]; guideChapter: string;
  }[];
  surfaces: { id: string; name: string; availability: 'available' | 'planned'; description: string }[];
  runtimeContract: {
    version: string;
    status: 'draft';
    executionEnabled: boolean;
    transport: 'https_json';
    authentication: 'emisell_runtime_jwt';
    tokenTtlSeconds: number;
    identityClaims: string[];
    headers: { name: string; required: boolean; description: string }[];
    operations: {
      id: string;
      capabilityId: string;
      mode: 'synchronous';
      mutation: boolean;
      idempotency: 'required' | 'not_required';
      timeoutMs: number;
      maximumAttempts: number;
      description: string;
    }[];
    invariants: string[];
  };
};
