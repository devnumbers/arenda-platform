import type { TopNavUser } from './top-nav-user-context';

/** Вход крыльев TopNav: флаги страницы, от которых зависит, рисуются ли
 * «крылья» (лого + ссылка профиля) на каждом ярусе. */
export type TopNavWingsInput = {
  /** На планшете у подэкранов с leading-кнопкой крыльев нет (аудит #563). */
  readonly hasLeading: boolean;
  /** Полноэкранная поверхность: ниже ПК крыльев нет, на ПК рисует сама
   * поверхность (решение 25.09, доработка #865). */
  readonly overlay: boolean;
  /** Крылья только на ПК — подэкраны без leading («Профиль», «Уведомления»). */
  readonly hideWingsBelowDesktop: boolean;
  /** Признак хаба: крылья и на мобайле. */
  readonly mobileWings: boolean;
  /** Экран живёт под PersistentNavWings (ScreenLayout держит крыло ПК сам) —
   * собственное крыло страницы на ПК гасится, дубля нет (22.10.2026). */
  readonly persistentWings: boolean;
};

/** Классы видимости крыльев TopNav (лого + ссылка профиля) по ярусам —
 * вынесено из компонента, чтобы покрываться юнит-тестами (канон
 * node-тестов, прецедент circlePhotoView): мобайл — только хабы
 * (mobileWings), планшет — без leading-кнопки, ПК — всегда. Постоянное
 * крыло ПК (PersistentNavWings) гасит крыло страницы на ПК — кроме
 * overlay: на поверхности крылья рисует она сама, и постоянный слой всё
 * равно скрыт правилом body:has(.fullscreen-surface) в globals.css.
 * desktopWingsClass идёт ПОСЛЕ tier-класса в cn — tailwind-merge решает
 * конфликт «desktop:flex против desktop:hidden» победой последнего. */
export function topNavWingClasses({
  hasLeading,
  overlay,
  hideWingsBelowDesktop,
  mobileWings,
  persistentWings,
}: TopNavWingsInput): {
  readonly wingsMobileClass: 'flex' | 'hidden';
  readonly wingsTierClass: 'tablet:flex' | 'desktop:flex';
  readonly desktopWingsClass: '' | 'desktop:hidden';
} {
  const wingsMobileClass = !overlay && mobileWings ? 'flex' : 'hidden';
  const wingsTierClass =
    hasLeading || overlay || hideWingsBelowDesktop ? 'desktop:flex' : 'tablet:flex';
  const desktopWingsClass = persistentWings && !overlay ? 'desktop:hidden' : '';
  return { wingsMobileClass, wingsTierClass, desktopWingsClass };
}

/** Содержимое «крыла» профиля из данных контекста — общее для крыла
 * страницы (TopNav) и постоянного крыла ПК (PersistentNavWings): pending —
 * useMe в полёте (скелетон вместо имени); имя-null/пустое/ошибка —
 * терминальные состояния, текстовый плейсхолдер «Пользователь» (аудит
 * #876, решение владельца 02.10). */
export function topNavWingUser(
  user: TopNavUser | null,
): { readonly wingName: string | undefined; readonly pending: boolean } {
  const firstName = user?.name;
  const pending = user !== null && user.pending === true && firstName === undefined;
  const wingName =
    pending ? undefined : firstName === undefined || firstName === '' ? 'Пользователь' : firstName;
  return { wingName, pending };
}
