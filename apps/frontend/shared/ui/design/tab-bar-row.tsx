'use client';

import type { JSX } from 'react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { MenuLines } from '@/shared/assets/icons';
import { getActiveMobileTab } from '@/shared/config/get-active-nav-item';
import { navSectionById, type NavSection } from '@/shared/config/navigation';
import { cn } from '@/shared/lib/cn';
import { useNavIntentLink } from './nav-intent';

/** Табы-ссылки из единой нав-модели (#558): «Объекты» и «Уведомления»;
 * третий таб «Еще» — кнопка шита, не раздел навигации (иконка та же, что
 * в Figma 1721:64793 — Icon/R/Menu, экспорт MenuLines). */
const LINK_TABS: ReadonlyArray<NavSection> = [
  navSectionById('properties'),
  navSectionById('notifications'),
];

/** Общий триггер таба/пункта навигации (Figma 1721:64793/57140): во всю
 * треть строки, фокус-кольцо. */
const TAB_TRIGGER_CLASS =
  'flex min-w-0 flex-1 cursor-pointer items-stretch justify-center rounded-button outline-none focus-visible:ring-2 focus-visible:ring-primary';

export type TabBarRowProps = {
  /** Шит «Еще» открыт — в нижнем ряду шита «Еще» подсвечен активным
   * независимо от страницы (Figma 1721:57140). */
  readonly moreActive?: boolean;
  /** Шит открыт (обычный бар): состояние кнопки «Еще» для aria-expanded. */
  readonly moreExpanded?: boolean;
  /** Тап по «Еще»: в обычном баре открывает шит, в шите — закрывает его. */
  readonly onMoreSelect: () => void;
  /** Тап по табу-ссылке; внутри шита закрывает его перед переходом. */
  readonly onNavigate?: () => void;
  /** Лендинг таба «Объекты» (карта #583): основной объект / единственный
   * активный / список — решает ScreenLayout (usePropertiesLandingHref);
   * undefined — базовый href нав-модели (список). */
  readonly propertiesHref?: string;
};

/** Ряд табов «Объекты / Уведомления / Еще» (Figma 1721:64793): один и тот же
 * компонент в реальном TabBar и нижним рядом шита «Еще» — идентичная
 * внутренняя геометрия (высота 72px, иконка 24 + подпись 13/15) гарантирует
 * отсутствие «прыжка» полосы при открытии шита; safe-area остаётся снаружи
 * ряда (у бара — паддинг nav, у шита — паддинг листа). Активность — из
 * нав-модели (getActiveMobileTab): «Объекты» — объект и его внутренние
 * страницы, «Уведомления» — раздел уведомлений, «Еще» — всё остальное
 * (тикет #560). Цвет живёт на внутреннем span, а не на ссылке/кнопке:
 * безслойный сброс `a { color: inherit }` в globals.css перебивает цветовые
 * утилиты на самой ссылке (см. комментарий про HeroUI-normalize в
 * globals.css). */
export function TabBarRow({
  moreActive = false,
  moreExpanded = false,
  onMoreSelect,
  onNavigate,
  propertiesHref,
}: TabBarRowProps): JSX.Element {
  const pathname = usePathname();
  const activeTab = moreActive ? 'more' : getActiveMobileTab(pathname);

  return (
    <div className="mx-auto flex h-[72px] w-full items-stretch px-4">
      {LINK_TABS.map((section) => (
        <TabNavLink
          key={section.id}
          section={section}
          href={section.id === 'properties' && propertiesHref !== undefined ? propertiesHref : section.href}
          active={activeTab === section.id}
          onClick={onNavigate}
        />
      ))}
      <button
        type="button"
        onClick={onMoreSelect}
        aria-expanded={moreActive || moreExpanded}
        className={TAB_TRIGGER_CLASS}
      >
        <TabLabel label="Еще" Icon={MenuLines} active={activeTab === 'more'} />
      </button>
    </div>
  );
}

/** Пункт-ссылка навигации: таб TabBar и пункты шита «Еще» — одна анатомия
 * (MoreSheet рендерит те же 6 разделов нав-модели этим компонентом). */
export function TabNavLink({
  section,
  href,
  active,
  onClick,
}: {
  readonly section: NavSection;
  /** Переопределение адреса (лендинг «Объектов»); по умолчанию — нав-модель. */
  readonly href?: string;
  readonly active: boolean;
  readonly onClick?: () => void;
}): JSX.Element {
  const intent = useNavIntentLink();
  const resolvedHref = href ?? section.href;
  if (resolvedHref === undefined) {
    // Программная ошибка: пункты-действия без адреса («Поддержка», #766)
    // рендерятся TabNavAction, не ссылкой.
    throw new Error('TabNavLink: у раздела без href нет адреса ссылки');
  }
  return (
    <Link
      href={resolvedHref}
      prefetch={intent.prefetch}
      aria-current={active ? 'page' : undefined}
      onClick={onClick}
      onPointerEnter={intent.onIntent}
      onPointerDown={intent.onIntent}
      onFocus={intent.onIntent}
      className={TAB_TRIGGER_CLASS}
    >
      <TabLabel label={section.label} Icon={section.Icon} active={active} />
    </Link>
  );
}

/** Пункт-действие шита «Еще» — та же анатомия TabLabel, но <button> вместо
 * ссылки: «Поддержка» (#766) открывает модалку вместо перехода. */
export function TabNavAction({
  label,
  Icon,
  onClick,
}: {
  readonly label: string;
  readonly Icon: NavSection['Icon'];
  readonly onClick: () => void;
}): JSX.Element {
  return (
    <button type="button" onClick={onClick} className={TAB_TRIGGER_CLASS}>
      <TabLabel label={label} Icon={Icon} active={false} />
    </button>
  );
}

function TabLabel({
  label,
  Icon,
  active,
}: {
  readonly label: string;
  readonly Icon: NavSection['Icon'];
  readonly active: boolean;
}): JSX.Element {
  return (
    <span
      className={cn(
        'flex min-w-0 flex-1 flex-col items-center justify-center gap-1.5 rounded-button transition-colors',
        active
          ? 'text-content'
          : 'text-content-tertiary hover:text-content-secondary active:text-content-secondary',
      )}
    >
      <Icon className="h-6 w-6 shrink-0" aria-hidden />
      {/* Подпись 13/15 (Mobile/Text/S/500, Figma 1721:64793) — общая для
       * табов TabBar и пунктов шита «Еще». */}
      <span className="text-[13px] font-medium leading-[15px]">{label}</span>
    </span>
  );
}
