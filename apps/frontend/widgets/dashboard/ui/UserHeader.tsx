'use client';

import type { JSX } from 'react';
import NextLink from 'next/link';
import { Card } from '@heroui/react/card';
import { Badge } from '@heroui/react/badge';
import { Icon } from '@/shared/ui/icon';
import { ArrowRight, BoldStar } from '@/shared/assets/icons';
import styles from './UserHeader.module.css';

export type UserHeaderProps = {
  readonly name?: string | null;
  readonly tariff?: 'basic' | 'pro' | 'business' | string | null;
};

const tariffLabels: Record<string, string> = {
  basic: 'Базовый',
  pro: 'Pro',
  business: 'Бизнес',
};

export function UserHeader({ name, tariff }: UserHeaderProps): JSX.Element {
  const tariffName = tariff ? (tariffLabels[tariff] ?? tariff) : 'Без подписки';

  return (
    <NextLink href="/profile" className={styles.link}>
      <Card className={styles.card}>
        <div className={styles.left}>
          <span className={styles.greeting}>
            {name ? `Привет, ${name}` : 'Привет'}
          </span>
          <Icon size="m">
            <ArrowRight />
          </Icon>
        </div>
        <Badge className={styles.badge} size="sm" variant="secondary">
          <Icon size="s">
            <BoldStar />
          </Icon>
          <span>{tariffName}</span>
        </Badge>
      </Card>
    </NextLink>
  );
}
