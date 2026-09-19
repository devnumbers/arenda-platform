'use client';

import { useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import {
  ArrowLeft,
  BoldHome,
  BoldUser,
  Check,
  Copy,
  SmallArrowRight,
} from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { notify } from '@/shared/lib/notifications';
import { contactFullName, type Contact } from '@/entities/contact';
import type { Property } from '@/entities/property';
import { useContact, useDeleteContact } from '@/features/contacts';
import { canMutateProperty, useProperty } from '@/features/properties';
import { useCopiedHint } from '@/shared/lib/hooks/use-copied-hint';
import { ConfirmDialog, IconButton, PageContent, TopNav, TopNavTitle } from '@/shared/ui/design';
import { contactValueRows } from '../lib/contact-detail-model';
import { ContactKebabMenu } from './contact-kebab-menu';
import { ContactDetailSkeleton, ContactsErrorCard } from './contacts-states';

/**
 * Экран «Контакт» (#510, макеты 1285:55112 / 1424:54725): карточка книги
 * контактов — серый круг-аватар, карточка с именем и ролью (переход на
 * правку) и контактными значениями с копированием в буфер, карточка
 * «Объект» — только когда контакт привязан (решение владельца 2026-09-03),
 * «Заметка» отдельной секцией. Правка и удаление — из кебаб-меню шапки
 * тому, кому можно мутировать (как на списке: не смотрящий и не архив);
 * удаление — через confirm дизайн-слоя (#505), после — возврат на список
 * с инвалидацией.
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
      ? canMutateProperty(propertyQuery.isSuccess ? propertyQuery.data : undefined)
      : contact?.propertyId !== undefined
        ? canMutateProperty(boundProperty)
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
            <ContactKebabMenu
              onEdit={() => router.push(editHref)}
              onDelete={() => setDeleteConfirmOpen(true)}
            />
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

/** Тело карточки (1285:55112): аватар, карточка «имя+роль» со строками
 * значений, «Объект» (только при привязке), «Заметка». */
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
  const router = useRouter();
  const rows = contactValueRows(contact);
  const boundPropertyId = contact.propertyId;

  return (
    <div className="flex flex-col gap-6 px-6 pb-8 pt-2">
      <div className="flex justify-center">
        <div aria-hidden className="flex h-24 w-24 items-center justify-center rounded-full bg-surface-muted">
          <BoldUser className="h-13 w-13 text-content-tertiary" />
        </div>
      </div>

      <section className="rounded-card bg-surface-muted px-6 py-4">
        {/* Правка — по мутационному доступу (ADR 0028): смотрящему шеврон
         * не рисуется, карточка имени статична. */}
        {canEdit ? (
          <button
            type="button"
            onClick={() => router.push(editHref)}
            className="flex w-full cursor-pointer items-center gap-2 pb-6 text-left outline-none focus-visible:ring-2 focus-visible:ring-primary"
            aria-label={`Изменить контакт ${contactFullName(contact)}`}
          >
            <HeadingText contact={contact} />
            <SmallArrowRight className="h-6 w-6 shrink-0 text-content-secondary" />
          </button>
        ) : (
          <div className="flex w-full items-center gap-2 pb-6">
            <HeadingText contact={contact} />
          </div>
        )}

        {rows.length > 0 && (
          <div className="flex flex-col">
            {rows.map((row) => (
              <ContactValueRowView key={row.key} row={row} />
            ))}
          </div>
        )}
      </section>

      {/* «Объект» — только у привязанного контакта (решение владельца
       * 2026-09-03): непривязанный — «общий», пункта у него нет. Вся
       * карточка — одна зона клика, ведёт на объект (решение владельца
       * в правке #510). */}
      {boundPropertyId !== undefined && (
        <button
          type="button"
          onClick={() => router.push(ROUTES.property(boundPropertyId))}
          aria-label="Открыть объект"
          className="w-full cursor-pointer rounded-card bg-surface-muted px-6 py-4 text-left transition-opacity outline-none hover:opacity-80 focus-visible:ring-2 focus-visible:ring-primary active:opacity-80"
        >
          <span className="flex w-full items-center gap-2">
            <span className="min-w-0 flex-1 text-xl font-semibold leading-6 text-content">
              Объект
            </span>
            <SmallArrowRight className="h-6 w-6 shrink-0 text-content-secondary" />
          </span>
          {boundProperty !== undefined && (
            <span className="flex items-center gap-3 pt-2">
              <span
                aria-hidden
                className="flex h-11 w-11 shrink-0 items-center justify-center overflow-hidden rounded-full bg-white"
              >
                {boundProperty.photos?.[0]?.url !== undefined ? (
                  <img
                    src={boundProperty.photos[0].url}
                    alt=""
                    className="h-full w-full object-cover"
                  />
                ) : (
                  <BoldHome className="h-6 w-6 text-content-tertiary" />
                )}
              </span>
              <span className="flex min-w-0 flex-col gap-1">
                <span className="truncate text-base font-medium leading-[18px] text-content">
                  {boundProperty.name}
                </span>
                <span className="truncate text-sm leading-4 text-content-secondary">
                  {boundProperty.address}
                </span>
              </span>
            </span>
          )}
        </button>
      )}

      <section className="flex flex-col gap-2">
        <span className="text-base font-medium leading-[18px] text-content">Заметка</span>
        <div className="min-h-[92px] whitespace-pre-wrap rounded-2xl bg-surface-muted px-[18px] py-4 text-base leading-[18px] text-content">
          {contact.note}
        </div>
      </section>
    </div>
  );
}

/** Строка значения с копированием (1285:55112): значение 16/500, подпись
 * 14 серым, справа кнопка копирования — иконка меняется на галочку на пару
 * секунд (канон useCopiedHint). */
function ContactValueRowView({
  row,
}: {
  readonly row: ReturnType<typeof contactValueRows>[number];
}): JSX.Element {
  const { copied, copy } = useCopiedHint();

  return (
    <div className="flex items-center gap-2 py-3">
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

/** Имя и роль на карточке — общий блок тапабельной и статичной версий. */
function HeadingText({ contact }: { readonly contact: Contact }): JSX.Element {
  return (
    <span className="min-w-0 flex-1">
      <span className="block text-xl font-semibold leading-6 text-content">
        {contactFullName(contact)}
      </span>
      {contact.role !== '' && (
        <span className="mt-2 block text-sm leading-4 text-content-secondary">
          {contact.role}
        </span>
      )}
    </span>
  );
}
