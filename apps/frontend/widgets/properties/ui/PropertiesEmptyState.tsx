'use client';

import type { JSX } from 'react';
import { Button, EmptyState } from '@/shared/ui/design';

/**
 * Пустое состояние хаба «Объектов» (Figma 1603:90604, тикет #586):
 * иллюстрация, «Объектов нет», серое пояснение и синяя кнопка «Добавить»
 * (Primary Small). Сценарий «зарегистрировался и сразу оформил тариф без
 * объекта» видит то же состояние. При исчерпанном лимите тарифа кнопка
 * гасится с подсказкой (гейт лимита — как у «+» создания).
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
      size="small"
      disabled={isLoading === true || canAdd === false}
      onClick={onAdd}
      aria-label={canAdd === false ? 'Добавить объект (достигнут лимит)' : 'Добавить объект'}
    >
      Добавить
    </Button>
  );

  return (
    <EmptyState
      imageSrc="/images/properties/properties-empty.png"
      imageAlt="Объектов нет"
      title="Объектов нет"
      description="Добавьте квартиру, дом или другое помещение, чтобы начать управлять объектом"
      className="pt-6"
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
