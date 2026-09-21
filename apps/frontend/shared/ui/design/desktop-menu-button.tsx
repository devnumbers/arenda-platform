'use client';

import type { JSX } from 'react';
import Link from 'next/link';
import type { NavSection } from '@/shared/config/navigation';
import { cn } from '@/shared/lib/cn';
import { useNavIntentLink } from './nav-intent';

export type DesktopMenuButtonProps = {
  readonly section: NavSection;
  /** Переопределение адреса (лендинг «Объектов», карта #583);
   * undefined — href нав-модели. */
  readonly href?: string;
  /** Пункт-действие вместо ссылки: рендерится <button> с той же анатомией
   * («Поддержка» пилюли открывает модалку, #766). У такого section нет
   * href, поэтому действие задаётся явно. */
  readonly onClick?: () => void;
  readonly active?: boolean;
  readonly className?: string;
  /** Число непрочитанных (пилюля «Уведомления», #747): канон NumbersAlerts
   * (Figma 1652:82624) — красный pill #FB2C36, белый текст 13/15 Medium,
   * прижат вправо. 0/undefined — бейджа нет. */
  readonly badge?: number;
};

/** Кнопка десктопной навигации (Figma 1675:54051, анатомия
 * DesktopMenuButton): 200×44, radius 16, px-12, иконка 24 + подпись
 * 14/16 Medium (Mobile/Text/M/500) с зазором 12. Активная — серая плашка
 * bg-surface-muted (#F3F4F6), остальные белые; hover-фона в макете нет —
 * канон строк (ListRow): только focus-ring и transition. Ширина 200 —
 * дефолт сайдбара и пилюли «Уведомления» (Figma 1675:54098); пилюля
 * «Поддержка» переопределяет на авто (w-fit, Figma 1675:54096).
 *
 * Плашка и цвет живут на внутреннем span, а не на ссылке/кнопке:
 * безслойные сбросы `a { background-color: transparent }` и
 * `a { color: inherit }` в globals.css перебивают слоёные утилиты Tailwind
 * на самой ссылке (канон TabBarRow). */
export function DesktopMenuButton({
  section,
  href,
  onClick,
  active = false,
  className,
  badge,
}: DesktopMenuButtonProps): JSX.Element {
  const intent = useNavIntentLink();
  const triggerClass = cn(
    'flex h-11 w-[200px] cursor-pointer overflow-clip rounded-button font-sans outline-none',
    'focus-visible:ring-2 focus-visible:ring-primary',
    className,
  );
  const content = (
    <span
      className={cn(
        'flex h-full w-full items-center gap-2 rounded-button px-3 text-content transition-colors',
        active ? 'bg-surface-muted' : 'bg-surface',
      )}
    >
      <span className="flex min-w-0 items-center gap-3">
        <section.Icon className="h-6 w-6 shrink-0" aria-hidden />
        <span className="truncate text-sm font-medium leading-4">{section.label}</span>
      </span>
      {badge !== undefined && badge > 0 && (
        // Число непрочитанных: без потолка — счётчик бэка всегда конечен;
        // текст 13/15 Medium белый, pill 18px (Figma NumbersAlerts,
        // padding 0 6); стоит вплотную к подписи с зазором 8 (Figma
        // 2329:148713 — внешний row gap 8, внутри icon+label gap 12),
        // не прижат к правому краю пилюли.
        <span className="flex h-[18px] shrink-0 items-center rounded-pill bg-danger px-1.5 text-[13px] font-medium leading-[15px] text-surface">
          {badge}
        </span>
      )}
    </span>
  );

  if (onClick !== undefined) {
    return (
      <button type="button" onClick={onClick} className={triggerClass}>
        {content}
      </button>
    );
  }

  const resolvedHref = href ?? section.href;
  if (resolvedHref === undefined) {
    // Программная ошибка: пункт без адреса обязан быть действием (onClick).
    throw new Error('DesktopMenuButton: у раздела без href нужен onClick');
  }

  return (
    <Link
      href={resolvedHref}
      prefetch={intent.prefetch}
      aria-current={active ? 'page' : undefined}
      onPointerEnter={intent.onIntent}
      onPointerDown={intent.onIntent}
      onFocus={intent.onIntent}
      className={triggerClass}
    >
      {content}
    </Link>
  );
}
