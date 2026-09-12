'use client';

import type { JSX } from 'react';
import type { ApartmentSummaryRow } from '../lib/apartment-summary';

/**
 * Сводка характеристик в секции «<Тип объекта>» (тикет #589, Figma
 * 1185:40820): строки «метка — значение» 14/16 в две равные колонки
 * (сетка принятого шаблона «Об объекте», 1550:97124): Комнат, Общая
 * площадь, Этаж, Ремонт. Тап — шапкой секции в «Об объекте».
 */
export function PropertyApartmentBlock({
  rows,
}: {
  readonly rows: ReadonlyArray<ApartmentSummaryRow>;
}): JSX.Element {
  return (
    <dl className="flex flex-col gap-2 px-6 pb-6 pt-4" data-testid="property-apartment-block">
      {rows.map((row) => (
        <div key={row.label} className="grid grid-cols-2 gap-3 text-sm leading-4">
          <dt className="text-content-secondary">{row.label}</dt>
          <dd className="text-content">{row.value}</dd>
        </div>
      ))}
    </dl>
  );
}
