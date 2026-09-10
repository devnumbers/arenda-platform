import type { JSX } from 'react';
import type { RentalTermsRow } from '@/features/rentals';

/** Строка условий «метка — значение» (Figma EL-c094fdcc): серая метка 14/16,
 * значение тем же кеглем; 12px между колонками. Общая детализации (#531)
 * и экранов «Прошлых аренд» (#535). */
export function TermRow({ label, value }: { readonly label: string; readonly value: string }): JSX.Element {
  return (
    <div className="flex gap-3">
      <span className="shrink-0 text-sm leading-4 text-content-secondary">{label}</span>
      <span className="min-w-0 text-sm leading-4 text-content">{value}</span>
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
