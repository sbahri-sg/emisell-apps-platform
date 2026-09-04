import { buildPostman, buildReference, selectSpec } from './contracts.mjs';

// The developer portal publishes one machine contract. Dashboard management is
// performed through the Console UI, while Payment/Shipping execution belongs
// to their approved runtime gateways and is documented as capability guidance.
export const developerContracts = Object.freeze(['provider']);

// Called only after developer identity verification. Validate before loading
// either source so query parameters can never select an internal contract.
export async function renderDeveloperDocumentation(params, { loadGuide, loadSources }) {
  const format = params.get('format') ?? 'guide';
  const contract = params.get('contract') ?? 'provider';
  if (!developerContracts.includes(contract) || !['guide', 'reference', 'openapi', 'postman'].includes(format)) {
    return Response.json({ error: { code: 'validation_error', message: 'Only the Partner API contract is available in the developer portal.' } }, { status: 400 });
  }
  if (format === 'guide') return Response.json(await loadGuide());
  const sources = await loadSources();
  const result = format === 'reference' ? buildReference(sources, contract, developerContracts)
    : format === 'openapi' ? selectSpec(sources, contract) : buildPostman(sources, contract);
  return Response.json(result, { headers: format === 'reference' ? {} : {
    'Content-Disposition': `attachment; filename="emisell-${contract}.${format === 'postman' ? 'postman_collection' : 'openapi'}.json"`,
  } });
}
