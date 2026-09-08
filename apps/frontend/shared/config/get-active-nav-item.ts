import type { NavSection } from './navigation';
import { allNavSections } from './navigation';

/** Активный раздел навигации по URL — единые правила активности хрома
 * (карта #556, тикет #558); наследник
 * widgets/cabinet-layout/lib/get-active-nav-item. Совпадение — точный адрес
 * или его префикс с границей сегмента (`/tasks` → и `/tasks/new`); из
 * совпавших побеждает длиннейший префикс. Страница объекта и все её
 * внутренние страницы подсвечивают «Объекты», глобальные разделы — свой
 * пункт, /profile/notifications* — «Уведомления». Остальное (профиль и
 * прочее) → null: на мобайле подсвечен «Еще» TabBar, на десктопе подсветки
 * нет. */
export function getActiveNavItem(
  pathname: string,
  sections: ReadonlyArray<NavSection> = allNavSections,
): NavSection | null {
  let active: NavSection | null = null;
  let activeLength = 0;

  for (const section of sections) {
    const matches = pathname === section.href || pathname.startsWith(`${section.href}/`);

    if (matches && section.href.length > activeLength) {
      active = section;
      activeLength = section.href.length;
    }
  }

  return active;
}
