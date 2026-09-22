import type { ReactNode, JSX } from 'react';
import { PropertyAvatar } from '@/entities/property';

export type PropertyMediaBlockProps = {
  readonly name: string;
  readonly address: string;
  /** Пилюли шапки propertyHeaderPills (#773) под адресом: «В архиве» —
   * её получает и владелец, и/или роль доступа (Figma 2200-97365). */
  readonly children?: ReactNode;
};

/** Медиа-блок детали объекта (Figma 1186:44997, решение владельца
 * 11.09): фото-круг 96 — пока только плейсхолдер (серый круг с домом;
 * рендер реального фото решается на приёмке #588; поверхность hero
 * канона PropertyAvatar), имя 28/32 SemiBold и адрес 14/16 серым, между
 * ними 12px. m-0 обязателен: легаси-маргин h1 (0.67em) иначе добавляет
 * ~19px сверху и снизу (урок HubTitle, DESIGN.md §2). Внутренний py-48:
 * pt-6 поверх pt-6 PageContent даёт 48 до круга. */
export function PropertyMediaBlock({
  name,
  address,
  children,
}: PropertyMediaBlockProps): JSX.Element {
  return (
    <section className="flex flex-col items-center px-0 pt-6 text-center">
      <div aria-hidden>
        <PropertyAvatar surface="hero" />
      </div>
      <h1 className="m-0 mt-6 max-w-[345px] text-2xl font-semibold leading-8 text-content">
        {name}
      </h1>
      <p className="m-0 mt-3 max-w-[345px] text-sm leading-4 text-content-secondary">{address}</p>
      {children}
    </section>
  );
}
