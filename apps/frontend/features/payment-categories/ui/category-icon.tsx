import type { JSX } from 'react';
import { BoldOther, StatusIconCheck, StatusIconDanger } from '@/shared/assets/icons';
import { categoryIconComponents } from '@/features/payment-categories/lib/icon-registry';
import { cn } from '@/shared/lib/cn';

/**
 * Иконка категории платежа (компонент Figma «Category Icon», 651:5925 —
 * резолюция #449): круглый контейнер 44×44 с цветом подложки из каталога
 * (#447), кантом 2.5px по цвету поверхности и bold-иконкой 24×24 белым.
 * Бейдж `danger` — красный глиф просрочки на верхнем левом углу иконки
 * (операции, резолюция #452). Бейдж `notification` — красная точка 10×10
 * в углу (651:6759): у ПЛАТЕЖА с накопленной просрочкой (1323:61133,
 * State=Expired — в обеих поверхностях). Проп `check` — синяя галочка
 * CheckWhite 24×24 в правом нижнем углу круга (889:25528, Show Check:
 * выступает на 8px за край, как Notification Dot в своём): пометка
 * платежа на удаление из избранного в режиме правки (тикет #579).
 */

export type CategoryIconBadge = 'danger' | 'notification';

/** Поверхность вокруг канта: белая или серая (#F3F4F6) — цвет канта следует фону. */
export type CategoryIconSurface = 'white' | 'muted';

export type CategoryIconProps = {
  /** Имя bold-иконки из каталога категорий (#447), например 'bold-key'. */
  readonly icon: string;
  /** Цвет подложки из каталога, например '#2B7FFF'. */
  readonly color: string;
  readonly badge?: CategoryIconBadge;
  readonly surface?: CategoryIconSurface;
  /** Пометка на удаление из избранного (889:25528, тикет #579). */
  readonly check?: boolean;
  readonly className?: string;
};

export function CategoryIcon({
  icon,
  color,
  badge,
  surface = 'white',
  check = false,
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
        <StatusIconDanger
          className={cn(
            'absolute -top-1.5 -left-1.5 h-6 w-6',
            // Кант бейджа красится под поверхность тем же механизмом, что и
            // кант круга (тень выше): в SVG он stroke="currentColor".
            surface === 'muted' ? 'text-surface-muted' : 'text-surface',
          )}
          aria-hidden
        />
      )}
      {badge === 'notification' && (
        <span
          className={cn(
            'absolute left-0 top-0 h-2.5 w-2.5 rounded-full bg-danger',
            surface === 'muted'
              ? 'shadow-[0_0_0_2.5px_var(--dl-surface-muted)]'
              : 'shadow-[0_0_0_2.5px_var(--dl-surface)]',
          )}
          aria-hidden
        />
      )}
      {check && (
        <StatusIconCheck
          className="absolute -bottom-2 -right-2 h-6 w-6"
          aria-hidden
        />
      )}
    </span>
  );
}
