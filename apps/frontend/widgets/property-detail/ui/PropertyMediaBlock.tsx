import type { JSX } from 'react';
import { BoldHome } from '@/shared/assets/icons';

export type PropertyMediaBlockProps = {
  readonly name: string;
  readonly address: string;
};

/** Медиа-блок детали объекта (Figma 1554:98469): фото-круг 96 — пока только
 * плейсхолдер (серый круг с домом; рендер реального фото решается на
 * приёмке #588), имя 24/32 SemiBold по центру и адрес 14/16 серым.
 * Внутренний pt-6 поверх pt-6 PageContent даёт 48 до круга, как в макете. */
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
      <h1 className="mt-6 max-w-[345px] text-2xl font-semibold leading-8 text-content">
        {name}
      </h1>
      <p className="mt-3 max-w-[345px] text-sm leading-4 text-content-secondary">{address}</p>
    </section>
  );
}
