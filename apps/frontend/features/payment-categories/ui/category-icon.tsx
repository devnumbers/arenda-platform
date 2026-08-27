import type { JSX } from 'react';
import { BoldOther, StatusIconDanger } from '@/shared/assets/icons';
import { categoryIconComponents } from '@/features/payment-categories/lib/icon-registry';
import { cn } from '@/shared/lib/cn';

/**
 * Иконка категории платежа (компонент Figma «Category Icon», 651:5925 —
 * резолюция #449): круглый контейнер 44×44 с цветом подложки из каталога
 * (#447), кантом 2.5px по цвету поверхности и bold-иконкой 24×24 белым.
 * Бейдж `danger` — красный глиф просрочки на верхнем левом углу иконки
 * (секция «Просроченные», резолюция #452).
 */

export type CategoryIconBadge = 'danger';

/** Поверхность вокруг канта: белая или серая (#F3F4F6) — цвет канта следует фону. */
export type CategoryIconSurface = 'white' | 'muted';

export type CategoryIconProps = {
  /** Имя bold-иконки из каталога категорий (#447), например 'bold-key'. */
  readonly icon: string;
  /** Цвет подложки из каталога, например '#2B7FFF'. */
  readonly color: string;
  readonly badge?: CategoryIconBadge;
  readonly surface?: CategoryIconSurface;
  readonly className?: string;
};

export function CategoryIcon({
  icon,
  color,
  badge,
  surface = 'white',
  className,
}: CategoryIconProps): JSX.Element {
  const Icon = categoryIconComponents[icon] ?? BoldOther;

  return (
    <span
      className={cn(
        'relative flex h-11 w-11 shrink-0 items-center justify-center rounded-pill',
        surface === 'muted'
          ? 'shadow-[0_0_0_2.5px_var(--dl-surface-muted)]'
          : 'shadow-[0_0_0_2.5px_var(--dl-surface)]',
        className,
      )}
      style={{ backgroundColor: color }}
    >
      <Icon className="h-6 w-6 text-white" aria-hidden />
      {badge === 'danger' && (
        <StatusIconDanger className="absolute -top-1.5 -left-1.5 h-6 w-6" aria-hidden />
      )}
    </span>
  );
}
