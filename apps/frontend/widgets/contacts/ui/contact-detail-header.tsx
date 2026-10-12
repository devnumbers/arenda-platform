import type { JSX } from 'react';
import { contactFullName, type Contact } from '@/entities/contact';
import { UserAvatar } from '@/shared/ui/design';

export type ContactDetailHeaderProps = {
  readonly contact: Contact;
  /** Путь выдачи фото с бастером кэша (#1229); null/undefined — фото нет. */
  readonly photoUrl: string | null | undefined;
};

/**
 * Шапка карточки контакта (макет 3304:75726, тикет #1333): серый круг
 * аватара 96, под ним ФИО крупно по центру (H1/600, 28/32) и роль
 * (14/16, серый #6F787C) — всё на фоне страницы, вне серой секции, без
 * тапа на правку (решение при чартинге карты #1332: блок ФИО статичен,
 * правка — тап по карточке «Информация»). Роль видна, когда заполнена.
 */
export function ContactDetailHeader({
  contact,
  photoUrl,
}: ContactDetailHeaderProps): JSX.Element {
  return (
    <div className="flex flex-col items-center">
      <UserAvatar size="hero" photoUrl={photoUrl} />
      {/* mb-0 гасит легаси-маржин h1 из @layer base (globals: margin
       * 0.67em) — иначе зазор до роли 27px вместо макетных 8. */}
      <h1 className="mt-4 mb-0 text-center text-[28px] font-semibold leading-8 text-content">
        {contactFullName(contact)}
      </h1>
      {contact.role !== '' && (
        <p className="mt-2 text-center text-sm leading-4 text-content-secondary">
          {contact.role}
        </p>
      )}
    </div>
  );
}
