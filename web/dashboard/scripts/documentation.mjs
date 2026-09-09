import { readFileSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { marked } from 'marked';

const root = new URL('../content/docs/', import.meta.url);
const output = new URL('../lib/documentation.generated.json', import.meta.url);

export function safeDocumentationLink(href) {
  if (
    typeof href !== 'string' ||
    /[\s\\]/.test(href) ||
    Array.from(href).some(
      (character) =>
        character.charCodeAt(0) < 32 || character.charCodeAt(0) === 127,
    )
  )
    return false;
  if (/^#[a-z0-9-]+$/.test(href)) return true;
  if (
    /^\/(?:docs\/[a-z0-9-]+(?:#[a-z0-9-]+)?|development(?:\?view=[a-z-]+)?|)$/.test(
      href,
    )
  )
    return true;
  try {
    const url = new URL(href);
    return url.protocol === 'https:' && !url.username && !url.password;
  } catch {
    return false;
  }
}

export function compileMarkdown(markdown) {
  const headings = [];
  const links = [];
  const ids = new Set();
  const convert = (tokens) =>
    tokens.flatMap((token) => {
      const children = () => convert(token.tokens ?? []);
      switch (token.type) {
        case 'space':
        case 'def':
          return [];
        case 'text':
        case 'escape':
          return token.tokens
            ? children()
            : [{ type: 'text', text: token.text }];
        case 'paragraph':
        case 'strong':
        case 'em':
        case 'del':
        case 'blockquote':
          return [{ type: token.type, children: children() }];
        case 'codespan':
          return [{ type: 'codespan', text: token.text }];
        case 'code':
          return [
            { type: 'code', text: token.text, language: token.lang || 'text' },
          ];
        case 'br':
        case 'hr':
          return [{ type: token.type }];
        case 'heading': {
          if (![2, 3].includes(token.depth))
            throw new Error(
              'Use only ## and ### headings; title belongs in navigation.json.',
            );
          const title = token.text.replace(/[`*_]/g, '');
          const base =
            title
              .toLowerCase()
              .normalize('NFKD')
              .replace(/[^a-z0-9]+/g, '-')
              .replace(/^-|-$/g, '') || 'section';
          let id = base,
            index = 2;
          while (ids.has(id)) id = `${base}-${index++}`;
          ids.add(id);
          headings.push({ id, title, depth: token.depth });
          return [
            { type: 'heading', id, depth: token.depth, children: children() },
          ];
        }
        case 'link':
          if (!safeDocumentationLink(token.href))
            throw new Error(`Unsupported link: ${token.href}`);
          links.push(token.href);
          return [{ type: 'link', href: token.href, children: children() }];
        case 'list':
          if (token.items.some((item) => item.task))
            throw new Error('Task checkboxes are not supported.');
          return [
            {
              type: 'list',
              ordered: token.ordered,
              start: token.start || 1,
              items: token.items.map((item) => convert(item.tokens)),
            },
          ];
        case 'table':
          return [
            {
              type: 'table',
              header: token.header.map((cell) => convert(cell.tokens)),
              rows: token.rows.map((row) =>
                row.map((cell) => convert(cell.tokens)),
              ),
            },
          ];
        default:
          throw new Error(
            `Unsupported Markdown: ${token.type}. Raw HTML, JSX and images are not permitted.`,
          );
      }
    });
  const nodes = convert(marked.lexer(markdown, { gfm: true }));
  return { nodes, headings, links };
}

export function generateDocumentation(check = false) {
  const navigation = JSON.parse(
    readFileSync(new URL('navigation.json', root), 'utf8'),
  );
  const seen = new Set();
  const articles = navigation.map((entry) => {
    if (!/^[a-z][a-z0-9-]*$/.test(entry.id) || seen.has(entry.id))
      throw new Error('Invalid or duplicate article ID.');
    seen.add(entry.id);
    for (const field of ['title', 'summary', 'category'])
      if (typeof entry[field] !== 'string' || !entry[field].trim())
        throw new Error(`Missing ${field}: ${entry.id}`);
    const markdown = readFileSync(new URL(`${entry.id}.md`, root), 'utf8');
    const compiled = compileMarkdown(markdown);
    if (!compiled.headings.length)
      throw new Error(`Article has no headings: ${entry.id}`);
    return {
      ...entry,
      ...compiled,
      searchText: markdown
        .replace(/[`#*|>]/g, ' ')
        .replace(/\s+/g, ' ')
        .trim(),
      minutes: Math.max(1, Math.ceil(markdown.split(/\s+/).length / 180)),
    };
  });
  for (const article of articles) {
    for (const href of article.links) {
      const match = href.match(/^\/docs\/([a-z0-9-]+)(?:#(.+))?$/);
      const target = match
        ? articles.find(({ id }) => id === match[1])
        : href.startsWith('#')
          ? article
          : null;
      const anchor = match?.[2] ?? (href.startsWith('#') ? href.slice(1) : '');
      if (
        (match && !target) ||
        (anchor && !target?.headings.some(({ id }) => id === anchor))
      )
        throw new Error(`Broken link in ${article.id}: ${href}`);
    }
  }
  const data =
    JSON.stringify(
      articles.map(({ links: _links, ...article }) => article),
      null,
      2,
    ) + '\n';
  const existing = (() => {
    try {
      return readFileSync(output, 'utf8');
    } catch {
      return '';
    }
  })();
  if (check && data !== existing)
    throw new Error('Documentation is stale. Run npm run docs:content.');
  if (!check && data !== existing) writeFileSync(output, data);
  return articles.length;
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  console.log(
    `${generateDocumentation(process.argv.includes('--check'))} public documentation articles validated.`,
  );
}
