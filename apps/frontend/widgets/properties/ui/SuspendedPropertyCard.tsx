'use client';

import type { JSX } from 'react';
import type { SuspendedSharedProperty } from '@/features/properties';
import { PropertyCard } from './PropertyCard';
import { LockSmall } from '@/shared/assets/icons';
import styles from './SuspendedPropertyCard.module.css';

export type SuspendedPropertyCardProps = {
  readonly placeholder: SuspendedSharedProperty;
  readonly onReason: (placeholder: SuspendedSharedProperty) => void;
};

/**
 * Блюр-карточка подвесшего чужого объекта (карта #692, тикет #702; Figma
 * 2213-99113) в списке-хабе: объект, скрытый тарифным лимитом получателя,
 * рисуется как ОБЫЧНАЯ карточка с теми же правилами и полями — настоящие
 * название, адрес и ряд владельца (решение владельца по итогам обхода
 * #756), аватар-заглушка — у объектов сейчас нет фото, — целиком под
 * blur(4px); сверху отдельным слоем — белая плашка с замком, «Объект
 * недоступен» и кнопкой «Узнать причину». Тап открывает шит причины —
 * ссылки на деталь объекта нет (nonInteractive), доступ приостановлен.
 */
export function SuspendedPropertyCard({
  placeholder,
  onReason,
}: SuspendedPropertyCardProps): JSX.Element {
  const property = {
    id: placeholder.propertyId,
    name: placeholder.name,
    // Тип в карточке не рендерится; контракт Property его требует.
    type: 'apartment' as const,
    address: placeholder.address,
    attributes: {},
    status: 'active' as const,
    members_count: 0,
    created_at: '',
    pinned_at: null,
    access: { role: placeholder.accessRole, ownerName: placeholder.ownerName },
  };
  return (
    <button
      type="button"
      className={styles.root}
      data-testid="suspended-property-card"
      onClick={() => onReason(placeholder)}
    >
      <span className={styles.blurLayer} aria-hidden>
        <PropertyCard property={property} nonInteractive />
      </span>
      <span className={styles.plaque}>
        <span className={styles.plaqueHead}>
          <LockSmall className={styles.lock} aria-hidden />
          <span className={styles.plaqueTitle}>Объект недоступен</span>
        </span>
        <span className={styles.plaqueButton}>Узнать причину</span>
      </span>
    </button>
  );
}
