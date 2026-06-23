export type NavItemConfig = {
  readonly label: string;
  readonly href: string;
  readonly icon: string;
  readonly showInBottomNav: boolean;
};

export const navItems: ReadonlyArray<NavItemConfig> = [
  { label: 'Главная', href: '/dashboard', icon: 'Home', showInBottomNav: true },
  { label: 'Объекты', href: '/properties', icon: 'Objects', showInBottomNav: true },
  { label: 'Арендаторы', href: '/tenants', icon: 'Arendators', showInBottomNav: false },
  { label: 'Финансы', href: '/finance', icon: 'BoldWallet', showInBottomNav: true },
  { label: 'Профиль', href: '/profile', icon: 'BoldProfile', showInBottomNav: true },
  { label: 'Поддержка', href: '/support', icon: 'Support', showInBottomNav: false },
];
