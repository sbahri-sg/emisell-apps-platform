'use client';

import { useEffect, useState } from 'react';
import { ArrowUpRight, FileCode2, RefreshCw } from 'lucide-react';
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
}: {
  api: PortalAPI;
  drafts: Draft[];
  busy: boolean;
  openDraft: (id: string) => void;
}) {
  const [attempt, setAttempt] = useState(0);
  const [state, setState] = useState<{
    data: AppLifecycle | null;
    failed: boolean;
  }>({ data: null, failed: false });
  useEffect(() => {
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
  }, [api, attempt]);
  return (
    <>
      <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
        <output className="text-sm text-muted-foreground">
          {state.failed
            ? 'Status rilis belum dapat dimuat. Coba perbarui status.'
            : 'Status rilis terpisah dari revisi draft. Testing bukan publikasi App Store.'}
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
    </>
  );
}
