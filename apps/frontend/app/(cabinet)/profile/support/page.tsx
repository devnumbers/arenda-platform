import type { Metadata } from 'next';
import fs from 'node:fs';
import path from 'node:path';
import { PageHeader } from '@/shared/ui/page-header';
import { PageShell } from '@/shared/ui/page-shell';
import { ROUTES } from '@/shared/config/routes';
import { MarkdownContent } from '@/shared/ui/markdown-content';
import styles from './page.module.css';

export const metadata: Metadata = {
  title: 'Поддержка — Arenda Platform',
  description: 'Контакты службы поддержки',
};

export default function SupportPage() {
  const content = fs.readFileSync(
    path.join(process.cwd(), 'content', 'support.md'),
    'utf-8'
  );

  return (
    <PageShell>
      <PageHeader title="Поддержка" backHref={ROUTES.profile} />
      <section className={styles.section}>
        <MarkdownContent>{content}</MarkdownContent>
      </section>
    </PageShell>
  );
}
