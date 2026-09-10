import type { Metadata } from 'next';
import NextLink from 'next/link';
import { Card } from '@heroui/react/card';
import { SubScreenShell } from '@/shared/ui/design';
import { Icon } from '@/shared/ui/icon';
import { SmallArrowRight } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import styles from './page.module.css';

export const metadata: Metadata = {
  title: 'Информация — Рентли',
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
    <>
      <SubScreenShell title="Информация" fallbackHref={ROUTES.profile}>
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
                  <SmallArrowRight />
                </Icon>
              </Card>
            </NextLink>
          ))}
        </nav>
      </SubScreenShell>
    </>
  );
}
