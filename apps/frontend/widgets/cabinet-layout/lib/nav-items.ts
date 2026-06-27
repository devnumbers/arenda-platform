export type NavItemConfig = {
  readonly label: string;
  readonly href: string;
  readonly icon: string;
  readonly bottomIcon?: string;
  readonly showInBottomNav: boolean;
};

export const navItems: ReadonlyArray<NavItemConfig> = [
  { label: 'Главная', href: '/dashboard', icon: 'NavHome', bottomIcon: 'BottomHome', showInBottomNav: true },
  { label: 'Объекты', href: '/properties', icon: 'NavObjects', bottomIcon: 'BottomObjects', showInBottomNav: true },
  { label: 'Арендаторы', href: '/tenants', icon: 'NavTenants', showInBottomNav: false },
  { label: 'Финансы', href: '/finance', icon: 'NavWallet', bottomIcon: 'BottomWallet', showInBottomNav: true },
  { label: 'Профиль', href: '/profile', icon: 'NavProfile', bottomIcon: 'BottomProfile', showInBottomNav: true },
  { label: 'Поддержка', href: '/profile/support', icon: 'NavSupport', showInBottomNav: false },
];
