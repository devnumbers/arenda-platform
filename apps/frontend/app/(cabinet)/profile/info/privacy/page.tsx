import type { Metadata } from 'next';
import fs from 'node:fs';
import path from 'node:path';
import { IconLink } from '@/shared/ui/icon-link';
import { ArrowLeft } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { MarkdownContent } from '@/shared/ui/markdown-content';
import styles from './page.module.css';

export const metadata: Metadata = {
  title: 'Политика конфиденциальности — Arenda Platform',
  description: 'Политика обработки персональных данных',
};

export default function PrivacyPage() {
  const content = fs.readFileSync(
    path.join(process.cwd(), 'content', 'privacy.md'),
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
          <h1 className={styles.title}>Политика конфиденциальности</h1>
        </header>
        <section className={styles.section}>
          <MarkdownContent>{content}</MarkdownContent>
        </section>
      </div>
    </div>
  );
}
