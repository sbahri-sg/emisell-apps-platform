'use client';

import { useEffect, useState } from 'react';
import { ArrowUpRight, Blocks, FileCode2, RefreshCw } from 'lucide-react';
import { Button } from '@/components/ui/button';
import type { Draft, PortalAPI } from '@/lib/portal';
import {
  appLifecycleStatus,
  loadAppLifecycle,
  type AppLifecycle,
} from '@/lib/app-lifecycle';

export default function DeveloperApps({
  api,
  drafts,
  busy,
  openDraft,
  filter = 'all',
  compact = false,
}: {
  api: PortalAPI;
  drafts: Draft[];
  busy: boolean;
  openDraft: (id: string) => void;
  filter?: string;
  compact?: boolean;
}) {
  const [attempt, setAttempt] = useState(0);
  const [state, setState] = useState<{
    data: AppLifecycle | null;
    failed: boolean;
  }>({ data: null, failed: false });
  useEffect(() => {
    if (compact) return;
    let current = true;
    loadAppLifecycle(api)
      .then((data) => {
        if (current) setState({ data, failed: false });
      })
      .catch(() => {
        if (current) setState({ data: null, failed: true });
      });
    return () => {
      current = false;
    };
  }, [api, attempt, compact]);
  if (compact)
    return (
      <div className="dev-apps-compact-list">
        {drafts.map((draft) => (
          <button
            type="button"
            className="dev-apps-compact-card"
            key={draft.id}
            disabled={busy}
            onClick={() => openDraft(draft.id)}
          >
            <span className="dev-apps-compact-icon" aria-hidden="true">
              <Blocks />
            </span>
            <span className="dev-apps-compact-copy">
              <strong>{draft.document.name}</strong>
              <span title={draft.id}>
                v{draft.document.version} · {draft.id}
              </span>
            </span>
          </button>
        ))}
      </div>
    );
  return (
    <>
      <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
        <output className="text-sm text-muted-foreground">
          {state.failed
            ? 'Status rilis belum dapat dimuat. Coba perbarui status.'
            : state.data
              ? `${drafts.length} aplikasi · Status rilis terbaru`
              : 'Memuat status rilis…'}
        </output>
        <Button
          variant="ghost"
          disabled={busy || (!state.data && !state.failed)}
          onClick={() => {
            setState({ data: null, failed: false });
            setAttempt((value) => value + 1);
          }}
        >
          <RefreshCw />
          Perbarui status
        </Button>
      </div>
      <div className="app-list">
        {drafts.map((draft) => {
          let result = {
            label: state.failed ? 'Belum terverifikasi' : 'Memuat status…',
            tone: 'draft',
            version: draft.document.version,
          };
          if (state.data) {
            try {
              result = appLifecycleStatus(draft, state.data);
            } catch {
              result = { ...result, label: 'Belum terverifikasi' };
            }
          }
          if (filter === 'published' && result.label !== 'Dipublikasikan')
            return null;
          if (filter === 'draft' && result.label !== 'Draft') return null;
          return (
            <button
              className="app-row"
              key={draft.id}
              disabled={busy}
              onClick={() => openDraft(draft.id)}
            >
              <span className="app-glyph">
                <FileCode2 />
              </span>
              <span className="app-row-main">
                <strong>{draft.document.name}</strong>
                <span>{draft.document.summary || 'Ringkasan belum diisi'}</span>
              </span>
              <span className="row-version">{draft.document.capability}</span>
              <span className="app-lifecycle-status">
                <span className={`status status-${result.tone}`}>
                  {result.label} · v{result.version}
                </span>
                <small>Draft r{draft.revision}</small>
              </span>
              <ArrowUpRight className="row-arrow" />
            </button>
          );
        })}
      </div>
      {drafts.length > 0 &&
        filter !== 'all' &&
        state.data &&
        !drafts.some((draft) => {
          try {
            const label = appLifecycleStatus(draft, state.data!).label;
            return filter === 'published'
              ? label === 'Dipublikasikan'
              : label === 'Draft';
          } catch {
            return false;
          }
        }) && (
          <p className="dev-empty-filter">
            Tidak ada aplikasi dengan status ini.
          </p>
        )}
    </>
  );
}
