'use client';

import type { ComponentType, JSX } from 'react';
import Link from 'next/link';
import clsx from 'clsx';
import {
  Home,
  Objects,
  Arendators,
  BoldWallet,
  BoldProfile,
  Support,
} from '@/shared/assets/icons';
import styles from './NavItem.module.css';

export type NavItemProps = {
  readonly href: string;
  readonly label: string;
  readonly iconName: string;
  readonly isActive: boolean;
  readonly variant: 'sidebar' | 'bottom';
};

type IconComponent = ComponentType<{ readonly className?: string }>;

const iconMap: Record<string, IconComponent> = {
  Home,
  Objects,
  Arendators,
  BoldWallet,
  BoldProfile,
  Support,
};

export function NavItem({
  href,
  label,
  iconName,
  isActive,
  variant,
}: NavItemProps): JSX.Element {
  const IconComponent = iconMap[iconName];

  if (!IconComponent) {
    return <></>;
  }

  const isSidebar = variant === 'sidebar';

  return (
    <Link
      href={href}
      className={clsx(
        styles.item,
        isActive && styles.active,
        isSidebar ? styles.sidebar : styles.bottom
      )}
      aria-current={isActive ? 'page' : undefined}
    >
      <IconComponent
        className={isSidebar ? styles.sidebarIcon : styles.bottomIcon}
      />
      <span className={isSidebar ? styles.sidebarLabel : styles.bottomLabel}>
        {label}
      </span>
    </Link>
  );
}
