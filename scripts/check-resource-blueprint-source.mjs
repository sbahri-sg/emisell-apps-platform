// Read-only audit of contract source mappings. Never imports backend code,
// connects to a database, runs a seed, or claims endpoint implementation.
import assert from 'node:assert/strict';
import { readFileSync, readdirSync } from 'node:fs';
import { resolve, join } from 'node:path';

const checkout = process.argv[2];
if (!checkout) throw new Error('Pass the api-service checkout path (read-only source inspection).');
const directory = resolve(checkout, 'prisma/models');
const files = readdirSync(directory).filter((name) => name.endsWith('.prisma'));
const source = files.map((name) => readFileSync(join(directory, name), 'utf8')).join('\n');
const blocks = (kind) => new Map([...source.matchAll(new RegExp(`^${kind} (\\w+) \\{([\\s\\S]*?)^\\}`, 'gm'))].map((match) => [match[1], match[2]]));
const models = blocks('model');
const enums = blocks('enum');
const spec = JSON.parse(readFileSync(new URL('../docs/emisell-resource-blueprint.openapi.json', import.meta.url), 'utf8'));
let checked = 0;
for (const schema of Object.values(spec.components.schemas)) {
  for (const value of Object.values(schema.properties ?? {})) {
    const mapping = value['x-emisell-source'];
    if (!mapping) continue;
    const [model, field] = mapping.split('.');
    assert.ok(models.has(model), `Missing model: ${model}`);
    const sourceField = models.get(model).match(new RegExp(`^\\s*${field}\\s+(\\w+)`, 'm'));
    assert.ok(sourceField, `Missing field: ${mapping}`);
    if (value['x-extensible-enum'] || value.enum) {
      const enumBody = enums.get(sourceField[1]);
      assert.ok(enumBody, `Missing enum for ${mapping}`);
      const values = enumBody.split('\n').map((line) => line.trim().split(/\s|\/\//)[0]).filter(Boolean);
      assert.deepEqual([...(value['x-extensible-enum'] ?? value.enum)].sort(), values.sort(), `Enum drift: ${mapping}`);
    }
    checked += 1;
  }
}
for (const item of Object.values(spec.paths)) {
  const operation = item.get;
  const backend = operation['x-emisell-backend'];
  for (const model of backend.models) assert.ok(models.has(model), `Missing source model: ${model}`);
  const schemaRef = operation.responses['200'].content['application/json'].schema.$ref.split('/').at(-1);
  const data = spec.components.schemas[schemaRef].properties.data;
  const projection = spec.components.schemas[(data.items ?? data).$ref.split('/').at(-1)];
  if (operation.parameters.some((parameter) => parameter.$ref.endsWith('/UpdatedAfter'))) {
    assert.ok(projection.properties.updatedAt, `${operation.operationId}: incremental endpoint without updatedAt`);
  }
}
console.log(`Resource blueprint: ${checked} source field mappings and ${Object.keys(spec.paths).length} target operations checked. No backend/database changes; runtime not verified.`);
