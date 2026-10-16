import type { JSX } from 'react';
import type { RentalTermsRow } from '@/features/rentals';

/** Строка условий «метка — значение» (Figma 1302:53784): сетка из двух равных
 * колонок с зазором 12 — значение всегда стоит в фиксированной правой колонке
 * (в макете x=154.5 при ширине контента 297), метка переносится в своей.
 * Общая экрана «Условия аренды» (#531), детализации и «Прошлых аренд» (#535). */
export function TermRow({ label, value }: { readonly label: string; readonly value: string }): JSX.Element {
  return (
    <div className="grid grid-cols-2 gap-3">
      <span className="text-sm leading-4 text-content-secondary">{label}</span>
      <span className="text-sm leading-4 text-content">{value}</span>
    </div>
  );
}

/** Строки условий единым столбцом с зазором 8 (карточки секций аренды). */
export function TermRows({ rows }: { readonly rows: ReadonlyArray<RentalTermsRow> }): JSX.Element {
  return (
    <div className="flex flex-col gap-2 px-6">
      {rows.map((row) => (
        <TermRow key={row.label} label={row.label} value={row.value} />
      ))}
    </div>
  );
}
