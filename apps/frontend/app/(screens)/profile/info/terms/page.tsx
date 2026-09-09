import type { Metadata } from 'next';
import fs from 'node:fs';
import path from 'node:path';
import { PageContent, TopNav, TopNavBackButton, TopNavTitle } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { MarkdownContent } from '@/shared/ui/markdown-content';
import styles from './page.module.css';

export const metadata: Metadata = {
  title: 'Пользовательское соглашение — Рентли',
  description: 'Условия использования платформы',
};

export default function TermsPage() {
  const content = fs.readFileSync(
    path.join(process.cwd(), 'content', 'terms.md'),
    'utf-8'
  );

  return (
    <>
      <TopNav leading={<TopNavBackButton fallbackHref={ROUTES.profileInfo} />}>
        <TopNavTitle title="Пользовательское соглашение" />
      </TopNav>
      <PageContent className="px-6">
        <section className={styles.section}>
          <MarkdownContent>{content}</MarkdownContent>
        </section>
      </PageContent>
    </>
  );
}
