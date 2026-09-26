'use client';

import type { JSX } from 'react';
import { BoldUser } from '@/shared/assets/icons';
import { CircleIcon } from '@/shared/ui/design';
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
        <CircleIcon variant="muted" aria-hidden>
          <BoldUser className="h-6 w-6 text-content" />
        </CircleIcon>
      }
      title={tenantName}
      subtitle={phone}
      onSelect={onSelect}
    />
  );
}
