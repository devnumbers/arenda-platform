'use client';

import type { JSX } from 'react';
import { BoldUser } from '@/shared/assets/icons';
import { PaymentRowButton } from '@/entities/payment';

/** Строка арендатора секции «Арендатор» (Figma 1232:62429): белый круг
 * с BoldUser на серой карточке, имя и телефон. Общая детализаций — текущей
 * (#531) и завершённой (#535). */
export function TenantRow({
  tenantName,
  phone,
  onSelect,
}: {
  readonly tenantName: string;
  readonly phone?: string;
  readonly onSelect?: () => void;
}): JSX.Element {
  return (
    <PaymentRowButton
      variant="gray"
      className="px-3 py-2"
      categoryIcon={
        <span
          aria-hidden
          className="flex h-11 w-11 items-center justify-center rounded-pill bg-surface shadow-[0_0_0_2.5px_var(--dl-surface-muted)]"
        >
          <BoldUser className="h-6 w-6 text-content" />
        </span>
      }
      title={tenantName}
      subtitle={phone}
      onSelect={onSelect}
    />
  );
}
