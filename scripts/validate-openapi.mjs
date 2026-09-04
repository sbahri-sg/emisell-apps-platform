import { readFileSync } from 'node:fs';

const contracts = [
  {
    file: new URL('../docs/shipping-provider.openapi.json', import.meta.url),
    title: 'Emisell Shipping Provider — Reference Contract',
    requiredSecurityScheme: 'provider_key',
    implementationStatus: 'planned',
  },
  {
    file: new URL('../docs/openapi.json', import.meta.url),
    title: 'Emisell App Platform API',
    requiredSecurityScheme: 'bearerAuth',
  },
  {
    file: new URL('../docs/provider-openapi.json', import.meta.url),
    title: 'Emisell Partner API',
    requiredSecurityScheme: 'installationBearer',
  },
  {
    file: new URL('../docs/emisell-resource-openapi.json', import.meta.url),
    title: 'Emisell Resource API',
    requiredSecurityScheme: 'appGatewayAssertion',
    implementationStatus: 'implemented_gated',
  },
  {
    file: new URL('../docs/emisell-resource-blueprint.openapi.json', import.meta.url),
    title: 'Emisell Resource API — Backend Blueprint',
    requiredSecurityScheme: 'appGatewayAssertion',
    implementationStatus: 'planned',
  },
];
const methods = new Set(['get', 'post', 'put', 'patch', 'delete', 'options', 'head']);
let totalOperations = 0;
let totalSchemas = 0;
let totalReferences = 0;

for (const contract of contracts) {
  const spec = JSON.parse(readFileSync(contract.file, 'utf8'));
  const errors = [];
  const refs = [];
  const operationIds = new Map();

  function visit(value, location = '#') {
    if (Array.isArray(value)) {
      value.forEach((item, index) => visit(item, `${location}/${index}`));
      return;
    }
    if (!value || typeof value !== 'object') return;
    if (typeof value.$ref === 'string') refs.push({ ref: value.$ref, location });
    for (const [key, child] of Object.entries(value)) visit(child, `${location}/${key}`);
  }

  function resolveLocalRef(ref) {
    if (!ref.startsWith('#/')) return undefined;
    return ref.slice(2).split('/').reduce(
      (current, token) => current?.[token.replaceAll('~1', '/').replaceAll('~0', '~')],
      spec,
    );
  }

  if (spec.openapi !== '3.1.0') errors.push('openapi must be 3.1.0');
  if (spec.info?.title !== contract.title) errors.push(`unexpected API title: ${spec.info?.title}`);
  if (!spec.paths || Object.keys(spec.paths).length === 0) errors.push('at least one API path is required');
  if (!spec.components?.securitySchemes?.[contract.requiredSecurityScheme]) {
    errors.push(`${contract.requiredSecurityScheme} security scheme is required`);
  }
  if (contract.implementationStatus) {
    const implementation = spec['x-emisell-implementation'];
    if (implementation?.status !== contract.implementationStatus) {
      errors.push(`x-emisell-implementation.status must be ${contract.implementationStatus}`);
    }
    if (!Array.isArray(implementation?.implementedPaths)) {
      errors.push('x-emisell-implementation.implementedPaths must be an array');
    }
    if (contract.implementationStatus === 'planned' && implementation?.implementedPaths?.length !== 0) {
      errors.push('a planned contract must not claim implemented paths');
    }
    const documentedPaths = Object.keys(spec.paths ?? {}).sort();
    if (contract.implementationStatus === 'implemented_gated' &&
        (implementation.defaultEnabled !== false || JSON.stringify([...implementation.implementedPaths].sort()) !== JSON.stringify(documentedPaths))) {
      errors.push('gated resource must be disabled by default and implemented paths must match the contract');
    }
    const targetPaths = Array.isArray(implementation?.targetPaths)
      ? [...implementation.targetPaths].sort()
      : [];
    if (JSON.stringify(documentedPaths) !== JSON.stringify(targetPaths)) {
      errors.push('x-emisell-implementation.targetPaths must exactly match documented paths');
    }
  }

  for (const [path, pathItem] of Object.entries(spec.paths ?? {})) {
    for (const [method, operation] of Object.entries(pathItem)) {
      if (!methods.has(method)) continue;
      const location = `${method.toUpperCase()} ${path}`;
      if (!operation.operationId) errors.push(`${location} is missing operationId`);
      if (operation.operationId) {
        const previous = operationIds.get(operation.operationId);
        if (previous) errors.push(`duplicate operationId ${operation.operationId}: ${previous} and ${location}`);
        operationIds.set(operation.operationId, location);
      }
      if (!operation.summary) errors.push(`${location} is missing summary`);
      if (!operation.responses || Object.keys(operation.responses).length === 0) errors.push(`${location} is missing responses`);

      const placeholders = [...path.matchAll(/\{([^}]+)\}/g)].map((match) => match[1]);
      const parameters = [...(pathItem.parameters ?? []), ...(operation.parameters ?? [])].map(
        (parameter) => parameter.$ref ? resolveLocalRef(parameter.$ref) : parameter,
      );
      for (const placeholder of placeholders) {
        const parameter = parameters.find((candidate) => candidate?.name === placeholder && candidate?.in === 'path');
        if (!parameter) errors.push(`${location} is missing path parameter ${placeholder}`);
        else if (parameter.required !== true) errors.push(`${location} path parameter ${placeholder} must be required`);
      }
    }
  }

  visit(spec);
  for (const { ref, location } of refs) {
    if (!ref.startsWith('#/')) errors.push(`${location} uses unsupported external ref ${ref}`);
    else if (resolveLocalRef(ref) === undefined) errors.push(`${location} contains unresolved ref ${ref}`);
  }

  for (const [name, schema] of Object.entries(spec.components?.schemas ?? {})) {
    if (!Array.isArray(schema.required) || !schema.properties) continue;
    for (const property of schema.required) {
      if (!(property in schema.properties)) errors.push(`schema ${name} requires undefined property ${property}`);
    }
  }

  if (errors.length) {
    console.error(`${contract.title} validation failed with ${errors.length} issue(s):`);
    errors.forEach((error) => console.error(`- ${error}`));
    process.exitCode = 1;
  } else {
    console.log(`${contract.title} valid: ${operationIds.size} operations, ${Object.keys(spec.components.schemas).length} schemas, ${refs.length} references.`);
  }
  totalOperations += operationIds.size;
  totalSchemas += Object.keys(spec.components?.schemas ?? {}).length;
  totalReferences += refs.length;
}

if (process.exitCode) process.exit(process.exitCode);
console.log(`All OpenAPI contracts valid: ${totalOperations} operations, ${totalSchemas} schemas, ${totalReferences} references.`);
