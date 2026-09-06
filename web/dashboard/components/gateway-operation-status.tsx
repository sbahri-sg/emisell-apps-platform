import { gateway } from '@/lib/api-docs';
import {
  readinessLabel,
  verifiedOperationState,
  verifiedScopeState,
  type ScopeCatalog,
  type ScopeVerification,
} from '@/lib/access-scopes';
import { scopeCatalogURL } from '@/lib/scope-readiness';

export default function GatewayOperationStatus({
  procedure,
  catalog,
  verification,
}: {
  procedure: string;
  catalog: ScopeCatalog | null;
  verification: ScopeVerification | null;
}) {
  const operation = gateway.operations.find((o) => o.procedure === procedure);
  const compatible =
    verification?.contractRevision === gateway.contractRevision &&
    verification?.profile === catalog?.profile;
  const report = compatible ? verification : null;
  const state = verifiedOperationState(
    report,
    procedure,
    gateway.contractRevision,
  );
  const row = report?.operations?.find((o) => o.procedure === procedure);
  return (
    <section
      className="gateway-operation-status"
      aria-label="Kesiapan endpoint dan scope"
    >
      <p>
        <strong>Status endpoint:</strong>{' '}
        <span className={`access-state access-state-${state}`}>
          {readinessLabel[state]}
        </span>
      </p>
      {row?.blockers.length ? (
        <ul>
          {row.blockers.map((b) => (
            <li key={b}>{b}</li>
          ))}
        </ul>
      ) : null}
      <p>Scope yang diterima untuk operasi ini:</p>
      <ul>
        {operation?.acceptedScopes.map((handle) => {
          const scopeState = catalog
            ? verifiedScopeState(catalog, report, handle)
            : 'unknown';
          return (
            <li key={handle}>
              <a href={scopeCatalogURL(handle)}>
                <code>{handle}</code>
              </a>{' '}
              · scope {readinessLabel[scopeState]}
            </li>
          );
        })}
      </ul>
      <p className="access-meta">
        Kesiapan endpoint tidak otomatis melengkapi seluruh scope. Scope Active
        tetap memerlukan consent/grant merchant.
      </p>
    </section>
  );
}
