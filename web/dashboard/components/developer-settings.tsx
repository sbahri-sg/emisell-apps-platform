'use client';

import { lazy, Suspense, useState } from 'react';
import { ArrowUpRight, ChevronDown } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible';
import { Skeleton } from '@/components/ui/skeleton';
import ApplicationCredentials from './application-credentials';
import ApplicationContact from './application-contact';
import type { Draft, PortalAPI, Session } from '@/lib/portal';
const AppClients = lazy(() => import('./app-clients'));

export default function DeveloperSettings({
  app,
  api,
  session,
  busy,
  perform,
  edit,
  navigate,
}: {
  app: Draft;
  api: PortalAPI;
  session: Session;
  busy: boolean;
  perform: (fn: () => Promise<void>, notice?: string) => Promise<void>;
  edit: () => void;
  navigate: (view: string) => void;
}) {
  const [advanced, setAdvanced] = useState(false);
  return (
    <section className="dev-settings-page">
      <header className="page-heading">
        <h1>Settings</h1>
      </header>
      <ApplicationCredentials
        key={app.id}
        appId={app.id}
        api={api}
        busy={busy}
      />
      <ApplicationContact
        key={app.id}
        appId={app.id}
        api={api}
        accountEmail={session.user.email}
      />
      <section className="dev-settings-card">
        <div className="dev-settings-card-heading">
          <h2>Identitas aplikasi</h2>
          <Button variant="ghost" disabled={busy} onClick={edit}>
            Edit
          </Button>
        </div>
        <dl className="dev-settings-facts">
          <div>
            <dt>Nama aplikasi</dt>
            <dd>{app.document.name}</dd>
          </div>
          <div>
            <dt>App ID</dt>
            <dd>
              <code>{app.id}</code>
            </dd>
          </div>
        </dl>
      </section>
      <section className="dev-settings-card">
        <div className="dev-settings-card-heading">
          <h2>Konfigurasi versi</h2>
          <Button
            variant="ghost"
            disabled={busy}
            onClick={() => navigate('versions')}
          >
            Versions <ArrowUpRight />
          </Button>
        </div>
        <dl className="dev-settings-facts">
          <div>
            <dt>Versi draft</dt>
            <dd>{app.document.version}</dd>
          </div>
          <div>
            <dt>Endpoint draft</dt>
            <dd>{app.document.endpoint || 'Belum dikonfigurasi'}</dd>
          </div>
        </dl>
        <p className="dev-settings-help">
          Credential aplikasi tetap sama saat versi atau endpoint diubah. Akses
          data toko memerlukan instalasi dan izin seller.
        </p>
      </section>
      <Collapsible open={advanced} onOpenChange={setAdvanced}>
        <CollapsibleTrigger render={<Button variant="ghost" />}>
          <ChevronDown />
          Kompatibilitas client rilis lama
        </CollapsibleTrigger>
        <CollapsibleContent className="dev-settings-advanced">
          <p className="dev-settings-help">
            Khusus integrasi lama yang masih memakai client terikat rilis.
            Credential utama aplikasi ada di bagian atas.
          </p>
          {advanced && (
            <Suspense fallback={<Skeleton className="h-40 w-full" />}>
              <AppClients
                api={api}
                app={app}
                session={session}
                busy={busy}
                perform={perform}
                embedded
              />
            </Suspense>
          )}
        </CollapsibleContent>
      </Collapsible>
    </section>
  );
}
