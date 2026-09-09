'use client';
/* oxlint-disable jsx-a11y/no-noninteractive-tabindex -- Long code lines must be horizontally scrollable by keyboard. */
import { useState } from 'react';
import { Check, Copy } from 'lucide-react';
import { Button } from '@/components/ui/button';

export default function DocumentationCode({
  text,
  language = 'text',
}: {
  text: string;
  language?: string;
}) {
  const [message, setMessage] = useState('');
  return (
    <div className="docs-code-block">
      <div className="docs-code-heading">
        <span>{language}</span>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          onClick={async () => {
            try {
              await navigator.clipboard.writeText(text);
              setMessage('Kode disalin');
            } catch {
              setMessage(
                'Tidak dapat menyalin. Pilih teks kode untuk menyalin manual.',
              );
            }
          }}
        >
          {message === 'Kode disalin' ? <Check /> : <Copy />}Salin
        </Button>
      </div>
      <pre
        className="docs-code"
        tabIndex={0}
        aria-label={`Contoh kode ${language}`}
      >
        <code>{text}</code>
      </pre>
      <output className="docs-copy-status">{message}</output>
    </div>
  );
}
