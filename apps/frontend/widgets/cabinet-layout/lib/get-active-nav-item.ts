import type { NavItemConfig } from './nav-items';

export function getActiveNavItem(
  pathname: string,
  items: ReadonlyArray<NavItemConfig>
): NavItemConfig | null {
  let active: NavItemConfig | null = null;
  let activeLength = 0;

  for (const item of items) {
    const candidates = [item.href, ...(item.match ?? [])];

    for (const candidate of candidates) {
      const isMatch = pathname === candidate || pathname.startsWith(`${candidate}/`);

      if (isMatch && candidate.length > activeLength) {
        active = item;
        activeLength = candidate.length;
      }
    }
  }

  return active;
}
