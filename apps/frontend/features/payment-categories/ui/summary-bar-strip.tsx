'use client';

import type { JSX } from 'react';
import type { SummaryBarSegment } from '../lib/summary-bar';

/** Пилюля полосы без операций в периоде (Figma 1510-77101): серая #D3D7D9. */
const EMPTY_BAR_COLOR = '#D3D7D9';

/**
 * Полоса-разбивка 6px из «пилюль» (Figma 1510-77101): по одной на цвет —
 * слияние категорий одного цвета делает билдер summaryBarSegments (#1075,
 * решение владельца 2026-10-02), контейнер только раскладывает готовые
 * сегменты: ширина пропорциональна сумме (flex-grow по весам), но у
 * каждой пилюли минимум 6px (min-w-1.5, решение владельца 2026-09-30):
 * крошечная категория остаётся видимой, крупные отдают лишнее
 * пропорционально. Без операций в периоде — единственная серая пилюля.
 * Канон для всех сводок операций: карточки «Расходы/Доходы» и секция
 * детали объекта (#589).
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
            className="h-full min-w-1.5 rounded-pill"
            style={{ backgroundColor: segment.color, flexGrow: segment.weight, flexBasis: 0 }}
          />
        ),
      )}
    </div>
  );
}
