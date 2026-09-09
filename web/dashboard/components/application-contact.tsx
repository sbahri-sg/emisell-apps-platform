'use client';

import { useEffect, useState } from 'react';
import { Pencil } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from '@/components/ui/dialog';
import { PortalError, type PortalAPI } from '@/lib/portal';

type Contact = { appId: string; email: string; revision: number };

export default function ApplicationContact({
  appId,
  api,
  accountEmail,
}: {
  appId: string;
  api: PortalAPI;
  accountEmail: string;
}) {
  const [contact, setContact] = useState<Contact | null>(null);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [attempt, setAttempt] = useState(0);
  const [open, setOpen] = useState(false);
  const [email, setEmail] = useState('');
  const [saving, setSaving] = useState(false);
  useEffect(() => {
    let alive = true;
    api
      .request<{ contact: Contact }>(`/apps/${appId}/contact`)
      .then((result) => {
        if (alive && result.contact.appId === appId) {
          setContact(result.contact);
          setError('');
        }
      })
      .catch(() => {
        if (alive) setError('Email kontak belum dapat dimuat.');
      });
    return () => {
      alive = false;
    };
  }, [api, appId, attempt]);
  async function save(event: React.SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!contact || saving) return;
    setSaving(true);
    setError('');
    try {
      const result = await api.request<{ contact: Contact }>(
        `/apps/${appId}/contact`,
        'PUT',
        { email: email.trim(), revision: contact.revision },
      );
      if (result.contact.appId !== appId) throw new Error();
      setContact(result.contact);
      setOpen(false);
      setNotice('Email kontak disimpan.');
    } catch (error) {
      if (error instanceof PortalError && error.status === 409) {
        setOpen(false);
        setAttempt((v) => v + 1);
        setNotice(
          'Kontak diubah di sesi lain. Periksa nilai terbaru sebelum mengedit kembali.',
        );
      } else
        setError(
          'Email belum tersimpan. Periksa alamat email, lalu coba lagi.',
        );
    } finally {
      setSaving(false);
    }
  }
  return (
    <section className="dev-settings-card dev-contact-card">
      <div className="dev-settings-card-heading">
        <h2>Contact information</h2>
      </div>
      <dl className="dev-contact-row">
        <dt>API contact email</dt>
        <dd>{contact ? contact.email || 'Belum diatur' : 'Memuat…'}</dd>
        <dd>
          <Button
            variant="ghost"
            disabled={!contact}
            onClick={() => {
              setEmail(contact?.email || accountEmail);
              setError('');
              setOpen(true);
            }}
          >
            <Pencil /> Edit
          </Button>
        </dd>
      </dl>
      {error && !open && (
        <p role="alert">
          {error}{' '}
          <Button variant="ghost" onClick={() => setAttempt((v) => v + 1)}>
            Coba lagi
          </Button>
        </p>
      )}
      {notice && <output className="dev-settings-help">{notice}</output>}
      <Dialog
        open={open}
        onOpenChange={(value) => {
          if (!saving) setOpen(value);
        }}
      >
        <DialogContent className="developer-contact-dialog">
          <DialogHeader>
            <DialogTitle>Edit API contact email</DialogTitle>
            <DialogDescription>
              Email untuk menghubungi pengembang aplikasi. Tidak mengubah email
              login atau kepemilikan aplikasi.
            </DialogDescription>
          </DialogHeader>
          <form onSubmit={(event) => void save(event)}>
            <label htmlFor="app-contact-email">API contact email</label>
            <Input
              id="app-contact-email"
              type="email"
              required
              maxLength={254}
              value={email}
              onChange={(event) => setEmail(event.target.value)}
              disabled={saving}
              autoComplete="email"
            />
            {error && <p role="alert">{error}</p>}
            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                disabled={saving}
                onClick={() => setOpen(false)}
              >
                Batal
              </Button>
              <Button type="submit" disabled={saving}>
                {saving ? 'Menyimpan…' : 'Simpan'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </section>
  );
}
