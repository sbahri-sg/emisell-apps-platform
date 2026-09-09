import Link from 'next/link';
import {
  ArrowLeft,
  ArrowRight,
  BookOpen,
  Code2,
  KeyRound,
  Layers,
  Terminal,
  ArrowUpRight,
} from 'lucide-react';
import {
  adjacentArticles,
  documentationArticles,
  documentationCategories,
  documentationTopics,
  type DocArticle,
  type DocumentationTopic,
} from '@/lib/documentation';
import DocumentationSearch from '@/components/documentation-search';
import DocumentationArticle from '@/components/documentation-article';
import DocumentationCode from '@/components/documentation-code';

function Contents({ article }: { article: DocArticle }) {
  return (
    <nav aria-label="Daftar isi artikel">
      <ul>
        {article.headings.map(({ id, title, depth }) => (
          <li key={id} className={depth === 3 ? 'docs-toc-nested' : undefined}>
            <Link href={`#${id}`}>{title}</Link>
          </li>
        ))}
      </ul>
    </nav>
  );
}

export default function Documentation({
  topic,
}: {
  topic?: DocumentationTopic;
}) {
  const current = documentationArticles.find(({ id }) => id === topic);
  const { previous, next } = adjacentArticles(topic ?? '');
  return (
    <div className="docs-site">
      <Link className="docs-skip" href="#docs-main">
        Lewati ke konten
      </Link>
      <header className="docs-header">
        <Link className="docs-brand" href="/" aria-label="Emisell Docs beranda">
          <span aria-hidden="true">e</span>
          <strong>emisell</strong>
          <small>docs</small>
        </Link>
        <DocumentationSearch />
        <nav aria-label="Navigasi utama">
          <Link href="/docs/getting-started">Panduan</Link>
          <Link href="/docs/cli">CLI</Link>
          <Link className="docs-dashboard-link" href="/development">
            Login <ArrowUpRight />
          </Link>
        </nav>
      </header>
      <div className="docs-layout">
        <aside className="docs-sidebar">
          <nav aria-label="Dokumentasi">
            <Link href="/" aria-current={!topic ? 'page' : undefined}>
              <BookOpen />
              Ringkasan
            </Link>
            {documentationCategories.map((category) => (
              <div className="docs-nav-group" key={category}>
                <p>{category}</p>
                {documentationTopics
                  .filter((entry) => entry.category === category)
                  .map(({ id, title }) => (
                    <Link
                      key={id}
                      href={`/docs/${id}`}
                      aria-current={id === topic ? 'page' : undefined}
                    >
                      {title}
                    </Link>
                  ))}
              </div>
            ))}
            <div className="docs-sidebar-footer">
              <span>Kelola aplikasi Anda</span>
              <Link href="/development">
                Buka dashboard <ArrowUpRight />
              </Link>
            </div>
          </nav>
        </aside>
        <main id="docs-main" className="docs-main">
          {current ? (
            <div className="docs-reading-grid">
              <article className="docs-article">
                <div className="docs-breadcrumb">
                  <Link href="/">Dokumentasi</Link>
                  <span>/</span>
                  <span>{current.category}</span>
                </div>
                <h1>{current.title}</h1>
                <p className="docs-lead">{current.summary}</p>
                <p className="docs-reading-time">
                  {current.minutes} menit baca
                </p>
                <details className="docs-toc-mobile">
                  <summary>Di halaman ini</summary>
                  <Contents article={current} />
                </details>
                <div className="docs-prose">
                  <DocumentationArticle nodes={current.nodes} />
                </div>
                <footer
                  className="docs-article-pagination"
                  aria-label="Navigasi antarartikel"
                >
                  {previous ? (
                    <Link href={`/docs/${previous.id}`}>
                      <span>
                        <ArrowLeft />
                        Sebelumnya
                      </span>
                      <strong>{previous.title}</strong>
                    </Link>
                  ) : (
                    <span />
                  )}
                  {next && (
                    <Link href={`/docs/${next.id}`}>
                      <span>
                        Berikutnya <ArrowRight />
                      </span>
                      <strong>{next.title}</strong>
                    </Link>
                  )}
                </footer>
              </article>
              <aside className="docs-toc">
                <strong>Di halaman ini</strong>
                <Contents article={current} />
                <Link className="docs-toc-help" href="/docs/troubleshooting">
                  Butuh bantuan? <ArrowUpRight />
                </Link>
              </aside>
            </div>
          ) : (
            <>
              <div className="docs-intro">
                <span className="docs-eyebrow">EMISELL DEVELOPERS</span>
                <h1>
                  Bangun aplikasi
                  <br />
                  untuk Emisell.
                </h1>
                <p>
                  Panduan, CLI, dan referensi izin untuk menghubungkan aplikasi
                  Anda dengan toko Emisell.
                </p>
                <Link className="docs-primary" href="/docs/getting-started">
                  Buat aplikasi pertama <ArrowRight />
                </Link>
              </div>
              <section className="docs-cards" aria-label="Panduan pilihan">
                {[
                  {
                    id: 'getting-started',
                    title: 'Bangun aplikasi',
                    description:
                      'Mulai dengan React Router, TypeScript, dan Vite.',
                    icon: Code2,
                  },
                  {
                    id: 'scopes',
                    title: 'Hubungkan data toko',
                    description:
                      'Pahami scope produk, pesanan, dan resource lainnya.',
                    icon: Layers,
                  },
                  {
                    id: 'authentication',
                    title: 'Autentikasi aplikasi',
                    description:
                      'Kenali identitas, credential, dan persetujuan seller.',
                    icon: KeyRound,
                  },
                ].map(({ id, title, description, icon: Icon }) => (
                  <Link className="docs-card" href={`/docs/${id}`} key={id}>
                    <Icon className="docs-card-icon" />
                    <h2>{title}</h2>
                    <p>{description}</p>
                    <span>
                      Baca panduan <ArrowRight />
                    </span>
                  </Link>
                ))}
              </section>
              <section className="docs-quickstart">
                <div>
                  <Terminal />
                  <h2>Dari terminal ke aplikasi.</h2>
                  <p>
                    Mulai project dengan Emisell CLI, lalu jalankan development
                    server lokal.
                  </p>
                  <Link className="docs-inline-link" href="/docs/cli">
                    Referensi CLI <ArrowRight />
                  </Link>
                </div>
                <DocumentationCode
                  language="sh"
                  text={
                    'npm install -g @emisell/cli@latest\nemisell app init\n\n# Masuk ke folder project Anda\nnpm install\nemisell app dev'
                  }
                />
              </section>
              <section
                className="docs-topic-directory"
                aria-label="Semua panduan"
              >
                <h2>Jelajahi dokumentasi</h2>
                <div>
                  {documentationCategories.map((category) => (
                    <section key={category}>
                      <h3>{category}</h3>
                      {documentationTopics
                        .filter((entry) => entry.category === category)
                        .map((entry) => (
                          <Link href={`/docs/${entry.id}`} key={entry.id}>
                            {entry.title}
                            <ArrowRight />
                          </Link>
                        ))}
                    </section>
                  ))}
                </div>
              </section>
              <section className="docs-review">
                <div>
                  <h2>Siap mengajukan aplikasi?</h2>
                  <p>
                    Pahami proses review, penugasan toko, dan persetujuan seller
                    sebelum menguji integrasi.
                  </p>
                </div>
                <Link className="docs-inline-link" href="/docs/submissions">
                  Alur pengajuan <ArrowRight />
                </Link>
              </section>
            </>
          )}
          <footer className="docs-footer">
            <span>Emisell Apps Platform</span>
            <Link href="https://emisell.com">
              emisell.com <ArrowUpRight />
            </Link>
          </footer>
        </main>
      </div>
    </div>
  );
}
