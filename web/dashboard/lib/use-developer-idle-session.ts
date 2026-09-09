'use client';

import { useEffect } from 'react';

// The backend owns expiry. Passive tabs read the shared deadline without extending
// it or revoking a session still used by another tab. No merchant cookies are touched.
export function useDeveloperIdleSession(enabled: boolean) {
  useEffect(() => {
    if (!enabled) return;
    let disposed = false;
    let pending = false;
    let lastRenewal = 0;
    let timer: ReturnType<typeof setTimeout>;
    async function check(renew = false) {
      if (pending || disposed) return;
      pending = true;
      try {
        const response = await fetch('/api/v1/developer/activity', {
          method: renew ? 'POST' : 'GET',
          credentials: 'same-origin',
          cache: 'no-store',
          ...(renew
            ? { headers: { 'Content-Type': 'application/json' }, body: '{}' }
            : {}),
          signal: AbortSignal.timeout(12000),
        });
        if (disposed) return;
        if (response.status === 401) {
          window.location.replace('/?session=expired');
          return;
        }
        if (!response.ok) throw new Error('Session check unavailable');
        const { expiresAt } = (await response.json()) as { expiresAt: string };
        const delay = Date.parse(expiresAt) - Date.now();
        if (!Number.isFinite(delay)) throw new Error('Invalid expiry');
        if (renew) lastRenewal = Date.now();
        clearTimeout(timer);
        timer = setTimeout(() => void check(), Math.max(1000, delay + 100));
      } catch {
        // A transient connection failure is not proof of logout.
        if (!disposed) timer = setTimeout(() => void check(), 15000);
      } finally {
        pending = false;
      }
    }
    function activity(event: Event) {
      if (
        event.isTrusted &&
        document.visibilityState === 'visible' &&
        Date.now() - lastRenewal >= 15000
      )
        void check(true);
    }
    function visibility() {
      if (document.visibilityState === 'visible') void check();
    }
    const events = [
      'pointerdown',
      'pointermove',
      'keydown',
      'wheel',
      'touchstart',
    ];
    events.forEach((name) =>
      window.addEventListener(name, activity, { passive: true }),
    );
    document.addEventListener('visibilitychange', visibility);
    void check();
    return () => {
      disposed = true;
      clearTimeout(timer);
      events.forEach((name) => window.removeEventListener(name, activity));
      document.removeEventListener('visibilitychange', visibility);
    };
  }, [enabled]);
}
