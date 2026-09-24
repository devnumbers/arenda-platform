'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import { useRouter } from 'next/navigation';
import {
  AccountSetting,
  Add,
  BoldUser,
  Change,
  Edit,
  Kebab,
  TrashBin,
} from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { notify } from '@/shared/lib/notifications';
import { ContactRowButton } from '@/entities/contact';
import { useContacts, useContact, useDeleteContact } from '@/features/contacts';
import {
  ConfirmDialog,
  IconButton,
  ListRow,
  Menu,
  MenuContent,
  MenuItem,
  MenuTrigger,
} from '@/shared/ui/design';
import { WizardHeading } from './wizard-chrome';

/**
 * Шаг 4 «Контакт арендатора» (Figma 1855:63385 / 64129 / 64385): заголовок
 * «Добавьте контакт арендатора» + подзаголовок; без выбора — два действия,
 * «Выбрать контакт» (экран выбора, отдельный маршрут 1855:64129) и «Создать
 * контакт» (ветвь ведёт в форму #509 с ?pick=rental и возвратом в визард);
 * с выбранным — строка арендатора с кебабом из четырёх пунктов (1855:64385,
 * меню 1855:64502 — иконки семейства R: AccountSetting / Change / Edit /
 * TrashBin): Открыть — карточка контакта, Выбрать другой — экран выбора,
 * Изменить — форма правки, Удалить — контакт удаляется из книги и
 * отвязывается от аренд и объектов (шит подтверждения; тап по строке —
 * тот же Открыть, решение владельца 2026-09-22). Строка «Создать контакт»
 * видна и в выбранном состоянии; пустая книга (макета нет, решение
 * владельца) — только «Создать контакт». Арендатор необязателен.
 */

export type ContactStepProps = {
  readonly propertyId: string;
  /** Выбранный арендатор; undefined — не выбран. */
  readonly contactId: string | undefined;
  readonly onContactChange: (contactId: string | undefined) => void;
};

export function ContactStep({
  propertyId,
  contactId,
  onContactChange,
}: ContactStepProps): JSX.Element {
  const router = useRouter();
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);

  // Книга нужна шагу, чтобы отличить пустую (одна строка создания) от
  // непустой (два действия); сам список живёт на экране выбора.
  const contactsQuery = useContacts(propertyId);
  const tenantQuery = useContact(contactId ?? '');
  const deleteContact = useDeleteContact(contactId ?? '');

  // Выбранный арендатор реально существует, только когда его карточка
  // загрузилась: id из черновика может указывать на удалённый контакт
  // (книгу стерли, откат сида) — тогда шаг деградирует к состоянию без
  // выбора, а не к «осиротевшему» кебабу.
  const selectedTenant =
    contactId !== undefined && !tenantQuery.isError ? tenantQuery.data : undefined;

  // Сбой чтения книги не блокирует шаг: экран выбора покажет свой
  // повтор, а «Создать контакт» работает всегда. Строка «Выбрать
  // контакт» — только при подтверждённой непустой книге (пустая книга —
  // единственная строка создания без мерцания, решение владельца).
  const showPickerRow =
    !contactsQuery.isPending && !contactsQuery.isError && contactsQuery.data.length > 0;

  const openPicker = (): void => {
    router.push(ROUTES.propertyRentalNewContact(propertyId));
  };

  const openCreateBranch = (): void => {
    router.push(
      `${ROUTES.propertyContactNew(propertyId)}?role=${encodeURIComponent('Арендатор')}&pick=rental`,
    );
  };

  const openContactCard = (contactId: string): void => {
    router.push(ROUTES.propertyContact(propertyId, contactId));
  };

  const handleDelete = (): void => {
    void (async () => {
      try {
        await deleteContact.mutateAsync();
        onContactChange(undefined);
        setDeleteDialogOpen(false);
        notify.scenarios.propertyContacts.deleted();
      } catch (error: unknown) {
        notify.scenarios.propertyContacts.deleteError(error);
      }
    })();
  };

  return (
    <>
      <WizardHeading
        title="Добавьте контакт арендатора"
        subtitle="Выберите существующий или создайте новый контакт"
      />
      <div className="flex flex-col gap-4 px-6 pt-6">
        {selectedTenant !== undefined && (
          <div className="flex items-center gap-1">
            <ContactRowButton
              contact={selectedTenant}
              surface="white"
              className="min-w-0 flex-1"
              /* Тап по выбранному — карточка контакта (решение владельца
                  2026-09-22); смена арендатора — через «Выбрать другой». */
              onSelect={() => openContactCard(selectedTenant.id)}
            />
            <Menu>
              <MenuTrigger asChild>
                <IconButton icon={<Kebab className="h-6 w-6" />} label="Меню контакта" />
              </MenuTrigger>
              <MenuContent collisionPadding={24}>
                {/* Иконки пунктов — семейство R по меню макета
                    (1855:64502): Icon/R/AccountSetting, Icon/R/Change,
                    Icon/R/Edit, Icon/R/TrashBin. */}
                <MenuItem
                  icon={<AccountSetting className="h-6 w-6" />}
                  onSelect={() => openContactCard(selectedTenant.id)}
                >
                  Открыть
                </MenuItem>
                <MenuItem icon={<Change className="h-6 w-6" />} onSelect={openPicker}>
                  Выбрать другой
                </MenuItem>
                <MenuItem
                  icon={<Edit className="h-6 w-6" />}
                  onSelect={() => router.push(ROUTES.propertyContactEdit(propertyId, selectedTenant.id))}
                >
                  Изменить
                </MenuItem>
                <MenuItem
                  icon={<TrashBin className="h-6 w-6" />}
                  onSelect={() => setDeleteDialogOpen(true)}
                >
                  Удалить
                </MenuItem>
              </MenuContent>
            </Menu>
          </div>
        )}

        {/* В выбранном состоянии (1855:64385) строка создания остаётся
            видимой; в состоянии без выбора (1855:63385) идут два действия,
            а в пустой книге (решение владельца) — только «Создать
            контакт». Пока книга едет — рисуем создание (безопасный
            дефолт: контакты появятся — добавится «Выбрать контакт»). */}
        {selectedTenant === undefined && showPickerRow && (
          <ListRow
            leading={<ActionRowIcon icon={<BoldUser className="h-6 w-6" />} />}
            title="Выбрать контакт"
            onSelect={openPicker}
            className="-mx-6"
          />
        )}
        <ListRow
          leading={<ActionRowIcon icon={<Add className="h-6 w-6" />} />}
          title="Создать контакт"
          onSelect={openCreateBranch}
          className="-mx-6"
        />
      </div>

      <ConfirmDialog
        open={deleteDialogOpen}
        onOpenChange={setDeleteDialogOpen}
        title="Удалить контакт?"
        description="Контакт будет удален и отвязан от аренды и объектов"
        confirmLabel="Удалить"
        confirmVariant="danger"
        pending={deleteContact.isPending}
        onConfirm={handleDelete}
      />
    </>
  );
}

/** Кружок 44 действий шага (1855:63385): приглушённая подложка с белым
 * кольцом — та же анатомия, что у аватара ContactRowButton. */
function ActionRowIcon({ icon }: { readonly icon: JSX.Element }): JSX.Element {
  return (
    <span
      aria-hidden
      className="flex h-11 w-11 items-center justify-center rounded-pill bg-surface-muted shadow-[0_0_0_2.5px_var(--dl-surface)]"
    >
      {icon}
    </span>
  );
}
