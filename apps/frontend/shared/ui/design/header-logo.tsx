import type { JSX } from 'react';
import { RentlyLogo as RentlyLogoIcon } from '@/shared/assets/icons';

/** Лого глобального top-header новых экранов (Figma 934:19654–55,
 * вариант Header=Logo): вектор «Рентли» 112×28, фирменные цвета запечены
 * в SVG. Навигационную обвязку (ссылка/кнопка) приносит потребитель —
 * сам компонент презентационный. */
export type HeaderLogoProps = {
  readonly className?: string;
};

export function HeaderLogo({ className }: HeaderLogoProps): JSX.Element {
  return <RentlyLogoIcon className={className} role="img" aria-label="Рентли" />;
}
