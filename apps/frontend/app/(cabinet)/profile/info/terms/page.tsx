import type { Metadata } from 'next';
import fs from 'node:fs';
import path from 'node:path';
import { IconLink } from '@/shared/ui/icon-link';
import { ArrowLeft } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { MarkdownContent } from '@/shared/ui/markdown-content';
import styles from './page.module.css';

export const metadata: Metadata = {
  title: 'Пользовательское соглашение — Arenda Platform',
  description: 'Условия использования платформы',
};

export default function TermsPage() {
  const content = fs.readFileSync(
    path.join(process.cwd(), 'content', 'terms.md'),
    'utf-8'
  );

  return (
    <div className={styles.root}>
      <div className={styles.content}>
        <header className={styles.header}>
          <IconLink
            href={ROUTES.profileInfo}
            aria-label="Назад"
            icon={<ArrowLeft />}
          />
          <h1 className={styles.title}>Пользовательское соглашение</h1>
        </header>
        <section className={styles.section}>
          <MarkdownContent>{content}</MarkdownContent>
        </section>
      </div>
    </div>
  );
}
