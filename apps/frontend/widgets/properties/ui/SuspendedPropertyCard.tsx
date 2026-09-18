'use client';

import type { JSX } from 'react';
import type { SuspendedSharedProperty } from '@/features/properties';
import { PropertyAvatar } from '@/entities/property';
import { LockSmall } from '@/shared/assets/icons';
import styles from './SuspendedPropertyCard.module.css';

export type SuspendedPropertyCardProps = {
  readonly placeholder: SuspendedSharedProperty;
  readonly onReason: (placeholder: SuspendedSharedProperty) => void;
};

/**
 * Блюр-карточка подвесшего чужого объекта (карта #692, тикет #702; Figma
 * 2213-99113) в списке-хабе: объект, скрытый тарифным лимитом получателя,
 * вместо сноски hidden_shared_count (#158 T4) рисуется карточкой под общим
 * размытием — анатомия обычной карточки (круг-фото, строки) целиком под
 * blur(4px), сверху белая плашка с замком, «Объект недоступен» и кнопкой
 * «Узнать причину». Данных объекта плейсхолдер не несёт; вся карточка —
 * кнопка открытия шита причины (chevron и ссылка на объект, в отличие от
 * обычной карточки, отсутствуют).
 */
export function SuspendedPropertyCard({
  placeholder,
  onReason,
}: SuspendedPropertyCardProps): JSX.Element {
  return (
    <button
      type="button"
      className={styles.root}
      data-testid="suspended-property-card"
      onClick={() => onReason(placeholder)}
    >
      <span className={styles.blurLayer} aria-hidden>
        <PropertyAvatar photoUrl={null} withAttentionDot={false} />
        <span className={styles.info}>
          <span className={styles.lineTitle} />
          <span className={styles.lineAddress} />
        </span>
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
