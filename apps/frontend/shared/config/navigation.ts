import type { FC, SVGProps } from 'react';
import {
  CheckmarkCircle,
  HomeMain,
  NotificationSettings,
  Support,
  Team,
  TimeHistory,
  UserCircle,
  Wallet,
} from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';

/** Идентификатор раздела единой навигации хрома (карта #556, тикет #558).
 * Один конфиг читают все три поверхности: десктопный сайдбар, мобильный
 * TabBar и шит «Еще». */
export type NavSectionId =
  | 'properties'
  | 'payments'
  | 'operations'
  | 'tasks'
  | 'contacts'
  | 'participants'
  | 'notifications'
  | 'support';

export type NavSection = {
  readonly id: NavSectionId;
  readonly label: string;
  readonly href: string;
  /** Иконка канон-каталога (DESIGN.md §10): Icon/R 24×24, цвет = currentColor. */
  readonly Icon: FC<SVGProps<SVGSVGElement>>;
};

/** 6 главных разделов — верхний блок десктопного сайдбара, порядок как в
 * Figma 1675:54051. «Платежи» и «Участники» — страницы-заглушки (#559):
 * содержание придет отдельными усилиями. */
export const mainNavSections: ReadonlyArray<NavSection> = [
  { id: 'properties', label: 'Объекты', href: ROUTES.properties, Icon: HomeMain },
  { id: 'payments', label: 'Платежи', href: ROUTES.payments, Icon: Wallet },
  { id: 'operations', label: 'Операции', href: ROUTES.operations, Icon: TimeHistory },
  { id: 'tasks', label: 'Задачи', href: ROUTES.tasks, Icon: CheckmarkCircle },
  { id: 'contacts', label: 'Контакты', href: ROUTES.contacts, Icon: UserCircle },
  { id: 'participants', label: 'Участники', href: ROUTES.participants, Icon: Team },
];

/** Уведомления и Поддержка — вне шестёрки главных: на десктопе это плавающие
 * пилюли по нижним углам (Figma 1675:54098/1675:54096), на мобайле
 * «Уведомления» — средний таб TabBar, «Поддержка» — последний пункт шита
 * «Еще» (Figma 1721:57140). */
export const secondaryNavSections: ReadonlyArray<NavSection> = [
  {
    id: 'notifications',
    label: 'Уведомления',
    href: ROUTES.profileNotifications,
    Icon: NotificationSettings,
  },
  { id: 'support', label: 'Поддержка', href: ROUTES.support, Icon: Support },
];

/** Все разделы — единый источник правил активности (getActiveNavItem). */
export const allNavSections: ReadonlyArray<NavSection> = [
  ...mainNavSections,
  ...secondaryNavSections,
];

/** Раздел по id — для поверхностей, берущих из конфига отдельные пункты
 * (табы TabBar). Ошибка — только при программной опечатке: id статически
 * ограничен типом NavSectionId. */
export function navSectionById(id: NavSectionId): NavSection {
  const section = allNavSections.find((entry) => entry.id === id);

  if (section === undefined) {
    throw new Error(`navigation: нет раздела «${id}»`);
  }

  return section;
}

/** Пункты шита «Еще» (Figma 1721:57140, тикет #560): главные разделы,
 * кроме «Объектов» (он — таб TabBar), плюс «Поддержка» — два ряда по три
 * в порядке Figma. «Уведомления» в шите нет — это средний таб TabBar. */
export const moreSheetNavSections: ReadonlyArray<NavSection> = [
  navSectionById('payments'),
  navSectionById('operations'),
  navSectionById('tasks'),
  navSectionById('contacts'),
  navSectionById('participants'),
  navSectionById('support'),
];
