'use client';

import Link from 'next/link';

import { useState, type ReactNode } from 'react';
import {
  ArrowRight,
  Check,
  Clipboard,
  ExternalLink,
  ImagePlus,
  ListFilter,
  Pencil,
  Search,
  X,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  NativeSelect,
  NativeSelectOption,
} from '@/components/ui/native-select';
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group';
import { Checkbox } from '@/components/ui/checkbox';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog';
import {
  catalogPreview,
  catalogPreviewCode,
  initialCatalogConfiguration,
  type CatalogConfiguration,
} from '@/lib/catalog-preview';

function FilterRow({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  return (
    <div className="catalog-filter-row">
      <span>{label}</span>
      <div>{children}</div>
    </div>
  );
}
function HighlightJSON({ code }: { code: string }) {
  return (
    <>
      {code
        .split(
          /("(?:[^"\\]|\\.)*"\s*:|"(?:[^"\\]|\\.)*"|\b(?:true|false|null)\b|\b\d+(?:\.\d+)?\b)/g,
        )
        .map((token, index) => {
          const kind = token.startsWith('"')
            ? token.trimEnd().endsWith(':')
              ? 'key'
              : 'string'
            : /^(true|false|null|\d)/.test(token)
              ? 'literal'
              : '';
          return (
            <span
              key={index}
              className={kind ? `catalog-code-${kind}` : undefined}
            >
              {token}
            </span>
          );
        })}
    </>
  );
}

export default function DeveloperCatalogs() {
  const [configuration, setConfiguration] = useState({
    ...initialCatalogConfiguration,
  });
  const [query, setQuery] = useState('');
  const [title, setTitle] = useState('catalog-example');
  const [draftTitle, setDraftTitle] = useState(title);
  const [editing, setEditing] = useState(false);
  const [deleted, setDeleted] = useState(false);
  const [tab, setTab] = useState('request');
  const [format, setFormat] = useState('json');
  const [message, setMessage] = useState('');
  const [copied, setCopied] = useState(false);
  const preview = catalogPreview(query, title, configuration);
  const code = catalogPreviewCode(preview, tab, format);
  const change = <K extends keyof CatalogConfiguration>(
    field: K,
    value: CatalogConfiguration[K],
  ) => {
    setConfiguration((previous) => ({ ...previous, [field]: value }));
    setCopied(false);
  };
  const select = (
    field: keyof CatalogConfiguration,
    label: string,
    placeholder: string,
    options: string[],
  ) => (
    <NativeSelect
      aria-label={label}
      value={String(configuration[field])}
      onChange={(event) => change(field, event.target.value)}
    >
      <NativeSelectOption value="">{placeholder}</NativeSelectOption>
      {options.map((option) => (
        <NativeSelectOption key={option} value={option}>
          {option}
        </NativeSelectOption>
      ))}
    </NativeSelect>
  );
  return (
    <section
      className="catalog-playground"
      aria-label="Catalogs — Mock / Contoh"
    >
      <aside className="catalog-configuration-column">
        <section
          className="catalog-surface catalog-configuration"
          aria-labelledby="catalog-configuration-title"
        >
          <h1 id="catalog-configuration-title">Configuration</h1>
          <div className="catalog-picker">
            <NativeSelect
              aria-label="Catalog"
              disabled={deleted}
              value={deleted ? '' : 'example'}
            >
              <NativeSelectOption value={deleted ? '' : 'example'}>
                {deleted ? 'No catalog' : title}
              </NativeSelectOption>
            </NativeSelect>
            <Button
              variant="outline"
              size="icon"
              disabled={deleted}
              aria-label="Edit catalog title"
              onClick={() => {
                setDraftTitle(title);
                setEditing(true);
              }}
            >
              <Pencil />
            </Button>
          </div>
          <fieldset disabled={deleted} className="catalog-filter-set">
            <legend>SOURCE</legend>
            <RadioGroup
              aria-label="Source"
              value={configuration.source}
              onValueChange={(value) => change('source', String(value))}
            >
              <label
                htmlFor="catalog-source-own"
                className={
                  configuration.source === 'own'
                    ? 'catalog-source-selected'
                    : ''
                }
              >
                <RadioGroupItem id="catalog-source-own" value="own" />
                All my stores
              </label>
              <label
                htmlFor="catalog-source-specific"
                className={
                  configuration.source === 'specific'
                    ? 'catalog-source-selected'
                    : ''
                }
              >
                <RadioGroupItem id="catalog-source-specific" value="specific" />
                Specific stores
              </label>
            </RadioGroup>
            {configuration.source === 'specific' && (
              <NativeSelect
                aria-label="Specific store (mock)"
                value={configuration.store}
                onChange={(event) => change('store', event.target.value)}
              >
                <NativeSelectOption value="store_example">
                  Example store · Mock
                </NativeSelectOption>
              </NativeSelect>
            )}
          </fieldset>
          <fieldset disabled={deleted} className="catalog-filter-set">
            <legend>QUERY</legend>
            <FilterRow label="Prefix">
              <Input
                aria-label="Prefix"
                placeholder="e.g. retail"
                value={configuration.prefix}
                maxLength={100}
                onChange={(event) => change('prefix', event.target.value)}
              />
            </FilterRow>
            <FilterRow label="Limit">
              <Input
                aria-label="Limit"
                type="number"
                min={1}
                max={100}
                value={configuration.limit}
                onChange={(event) =>
                  change(
                    'limit',
                    Math.max(1, Math.min(100, Number(event.target.value) || 1)),
                  )
                }
              />
            </FilterRow>
          </fieldset>
          <fieldset disabled={deleted} className="catalog-filter-set">
            <legend>REGION</legend>
            <FilterRow label="Buyer country">
              {select('buyerCountry', 'Buyer country', '🌐 Anywhere', [
                'Indonesia',
                'Singapore',
                'Malaysia',
              ])}
            </FilterRow>
            <FilterRow label="Ships to">
              {select('shipsTo', 'Ships to', '🌐 Anywhere', [
                'Indonesia',
                'Singapore',
                'Malaysia',
              ])}
            </FilterRow>
            <FilterRow label="Ships from">
              {select('shipsFrom', 'Ships from', 'Anywhere', [
                'Indonesia',
                'Singapore',
                'Malaysia',
              ])}
            </FilterRow>
          </fieldset>
          <fieldset disabled={deleted} className="catalog-filter-set">
            <legend>ATTRIBUTES</legend>
            <FilterRow label="Categories">
              {select('categories', 'Categories', 'Any category', [
                'Apparel',
                'Electronics',
                'Home & garden',
                'Sporting goods',
              ])}
            </FilterRow>
            <FilterRow label="Color">
              {select('color', 'Color', 'Any color', [
                'Black',
                'White',
                'Blue',
                'Red',
              ])}
            </FilterRow>
            <FilterRow label="Gender">
              {select('gender', 'Gender', 'Any gender', [
                'Unisex',
                'Women',
                'Men',
              ])}
            </FilterRow>
            <FilterRow label="Size">
              {select('size', 'Size', 'Any size', ['S', 'M', 'L', 'XL'])}
            </FilterRow>
            <FilterRow label="Condition">
              {select('condition', 'Condition', 'Any condition', [
                'New',
                'Used',
                'Refurbished',
              ])}
            </FilterRow>
          </fieldset>
          <fieldset disabled={deleted} className="catalog-filter-set">
            <legend>LISTING</legend>
            <FilterRow label="Options">
              <label
                className="catalog-stock-option"
                htmlFor="catalog-in-stock"
              >
                <Checkbox
                  id="catalog-in-stock"
                  checked={configuration.inStock}
                  onCheckedChange={(value) => change('inStock', Boolean(value))}
                />
                In stock only
              </label>
            </FilterRow>
            <FilterRow label="Price">
              <div className="catalog-price-range">
                <Input
                  aria-label="Minimum price"
                  type="number"
                  min={0}
                  value={configuration.minPrice}
                  onChange={(event) => change('minPrice', event.target.value)}
                />
                <Input
                  aria-label="Maximum price"
                  type="number"
                  min={0}
                  placeholder="No max"
                  value={configuration.maxPrice}
                  onChange={(event) => change('maxPrice', event.target.value)}
                />
              </div>
            </FilterRow>
            <FilterRow label="Price tier">
              {select('priceTier', 'Price tier', 'Any tier', [
                'Budget',
                'Mid-range',
                'Premium',
              ])}
            </FilterRow>
            <FilterRow label="Min rating">
              {select('minRating', 'Min rating', 'Any rating', [
                '1',
                '2',
                '3',
                '4',
                '5',
              ])}
            </FilterRow>
            <FilterRow label="Min reviews">
              <Input
                aria-label="Min reviews"
                type="number"
                min={0}
                placeholder="Any count"
                value={configuration.minReviews}
                onChange={(event) => change('minReviews', event.target.value)}
              />
            </FilterRow>
          </fieldset>
          <div className="catalog-configuration-footer">
            <Button
              variant="outline"
              className="catalog-delete"
              disabled={deleted}
              onClick={() => {
                setDeleted(true);
                setMessage(
                  'Katalog contoh disembunyikan. Tidak ada data backend yang dihapus.',
                );
              }}
            >
              Delete catalog
            </Button>
            <Link href="/docs/scopes">
              Docs <ExternalLink size={12} />
            </Link>
          </div>
        </section>
        <section className="catalog-surface catalog-api-key">
          <span>API KEY</span>
          <NativeSelect aria-label="API key (mock only)" value="mock">
            <NativeSelectOption value="mock">
              Default API Key · Mock
            </NativeSelectOption>
          </NativeSelect>
        </section>
      </aside>
      <section
        className="catalog-surface catalog-search-preview"
        aria-labelledby="catalog-preview-title"
      >
        <div className="catalog-preview-heading">
          <h2 id="catalog-preview-title">Search preview</h2>
          <span
            className="catalog-mock-label"
            title="Mock / Contoh — Tidak dikirim ke backend"
          >
            Mock · Tidak dikirim
          </span>
        </div>
        <form
          className="catalog-search-composer"
          onSubmit={(event) => {
            event.preventDefault();
            setTab('response');
            setCopied(false);
            setMessage(
              'Response contoh. Filter tambahan hanya ditampilkan dalam konfigurasi mock.',
            );
          }}
        >
          <Search size={16} />
          <Input
            aria-label="Query"
            placeholder="Search catalogs"
            value={query}
            disabled={deleted}
            maxLength={100}
            onChange={(event) => {
              setQuery(event.target.value);
              setCopied(false);
            }}
          />
          <Button
            type="button"
            variant="ghost"
            size="icon"
            aria-label="Clear search"
            disabled={!query || deleted}
            onClick={() => {
              setQuery('');
              setCopied(false);
            }}
          >
            <X />
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            aria-label="Attach image (not available in mock)"
            disabled
            title="Image search belum tersedia pada sample katalog"
          >
            <ImagePlus />
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            aria-label="Filters"
            onClick={() => {
              document
                .getElementById('catalog-configuration-title')
                ?.scrollIntoView({ behavior: 'smooth', block: 'start' });
              setMessage('Filter tersedia di panel Configuration.');
            }}
          >
            <ListFilter />
          </Button>
          <Button
            type="submit"
            size="icon"
            aria-label="Search sample"
            disabled={deleted}
          >
            <ArrowRight />
          </Button>
        </form>
        {deleted ? (
          <div className="catalog-deleted">
            <h3>No catalog selected</h3>
            <p>Katalog contoh telah disembunyikan dari preview ini.</p>
            <Button
              variant="outline"
              onClick={() => {
                setDeleted(false);
                setMessage('');
              }}
            >
              Restore example catalog
            </Button>
          </div>
        ) : (
          <Tabs
            className="catalog-inspector"
            value={tab}
            onValueChange={(value) => {
              setTab(String(value));
              setCopied(false);
            }}
          >
            <TabsList variant="line" aria-label="Catalog search inspector">
              <TabsTrigger value="request">Request</TabsTrigger>
              <TabsTrigger value="response">Response</TabsTrigger>
            </TabsList>
            {['request', 'response'].map((panel) => (
              <TabsContent key={panel} value={panel}>
                <Tabs
                  value={panel === 'response' ? 'json' : format}
                  onValueChange={(value) => {
                    setFormat(String(value));
                    setCopied(false);
                  }}
                >
                  <TabsList
                    variant="line"
                    aria-label={
                      panel === 'response'
                        ? 'Response format'
                        : 'Request format'
                    }
                  >
                    <TabsTrigger value="json">JSON</TabsTrigger>
                    {panel === 'request' && (
                      <>
                        <TabsTrigger value="curl">cURL</TabsTrigger>
                        <TabsTrigger value="js">JS</TabsTrigger>
                      </>
                    )}
                  </TabsList>
                  {(panel === 'response'
                    ? ['json']
                    : ['json', 'curl', 'js']
                  ).map((language) => (
                    <TabsContent key={language} value={language}>
                      <div className="catalog-code-box">
                        <Button
                          variant="ghost"
                          size="icon"
                          className="catalog-code-copy"
                          aria-label="Copy code to clipboard"
                          onClick={() => {
                            if (!navigator.clipboard) {
                              setMessage(
                                'Pilih dan salin teks contoh secara manual.',
                              );
                              return;
                            }
                            void navigator.clipboard
                              .writeText(code)
                              .then(() => {
                                setCopied(true);
                                setMessage('Contoh tersalin.');
                              })
                              .catch(() =>
                                setMessage(
                                  'Pilih dan salin teks contoh secara manual.',
                                ),
                              );
                          }}
                        >
                          {copied ? <Check /> : <Clipboard />}
                        </Button>
                        <pre aria-label={`${panel} ${language} example`}>
                          <code>
                            {language === 'json' ? (
                              <HighlightJSON code={code} />
                            ) : (
                              code
                            )}
                          </code>
                        </pre>
                      </div>
                    </TabsContent>
                  ))}
                </Tabs>
              </TabsContent>
            ))}
          </Tabs>
        )}
        <p className="catalog-sample-explanation">
          Sample saja. Katalog, filter, dan API key tidak terhubung ke backend.
          Endpoint contoh tetap <code>GET /v1/catalogs</code>; region,
          attributes, dan listing belum menjadi parameter API.
        </p>
        <output className="catalog-preview-status">{message}</output>
      </section>
      <Dialog open={editing} onOpenChange={setEditing}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Edit catalog title</DialogTitle>
            <DialogDescription>
              Ubah nama katalog contoh di preview ini saja.
            </DialogDescription>
          </DialogHeader>
          <form
            onSubmit={(event) => {
              event.preventDefault();
              if (draftTitle.trim()) {
                setTitle(draftTitle.trim());
                setEditing(false);
                setCopied(false);
              }
            }}
          >
            <Input
              aria-label="Catalog title"
              value={draftTitle}
              maxLength={100}
              required
              onChange={(event) => setDraftTitle(event.target.value)}
            />
            <Button type="submit">Save</Button>
          </form>
        </DialogContent>
      </Dialog>
    </section>
  );
}
