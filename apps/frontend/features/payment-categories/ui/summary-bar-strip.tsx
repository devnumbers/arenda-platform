'use client';

import type { JSX } from 'react';
import type { SummaryBarSegment } from '../lib/summary-bar';

/** Пилюля полосы без операций в периоде (Figma 1510-77101): серая #D3D7D9. */
const EMPTY_BAR_COLOR = '#D3D7D9';

/**
 * Полоса-разбивка 6px из «пилюль» категорий (Figma 1510-77101): по одной
 * на категорию, зазор 2px держит раздельно даже совпадающие цвета
 * каталога, ширина пропорциональна сумме (flex-grow по весам). Без
 * операций в периоде — единственная серая пилюля. Канон для всех сводок
 * операций: карточки «Расходы/Доходы» и секция детали объекта (#589).
 */
export function SummaryBarStrip({
  segments,
}: {
  readonly segments: ReadonlyArray<SummaryBarSegment>;
}): JSX.Element {
  return (
    <div className="flex h-1.5 w-full gap-[2px]">
      {(segments.length > 0 ? segments : [{ color: EMPTY_BAR_COLOR, weight: 1 }]).map(
        (segment, index) => (
          <span
            key={`${segment.color}-${index}`}
            className="h-full rounded-pill"
            style={{ backgroundColor: segment.color, flexGrow: segment.weight, flexBasis: 0 }}
          />
        ),
      )}
    </div>
  );
}
