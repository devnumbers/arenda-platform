'use client';

import { useState, type JSX } from 'react';
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
import { pushPage } from '@/shared/lib/navigation';
import { ContactRowButton } from '@/entities/contact';
import { useContacts, useContact, useDeleteContact } from '@/features/contacts';
import {
  CircleIcon,
  ConfirmDialog,
  IconButton,
  ListRow,
  Menu,
  MenuContent,
  MenuItem,
  MenuTrigger,
} from '@/shared/ui/design';
import { FieldTitle } from './wizard-chrome';

/**
 * Секция «Контакт арендатора» формы «Изменить условия» (решение владельца
 * 28.10.2026, #1623; макеты 40000101:153003 — без выбора, 40000101:153262 —
 * с выбранным): кнопки и логика 1-в-1 с шагом 4 визарда (#807), стили —
 * канон формы. Без выбора — строки «Выбрать контакт» (пикер книги объекта,
 * гейт непустой книги — тот же, что на шаге) и «Создать контакт»; с
 * выбранным — строка арендатора с кебабом меню 1855:64502: Открыть —
 * карточка, Выбрать другой — пикер (?return=terms), Изменить — форма правки
 * карточки, Удалить — карточка удаляется из книги целиком (шит
 * подтверждения шага). Смена арендатора живёт в форме и едет общим диффом
 * (PATCH contactId), действия кебаба над карточкой — сразу, мимо формы.
 * Уход в под-маршрут помечает носитель сессии формы живым (onSubRouteOpen),
 * чтобы значения полей пережили круговой маршрут.
 */

export type TermsContactSectionProps = {
  readonly propertyId: string;
  /** Выбранный арендатор формы; null — не выбран. */
  readonly contactId: string | null;
  readonly onContactChange: (contactId: string | null) => void;
  /** Уход в под-маршрут (пикер, создание, карточка, правка): носитель
   * сессии формы помечается живым, снимок формы сохраняется до push. */
  readonly onSubRouteOpen: () => void;
};

export function TermsContactSection({
  propertyId,
  contactId,
  onContactChange,
  onSubRouteOpen,
}: TermsContactSectionProps): JSX.Element {
  const router = useRouter();
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);

  // Книга нужна секции, чтобы отличить пустую (одна строка создания) от
  // непустой (два действия); сам список живёт на экране выбора.
  const contactsQuery = useContacts(propertyId);
  const tenantQuery = useContact(contactId ?? '');
  const deleteContact = useDeleteContact(contactId ?? '');

  // Выбранный арендатор реально существует, только когда его карточка
  // загрузилась: id формы может указывать на удалённый контакт — секция
  // деградирует к состоянию без выбора, а не к «осиротевшему» кебабу.
  const selectedTenant =
    contactId !== null && !tenantQuery.isError ? tenantQuery.data : undefined;

  // Сбой чтения книги не блокирует секцию: экран выбора покажет свой
  // повтор, а «Создать контакт» работает всегда. Строка «Выбрать контакт» —
  // только при подтверждённой непустой книге (решение владельца, шаг 4).
  const showPickerRow =
    !contactsQuery.isPending && !contactsQuery.isError && contactsQuery.data.length > 0;

  const openPicker = (): void => {
    onSubRouteOpen();
    pushPage(router, `${ROUTES.propertyRentalNewContact(propertyId)}?return=terms`);
  };

  const openCreateBranch = (): void => {
    onSubRouteOpen();
    pushPage(router,
      `${ROUTES.propertyContactNew(propertyId)}?role=${encodeURIComponent('Арендатор')}&pick=terms`,
    );
  };

  const openContactCard = (cardId: string): void => {
    onSubRouteOpen();
    pushPage(router, ROUTES.propertyContact(propertyId, cardId));
  };

  const openContactEdit = (cardId: string): void => {
    onSubRouteOpen();
    pushPage(router, ROUTES.propertyContactEdit(propertyId, cardId));
  };

  const handleDelete = (): void => {
    void (async () => {
      try {
        await deleteContact.mutateAsync();
        onContactChange(null);
        setDeleteDialogOpen(false);
        notify.scenarios.propertyContacts.deleted();
      } catch (error: unknown) {
        notify.scenarios.propertyContacts.deleteError(error);
      }
    })();
  };

  return (
    <div className="flex w-full flex-col gap-2 font-sans">
      <FieldTitle title="Контакт арендатора" />

      {selectedTenant !== undefined && (
        <div className="flex items-center gap-1">
          <ContactRowButton
            contact={selectedTenant}
            surface="white"
            className="min-w-0 flex-1"
            /* Тап по выбранному — карточка контакта (канон шага 4);
                смена арендатора — через «Выбрать другой». */
            onSelect={() => openContactCard(selectedTenant.id)}
          />
          <Menu>
            <MenuTrigger asChild>
              <IconButton icon={<Kebab className="h-6 w-6" />} label="Меню контакта" />
            </MenuTrigger>
            <MenuContent collisionPadding={24}>
              {/* Пункты и иконки — меню шага 4 (1855:64502): семейство R. */}
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
                onSelect={() => openContactEdit(selectedTenant.id)}
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

      {/* Без выбора (40000101:153003) идут два действия, в пустой книге —
          только «Создать контакт» (решение владельца); в выбранном
          состоянии (40000101:153262) строка создания остаётся видимой —
          канон шага 4. Пока книга едет — рисуем создание (безопасный
          дефолт шага 4). */}
      {selectedTenant === undefined && showPickerRow && (
        <ListRow
          leading={<SectionActionIcon icon={<BoldUser className="h-6 w-6" />} />}
          title="Выбрать контакт"
          onSelect={openPicker}
          className="-mx-6"
        />
      )}
      <ListRow
        leading={<SectionActionIcon icon={<Add className="h-6 w-6" />} />}
        title="Создать контакт"
        onSelect={openCreateBranch}
        className="-mx-6"
      />

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
    </div>
  );
}

/** Кружок 44 действий секции — канон CircleIcon шага 4 (1855:63385). */
function SectionActionIcon({ icon }: { readonly icon: JSX.Element }): JSX.Element {
  return (
    <CircleIcon variant="white" aria-hidden>
      {icon}
    </CircleIcon>
  );
}
