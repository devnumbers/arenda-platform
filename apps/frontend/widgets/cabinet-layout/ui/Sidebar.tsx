'use client';

import type { JSX } from 'react';
import { usePathname } from 'next/navigation';
import { Logo } from '@/shared/assets/icons';
import { navItems } from '../lib/nav-items';
import { NavItem } from './NavItem';
import styles from './Sidebar.module.css';

export function Sidebar(): JSX.Element {
  const pathname = usePathname();

  return (
    <aside className={styles.sidebar}>
      <Logo className={styles.logo} />
      <nav>
        <ul className={styles.nav}>
          {navItems.map((item) => (
            <li key={item.href}>
              <NavItem
                href={item.href}
                label={item.label}
                iconName={item.icon}
                variant="sidebar"
                isActive={pathname === item.href}
              />
            </li>
          ))}
        </ul>
      </nav>
    </aside>
  );
}
