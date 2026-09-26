import type { ComponentProps, JSX } from 'react';
import { cn } from '@/shared/lib/cn';
import { circleIconPair, type CircleIconVariant } from './circle-icon-variants';

export type CircleIconProps = ComponentProps<'span'> & {
  /** Подложка под кругом: muted — серая (--dl-surface-muted), white — белая
   * (--dl-surface); кант красится цветом подложки
   * (circle-icon-variants.ts). */
  readonly variant: CircleIconVariant;
};

/**
 * Круглый слот иконки/аватара строк списков — канон «круг 44 с кольцом
 * 2.5px» (круг 44×44 с гало по цвету подложки; Figma «Category Icon»
 * 651:5925 без бейджей и каталог-подложки, «User Button» 699:8867,
 * аватар Row Button 936:39348). Вариантная карта и словарь вариантов —
 * в чистом модуле circle-icon-variants; вариант = подложка, на которой
 * лежит круг: muted — белый круг с серым кантом (строки на серых
 * карточках), white — серый круг с белым кантом (строки на белом). Содержимое — глиф 24×24
 * или фото (тогда `overflow-hidden rounded-full` классом: кантом
 * клипается вместе с кругом). Размер и скругление переопределяются
 * className (круг 96 плейсхолдеров — `h-24 w-24`); круги с фоном вне пары
 * surface собирают span сами на `circleIconRing` (подложки каталога,
 * primary/10, точки-бейджи). Бейджи поверх круга (Category Icon,
 * 651:5925) — канон `CategoryIcon`, сюда не входят.
 */
export function CircleIcon({
  variant,
  className,
  children,
  ...props
}: CircleIconProps): JSX.Element {
  return (
    <span
      className={cn(
        'flex h-11 w-11 shrink-0 items-center justify-center rounded-pill',
        circleIconPair[variant],
        className,
      )}
      {...props}
    >
      {children}
    </span>
  );
}
