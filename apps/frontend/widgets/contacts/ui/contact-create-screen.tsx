'use client';

import { useState, type JSX, type SubmitEvent } from 'react';
import { useRouter } from 'next/navigation';
import { Cancel, Check } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { ApiError } from '@/shared/api/errors';
import { goBack } from '@/shared/lib/navigation';
import { notify } from '@/shared/lib/notifications';
import { formatPhoneInput } from '@/shared/lib/phone';
import { useProperties } from '@/features/properties';
import {
  buildContactCreateCommand,
  contactFormErrors,
  contactFormReady,
  useCreateContact,
  type ContactFormFields,
} from '@/features/contacts';
import {
  Button,
  IconButton,
  PageContent,
  PickerField,
  StickyBottomBar,
  TextField,
  Textarea,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import { ContactCreateAvatar } from './contact-create-avatar';

/** Лимиты ввода (молчаливая обрезка сверх — как нативный maxLength, но без
 * счётчика: по макетам #509 у полей нет нижней строки). Именные поля —
 * 255 символов домена contacts (#506), заметка — 1024 из макета. */
const NAME_MAX = 255;
const SHORT_TEXT_MAX = 256;
const NOTE_MAX = 1024;

const FIELD_NAMES: readonly (keyof ContactFormFields)[] = [
  'firstName',
  'lastName',
  'patronymic',
  'role',
  'phone',
  'email',
  'messengerName',
  'messengerUsername',
  'note',
  'propertyId',
];

const isFormField = (value: string): value is keyof ContactFormFields =>
  (FIELD_NAMES as readonly string[]).includes(value);

/**
 * Экран «Создать контакт» (#509, макеты 1281:48439 / 1282:49285): шапка
 * «Отменить | Создать контакт | ✓» (✓ — быстрая отправка, как на правке
 * платежа), аватар с «Добавить фото» и поля по группам макета. Простые
 * поля — titleIn (плавающий лейбл), «Роль», «Привязанный объект» и
 * «Заметка» — titleOut. Обязательно только Имя; телефон набирается в маске
 * и уходит нормализованным +7XXXXXXXXXX, почта — по формату (те же правила,
 * что в домене contacts; серверные fieldErrors ложатся поверх). Роль —
 * свободный текст: вход с будущей страницы создания аренды даёт
 * ?role=Арендатор, и поле подставлено сразу. Кнопки внизу нет, пока форма
 * не готова — по макету пустого состояния; заполненному — полноширинная
 * «Создать контакт». Объект — пикер #505, предзаполнен текущим объектом,
 * очистка — «Без объекта» (⚠️ неподтверждено владельцем — проверить на
 * приёмке).
 */
export function ContactCreateScreen({
  propertyId,
  initialRole,
}: {
  readonly propertyId: string;
  readonly initialRole: string;
}): JSX.Element {
  const router = useRouter();
  const createContact = useCreateContact();
  const propertiesQuery = useProperties();

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

  const propertyOptions = (propertiesQuery.data ?? []).map((property) => ({
    value: property.id,
    label: property.name,
  }));

  const submit = async (): Promise<void> => {
    setSubmitAttempted(true);
    const errors = contactFormErrors(form);
    if (!ready || Object.keys(errors).length > 0) {
      return;
    }
    try {
      await createContact.mutateAsync(buildContactCreateCommand(form));
      notify.scenarios.propertyContacts.created();
      goBack(router, ROUTES.propertyContacts(propertyId));
    } catch (error: unknown) {
      if (error instanceof ApiError && error.fieldErrors !== undefined) {
        const mapped: Partial<Record<keyof ContactFormFields, string>> = {};
        for (const fieldError of error.fieldErrors) {
          if (isFormField(fieldError.field)) {
            mapped[fieldError.field] = fieldError.detail;
          }
        }
        setServerErrors(mapped);
      }
      notify.scenarios.propertyContacts.createError(error);
    }
  };

  const handleSubmit = (event: SubmitEvent<HTMLFormElement>): void => {
    event.preventDefault();
    void submit();
  };

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
        <form className="flex flex-col gap-8 px-6" onSubmit={handleSubmit}>
          <ContactCreateAvatar />

          <section className="flex flex-col gap-2">
            <span className="text-base font-medium leading-[18px] text-content">
              Личные данные
            </span>
            <div className="flex flex-col gap-2">
              <TextField
                variant="titleIn"
                title="Имя *"
                value={form.firstName}
                error={errorOf('firstName')}
                onClear={() => update('firstName', '')}
                onChange={(event) =>
                  update('firstName', event.target.value.slice(0, NAME_MAX))
                }
              />
              <TextField
                variant="titleIn"
                title="Фамилия"
                value={form.lastName}
                error={errorOf('lastName')}
                onClear={() => update('lastName', '')}
                onChange={(event) =>
                  update('lastName', event.target.value.slice(0, NAME_MAX))
                }
              />
              <TextField
                variant="titleIn"
                title="Отчество"
                value={form.patronymic}
                error={errorOf('patronymic')}
                onClear={() => update('patronymic', '')}
                onChange={(event) =>
                  update('patronymic', event.target.value.slice(0, NAME_MAX))
                }
              />
            </div>
          </section>

          <TextField
            variant="titleOut"
            title="Роль"
            placeholder="Например, сантехник"
            value={form.role}
            error={errorOf('role')}
            onClear={() => update('role', '')}
            onChange={(event) =>
              update('role', event.target.value.slice(0, SHORT_TEXT_MAX))
            }
          />

          <section className="flex flex-col gap-2">
            <span className="text-base font-medium leading-[18px] text-content">
              Телефон и почта
            </span>
            <div className="flex flex-col gap-2">
              <TextField
                variant="titleIn"
                title="Телефон"
                type="tel"
                inputMode="tel"
                autoComplete="tel"
                value={form.phone}
                error={errorOf('phone')}
                onClear={() => update('phone', '')}
                onChange={(event) =>
                  update('phone', formatPhoneInput(event.target.value))
                }
              />
              <TextField
                variant="titleIn"
                title="Электронная почта"
                type="text"
                inputMode="email"
                autoComplete="email"
                value={form.email}
                error={errorOf('email')}
                onClear={() => update('email', '')}
                onChange={(event) => update('email', event.target.value)}
              />
            </div>
          </section>

          <section className="flex flex-col gap-2">
            <span className="text-base font-medium leading-[18px] text-content">
              Мессенджер
            </span>
            <div className="flex flex-col gap-2">
              <TextField
                variant="titleIn"
                title="Название мессенджера"
                value={form.messengerName}
                error={errorOf('messengerName')}
                onClear={() => update('messengerName', '')}
                onChange={(event) =>
                  update('messengerName', event.target.value.slice(0, SHORT_TEXT_MAX))
                }
              />
              <TextField
                variant="titleIn"
                title="Имя пользователя"
                value={form.messengerUsername}
                error={errorOf('messengerUsername')}
                onClear={() => update('messengerUsername', '')}
                onChange={(event) =>
                  update('messengerUsername', event.target.value.slice(0, SHORT_TEXT_MAX))
                }
              />
            </div>
          </section>

          <PickerField
            title="Привязанный объект"
            placeholder="Без объекта"
            value={form.propertyId}
            options={propertyOptions}
            clearable
            disabled={propertiesQuery.isPending}
            error={
              propertiesQuery.isError
                ? 'Не удалось загрузить объекты'
                : errorOf('propertyId')
            }
            onValueChange={(next) => update('propertyId', next)}
          />

          <Textarea
            title="Заметка"
            value={form.note}
            error={errorOf('note')}
            onChange={(event) =>
              update('note', event.target.value.slice(0, NOTE_MAX))
            }
          />
        </form>
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
