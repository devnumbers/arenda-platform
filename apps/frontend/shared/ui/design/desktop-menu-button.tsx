'use client';

import type { JSX } from 'react';
import Link from 'next/link';
import type { NavSection } from '@/shared/config/navigation';
import { cn } from '@/shared/lib/cn';
import { useNavIntentLink } from './nav-intent';

export type DesktopMenuButtonProps = {
  readonly section: NavSection;
  readonly active?: boolean;
  readonly className?: string;
  /** Число непрочитанных (пилюля «Уведомления», #747): канон NumbersAlerts
   * (Figma 1652:82624) — красный pill #FB2C36, белый текст 13/15 Medium,
   * прижат вправо. 0/undefined — бейджа нет. */
  readonly badge?: number;
};

/** Кнопка десктопной навигации (Figma 1675:54051, анатомия
 * DesktopMenuButton): 200×44, radius 16, px-12, иконка 24 + подпись
 * 14/16 Medium (Mobile/Text/M/500) с зазором 12. Фон — по сету состояний
 * DesktopMenuButton (Figma 1652:82711, слово владельца 12.10):
 * невыбранная — белая, hover bg-surface-muted, при нажатии
 * bg-surface-muted-hover; активная — bg-surface-muted, hover
 * bg-surface-muted-hover, при нажатии bg-surface-muted-active (hover
 * невыбранной совпадает с фоном активной — так в макете; градации как
 * у IconButton muted). Focus-ring — прежний, вне этого сета. Ширина 200 —
 * дефолт сайдбара и пилюли «Уведомления» (Figma 1675:54098); пилюля
 * «Поддержка» переопределяет на авто (w-fit, Figma 1675:54096).
 *
 * Плашка и цвет живут на внутреннем span, а не на ссылке/кнопке:
 * безслойные сбросы `a { background-color: transparent }` и
 * `a { color: inherit }` в globals.css перебивают слоёные утилиты Tailwind
 * на самой ссылке (канон TabBarRow); :hover/:active ссылки/кнопки
 * распространяются на потомков, поэтому варианты работают на span. */
export function DesktopMenuButton({
  section,
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
  const resolvedHref = section.href;
  if (resolvedHref === undefined) {
    // Программная ошибка: пункт пилюль/сайдбара без адреса не существует.
    throw new Error('DesktopMenuButton: у раздела без href нет адреса ссылки');
  }
  const content = (
      <span
        className={cn(
          'flex h-full w-full items-center gap-2 rounded-button px-3 text-content transition-colors',
          active
            ? 'bg-surface-muted hover:bg-surface-muted-hover active:bg-surface-muted-active'
            : 'bg-surface hover:bg-surface-muted active:bg-surface-muted-hover',
        )}
      >
      <span className="flex min-w-0 items-center gap-3">
        <section.Icon className="h-6 w-6 shrink-0" aria-hidden />
        <span className="truncate text-sm font-medium leading-4">{section.label}</span>
      </span>
      {badge !== undefined && badge > 0 && (
        // Число непрочитанных: без потолка — счётчик бэка всегда конечен;
        // текст 13/15 Medium белый, pill 20×20 (Figma NumbersAlerts,
        // padding 0 6; правка владельца 17.10: min-width 20, контент по
        // центру, padding-top 2); стоит вплотную к подписи с зазором 8
        // (Figma 2329:148713 — внешний row gap 8, внутри icon+label gap 12),
        // не прижат к правому краю пилюли.
        <span className="flex h-[20px] min-w-[20px] shrink-0 items-center justify-center rounded-pill bg-danger px-1.5 pt-0.5 text-[13px] font-medium leading-[15px] text-surface">
          {badge}
        </span>
      )}
    </span>
  );

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
