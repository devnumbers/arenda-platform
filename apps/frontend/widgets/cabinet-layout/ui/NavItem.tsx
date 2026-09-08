'use client';

import type { ComponentType, JSX } from 'react';
import Link from 'next/link';
import clsx from 'clsx';
import {
  BoldBill,
  BoldWallet,
  HomeMain,
  Support,
  Team,
  UserCircle,
} from '@/shared/assets/icons';
import styles from './NavItem.module.css';

// text-error — маркер замены (07.09): канон «по смыслу» до правильных
// иконок навигации от владельца.
function redIcon(Icon: IconComponent): IconComponent {
  return function RedIcon({ className }) {
    return <Icon className={clsx(className, 'text-error')} />;
  };
}

export type NavItemProps = {
  readonly href: string;
  readonly label: string;
  readonly iconName: string;
  readonly bottomIconName?: string;
  readonly isActive: boolean;
  readonly variant: 'sidebar' | 'bottom';
};

type IconComponent = ComponentType<{ readonly className?: string }>;

const iconMap: Record<string, IconComponent> = {
  NavObjects: redIcon(HomeMain),
  NavProfile: redIcon(UserCircle),
  NavSupport: redIcon(Support),
  NavOperations: BoldBill,
  NavPayments: BoldWallet,
  NavContacts: Team,
  BottomObjects: redIcon(HomeMain),
  BottomProfile: redIcon(UserCircle),
};

export function NavItem({
  href,
  label,
  iconName,
  bottomIconName,
  isActive,
  variant,
}: NavItemProps): JSX.Element {
  const isSidebar = variant === 'sidebar';
  const resolvedIconName = isSidebar ? iconName : (bottomIconName ?? iconName);
  const IconComponent = iconMap[resolvedIconName];

  if (!IconComponent) {
    return <></>;
  }

  return (
    <Link
      href={href}
      className={clsx(
        styles.item,
        isActive && styles.active,
        isSidebar ? styles.sidebar : styles.bottom
      )}
      aria-current={isActive ? 'page' : undefined}
      aria-label={isSidebar ? undefined : label}
      title={isSidebar ? undefined : label}
    >
      <IconComponent
        className={clsx(
          isSidebar ? styles.sidebarIcon : styles.bottomIcon,
          !isSidebar && isActive && styles.bottomIconActive
        )}
      />
      {isSidebar && (
        <span className={styles.sidebarLabel}>{label}</span>
      )}
    </Link>
  );
}
