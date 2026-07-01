import type { Metadata } from 'next';
import NextLink from 'next/link';
import { Card } from '@heroui/react/card';
import { PageHeader } from '@/shared/ui/page-header';
import { PageShell } from '@/shared/ui/page-shell';
import { Icon } from '@/shared/ui/icon';
import { ArrowRight } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import styles from './page.module.css';

export const metadata: Metadata = {
  title: 'Информация — Arenda Platform',
  description: 'Правовая информация и документы',
};

type InfoItem = {
  title: string;
  href: string;
};

const infoItems: InfoItem[] = [
  {
    title: 'Политика конфиденциальности',
    href: ROUTES.profilePrivacy,
  },
  {
    title: 'Пользовательское соглашение',
    href: ROUTES.profileTerms,
  },
];

export default function InfoPage() {
  return (
    <PageShell>
      <PageHeader title="Информация" backHref={ROUTES.profile} />
      <nav className={styles.list} aria-label="Правовая информация">
        {infoItems.map((item) => (
          <NextLink
            key={item.href}
            href={item.href}
            className={styles.link}
            aria-label={`Перейти к документу «${item.title}»`}
          >
            <Card className={styles.card}>
              <span className={styles.itemTitle}>{item.title}</span>
              <Icon size="s">
                <ArrowRight />
              </Icon>
            </Card>
          </NextLink>
        ))}
      </nav>
    </PageShell>
  );
}
