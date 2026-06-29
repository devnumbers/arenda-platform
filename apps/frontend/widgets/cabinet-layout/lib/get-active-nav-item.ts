import type { NavItemConfig } from './nav-items';

export function getActiveNavItem(
  pathname: string,
  items: ReadonlyArray<NavItemConfig>
): NavItemConfig | null {
  let active: NavItemConfig | null = null;

  for (const item of items) {
    const { href } = item;
    const isMatch = pathname === href || pathname.startsWith(`${href}/`);

    if (isMatch && (!active || href.length > active.href.length)) {
      active = item;
    }
  }

  return active;
}
