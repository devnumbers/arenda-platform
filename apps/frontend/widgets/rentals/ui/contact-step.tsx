'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import { useRouter } from 'next/navigation';
import {
  Add,
  BoldUser,
  Edit,
  Kebab,
  TrashBin,
} from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { notify } from '@/shared/lib/notifications';
import {
  ContactRowButton,
  contactSortByName,
  groupContactsByLetter,
} from '@/entities/contact';
import type { Contact } from '@/entities/contact';
import { useContacts, useContact, useDeleteContact } from '@/features/contacts';
import {
  Button,
  ConfirmDialog,
  IconButton,
  ListRow,
  Menu,
  MenuContent,
  MenuItem,
  MenuTrigger,
} from '@/shared/ui/design';
import { RentalContactBookSkeleton } from './rental-skeletons';
import { WizardHeading } from './wizard-chrome';

/**
 * Шаг 4 «Контакт арендатора» (Figma 1270:46738 / 1285:54887 / 1285:54989 /
 * 1419:27158): выбранному контакту — строка с кебабом «Открыть / Изменить /
 * Удалить» (меню макета 1285:54989; Открыть — карточка контакта, Изменить —
 * форма правки, Удалить — контакт удаляется из книги и отвязывается от
 * аренд и объектов, шит подтверждения 1419:27158); без выбора — строка
 * «Создать контакт» (ветвь ведёт в форму #509 с ?pick=rental и возвратом в
 * визард) и книга объекта: серая карточка с алфавитными группами (канон
 * «Контактов объекта» #508, макет 1539:83613), тап по строке выбирает
 * арендатора и сворачивает список. Арендатор необязателен.
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
  const [picking, setPicking] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);

  const contactsQuery = useContacts(propertyId);
  const tenantQuery = useContact(contactId ?? '');
  const deleteContact = useDeleteContact(contactId ?? '');

  // Выбранный арендатор реально существует, только когда его карточка
  // загрузилась: id из черновика может указывать на удалённый контакт
  // (книгу стерли, откат сида) — тогда шаг деградирует к списку книги,
  // а не к «осиротевшему» кебабу. picking — тап по выбранному, чтобы
  // сменить его.
  const selectedTenant =
    contactId !== undefined && !picking && !tenantQuery.isError
      ? tenantQuery.data
      : undefined;

  const openCreateBranch = (): void => {
    router.push(
      `${ROUTES.propertyContactNew(propertyId)}?role=${encodeURIComponent('Арендатор')}&pick=rental`,
    );
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
      <WizardHeading title="Контакт арендатора" />
      <div className="flex flex-col gap-6 px-6 pt-6">
        {/* Строка создания видна, пока арендатор не выбран (макет
            1270:46738): в выбранном состоянии (1285:54887) макет показывает
            только строку арендатора с кебабом. */}
        {selectedTenant === undefined && (
          <ListRow
            leading={
              <span
                aria-hidden
                className="flex h-11 w-11 items-center justify-center rounded-pill bg-surface-muted shadow-[0_0_0_2.5px_var(--dl-surface)]"
              >
                <Add className="h-6 w-6" />
              </span>
            }
            title="Создать контакт"
            onSelect={openCreateBranch}
            className="-mx-6"
          />
        )}

        {selectedTenant === undefined ? (
          <BookList query={contactsQuery} onPick={(picked) => {
            onContactChange(picked);
            setPicking(false);
          }} />
        ) : (
          <div className="flex items-center gap-1">
            <ContactRowButton
              contact={selectedTenant}
              surface="white"
              className="min-w-0 flex-1"
              /* Тап по выбранному контакту снова раскрывает книгу —
                  не разрушительный способ сменить арендатора. */
              onSelect={() => setPicking(true)}
            />
            <Menu>
              <MenuTrigger asChild>
                <IconButton icon={<Kebab className="h-6 w-6" />} label="Меню контакта" />
              </MenuTrigger>
              <MenuContent collisionPadding={24}>
                <MenuItem
                  icon={<BoldUser className="h-6 w-6" />}
                  onSelect={() => router.push(ROUTES.propertyContact(propertyId, selectedTenant.id))}
                >
                  Открыть
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

/** Книга объекта (канон #508): серая карточка с алфавитными группами,
 * сортировка по имени А→Я; тап по строке выбирает арендатора. */
function BookList({
  query,
  onPick,
}: {
  readonly query: ReturnType<typeof useContacts>;
  readonly onPick: (contactId: string) => void;
}): JSX.Element {
  const contacts = query.data ?? [];

  if (query.isPending) {
    // Паритет книги (#607): та же карточка с группами, без своей вставки —
    // контейнер шага приносит px-6.
    return <RentalContactBookSkeleton />;
  }
  if (query.isError) {
    return (
      <div className="flex flex-col items-center gap-4 pt-6">
        <p className="text-center text-base leading-[18px] text-content-secondary">
          Не удалось загрузить контакты
        </p>
        <Button variant="secondary" size="small" onClick={() => void query.refetch()}>
          Повторить
        </Button>
      </div>
    );
  }
  if (contacts.length === 0) {
    // Книга пуста — только «Создать контакт» (макет 1270:46738).
    return <div />;
  }

  const groups = groupContactsByLetter(contactSortByName(contacts, 'asc'));

  return (
    <section
      aria-label="Контакты объекта: выберите арендатора"
      className="flex flex-col gap-4 rounded-card bg-surface-muted pb-3 pl-5 pr-4 pt-6"
    >
      {groups.map((group) => (
        <div key={group.letter} className="flex flex-col">
          <span aria-hidden className="pl-2 text-base font-medium text-content-tertiary">
            {group.letter}
          </span>
          <div className="flex flex-col">
            {group.contacts.map((contact: Contact) => (
              <ContactRowButton
                key={contact.id}
                contact={contact}
                surface="muted"
                onSelect={() => onPick(contact.id)}
              />
            ))}
          </div>
        </div>
      ))}
    </section>
  );
}
