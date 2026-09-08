export type NavItemConfig = {
  readonly label: string;
  readonly href: string;
  /** Extra path prefixes that also activate this item (e.g. nested routes without their own nav entry). */
  readonly match?: ReadonlyArray<string>;
  readonly icon: string;
  readonly bottomIcon?: string;
  readonly showInBottomNav: boolean;
};

export const navItems: ReadonlyArray<NavItemConfig> = [
  { label: 'Объекты', href: '/properties', icon: 'NavObjects', bottomIcon: 'BottomObjects', showInBottomNav: true },
  { label: 'Операции', href: '/operations', icon: 'NavOperations', showInBottomNav: false },
  // Точка входа книги контактов — только сайдбар десктопа (решение владельца
  // 2026-09-04: мобильную навигацию ставит он позже).
  { label: 'Контакты', href: '/contacts', icon: 'NavContacts', showInBottomNav: false },
  // Глобальные платежи (карта #573, #578): сайдбар на ПК + пункт меню
  // «Ещё» на мобиле; вложенные страницы (/payments/*) наследуют подсветку
  // по префиксу.
  { label: 'Платежи', href: '/payments', icon: 'NavPayments', showInBottomNav: false },
  { label: 'Профиль', href: '/profile', icon: 'NavProfile', bottomIcon: 'BottomProfile', showInBottomNav: true },
  { label: 'Поддержка', href: '/support', icon: 'NavSupport', showInBottomNav: false },
];
