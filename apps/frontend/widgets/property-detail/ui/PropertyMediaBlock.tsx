import type { JSX } from 'react';
import { BoldHome } from '@/shared/assets/icons';

export type PropertyMediaBlockProps = {
  readonly name: string;
  readonly address: string;
};

/** Медиа-блок детали объекта (Figma 1186:44997, решение владельца
 * 11.09): фото-круг 96 — пока только плейсхолдер (серый круг с домом;
 * рендер реального фото решается на приёмке #588), имя 28/32 SemiBold и
 * адрес 14/16 серым, между ними 12px. m-0 обязателен: легаси-маргин h1
 * (0.67em) иначе добавляет ~19px сверху и снизу (урок HubTitle,
 * DESIGN.md §2). Внутренний py-48: pt-6 поверх pt-6 PageContent даёт 48
 * до круга. */
export function PropertyMediaBlock({
  name,
  address,
}: PropertyMediaBlockProps): JSX.Element {
  return (
    <section className="flex flex-col items-center px-0 pt-6 text-center">
      <span
        className="flex h-24 w-24 items-center justify-center rounded-pill bg-surface-muted"
        aria-hidden
      >
        <BoldHome className="h-10 w-10 text-[#D3D7D9]" />
      </span>
      <h1 className="m-0 mt-6 max-w-[345px] text-2xl font-semibold leading-8 text-content">
        {name}
      </h1>
      <p className="m-0 mt-3 max-w-[345px] text-sm leading-4 text-content-secondary">{address}</p>
    </section>
  );
}
