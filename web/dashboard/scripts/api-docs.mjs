// Generate a read-only reference from repository contract sources, never live data.
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { resolve } from 'node:path';

const root = fileURLToPath(new URL('../../../', import.meta.url));
const gateway = JSON.parse(
  execFileSync('go', ['run', './cmd/cli', 'gateway-contract'], {
    cwd: root,
    encoding: 'utf8',
    maxBuffer: 1024 * 1024,
  }),
);
const gatewayByProcedure = new Map(
  gateway.operations.map((o) => [o.procedure, o]),
);
const names = [
  'portals.v1.json',
  'catalog.v1.json',
  'access-scopes.v1.json',
  'platform-keys.v1.json',
  'app-access.v1.json',
  'integration-releases.v1.json',
  'managed-shipping.v1.json',
  'app-clients.v1.json',
  'testing.v1.json',
  'embedded-launches.v1.json',
  'core-reviewed-ui.v1.json',
];
const documents = names.map((name) => ({
  name,
  format: 'OpenAPI',
  content: readFileSync(resolve(root, 'api/openapi', name), 'utf8'),
}));
const specs = Object.fromEntries(
  documents.map((d) => [d.name, JSON.parse(d.content)]),
);
const methods = ['get', 'post', 'put', 'patch', 'delete'];

function expand(value, file, stack = []) {
  if (Array.isArray(value)) return value.map((v) => expand(v, file, stack));
  if (!value || typeof value !== 'object') return value;
  if (value.$ref) {
    const [external, fragment] = value.$ref.split('#');
    const targetFile = (external || file).replace(/^\.\//, '');
    const key = targetFile + '#' + fragment;
    if (stack.includes(key))
      throw new Error(
        `Recursive schema needs explicit rendering support: ${key}`,
      );
    let target = specs[targetFile];
    for (const part of fragment.slice(1).split('/'))
      target = target?.[part.replaceAll('~1', '/').replaceAll('~0', '~')];
    if (!target) throw new Error(`Unresolved contract reference ${key}`);
    return expand(
      {
        ...target,
        ...Object.fromEntries(
          Object.entries(value).filter(([k]) => k !== '$ref'),
        ),
      },
      targetFile,
      [...stack, key],
    );
  }
  return Object.fromEntries(
    Object.entries(value).map(([k, v]) => [k, expand(v, file, stack)]),
  );
}

const operations = new Map();
for (const name of names) {
  const spec = specs[name];
  for (const [path, item] of Object.entries(spec.paths)) {
    // Imported path items belong to the source contract, not the importing file.
    const source = (item.$ref?.split('#')[0] || name).replace(/^\.\//, '');
    const pathItem = expand(item, name);
    for (const method of methods) {
      const op = pathItem[method];
      if (!op) continue;
      const security = op.security ?? specs[source].security ?? [];
      const group = path.startsWith('/api/v1/admin/')
        ? 'admin'
        : path.startsWith('/api/v1/developer/')
          ? 'developer'
          : path.startsWith('/api/v1/store/')
            ? 'store'
            : [
                  '/api/v1/app/installation-access',
                  '/api/v1/app/client-check',
                ].includes(path)
              ? 'app-access'
              : name === 'core-reviewed-ui.v1.json' && path.startsWith('/v1/app-platform/core/')
                ? 'core'
                : null;
      if (!group)
        throw new Error(`Out-of-scope endpoint in portal docs: ${path}`);
      const id = `${method.toUpperCase()} ${path}`;
      const value = {
        id,
        group,
        method: method.toUpperCase(),
        path,
        source,
        title:
          op.summary ||
          op.operationId ||
          path.split('/').filter(Boolean).slice(-2).join(' / '),
        description: op.description || '',
        auth: security.length
          ? security
              .map((s) =>
                Object.keys(s)
                  .map((key) => {
                    const scheme =
                      specs[source].components?.securitySchemes?.[key];
                    return scheme?.in === 'cookie'
                      ? `Cookie ${scheme.name}`
                      : key;
                  })
                  .join(' + '),
              )
              .join(' atau ')
          : 'Tanpa sesi',
        parameters: [...(pathItem.parameters ?? []), ...(op.parameters ?? [])],
        request: op.requestBody?.content?.['application/json']?.schema ?? null,
        responses: Object.entries(op.responses).map(([status, response]) => ({
          status,
          description: response.description,
          schema: response.content?.['application/json']?.schema ?? null,
        })),
      };
      if (
        operations.has(id) &&
        JSON.stringify(operations.get(id)) !== JSON.stringify(value)
      )
        throw new Error(`Conflicting operation ${id}`);
      operations.set(id, value);
    }
  }
}

// Use the actual Protobuf compiler descriptor, not a regex parser or handwritten fields.
const descriptor = JSON.parse(
  execFileSync(
    resolve(root, 'bin/buf'),
    ['build', '--as-file-descriptor-set', '-o', '-#format=json'],
    { cwd: root, encoding: 'utf8', maxBuffer: 8 * 1024 * 1024 },
  ),
);
const messages = new Map();
const enums = new Map();
const protoComments = new Map();
for (const file of descriptor.file) {
  if (!file.package.startsWith('emisell.')) continue;
  for (const message of file.messageType ?? [])
    messages.set(`.${file.package}.${message.name}`, message);
  for (const [mi, message] of (file.messageType ?? []).entries()) {
    const type = `.${file.package}.${message.name}`;
    for (const [fi, field] of (message.field ?? []).entries()) {
      const comment = file.sourceCodeInfo?.location
        ?.find((l) => l.path?.join('.') === `4.${mi}.2.${fi}`)
        ?.leadingComments?.trim();
      if (comment) protoComments.set(`${type}.${field.jsonName}`, comment);
    }
  }
  for (const e of file.enumType ?? [])
    enums.set(
      `.${file.package}.${e.name}`,
      e.value.map((v) => v.name),
    );
}
function messageSchema(typeName) {
  if (typeName === '.google.protobuf.Timestamp')
    return { type: 'string', format: 'date-time' };
  const message = messages.get(typeName);
  if (!message) throw new Error(`Unknown message ${typeName}`);
  return {
    type: 'object',
    properties: Object.fromEntries(
      // Deprecated wire aliases stay in downloadable compatibility contracts,
      // not in primary request/response schemas or generated examples.
      (message.field ?? [])
        .filter((field) => !field.options?.deprecated)
        .map((field) => {
          let schema;
          if (field.type === 'TYPE_MESSAGE')
            schema = messageSchema(field.typeName);
          else if (field.type === 'TYPE_ENUM')
            schema = { type: 'string', enum: enums.get(field.typeName) };
          else if (field.type === 'TYPE_BOOL') schema = { type: 'boolean' };
          else if (field.type === 'TYPE_STRING') schema = { type: 'string' };
          else if (field.type === 'TYPE_INT64')
            schema = {
              type: 'string',
              format: 'int64',
              description: 'ProtoJSON int64 dikirim sebagai string desimal.',
            };
          else if (field.type === 'TYPE_INT32')
            schema = { type: 'integer', format: 'int32' };
          else throw new Error(`Unsupported Protobuf type ${field.type}`);
          if (field.label === 'LABEL_REPEATED')
            schema = { type: 'array', items: schema };
          const comment = protoComments.get(`${typeName}.${field.jsonName}`);
          if (comment)
            schema.description = [comment, schema.description]
              .filter(Boolean)
              .join(' ');
          return [field.jsonName, schema];
        }),
    ),
  };
}
const scopeByService = {
  EngineGrantService: {
    Check:
      'independent local engine credential + current managed shipping.read grant',
  },
  TestDistributionService: {
    ListAssignments:
      'platform full-access Core key + verified merchant app manager (approved test distribution metadata only)',
  },
  ConnectionService: {
    Check: 'valid Core credential (own identity only; no resource grant)',
  },
  InstallIntentService: {
    Prepare: 'apps.install_intents.write',
    Get: 'apps.install_intents.read',
    Decide: 'apps.install_intents.consent',
  },
  InstallationService: {
    ListInstallations:
      'platform full-access Core key + verified merchant app manager (read-only summaries)',
    Consume: 'platform full-access Core key + owner-bound consent',
    GetInstallation: 'platform full-access Core key + installation owner',
    Activate: 'platform full-access Core key + installation owner',
    IssueToken: 'platform full-access Core key + active installation owner',
    Uninstall: 'platform full-access Core key + installation owner',
  },
  PaymentService: {
    Create: 'payments.write',
    Capture: 'payments.write',
    Refund: 'payments.write',
    Status: 'payments.read',
  },
  ShippingService: {
    GetRates: 'shipping.read',
    Create: 'shipping.write',
    Track: 'shipping.read',
  },
};
for (const file of descriptor.file) {
  if (!file.package.startsWith('emisell.')) continue;
  const source = file.name;
  documents.push({
    name: source,
    format: 'Protobuf',
    content: readFileSync(resolve(root, 'api/proto', source), 'utf8'),
  });
  for (const service of file.service ?? [])
    for (const method of service.method) {
      const path = `/${file.package}.${service.name}/${method.name}`;
      const planned = gatewayByProcedure.get(path);
      const scope =
        planned?.requiredScope ?? scopeByService[service.name]?.[method.name];
      if (!scope)
        throw new Error(`Document the authorization scope for ${path}`);
      const isIntent = service.name === 'InstallIntentService';
      operations.set(path, {
        id: path,
        group: planned ? 'gateway' : 'core',
        availability: planned ? 'planned' : 'local_reference',
        target: planned ? 'emisell-core' : 'app-platform',
        method: 'RPC',
        path,
        source,
        title: `${service.name} · ${method.name}`,
        description: planned
          ? `${planned.description} Kontrak Platform → Core. Kesiapan operasional diperiksa melalui verifikasi backend; base URL disediakan tim Core, bukan port Platform :8088.`
          : service.name === 'EngineGrantService'
            ? 'Pemeriksaan native grant shipping.read untuk engine API-Kurir, bukan resource scope read_shipping/write_shipping. Kontrak lokal hanya provider emisell, environment local-isolated, operasi rates.read/settings.read. Pilot API-Kurir utama menerapkan rates.read sebelum cache tarif; settings.read dalam kontrak bukan bukti seluruh endpoint pengaturan utama sudah dilindungi. Merchant/provider berasal dari konteks engine terverifikasi. Periksa signature/release, assignment dan instalasi aktif tanpa cache grant; revocation/suspension atau outage menolak akses. Request yang sudah terotorisasi dan berbatas waktu dapat selesai saat revocation berlangsung. Tidak membuka shipment/tracking atau scope resource Planned. Status resource tetap di Katalog scope. Bukan endpoint Core atau developer; bukan bukti rollout produksi.'
            : service.name === 'TestDistributionService'
              ? 'Assignment pengujian untuk merchant terverifikasi. Bukan consent/grant. Release terkelola signed dan assignment approved dapat installable=true hanya pada komposisi engine lokal terisolasi. Integration runtime umum tetap belum tersedia. Maksimum 20 per halaman; cursor bukan identitas. Core memeriksa sesi dan izin apps setiap halaman.'
              : service.name === 'ConnectionService'
                ? 'Uji autentikasi key Core: key platform mengembalikan platformFullAccess=true tanpa merchant/scopes/expiry. Key legacy tetap mengembalikan binding merchant/scopes/expiry. Tidak membaca data merchant, menjalankan transaksi, atau membuktikan scope resource aktif.'
                : isIntent
                  ? 'Consent record lokal untuk fixture compatibility dan managed shipping signed/assigned (ADR 0027). Key platform wajib mengirim merchantId yang telah diotorisasi oleh backend Core; key legacy tetap terikat merchant credential (merchantId boleh dihilangkan). TTL 10 menit; terikat merchant, service, actor, versi, dan scope. Managed source serta engine readiness diperiksa terkini. Tidak membuat instalasi, token atau active grant. executionAllowed selalu false.'
                  : service.name === 'InstallationService' &&
                      method.name === 'ListInstallations'
                    ? 'Daftar read-only instalasi intent-managed saat ini milik merchant, lintas actor/key pemasang. Core memverifikasi sesi dan izin kelola apps setiap halaman. Maksimum 20 per halaman, afterId hanya posisi baca; tanpa token, digest consent atau kewenangan mutation. Pending/disabling dibedakan dari Active; riwayat uninstalled dan instalasi legacy tidak disertakan.'
                    : service.name === 'InstallationService'
                      ? 'Lifecycle lokal: Consume consent baru menjadi pending; Activate memverifikasi release/runtime lalu mengaktifkan grant. Managed shipping memerlukan signed release, approved assignment dan engine lokal siap; tidak memilih provider checkout dan tidak mendukung IssueToken. IssueToken fixture menerbitkan token self-check 15 menit sekali tampil (retry tanpa secret). Uninstall mencabut grant/token dan routing, tetap tersedia saat source/engine gagal. Gratis, bukan katalog metadata atau scope resource Plan. Merchant, service dan actor harus cocok. Retry mengembalikan status terkini, bukan mengaktifkan kembali instalasi.'
                      : service.name === 'PaymentService'
                        ? 'INTERNAL / LEGACY FIXTURE ONLY. RPC payment dipertahankan untuk compatibility instalasi lokal; bukan API payment gateway developer umum atau transaksi nyata. Payment gateway checkout dikelola modul internal Emisell melalui Settings → Payments. Tidak tersedia melalui authoring/distribusi publik App Platform (ADR 0022).'
                        : 'Capability shipping provider-neutral dengan instalasi merchant aktif. Reference lokal adalah simulasi, bukan pengiriman nyata. Referensi API-Kurir opt-in hanya GetRates dengan originZone dan destinationZone netral serta scope shipping.read; runtime memetakan zona ke gateway tiruan. Bukan quote fulfillment/booking, tidak mengaktifkan provider, dan tidak membuka runtime developer umum (ADR 0023).',
        auth: planned
          ? `Delegasi server-to-server terverifikasi + current active grant · ${planned.acceptedScopes.join(' atau ')}. AccessContext bukan otorisasi. Trust/issuer masih gate integrasi.`
          : service.name === 'EngineGrantService'
            ? 'Bearer independent engine credential (local operator provisioning). Full Core key, developer key, app token dan browser session DITOLAK.'
            : ['InstallationService', 'TestDistributionService'].includes(
                  service.name,
                )
              ? `Bearer ${scope}. Legacy key dan token aplikasi ditolak. Core wajib memverifikasi kewenangan staf; merchantId wajib.`
              : `Bearer platform full-access key atau legacy service account · ${scope}. Operasi data tetap memerlukan merchant dan installation/grant yang valid.`,
        parameters: [],
        request: messageSchema(method.inputType),
        responses: [
          {
            status: '200',
            description: 'Connect unary response (ProtoJSON).',
            schema: messageSchema(method.outputType),
          },
          ...[
            'unauthenticated',
            'permission_denied',
            'not_found',
            'invalid_argument',
            ...(planned ? ['resource_exhausted'] : ['already_exists']),
            'unavailable',
            'deadline_exceeded',
            'canceled',
            'internal',
          ].map((status) => ({
            status,
            description: 'Connect error; detail internal tidak dikirim.',
            schema: {
              type: 'object',
              properties: {
                code: { const: status },
                message: { type: 'string' },
              },
            },
          })),
        ],
      });
    }
}
for (const [path] of gatewayByProcedure)
  if (!operations.has(path))
    throw new Error(`Gateway mapping missing in Protobuf: ${path}`);
const coverageOutput = JSON.stringify(gateway, null, 2) + '\n';
documents.push({
  name: 'docs/core-installation-lifecycle.md',
  format: 'Markdown',
  content: readFileSync(
    resolve(root, 'docs/core-installation-lifecycle.md'),
    'utf8',
  ),
});
documents.push({
  name: 'docs/emisell-gateway-handoff.md',
  format: 'Markdown',
  content: readFileSync(
    resolve(root, 'docs/emisell-gateway-handoff.md'),
    'utf8',
  ),
});
documents.push({
  name: 'gateway-coverage.v1.generated.json',
  format: 'JSON',
  content: coverageOutput,
});
const output =
  JSON.stringify(
    { documents, operations: [...operations.values()], gateway },
    null,
    2,
  ) + '\n';
const target = resolve(root, 'web/dashboard/lib/api-docs.generated.json');
const coverageTarget = resolve(root, 'api/gateway/coverage.v1.generated.json');
if (process.argv.includes('--check')) {
  if (
    readFileSync(target, 'utf8') !== output ||
    readFileSync(coverageTarget, 'utf8') !== coverageOutput
  )
    throw new Error(
      'API docs stale: run npm run docs:generate in web/dashboard',
    );
  console.log(
    `API docs match sources: ${operations.size} operations, ${documents.length} contracts.`,
  );
} else {
  // Generated output directory only; never write credentials or runtime state.
  mkdirSync(resolve(root, 'api/gateway'), { recursive: true });
  writeFileSync(coverageTarget, coverageOutput);
  writeFileSync(target, output);
  console.log(
    `Generated ${operations.size} API operations from ${documents.length} contracts.`,
  );
}
