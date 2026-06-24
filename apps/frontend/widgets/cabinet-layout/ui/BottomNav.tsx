'use client';

import type { JSX } from 'react';
import { usePathname } from 'next/navigation';
import { navItems } from '../lib/nav-items';
import { NavItem } from './NavItem';
import styles from './BottomNav.module.css';

export function BottomNav(): JSX.Element {
  const pathname = usePathname();
  const visibleItems = navItems.filter((item) => item.showInBottomNav);

  return (
    <nav className={styles.bottomNav}>
      {visibleItems.map((item) => (
        <NavItem
          key={item.href}
          href={item.href}
          label={item.label}
          iconName={item.icon}
          bottomIconName={item.bottomIcon}
          variant="bottom"
          isActive={pathname === item.href}
        />
      ))}
    </nav>
  );
}
