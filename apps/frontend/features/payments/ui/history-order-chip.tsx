'use client';

import type { JSX } from 'react';
import { ChangeVertical } from '@/shared/assets/icons';
import { ChipButton } from '@/shared/ui/design';
import type { HistoryOrder } from '../lib/operations-history';

/**
 * Чип сортировки «Истории операций» — общий канон экрана платежа (#466)
 * и завершённой аренды (#535, Figma 1302:52209): пилюля «Сначала новые» /
 * «Сначала старые» с ChangeVertical справа; подпись показывает текущее
 * направление, aria-label называет то, на что переключит тап. Состояние
 * направления живёт в useHistoryOrder (пишется в адрес, #785) — компонент
 * презентационный: принимает текущее значение и переключатель, сам ничего
 * не хранит.
 */
export function HistoryOrderChip({
  order,
  onToggle,
}: {
  readonly order: HistoryOrder;
  readonly onToggle: () => void;
}): JSX.Element {
  return (
    <ChipButton
      trailingIcon={<ChangeVertical />}
      onClick={onToggle}
      aria-label={
        order === 'desc'
          ? 'Сортировка: сначала новые — переключить на «сначала старые»'
          : 'Сортировка: сначала старые — переключить на «сначала новые»'
      }
    >
      {order === 'desc' ? 'Сначала новые' : 'Сначала старые'}
    </ChipButton>
  );
}
