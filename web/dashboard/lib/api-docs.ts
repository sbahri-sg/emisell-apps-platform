import generated from './api-docs.generated.json' with { type: 'json' };

export type Schema = {
  type?: string | string[];
  properties?: Record<string, Schema>;
  items?: Schema;
  required?: string[];
  enum?: unknown[];
  const?: unknown;
  description?: string;
  format?: string;
  minimum?: number;
  maximum?: number;
  minLength?: number;
  maxLength?: number;
  pattern?: string;
};
export type Operation = {
  availability?: string;
  target?: string;
  id: string;
  group: string;
  method: string;
  path: string;
  source: string;
  title: string;
  description: string;
  auth: string;
  parameters: {
    name: string;
    in: string;
    required?: boolean;
    description?: string;
    schema?: Schema;
  }[];
  request: Schema | null;
  responses: { status: string; description: string; schema: Schema | null }[];
};
export const operations = generated.operations as Operation[];
export const documents = generated.documents;
export const gateway = generated.gateway;
export const groups = [
  {
    id: 'admin',
    label: 'Dashboard Admin',
    note: 'Sesi admin platform. Administrator mengelola publikasi; administrator/reviewer memutuskan review. Operator hanya-baca.',
  },
  {
    id: 'developer',
    label: 'Portal Developer',
    note: 'Sesi developer terpisah; aplikasi dibatasi organisasi terautentikasi. Akun admin tidak menggantikan ownership developer.',
  },
  {
    id: 'store',
    label: 'App Store publik',
    note: 'Tanpa cookie atau konteks merchant. Hanya katalog published yang terverifikasi; semua listing gratis dan installable: false.',
  },
  {
    id: 'core',
    label: 'Core · ConnectRPC',
    note: 'Server-to-server pada port 8088. Key platform full access memakai merchant per request; key legacy tetap merchant-bound dengan scope per operasi. Origin dan Cookie browser ditolak. Bukan REST publik.',
  },
  {
    id: 'app-access',
    label: 'Aplikasi · akses lokal',
    note: 'Dua self-check terpisah pada port 8087: identitas app-client (Basic client ID/secret, tanpa grant) dan token installation (Bearer, merchant/app/installation, 15 menit). Keduanya bukan key Core, sesi portal, OAuth token exchange atau akses resource gateway. Tanpa Origin/Cookie.',
  },
  {
    id: 'gateway',
    label: 'Gateway Emisell',
    note: 'Kontrak Platform → Core untuk tim backend. Kesiapan endpoint dan scope dibaca dari verifikasi backend, bukan snapshot dokumen. Host gateway disediakan tim Core, bukan port Platform :8088.',
  },
] as const;

export function endpointAddress(o: Operation): string {
  if (o.source === 'core-reviewed-ui.v1.json') return `http://127.0.0.1:8000${o.path}`;
  return o.target === 'emisell-core'
    ? `CORE_GATEWAY_BASE_URL${o.path}`
    : `http://127.0.0.1:${o.method === 'RPC' ? '8088' : '8087'}${o.path}`;
}

export function filterCoverage(query: string, state: string) {
  const term = query.trim().toLowerCase();
  return gateway.coverage.filter(
    (c) =>
      (state === 'all' || c.contractStatus === state) &&
      `${c.scope} ${c.resource} ${c.operations.join(' ')} ${c.nextAction}`
        .toLowerCase()
        .includes(term),
  );
}

export function filterOperations(group: string, query: string): Operation[] {
  const term = query.trim().toLowerCase();
  return operations.filter(
    (o) =>
      o.group === group &&
      `${o.title} ${o.method} ${o.path} ${o.auth}`.toLowerCase().includes(term),
  );
}
export function schemaType(schema: Schema): string {
  if (schema.enum) return 'enum';
  if ('const' in schema) return JSON.stringify(schema.const);
  if (schema.type === 'array') return `${schemaType(schema.items ?? {})}[]`;
  return Array.isArray(schema.type)
    ? schema.type.join(' | ')
    : (schema.type ?? 'object');
}
// Structural examples only. No live sessions, IDs, tokens, or automatic requests.
export function example(schema: Schema, name = ''): unknown {
  if ('const' in schema) return schema.const;
  if (schema.enum)
    return (
      schema.enum.find(
        (v) => typeof v !== 'string' || !v.endsWith('_UNSPECIFIED'),
      ) ?? schema.enum[0]
    );
  if (schema.properties)
    return Object.fromEntries(
      Object.entries(schema.properties).map(([key, value]) => [
        key,
        example(value, key),
      ]),
    );
  if (schema.type === 'array') return [example(schema.items ?? {})];
  if (schema.type === 'boolean') return false;
  if (schema.type === 'integer' || schema.type === 'number')
    return schema.minimum ?? 1;
  if (schema.format === 'int64') return '10000';
  if (schema.format === 'date-time') return '2026-09-05T00:00:00Z';
  if (name === 'password') return 'YOUR_PASSWORD';
  if (name === 'email') return 'admin@example.invalid';
  if (name === 'idempotencyKey') return 'example-operation-0001';
  if (name === 'version') return '1.0.0';
  if (name === 'executionAllowed') return false;
  return `YOUR_${name.replace(/([a-z])([A-Z])/g, '$1_$2').toUpperCase() || 'VALUE'}`;
}
