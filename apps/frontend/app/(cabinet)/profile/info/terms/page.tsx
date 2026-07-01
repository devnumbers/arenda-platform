import type { Metadata } from 'next';
import fs from 'node:fs';
import path from 'node:path';
import { PageHeader } from '@/shared/ui/page-header';
import { PageShell } from '@/shared/ui/page-shell';
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
    <PageShell>
      <PageHeader title="Пользовательское соглашение" backHref={ROUTES.profileInfo} />
      <section className={styles.section}>
        <MarkdownContent>{content}</MarkdownContent>
      </section>
    </PageShell>
  );
}
