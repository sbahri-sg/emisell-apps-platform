"use client";

import { useEffect, useState } from "react";
import { AlertCircle, ArrowRight, CheckCircle2, Info, LoaderCircle, RefreshCw } from "lucide-react";
import { appPlatformClient, apiErrorMessage } from "../lib/app-platform/client";
import type { IntegrationReadiness, IntegrationSection } from "../lib/app-platform/integration";
import { InlineNotice, Panel } from "./components/ui";

const statusLabels = { pass: "Observed", attention: "Needs review", blocked: "Missing", info: "Information" };

type IntegrationReadinessProps = {
  appId: string;
  organizationId?: string;
  onNavigate?: (section: IntegrationSection) => void;
  onLoaded?: (report: IntegrationReadiness | null) => void;
};

export function IntegrationReadinessView(props: IntegrationReadinessProps) {
  return <IntegrationReadinessPanel key={`${props.organizationId ?? "developer"}:${props.appId}`} {...props} />;
}

function IntegrationReadinessPanel({ appId, organizationId, onNavigate, onLoaded }: IntegrationReadinessProps) {
  const [report, setReport] = useState<IntegrationReadiness | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [refresh, setRefresh] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    appPlatformClient.getIntegrationReadiness(appId, organizationId, controller.signal)
      .then((data) => {
        if (controller.signal.aborted) return;
        setReport(data);
        onLoaded?.(data);
      })
      .catch((err) => { if (!controller.signal.aborted) setError(apiErrorMessage(err)); })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [appId, organizationId, refresh, onLoaded]);

  return (
    <section className="integration-readiness" aria-label="App integration inspection" aria-busy={loading}>
      <header className="integration-heading">
        <div><p className="section-kicker">Emisell connection</p><h2>Integration readiness</h2><p>Inspect saved configuration and installation evidence before handoff.</p></div>
        <button className="secondary-button" type="button" disabled={loading} onClick={() => { setLoading(true); setError(null); setReport(null); onLoaded?.(null); setRefresh((value) => value + 1); }}><RefreshCw size={14} />Refresh</button>
      </header>
      <InlineNotice title="Configuration inspection, not an end-to-end test">
        This view makes no external requests and grants no production access. Merchant consent, provider behavior, resource access and uninstall handling still need an isolated integration test.
      </InlineNotice>
      {loading ? <div className="integration-loading" role="status"><LoaderCircle className="spin" size={18} />Reading App Gateway…</div> : error ? (
        <div role="alert"><InlineNotice tone="warning" title="Inspection unavailable">{error} Refresh to retry; no readiness result is assumed.</InlineNotice></div>
      ) : report ? <>
        <dl className="integration-summary">
          <div><dt>Active version</dt><dd>{report.activeVersion ?? "Not activated"}</dd></div>
          <div><dt>Publication</dt><dd>{report.listingStatus}</dd></div>
          <div><dt>Current-version installations</dt><dd>{report.installations.activeCurrentVersion}{report.installations.hasMore ? " in sample" : ""}</dd></div>
          <div><dt>Inspected</dt><dd>{new Date(report.checkedAt).toLocaleString()}</dd></div>
        </dl>
        <Panel className="integration-checks">
          <h3>Configuration & evidence</h3>
          <ul>
            {report.checks.map((check) => {
              const Icon = check.status === "pass" ? CheckCircle2 : check.status === "info" ? Info : AlertCircle;
              return <li key={check.code}>
                <Icon className={`integration-state ${check.status}`} size={18} aria-hidden="true" />
                <div><div className="integration-check-title"><h4>{check.title}</h4><span className={`integration-label ${check.status}`}>{statusLabels[check.status]}</span></div><p>{check.detail}</p></div>
                {onNavigate ? <button className="text-button" type="button" aria-label={`Open configuration for ${check.title}`} onClick={() => onNavigate(check.section)}>Open <ArrowRight size={14} /></button> : null}
              </li>;
            })}
          </ul>
        </Panel>
        <Panel className="integration-scopes">
          <h3>Resources available to this version</h3>
          <p>Derived from the gateway scope catalog. Planned scopes are not a promise of an available endpoint.</p>
          {report.scopes.length ? <div className="integration-table-wrap"><table><thead><tr><th>Scope</th><th>Consent</th><th>Availability</th><th>Provider endpoint</th></tr></thead><tbody>{report.scopes.map((scope) => <tr key={scope.scope}><td><code>{scope.scope}</code></td><td>{scope.access}</td><td>{scope.availability}</td><td>{scope.endpoints.length ? scope.endpoints.map((endpoint) => <code key={`${endpoint.method} ${endpoint.path}`}>{endpoint.method} {endpoint.path}</code>) : "Not generally available"}</td></tr>)}</tbody></table></div> : <p className="integration-empty">No resource scopes in the active snapshot.</p>}
        </Panel>
        <Panel className="integration-handoff">
          <h3>Handoff responsibilities</h3>
          <dl>
            <div><dt>Developer app</dt><dd>Implement OAuth with PKCE, store installation tokens on the server, honor scopes, verify webhook signatures and handle revoked access.</dd></div>
            <div><dt>Emisell backend</dt><dd>Supply verified merchant identity through the signed session bridge. Merchant ID alone never authorizes installation or resource access.</dd></div>
            <div><dt>Emisell admin</dt><dd>Review the intended use and test evidence, then decide publication separately from production entitlement. Configuration checks are not an approval.</dd></div>
          </dl>
          {organizationId ? <a className="text-button" href="/admin/docs?contract=emisell&view=reference">Open Emisell integration API <ArrowRight size={14} /></a> : null}
        </Panel>
      </> : null}
    </section>
  );
}
