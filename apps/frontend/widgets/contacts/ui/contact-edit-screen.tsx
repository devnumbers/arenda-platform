'use client';

import { useState, type JSX, type SubmitEvent } from 'react';
import { useRouter } from 'next/navigation';
import { Cancel, Check } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { ApiError } from '@/shared/api/errors';
import { goBack } from '@/shared/lib/navigation';
import { notify } from '@/shared/lib/notifications';
import { formatPhoneDisplay } from '@/shared/lib/phone';
import {
  buildContactUpdateCommand,
  contactFormErrors,
  contactFormReady,
  contactServerFieldErrors,
  useContact,
  useUpdateContact,
  type ContactFormFields,
} from '@/features/contacts';
import type { Contact } from '@/entities/contact';
import { canMutateProperty, useProperty } from '@/features/properties';
import {
  Button,
  IconButton,
  PageContent,
  StickyBottomBar,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import { ContactForm } from './contact-form';
import { ContactObjectSelectPage } from './contact-object-select';
import { ContactsErrorCard, ContactsSkeleton, ContactsUnavailableCard } from './contacts-states';

/**
 * Экран «Изменить контакт» (#510, макет 1302-58933): форма создания
 * (#509) в режиме правки — те же поля (общий ContactForm), значения
 * предзаполнены карточкой; телефон в маске. Шапка «✕ | Изменить контакт |
 * ✓» (✓ — быстрая отправка, как на создании), внизу — «Сохранить».
 * Сохранение — PATCH полной командой (пустая строка очищает поле,
 * propertyId: null снимает привязку), после — возврат на карточку с
 * инвалидацией книги. Доступ: правит тот, кому можно мутировать (кебаб
 * карточки смотрящему не рисуется), сервер — последняя инстанция.
 *
 * Режим книги (глобальная страница контактов, propertyId не задан):
 * доступ определяется привязкой самой карточки — без объекта правит
 * владелец книги (сам актёр), привязанная — по доступу её объекта; архив
 * привязанного объекта делает карточку read-only. Выход — на карточку
 * книги.
 */
export function ContactEditScreen({
  propertyId,
  contactId,
}: {
  readonly propertyId?: string;
  readonly contactId: string;
}): JSX.Element {
  const router = useRouter();
  const contactQuery = useContact(contactId);
  const propertyQuery = useProperty(propertyId ?? '');
  const contact = contactQuery.isSuccess ? contactQuery.data : undefined;
  const boundPropertyQuery = useProperty(
    propertyId === undefined ? (contact?.propertyId ?? '') : '',
  );

  const backHref =
    propertyId !== undefined ? ROUTES.propertyContact(propertyId, contactId) : ROUTES.contact(contactId);
  const backToCard = (): void => goBack(router, backHref);

  const contextProperty = propertyQuery.isSuccess ? propertyQuery.data : undefined;
  const boundProperty = boundPropertyQuery.isSuccess ? boundPropertyQuery.data : undefined;
  // В книжном режиме объект-привязка есть не у всякой карточки: ждать её
  // загрузки (и её архива в гейте) нужно только когда привязка есть — иначе
  // выключенный запрос навсегда pending и форма не открывается.
  const needBoundProperty =
    propertyId === undefined && contact !== undefined && contact.propertyId !== undefined;
  const archivedStatus =
    propertyId !== undefined
      ? (contextProperty?.status === 'archived' ? contextProperty.status : undefined)
      : (boundProperty?.status === 'archived' ? boundProperty.status : undefined);
  const loading =
    contactQuery.isPending ||
    (propertyId !== undefined
      ? propertyQuery.isPending
      : (needBoundProperty && boundPropertyQuery.isPending));
  const failed =
    contactQuery.isError ||
    (propertyId !== undefined
      ? propertyQuery.isError
      : (needBoundProperty && boundPropertyQuery.isError));
  // Правка — по мутационному доступу на объекте (ADR 0028); в режиме книги —
  // по привязке самой карточки (без объекта — своя книга). Сервер —
  // последняя инстанция, как на правке платежей.
  const canEdit =
    propertyId !== undefined
      ? canMutateProperty(contextProperty)
      : contact === undefined
        ? false
        : contact.propertyId !== undefined
          ? canMutateProperty(boundProperty)
          : true;

  if (loading) {
    return (
      <>
        <EditHeader onBack={backToCard} />
        <PageContent>
          <ContactsSkeleton />
        </PageContent>
      </>
    );
  }

  if (failed) {
    return (
      <>
        <EditHeader onBack={backToCard} />
        <PageContent>
          <ContactsErrorCard
            onRetry={() => {
              void contactQuery.refetch();
              if (propertyId !== undefined) {
                void propertyQuery.refetch();
              } else {
                void boundPropertyQuery.refetch();
              }
            }}
          />
        </PageContent>
      </>
    );
  }

  // Смотрящему и архиву формы не показываем вовсе (как у правки платежей).
  if (!canEdit) {
    return (
      <>
        <EditHeader onBack={backToCard} />
        <PageContent>
          <ContactsUnavailableCard
            hint={
              archivedStatus === 'archived'
                ? 'Объект в архиве — контакты можно только смотреть'
                : 'У вас доступ только для просмотра этого объекта'
            }
          />
        </PageContent>
      </>
    );
  }

  if (contact === undefined) {
    // Недостижимо при !loading && !failed — страховка типов.
    return (
      <>
        <EditHeader onBack={backToCard} />
        <PageContent>
          <ContactsSkeleton />
        </PageContent>
      </>
    );
  }

  return <ContactEditForm backHref={backHref} contact={contact} />;
}

/** Шапка правки без ✓: пока карточка грузится, отправлять нечего. */
function EditHeader({ onBack }: { readonly onBack: () => void }): JSX.Element {
  return (
    <TopNav
      leading={<IconButton icon={<Cancel />} label="Назад" onClick={onBack} />}
    >
      <TopNavTitle title="Изменить контакт" />
    </TopNav>
  );
}

/** Форма правки: рендерится, когда карточка загружена — предзаполнение
 * из неё (паттерн payment-edit-screen). */
function ContactEditForm({
  backHref,
  contact,
}: {
  readonly backHref: string;
  readonly contact: Contact;
}): JSX.Element {
  const router = useRouter();
  const updateContact = useUpdateContact(contact.id);

  const [form, setForm] = useState<ContactFormFields>(() => formFromContact(contact));
  const [submitAttempted, setSubmitAttempted] = useState(false);
  const [serverErrors, setServerErrors] = useState<
    Partial<Record<keyof ContactFormFields, string>>
  >({});
  const [objectSelectOpen, setObjectSelectOpen] = useState(false);
  const [propertyIdDraft, setPropertyIdDraft] = useState<string | null>(null);

  const ready = contactFormReady(form);
  const clientErrors = submitAttempted ? contactFormErrors(form) : {};
  const errorOf = (field: keyof ContactFormFields): string | undefined =>
    serverErrors[field] ?? clientErrors[field];

  const update = <K extends keyof ContactFormFields>(
    key: K,
    value: ContactFormFields[K],
  ): void => {
    setForm((prev) => ({ ...prev, [key]: value }));
    setServerErrors((prev) =>
      prev[key] === undefined ? prev : { ...prev, [key]: undefined },
    );
  };

  const submit = async (): Promise<void> => {
    setSubmitAttempted(true);
    const errors = contactFormErrors(form);
    if (!ready || Object.keys(errors).length > 0) {
      return;
    }
    try {
      await updateContact.mutateAsync(buildContactUpdateCommand(form));
      notify.scenarios.propertyContacts.updated();
      goBack(router, backHref);
    } catch (error: unknown) {
      if (error instanceof ApiError && error.fieldErrors !== undefined) {
        setServerErrors(contactServerFieldErrors(error.fieldErrors));
      }
      notify.scenarios.propertyContacts.updateError(error);
    }
  };

  const handleSubmit = (event: SubmitEvent<HTMLFormElement>): void => {
    event.preventDefault();
    void submit();
  };

  // Страница выбора объекта (1539:83846): URL не меняется, несохранённая
  // форма живёт в состоянии виджета (как страницы платежей).
  if (objectSelectOpen) {
    return (
      <ContactObjectSelectPage
        draft={propertyIdDraft}
        onDraftChange={setPropertyIdDraft}
        onApply={() => {
          update('propertyId', propertyIdDraft);
          setObjectSelectOpen(false);
        }}
        onDismiss={() => setObjectSelectOpen(false)}
      />
    );
  }

  return (
    <>
      <TopNav
        leading={
          <IconButton
            icon={<Cancel />}
            label="Назад"
            onClick={() => goBack(router, backHref)}
          />
        }
        trailing={
          <IconButton
            icon={<Check />}
            label="Сохранить изменения"
            disabled={!ready || updateContact.isPending}
            onClick={() => void submit()}
          />
        }
      >
        <TopNavTitle title="Изменить контакт" />
      </TopNav>

      <PageContent>
        <ContactForm
          form={form}
          errorOf={errorOf}
          onFieldChange={update}
          onOpenObjectSelect={() => {
            setPropertyIdDraft(form.propertyId);
            setObjectSelectOpen(true);
          }}
          onSubmit={handleSubmit}
        />
      </PageContent>

      {/* Как на создании (#509): нижней кнопки нет, пока форма не готова
       * (заполнено Имя). */}
      {ready && (
        <StickyBottomBar>
          <Button
            className="w-full"
            loading={updateContact.isPending}
            onClick={() => void submit()}
          >
            Сохранить
          </Button>
        </StickyBottomBar>
      )}
    </>
  );
}

/** Карточка → поля формы: телефон из канона в маску, привязка
 * «без объекта» — null. */
function formFromContact(contact: Contact): ContactFormFields {
  return {
    firstName: contact.firstName,
    lastName: contact.lastName,
    patronymic: contact.patronymic,
    role: contact.role,
    phone: formatPhoneDisplay(contact.phone),
    email: contact.email,
    messengerName: contact.messengerName,
    messengerUsername: contact.messengerUsername,
    note: contact.note,
    propertyId: contact.propertyId ?? null,
  };
}
