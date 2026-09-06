'use client';

import { useEffect, useState } from 'react';
import type { PortalAPI } from '@/lib/portal';
import { emptyReadiness, loadScopeReadiness } from '@/lib/scope-readiness';

export function useScopeReadiness(api: PortalAPI) {
  const [snapshot, setSnapshot] = useState(emptyReadiness);
  const [checking, setChecking] = useState(true);
  const [revision, setRevision] = useState(0);
  const refreshStatus = () => {
    setSnapshot((old) => ({
      ...old,
      verification: null,
      verificationError: '',
    }));
    setChecking(true);
    setRevision((r) => r + 1);
  };
  useEffect(() => {
    let current = true;
    void loadScopeReadiness(api).then((next) => {
      if (current) {
        setSnapshot(next);
        setChecking(false);
      }
    });
    return () => {
      current = false;
    };
  }, [api, revision]);
  return { ...snapshot, checking, refreshStatus };
}
