'use client';

import type { JSX } from 'react';
import { Button, EmptyState } from '@/shared/ui/design';

/**
 * Пустое состояние хаба «Объектов» (Figma 3229-94647 / 3235-74057,
 * тикет #1234): иллюстрация, «Объектов нет», серое пояснение и синяя
 * кнопка «Добавить объект» — дефолтный размер Button, канон CTA пустых
 * состояний (#1004). Сценарий «зарегистрировался и сразу оформил тариф
 * без объекта» видит то же состояние. При исчерпанном лимите тарифа
 * кнопка гасится с подсказкой (гейт лимита — как у «+» создания).
 */
export type PropertiesEmptyStateProps = {
  readonly canAdd?: boolean;
  readonly isLoading?: boolean;
  readonly onAdd?: () => void;
};

export function PropertiesEmptyState({
  canAdd,
  isLoading,
  onAdd,
}: PropertiesEmptyStateProps): JSX.Element {
  const button = (
    <Button
      disabled={isLoading === true || canAdd === false}
      onClick={onAdd}
      aria-label={canAdd === false ? 'Добавить объект (достигнут лимит)' : 'Добавить объект'}
    >
      Добавить объект
    </Button>
  );

  return (
    <EmptyState
      imageSrc="/images/properties/properties-empty.png"
      imageAlt="Объектов нет"
      title="Объектов нет"
      description="Добавьте квартиру, дом или помещение, чтобы начать управлять объектом"
      action={
        canAdd === false && !isLoading ? (
          // Подсказка о лимите — нативный title на обёртке (HeroUI Tooltip
          // снесён, #901): кнопка задизейблена и тап не ловит, подсказку
          // показывает обёртка; смысл продублирован в aria-label кнопки.
          <span title="Достигнут лимит объектов по тарифу">{button}</span>
        ) : (
          button
        )
      }
    />
  );
}
