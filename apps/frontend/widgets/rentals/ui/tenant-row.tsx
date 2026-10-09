'use client';

import type { JSX } from 'react';
import { UserAvatar } from '@/shared/ui/design';
import { PaymentRowButton } from '@/entities/payment';

/** Строка арендатора секции «Арендатор» (Figma 1232:62429): белый круг
 * на серой карточке — фото контакта-арендатора из книги (ADR 0065,
 * решение #1286) или BoldUser, имя и телефон. Общая детализаций —
 * текущей (#531) и завершённой (#535). */
export function TenantRow({
  tenantName,
  phone,
  photoUrl,
  onSelect,
}: {
  readonly tenantName: string;
  readonly phone?: string;
  readonly photoUrl?: string | null;
  readonly onSelect?: () => void;
}): JSX.Element {
  return (
    <PaymentRowButton
      variant="gray"
      className="px-3 py-2"
      categoryIcon={<UserAvatar variant="muted" photoUrl={photoUrl} />}
      title={tenantName}
      subtitle={phone}
      onSelect={onSelect}
    />
  );
}
