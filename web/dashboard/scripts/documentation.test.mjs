import assert from 'node:assert/strict';
import test from 'node:test';
import {
  compileMarkdown,
  generateDocumentation,
  safeDocumentationLink,
} from './documentation.mjs';

test('public Markdown compiles tables, nested formatting, code, and stable unique heading anchors', () => {
  const result = compileMarkdown(
    '## Mulai\n\n**Teks** dan `kode`.\n\n## Mulai\n\n### Detail\n\n| Izin | Data |\n| --- | --- |\n| `read_products` | Produk |\n\n```js\nconst text = "<script>";\n```\n',
  );
  assert.deepEqual(
    result.headings.map(({ id }) => id),
    ['mulai', 'mulai-2', 'detail'],
  );
  assert.ok(result.nodes.some(({ type }) => type === 'table'));
  assert.equal(result.nodes.at(-1).text, 'const text = "<script>";');
});

test('HTML, executable JSX, images and unsafe link schemes are rejected rather than rendered', () => {
  for (const markdown of [
    '<script>alert(1)</script>',
    '<iframe src="https://example.com" />',
    '![x](https://example.com/image.png)',
    '[x](javascript:alert)',
    '[x](data:text/html,test)',
    '[x](//evil.example)',
    '[x](/admin)',
    '# Hidden title',
  ]) {
    assert.throws(() => compileMarkdown(markdown), undefined, markdown);
  }
  for (const href of [
    'javascript:alert(1)',
    'data:text/html,test',
    '//evil.example',
    'https://user:secret@example.com',
    '/docs/../admin',
    '/docs/test\\bad',
  ])
    assert.equal(safeDocumentationLink(href), false);
  for (const href of [
    '/docs/authentication#cara-kerja',
    '/development?view=reviews',
    '#mulai',
    'https://emisell.com',
  ])
    assert.equal(safeDocumentationLink(href), true);
});

test('checked-in documentation is reproducible and every internal article/anchor link resolves', () => {
  assert.equal(generateDocumentation(true), 11);
});
