export type ContractId = 'admin' | 'developer' | 'provider' | 'emisell' | 'identity' | 'resource' | 'resource-blueprint' | 'managed' | 'shipping-provider';

export type DocumentationContract = {
  id: ContractId;
  title: string;
  audience: string;
  description: string;
  source: string;
  planned: boolean;
  visibility: 'primary' | 'operator' | 'roadmap';
  status: string;
  pilot?: boolean;
  version: string;
  count: number;
};

export type DocumentationField = {
  name: string;
  location: string;
  required: boolean;
  type: string;
  description: string;
  constraints: string;
};

export type DocumentationOperation = {
  id: string;
  method: string;
  path: string;
  summary: string;
  description: string;
  tag: string;
  authentication: string[];
  backend: {
    models: string[];
    tenantRule: string;
    pagination: string;
    providerTargetPath: string;
    scopes: string[];
    fields: { name: string; source: string; description: string }[];
  } | null;
  fields: DocumentationField[];
  request: { contentType: string; example: unknown; schema: unknown } | null;
  responses: { status: string; description: string; example: unknown; schema: unknown }[];
  curl: string;
};

export type DocumentationGroup = {
  id: string;
  title: string;
  description: string;
  count: number;
  operationIds: string[];
};

export type DocumentationFlowStep = Pick<DocumentationOperation, 'id' | 'method' | 'path' | 'summary'>;

export type DocumentationFlow = {
  status: string;
  outcome: string;
  actors: string[];
  routeFamilies: string[];
  quickStart: DocumentationFlowStep[];
  guardrails: string[];
  groups: DocumentationGroup[];
};

export type DocumentationGatewayIntegration = {
  id: string;
  title: string;
  status: 'pilot_default_off';
  summary: string;
  operationId: string;
  sequence: { actor: string; action: string }[];
  hops: {
    id: string;
    label: string;
    from: string;
    to: string;
    method: string;
    path: string;
    contentType: string;
    authentication: string;
    identity: string;
    fields: string[];
    headers: string[];
    forbiddenSelectors: string[];
    timeoutMs: number;
  }[];
  ownership: { concern: string; owner: string; rule: string }[];
  failurePolicy: string[];
  sandboxChecklist: string[];
};

export type DocumentationReference = {
  contracts: DocumentationContract[];
  selected: DocumentationContract;
  operations: DocumentationOperation[];
  flow: DocumentationFlow;
  gatewayIntegrations: DocumentationGatewayIntegration[];
  handoff: {
    reviewedOn: string;
    source: string;
    protocol: string;
    decisions: { topic: string; shopify: string; emisell: string; source: string }[];
    acceptance: string[];
    deferred: string[];
  } | null;
};
