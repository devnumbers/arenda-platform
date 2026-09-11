'use client';

import type { JSX } from 'react';
import { BoldUser } from '@/shared/assets/icons';
import { ContactRowButton } from '@/entities/contact';
import type { Contact } from '@/entities/contact';
import type { RentalTenant } from '@/entities/rental';
import { rentalTenantTitle } from '@/features/rentals';
import { useKeyboardActivation } from '@/shared/lib/hooks/useKeyboardActivation';

type PropertyContactsBlockProps = {
  /** Арендатор текущей аренды (бейдж-подпись роли «Арендатор»). */
  readonly tenant: RentalTenant | null;
  readonly contacts: ReadonlyArray<Contact>;
  readonly onOpenContact: (contactId: string) => void;
};

/** Строка арендатора — та же анатомия, что ContactRowButton (круг 44
 * с BoldUser, имя 16/18, подпись роли 14/16 серым), но данные приходят
 * из аренды, а не из книги контактов (тикет #589, Figma 1185:40820:
 * «Максим — Арендатор»). div с role=button, как канонные строки:
 * клавиатурную активацию несёт общий хук. */
function TenantRowButton({
  tenant,
  onSelect,
}: {
  readonly tenant: RentalTenant;
  readonly onSelect: () => void;
}): JSX.Element {
  const activatorProps = useKeyboardActivation({ onSelect });
  return (
    <div
      {...activatorProps}
      className="flex w-full cursor-pointer items-center gap-2 py-2 text-left outline-none transition-opacity hover:opacity-80 active:opacity-80 focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--dl-surface-muted)]"
      data-testid="property-tenant-row"
    >
      <span
        aria-hidden
        className="flex h-11 w-11 shrink-0 items-center justify-center rounded-pill bg-surface shadow-[0_0_0_2.5px_var(--dl-surface-muted)]"
      >
        <BoldUser className="h-6 w-6" />
      </span>
      <span className="flex min-w-0 flex-1 flex-col justify-center gap-1">
        <span className="truncate text-base font-medium text-content">
          {rentalTenantTitle(tenant)}
        </span>
        <span className="truncate text-sm text-content-secondary">Арендатор</span>
      </span>
    </div>
  );
}

/**
 * Заполненная секция «Контакты» (тикет #589, Figma 1185:40820): первым —
 * арендатор из текущей аренды с подписью роли, ниже — контакты объекта
 * (канонная ContactRowButton на серой карточке — белые круги). Тап по
 * строке — карточка контакта; шапка секции ведёт в книгу объекта.
 */
export function PropertyContactsBlock({
  tenant,
  contacts,
  onOpenContact,
}: PropertyContactsBlockProps): JSX.Element {
  return (
    <div className="flex flex-col px-3 pb-6 pt-4" data-testid="property-contacts-block">
      {tenant !== null && (
        <TenantRowButton
          tenant={tenant}
          onSelect={() => onOpenContact(tenant.contactId)}
        />
      )}
      {contacts.map((contact) => (
        <ContactRowButton
          key={contact.id}
          contact={contact}
          surface="muted"
          onSelect={() => onOpenContact(contact.id)}
        />
      ))}
    </div>
  );
}
