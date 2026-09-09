/* oxlint-disable jsx-a11y/no-noninteractive-tabindex -- Scrollable tables need a keyboard focus target. */
import { Fragment } from 'react';
import Link from 'next/link';
import type { DocNode } from '@/lib/documentation';
import DocumentationCode from '@/components/documentation-code';

export default function DocumentationArticle({ nodes }: { nodes: DocNode[] }) {
  return nodes.map((node, index) => {
    const children = <DocumentationArticle nodes={node.children ?? []} />;
    switch (node.type) {
      case 'text':
        return <Fragment key={index}>{node.text}</Fragment>;
      case 'paragraph':
        return <p key={index}>{children}</p>;
      case 'strong':
        return <strong key={index}>{children}</strong>;
      case 'em':
        return <em key={index}>{children}</em>;
      case 'del':
        return <del key={index}>{children}</del>;
      case 'codespan':
        return <code key={index}>{node.text}</code>;
      case 'br':
        return <br key={index} />;
      case 'hr':
        return <hr key={index} />;
      case 'heading':
        return node.depth === 3 ? (
          <h3 id={node.id} key={index}>
            <Link href={`#${node.id}`}>{children}</Link>
          </h3>
        ) : (
          <h2 id={node.id} key={index}>
            <Link href={`#${node.id}`}>{children}</Link>
          </h2>
        );
      case 'link':
        return (
          <Link key={index} href={node.href!}>
            {children}
          </Link>
        );
      case 'blockquote':
        return (
          <blockquote className="docs-note" key={index}>
            {children}
          </blockquote>
        );
      case 'code':
        return (
          <DocumentationCode
            key={index}
            text={node.text ?? ''}
            language={node.language}
          />
        );
      case 'list': {
        const items = node.items?.map((item, n) => (
          <li key={n}>
            <DocumentationArticle nodes={item} />
          </li>
        ));
        return node.ordered ? (
          <ol key={index} start={node.start}>
            {items}
          </ol>
        ) : (
          <ul key={index}>{items}</ul>
        );
      }
      case 'table':
        return (
          <section
            className="docs-table-wrap"
            key={index}
            aria-label="Tabel referensi"
            tabIndex={0}
          >
            <table>
              <thead>
                <tr>
                  {node.header?.map((cell, n) => (
                    <th scope="col" key={n}>
                      <DocumentationArticle nodes={cell} />
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {node.rows?.map((row, n) => (
                  <tr key={n}>
                    {row.map((cell, c) => (
                      <td key={c}>
                        <DocumentationArticle nodes={cell} />
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </section>
        );
      default:
        return null;
    }
  });
}
