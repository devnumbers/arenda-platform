'use client';

import type { JSX } from 'react';
import { usePathname } from 'next/navigation';
import { navItems } from '../lib/nav-items';
import { getActiveNavItem } from '../lib/get-active-nav-item';
import { NavItem } from './NavItem';
import styles from './BottomNav.module.css';

export function BottomNav(): JSX.Element {
  const pathname = usePathname();
  const visibleItems = navItems.filter((item) => item.showInBottomNav);
  const activeItem = getActiveNavItem(pathname, visibleItems);

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
          isActive={activeItem?.href === item.href}
        />
      ))}
    </nav>
  );
}
