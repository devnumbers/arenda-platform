'use client';

import type { JSX } from 'react';
import { Button, PageContent, TopNav, TopNavBackButton, TopNavTitle } from '@/shared/ui/design';
import { historyScreenBackHref, historyScreenTitle } from '../lib/screen-chrome';
import { historyFeedSkeleton } from './history-states';

/**
 * Хром страницы истории в фазе загрузки (карта #1324, #1281): fallback
 * Suspense-границы собирает экран сам (TopNav, PageContent, «Настройки»),
 * поэтому без хрома внутри фолбэка скелетон рисовался голым у верха
 * вьюпорта и «прыгал» в хром при приходе контента. Архетип —
 * OperationsLoading (§7): хром экрана — вне фазы загрузки, скелетон
 * паритетен pending- и загруженной композиции. Лента прижимается к нижнему
 * краю, как загруженная (flex-1 justify-end): пришедший контент встаёт на
 * место скелетона без сдвига, у длинной ленты первый кадр тоже у дна.
 * Кнопка «Настройки» в покое — интерактивный экран подменяет её без
 * сдвига.
 */
export function HistoryFeedLoading({
  participantId,
  propertyId,
}: {
  readonly participantId?: string;
  readonly propertyId?: string;
}): JSX.Element {
  return (
    <>
      <TopNav
        leading={<TopNavBackButton fallbackHref={historyScreenBackHref(participantId, propertyId)} />}
      >
        <TopNavTitle title={historyScreenTitle(participantId, propertyId)} />
      </TopNav>

      <PageContent className="flex-1 justify-end px-4" aboveTabBarFooter>
        {historyFeedSkeleton(propertyId)}
      </PageContent>

      {/* Зеркало плавающей кнопки «Настройки» живого экрана (паттерн «CTA
        * над TabBar» #1166): контейнер и оффсеты те же — подмена без
        * сдвига. */}
      <div className="pointer-events-none fixed inset-x-0 bottom-[max(1.5rem,env(safe-area-inset-bottom))] max-desktop:bottom-[calc(72px_+_max(12px,env(safe-area-inset-bottom))_+_12px)] z-40 flex justify-center">
        <div className="pointer-events-auto mx-auto flex w-full max-w-column justify-center">
          <Button type="button" disabled>
            Настройки
          </Button>
        </div>
      </div>
    </>
  );
}
