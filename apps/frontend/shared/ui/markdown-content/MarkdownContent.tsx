import type { JSX } from 'react';
import ReactMarkdown from 'react-markdown';
import styles from './MarkdownContent.module.css';

export type MarkdownContentProps = {
  readonly children: string;
};

export function MarkdownContent({ children }: MarkdownContentProps): JSX.Element {
  return (
    <div className={styles.markdown}>
      <ReactMarkdown>{children}</ReactMarkdown>
    </div>
  );
}
