import type { JSX } from 'react';
import { ArchiveSmall } from '@/shared/assets/icons';
import { propertyPillClass } from './property-pill-class';

/** Пилюля «В архиве» в шапке детали (#773): анатомия пилюли доступа
 * (Figma 2200-97365, StatusHouseBadge-семья), иконка архива и слово —
 * канон бейджа экрана «Архивные объекты» (Figma 1603-92103, бейдж
 * карточки #587). На детали — серая (bg-surface-muted): фон страницы
 * белый, белая карточная пилюля канона на нём не читается. Читается
 * любым читателем архивного объекта: deep-link — единственный путь к
 * чужому архивному (ленты фильтруют archived), признак архива обязан
 * быть на виду. */
export function PropertyArchivedPill(): JSX.Element {
  return (
    <p data-testid="property-archived-pill" className={propertyPillClass}>
      <ArchiveSmall aria-hidden className="h-4 w-4" />
      В архиве
    </p>
  );
}
