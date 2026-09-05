'use client';

import { useState, type JSX, type SubmitEvent } from 'react';
import { useRouter } from 'next/navigation';
import { Cancel, Check } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { ApiError } from '@/shared/api/errors';
import { goBack } from '@/shared/lib/navigation';
import { notify } from '@/shared/lib/notifications';
import {
  buildContactCreateCommand,
  contactFormErrors,
  contactFormReady,
  contactServerFieldErrors,
  useCreateContact,
  type ContactFormFields,
} from '@/features/contacts';
import { useRentalWizardDraft } from '@/features/rentals';
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

const SHORT_TEXT_MAX = 256;

/**
 * Экран «Создать контакт» (#509, макеты 1281:48439 / 1282:49285): шапка
 * «Отменить | Создать контакт | ✓» (✓ — быстрая отправка, как на правке
 * платежа), поля формы — общий ContactForm (#509/#510). Обязательно только
 * Имя; телефон набирается в маске и уходит нормализованным +7XXXXXXXXXX,
 * почта — по формату (те же правила, что в домене contacts; серверные
 * fieldErrors ложатся поверх). Роль — свободный текст: вход с будущей
 * страницы создания аренды даёт ?role=Арендатор, и поле подставлено сразу.
 * Кнопки внизу нет, пока форма не готова — по макету пустого состояния;
 * заполненному — полноширинная «Создать контакт».
 *
 * «Привязанный объект» — отдельная страница «Выбрать объект» на том же
 * маршруте (макет 1539:83846, паттерн страниц категории/периодичности
 * платежей): строгий черновик — применяются только «Выбрать»/✓ (решения
 * владельца 2026-09-03).
 */
export function ContactCreateScreen({
  propertyId,
  initialRole,
  returnToRentalWizard = false,
}: {
  readonly propertyId: string;
  readonly initialRole: string;
  /** Ветвь «Создать контакт» визарда аренды (#530): после создания
   * возвращаемся в визард с id нового контакта (арендатор выбран сразу),
   * а не на карточку. */
  readonly returnToRentalWizard?: boolean;
}): JSX.Element {
  const router = useRouter();
  const createContact = useCreateContact();
  // Ветвь визарда аренды (#530): черновик аренды патчится тем же хуком
  // (экземпляр на маунт, хранилище общее через localStorage) — возврат
  // goBack без URL-параметров, идентификатор едет в черновике.
  const rentalDraft = useRentalWizardDraft(propertyId);

  const [form, setForm] = useState<ContactFormFields>(() => ({
    firstName: '',
    lastName: '',
    patronymic: '',
    role: initialRole.slice(0, SHORT_TEXT_MAX),
    phone: '',
    email: '',
    messengerName: '',
    messengerUsername: '',
    note: '',
    propertyId,
  }));
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
      const created = await createContact.mutateAsync(buildContactCreateCommand(form));
      notify.scenarios.propertyContacts.created();
      if (returnToRentalWizard) {
        // Возврат в визард аренды (#530): контакт выбирается арендатором
        // через общий черновик (переживёт goBack), запись истории создания
        // выталкивается — Back из визарда не приводит обратно в форму.
        rentalDraft.setDraft((prev) => ({ ...prev, contactId: created.id }));
        goBack(router, ROUTES.propertyRentalNew(propertyId));
        return;
      }
      goBack(router, ROUTES.propertyContacts(propertyId));
    } catch (error: unknown) {
      if (error instanceof ApiError && error.fieldErrors !== undefined) {
        setServerErrors(contactServerFieldErrors(error.fieldErrors));
      }
      notify.scenarios.propertyContacts.createError(error);
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
            label="Отменить создание"
            onClick={() => goBack(router, ROUTES.propertyContacts(propertyId))}
          />
        }
        trailing={
          <IconButton
            icon={<Check />}
            label="Создать контакт"
            disabled={!ready || createContact.isPending}
            onClick={() => void submit()}
          />
        }
      >
        <TopNavTitle title="Создать контакт" />
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

      {/* По макету пустой формы (1281:48439) кнопки нет — появляется вместе
       * с готовностью (заполнено Имя), как в макете заполнения
       * (1282:49285). */}
      {ready && (
        <StickyBottomBar>
          <Button
            className="w-full"
            loading={createContact.isPending}
            onClick={() => void submit()}
          >
            Создать контакт
          </Button>
        </StickyBottomBar>
      )}
    </>
  );
}
