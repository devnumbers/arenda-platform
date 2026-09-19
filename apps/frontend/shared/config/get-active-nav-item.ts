import type { NavSection } from './navigation';
import { allNavSections } from './navigation';

/** Страницы вне префикса своего раздела, которые он подсвечивает: экран
 * «Настроить уведомления» живёт в дереве профиля, а подсвечивает
 * «Уведомления» — правило #558 и правило пилюль #561 (#746). */
const SECTION_ALIASES: ReadonlyArray<readonly [prefix: string, sectionId: string]> = [
  ['/profile/notifications', 'notifications'],
];

function matchesSection(pathname: string, section: NavSection): boolean {
  if (pathname === section.href || pathname.startsWith(`${section.href}/`)) {
    return true;
  }
  return SECTION_ALIASES.some(
    ([prefix, sectionId]) =>
      sectionId === section.id &&
      (pathname === prefix || pathname.startsWith(`${prefix}/`)),
  );
}

/** Активный раздел навигации по URL — единые правила активности хрома
 * (карта #556, тикет #558). Совпадение — точный адрес или его префикс
 * с границей сегмента (`/tasks` → и `/tasks/new`); из совпавших побеждает
 * длиннейший префикс. Страница объекта и все её внутренние страницы
 * подсвечивают «Объекты», глобальные разделы — свой пункт,
 * /profile/notifications* — «Уведомления». Остальное (профиль и прочее)
 * → null: на мобайле подсвечен «Еще» TabBar, на десктопе подсветки нет. */
export function getActiveNavItem(
  pathname: string,
  sections: ReadonlyArray<NavSection> = allNavSections,
): NavSection | null {
  let active: NavSection | null = null;
  let activeLength = 0;

  for (const section of sections) {
    const matches = matchesSection(pathname, section);

    if (matches && section.href.length > activeLength) {
      active = section;
      activeLength = section.href.length;
    }
  }

  return active;
}

/** Идентификатор таба мобильного TabBar («Объекты / Уведомления / Еще»). */
export type MobileTabId = 'properties' | 'notifications' | 'more';

/** Активный таб TabBar по URL — правила разделов (#558), спроецированные на
 * три таба: свой таб есть только у «Объектов» (корень и внутренние страницы
 * объекта) и «Уведомлений»; глобальные разделы без своего таба, профиль и
 * прочее подсвечивают «Еще» (тикет #560). */
export function getActiveMobileTab(pathname: string): MobileTabId {
  const section = getActiveNavItem(pathname);

  if (section?.id === 'properties') return 'properties';
  if (section?.id === 'notifications') return 'notifications';

  return 'more';
}
