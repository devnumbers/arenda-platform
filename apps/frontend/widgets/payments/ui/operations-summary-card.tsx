'use client';

import type { JSX } from 'react';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import { SummaryBarStrip } from '@/features/payment-categories';
import type { SummaryBarSegment } from '@/features/payment-categories';


/**
 * Карточка сводки (Figma 1510-77101, EL-3091ce92): серая карточка radius 24,
 * сумма 16/500, подпись 14/400, полоса 6px из «пилюль» категорий — по одной
 * на категорию, зазор 2px держит раздельно даже совпадающие цвета каталога,
 * ширина пропорциональна сумме (flex-grow по весам). Без операций в периоде —
 * единственная серая пилюля #D3D7D9. С `onOpen` — кнопка на экран направления
 * (#475), как заголовки секций «Платежей объекта»; глобальная лента (#541)
 * делает карточки кнопками на страницы направления (#548 — отменяет решение
 * #539 о некликабельных карточках), а страница направления (#548) рендерит
 * карточку без onOpen — статичный div без интерактивных состояний.
 */
export function OperationsSummaryCard({
  label,
  totalKopecks,
  segments,
  openLabel,
  onOpen,
}: {
  readonly label: string;
  readonly totalKopecks: number | undefined;
  readonly segments: ReadonlyArray<SummaryBarSegment>;
  /** Aria-label кнопки; у некликабельной карточки не нужен. */
  readonly openLabel?: string;
  readonly onOpen?: () => void;
}): JSX.Element {
  const body = (
    <>
      <div className="flex flex-col gap-0.5">
        <span className="text-base font-medium text-content">
          {totalKopecks === undefined ? '—' : formatMoneyKopecks(totalKopecks)}
        </span>
        <span className="text-sm text-content">{label}</span>
      </div>
      <div className="mt-4">
        <SummaryBarStrip segments={segments} />
      </div>
    </>
  );
  if (onOpen === undefined) {
    return <div className="min-w-0 flex-1 rounded-card bg-surface-muted px-6 pb-6 pt-5">{body}</div>;
  }
  return (
    <button
      type="button"
      onClick={onOpen}
      aria-label={openLabel}
      className="min-w-0 flex-1 cursor-pointer rounded-card bg-surface-muted px-6 pb-6 pt-5 text-left outline-none transition-opacity hover:opacity-80 active:opacity-80 focus-visible:ring-4 focus-visible:ring-primary"
    >
      {body}
    </button>
  );
}
