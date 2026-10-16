import type { ComponentProps, JSX } from 'react';
import { cn } from '@/shared/lib/cn';
import { TrashBin } from '@/shared/assets/icons';
import { IconButton } from './icon-button';

/**
 * Бейдж удаления фото в выемке круга-аватара (Figma 1299:51869 — правка
 * фото контакта; канон правки фото карты #1217): канон IconButton 44×44
 * под иконку 24 с белой подложкой, ложится на правый-верхний край круга —
 * выступает за него на 16px вверх и вправо. Выемку держит пара: круг под
 * бейджем с кольцом цвета подложки (`circleIconRing`, наносит потребитель)
 * и белая подложка самого бейджа. Глиф — TrashBin Icon/R (канон иконок
 * 07.09). Потребитель оборачивает круг в `relative` и глушит бейдж на
 * время запросов; confirm за бейджем — его дело.
 */
export type PhotoRemoveBadgeProps = Omit<ComponentProps<typeof IconButton>, 'icon' | 'label'> & {
  /** Имя для screen reader (канон IconButton); по умолчанию «Удалить фото». */
  readonly label?: string;
};

export function PhotoRemoveBadge({
  className,
  label = 'Удалить фото',
  ...props
}: PhotoRemoveBadgeProps): JSX.Element {
  return (
    <IconButton
      icon={<TrashBin />}
      label={label}
      className={cn('absolute -top-4 -right-4 bg-surface', className)}
      {...props}
    />
  );
}
