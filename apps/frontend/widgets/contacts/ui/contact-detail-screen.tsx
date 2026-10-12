'use client';

import { useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft, Check, Copy } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { notify } from '@/shared/lib/notifications';
import { type Contact } from '@/entities/contact';
import type { Property } from '@/entities/property';
import { propertyPermissions, propertyTypeIcons } from '@/entities/property';
import {
  useContact,
  useContactPhotoBuster,
  useDeleteContact,
} from '@/features/contacts';
import { useProperty } from '@/features/properties';
import { useCopiedHint } from '@/shared/lib/hooks/use-copied-hint';
import { photoDisplayUrl } from '@/shared/lib/photo';
import { ConfirmDialog, IconButton, PageContent, TopNav, TopNavTitle } from '@/shared/ui/design';
import { contactValueRows } from '../lib/contact-detail-model';
import { ContactDetailHeader } from './contact-detail-header';
import { ContactKebabMenu } from './contact-kebab-menu';
import { ContactSectionCard } from './contact-section-card';
import { ContactDetailSkeleton, ContactsErrorCard } from './contacts-states';

/**
 * Экран «Контакт» (перекомпоновка #1333, макеты 3304:75726 / 3304:76141):
 * серый круг-аватар, ФИО крупно по центру на фоне страницы и роль под ним —
 * статичный блок без тапа на правку (решение карты #1332); строки связи —
 * в карточке «Информация» с шевроном, тап по карточке ведёт на правку;
 * карточка «Объект» — только когда контакт привязан (решение владельца
 * 2026-09-03), «Заметка» отдельной секцией. Кебаб шапки — только удаление,
 * тому, кому можно мутировать (как на списке: не смотрящий и не архив,
 * ADR 0028); удаление — через confirm дизайн-слоя (#505), после — возврат
 * на список с инвалидацией.
 *
 * Режим книги (глобальная страница контактов, propertyId не задан):
 * мутационный вход определяется привязкой самой карточки — без объекта
 * карточка лежит в книге самого актёра (править можно всегда), привязанная
 * правится по доступу её объекта; выход — в корень книги.
 */
export function ContactDetailScreen({
  propertyId,
  contactId,
}: {
  readonly propertyId?: string;
  readonly contactId: string;
}): JSX.Element {
  const router = useRouter();
  const contactQuery = useContact(contactId);
  const propertyQuery = useProperty(propertyId ?? '');
  const deleteContact = useDeleteContact(contactId);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);

  const contact = contactQuery.isSuccess ? contactQuery.data : undefined;
  const boundPropertyQuery = useProperty(contact?.propertyId ?? '');
  const boundProperty = boundPropertyQuery.isSuccess ? boundPropertyQuery.data : undefined;
  const backHref = propertyId !== undefined ? ROUTES.propertyContacts(propertyId) : ROUTES.contacts;
  const editHref =
    propertyId !== undefined
      ? ROUTES.propertyContactEdit(propertyId, contactId)
      : ROUTES.contactEdit(contactId);
  // Мутационный вход — как на списке (#508): смотрящий и архив читают.
  // В режиме книги привязка самой карточки: без объекта — своя книга,
  // с объектом — доступ этого объекта.
  const canMutate =
    propertyId !== undefined
      ? propertyPermissions(propertyQuery.isSuccess ? propertyQuery.data : undefined).canEdit
      : contact?.propertyId !== undefined
        ? propertyPermissions(boundProperty).canEdit
        : contact !== undefined;

  const handleDelete = (): void => {
    setDeleteConfirmOpen(false);
    void deleteContact
      .mutateAsync()
      .then(() => {
        notify.scenarios.propertyContacts.deleted();
        // Стандарт навигации (удаление сущности): goBack — из истории
        // [список, карточка] назад ведёт на список, не на удалённый
        // контакт; router.replace создал бы «мёртвый» Back.
        goBack(router, backHref);
      })
      .catch((error: unknown) => {
        notify.scenarios.propertyContacts.deleteError(error);
      });
  };

  return (
    <>
      <TopNav
        leading={
          <IconButton
            icon={<ArrowLeft />}
            label="Назад"
            onClick={() => goBack(router, backHref)}
          />
        }
        trailing={
          canMutate ? (
            <ContactKebabMenu onDelete={() => setDeleteConfirmOpen(true)} />
          ) : undefined
        }
      >
        <TopNavTitle title="Контакт" />
      </TopNav>

      <PageContent>
        {contactQuery.isPending ? (
          <ContactDetailSkeleton />
        ) : contactQuery.isError || contact === undefined ? (
          <ContactsErrorCard onRetry={() => void contactQuery.refetch()} />
        ) : (
          <ContactCardBody
            editHref={editHref}
            contact={contact}
            boundProperty={boundProperty}
            canEdit={canMutate}
          />
        )}
      </PageContent>

      <ConfirmDialog
        open={deleteConfirmOpen}
        onOpenChange={setDeleteConfirmOpen}
        title="Удалить контакт?"
        description="Контакт будет удален и отвязан от аренды и объектов"
        confirmLabel="Удалить"
        confirmVariant="danger"
        onConfirm={handleDelete}
      />
    </>
  );
}

/** Тело карточки (3304:75726): шапка ФИО+роль на фоне страницы, карточка
 * «Информация» со строками связи, «Объект» (только при привязке),
 * «Заметка». Зазоры по макету: шапка →32, секции →16 (PageContent несёт
 * верхние 24 сам). */
function ContactCardBody({
  editHref,
  contact,
  boundProperty,
  canEdit,
}: {
  readonly editHref: string;
  readonly contact: Contact;
  readonly boundProperty: Property | undefined;
  readonly canEdit: boolean;
}): JSX.Element {
  const rows = contactValueRows(contact);
  const boundPropertyId = contact.propertyId;
  // Выборка из статичного реестра, не вызов: react-hooks/static-components.
  const BoundPropertyGlyph = boundProperty !== undefined ? propertyTypeIcons[boundProperty.type] : undefined;
  // Бастер кэша выдачи (тикет #1229, ADR 0065): правка фото возвращает
  // на карточку с тем же путём выдачи — без ?v=N <img> показал бы прежние
  // байты из кэша браузера до 5 минут.
  const photoBuster = useContactPhotoBuster(contact.id);
  const photoUrl =
    contact.photoUrl !== undefined
      ? photoDisplayUrl(contact.photoUrl, photoBuster)
      : null;

  return (
    <div className="flex flex-col px-6 pb-8">
      {/* Блок ФИО статичен: правка — тап по карточке «Информация»
       * (решение карты #1332). Смотрящему шеврон не рисуется, карточка
       * «Информация» статична (ADR 0028). */}
      <ContactDetailHeader contact={contact} photoUrl={photoUrl} />

      <ContactSectionCard className="mt-8" title="Информация" href={canEdit ? editHref : undefined}>
        {rows.length > 0 && (
          <div className="mt-4 flex flex-col pb-4">
            {rows.map((row) => (
              <ContactValueRowView key={row.key} row={row} />
            ))}
          </div>
        )}
      </ContactSectionCard>

      {/* «Объект» — только у привязанного контакта (решение владельца
       * 2026-09-03): непривязанный — «общий», пункта у него нет. Вся
       * карточка ведёт на объект (решение владельца в правке #510). */}
      {boundPropertyId !== undefined && (
        <ContactSectionCard title="Объект" href={ROUTES.property(boundPropertyId)}>
          {boundProperty !== undefined && (
            <div className="mt-2 flex items-center gap-3 pb-4 pl-6 pr-6">
              <span
                aria-hidden
                className="flex h-11 w-11 shrink-0 items-center justify-center overflow-hidden rounded-full bg-white"
              >
                {boundProperty.photoUrl ? (
                  <img
                    src={boundProperty.photoUrl}
                    alt=""
                    className="h-full w-full object-cover"
                  />
                ) : BoundPropertyGlyph !== undefined ? (
                  <BoundPropertyGlyph className="h-6 w-6 text-content-tertiary" />
                ) : null}
              </span>
              <span className="flex min-w-0 flex-col gap-1">
                <span className="truncate text-base font-medium leading-[18px] text-content">
                  {boundProperty.name}
                </span>
                <span className="truncate text-sm leading-4 text-content-secondary">
                  {boundProperty.address}
                </span>
              </span>
            </div>
          )}
        </ContactSectionCard>
      )}

      <section className="mt-4 flex flex-col gap-2">
        <span className="text-base font-medium leading-[18px] text-content">Заметка</span>
        <div className="min-h-[92px] whitespace-pre-wrap rounded-2xl bg-surface-muted px-[18px] py-4 text-base leading-[18px] text-content">
          {contact.note}
        </div>
      </section>
    </div>
  );
}

/** Строка значения в карточке «Информация» (3304:75726, Row Button):
 * значение 16/500, подпись 14 серым, справа кнопка копирования — иконка
 * меняется на галочку на пару секунд (канон useCopiedHint). Паддинги
 * строки — по макету Row Button: текст 24 от края карточки, кнопка
 * копирования 12 (правый край слота 44). */
function ContactValueRowView({
  row,
}: {
  readonly row: ReturnType<typeof contactValueRows>[number];
}): JSX.Element {
  const { copied, copy } = useCopiedHint();

  return (
    <div className="flex items-center gap-3 py-3 pl-6 pr-3">
      <span className="flex min-w-0 flex-1 flex-col gap-1">
        <span className="truncate text-base font-medium leading-[18px] text-content">
          {row.value}
        </span>
        <span className="truncate text-sm leading-4 text-content-secondary">{row.label}</span>
      </span>
      <IconButton
        icon={copied ? <Check className="h-5 w-5" /> : <Copy className="h-5 w-5" />}
        label={copied ? 'Скопировано' : `Скопировать: ${row.label}`}
        onClick={() => copy(row.value)}
      />
    </div>
  );
}
