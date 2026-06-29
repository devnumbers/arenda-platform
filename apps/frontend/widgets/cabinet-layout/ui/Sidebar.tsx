'use client';

import type { JSX } from 'react';
import { usePathname } from 'next/navigation';
import { navItems } from '../lib/nav-items';
import { getActiveNavItem } from '../lib/get-active-nav-item';
import { NavItem } from './NavItem';
import styles from './Sidebar.module.css';

export function Sidebar(): JSX.Element {
  const pathname = usePathname();
  const activeItem = getActiveNavItem(pathname, navItems);

  return (
    <aside className={styles.sidebar}>
      <nav>
        <ul className={styles.nav}>
          {navItems.map((item) => (
            <li key={item.href}>
              <NavItem
                href={item.href}
                label={item.label}
                iconName={item.icon}
                variant="sidebar"
                isActive={activeItem?.href === item.href}
              />
            </li>
          ))}
        </ul>
      </nav>
    </aside>
  );
}
