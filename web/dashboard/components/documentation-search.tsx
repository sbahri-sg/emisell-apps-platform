'use client';
import { useEffect, useRef, useState } from 'react';
import Link from 'next/link';
import { Search, X } from 'lucide-react';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { searchDocumentation } from '@/lib/documentation';

export default function DocumentationSearch() {
  const [query, setQuery] = useState('');
  const [open, setOpen] = useState(false);
  const input = useRef<HTMLInputElement>(null);
  useEffect(() => {
    if (!open) return;
    const escape = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      input.current?.focus();
      setOpen(false);
    };
    document.addEventListener('keydown', escape);
    return () => document.removeEventListener('keydown', escape);
  }, [open]);
  const results = searchDocumentation(query);
  return (
    <search className="docs-search">
      <form
        onSubmit={(event) => {
          event.preventDefault();
          setOpen(true);
        }}
      >
        <Search aria-hidden="true" />
        <Input
          ref={input}
          type="search"
          aria-label="Cari dokumentasi"
          aria-controls="docs-search-results"
          placeholder="Cari panduan, scope, atau perintah…"
          value={query}
          maxLength={200}
          onChange={(event) => {
            setQuery(event.target.value);
            setOpen(true);
          }}
          onFocus={() => setOpen(true)}
        />
        {open && query.trim() && (
          <section
            className="docs-search-results"
            id="docs-search-results"
            aria-label="Hasil pencarian"
          >
            <div className="docs-search-heading">
              <output>
                {results.length
                  ? `${results.length} hasil ditampilkan`
                  : 'Tidak ada hasil'}
              </output>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                aria-label="Tutup hasil pencarian"
                onClick={() => {
                  input.current?.focus();
                  setOpen(false);
                }}
              >
                <X />
              </Button>
            </div>
            {results.length ? (
              <ul>
                {results.map((result) => (
                  <li key={result.id}>
                    <Link href={result.href} onClick={() => setOpen(false)}>
                      <small>{result.category}</small>
                      <strong>{result.title}</strong>
                      <span>{result.summary}</span>
                    </Link>
                  </li>
                ))}
              </ul>
            ) : (
              <p>Coba kata lain, misalnya “produk”, “secret”, atau “Docker”.</p>
            )}
          </section>
        )}
      </form>
    </search>
  );
}
