import type { JSX } from 'react';
import { ArrowDown } from '@/shared/assets/icons';
import { ChipButton } from '@/shared/ui/design';
import { ParticipantsListSkeleton } from './participants-list-skeletons';

/**
 * Route-loading архетип списка участников (#609, тикет #1422): чип
 * сортировки — реальный с дефолтным лейблом «Имя» (без onClick — фаза
 * загрузки не действует; канон чипов в скелетонах, operations-loading
 * #609), тело — скелетон списка (паритет #604). Тот же кадр рисует
 * fallback Suspense-границы страницы и loading.tsx.
 */
export function ParticipantsListLoading(): JSX.Element {
  return (
    <>
      <div className="mt-4 px-6" data-testid="participants-list-loading">
        <ChipButton trailingIcon={<ArrowDown />}>Имя</ChipButton>
      </div>
      <ParticipantsListSkeleton />
    </>
  );
}
