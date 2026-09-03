'use client';

import { useState, type JSX, type SubmitEvent } from 'react';
import { useRouter } from 'next/navigation';
import {
  BoldUser,
  Cancel,
  Check,
  ChevronDown,
  HomeCopy,
  HomeFilled,
  RadioFalse,
  RadioTrue,
} from '@/shared/assets/icons';
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
  StickyBottomBar,
  TextField,
  Textarea,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';

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
 * платежа), серый круг-аватар и поля по группам макета. Простые поля —
 * titleIn (плавающий лейбл), «Роль», «Привязанный объект» и «Заметка» —
 * titleOut. Обязательно только Имя; телефон набирается в маске и уходит
 * нормализованным +7XXXXXXXXXX, почта — по формату (те же правила, что в
 * домене contacts; серверные fieldErrors ложатся поверх). Роль — свободный
 * текст: вход с будущей страницы создания аренды даёт ?role=Арендатор, и
 * поле подставлено сразу. Кнопки внизу нет, пока форма не готова — по
 * макету пустого состояния; заполненному — полноширинная «Создать контакт».
 *
 * «Привязанный объект» — отдельная страница «Выбрать объект» на том же
 * маршруте (макет 1539:83846, паттерн страниц категории/периодичности
 * платежей): строгий черновик — тап по радио-строке меняет подсветку,
 * применяются только «Выбрать»/✓, ✕ отбрасывает. Первая строка —
 * «Общий контакт / Не привязан к объектам» (контакт без привязки), объекты
 * — имя и адрес, с фото-аватаром или серым домом; выбор по умолчанию —
 * «Общий контакт» (решения владельца 2026-09-03).
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

  const openObjectSelect = (): void => {
    setPropertyIdDraft(form.propertyId);
    setObjectSelectOpen(true);
  };

  const applyObjectDraft = (): void => {
    update('propertyId', propertyIdDraft);
    setObjectSelectOpen(false);
  };

  const selectedObjectLabel =
    form.propertyId === null
      ? 'Общий контакт'
      : (propertiesQuery.data?.find((property) => property.id === form.propertyId)?.name ?? '');

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

  // Страница выбора объекта (1539:83846): URL не меняется, несохранённая
  // форма живёт в состоянии виджета (как страницы платежей).
  if (objectSelectOpen) {
    return (
      <>
        <TopNav
          leading={
            <IconButton
              icon={<Cancel />}
              label="Не менять объект"
              onClick={() => setObjectSelectOpen(false)}
            />
          }
          trailing={
            <IconButton
              icon={<Check />}
              label="Выбрать объект"
              onClick={applyObjectDraft}
            />
          }
        >
          <TopNavTitle title="Выбрать объект" />
        </TopNav>

        <PageContent>
          <div role="radiogroup" aria-label="Привязка контакта к объекту" className="px-6">
            <ObjectRowButton
              title="Общий контакт"
              subtitle="Не привязан к объектам"
              isGeneral
              checked={propertyIdDraft === null}
              onCheck={() => setPropertyIdDraft(null)}
            />
            {propertiesQuery.isPending && <ObjectRowsSkeleton />}
            {propertiesQuery.isError && (
              <section className="rounded-card bg-surface-muted px-6 py-6">
                <h2 className="text-base font-medium leading-[18px] text-content">
                  Не удалось загрузить объекты
                </h2>
                <p className="mt-2 text-sm leading-4 text-content-secondary">
                  Проверьте подключение и попробуйте еще раз
                </p>
                <div className="mt-4">
                  <Button
                    size="small"
                    variant="secondary"
                    onClick={() => void propertiesQuery.refetch()}
                  >
                    Повторить
                  </Button>
                </div>
              </section>
            )}
            {(propertiesQuery.data ?? []).length > 0 && (
              <div aria-hidden className="h-px bg-surface-muted" />
            )}
            {(propertiesQuery.data ?? []).map((property) => (
              <ObjectRowButton
                key={property.id}
                title={property.name}
                subtitle={property.address}
                photoUrl={property.photos?.[0]?.url}
                checked={propertyIdDraft === property.id}
                onCheck={() => setPropertyIdDraft(property.id)}
              />
            ))}
          </div>
        </PageContent>

        <StickyBottomBar>
          <Button className="w-full" onClick={applyObjectDraft}>
            Выбрать
          </Button>
        </StickyBottomBar>
      </>
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
        <form className="flex flex-col gap-8 px-6" onSubmit={handleSubmit}>
          {/* Круг-аватар (1281:48439); кнопка «Добавить фото» убрана —
           * решение владельца 2026-09-03 (фото в контракте #507 нет). */}
          <div className="flex justify-center">
            <div aria-hidden className="flex h-24 w-24 items-center justify-center rounded-full bg-surface-muted">
              <BoldUser className="h-13 w-13 text-content-tertiary" />
            </div>
          </div>

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

          <section className="flex flex-col gap-2">
            <span className="text-base font-medium leading-[18px] text-content">
              Привязанный объект
            </span>
            <button
              type="button"
              aria-label={`Привязанный объект: ${selectedObjectLabel || 'не выбран'}`}
              onClick={openObjectSelect}
              className="flex h-14 w-full cursor-pointer items-center rounded-button bg-surface-muted py-0 pl-[18px] pr-2 text-left transition-shadow outline-none hover:shadow-[inset_0_0_0_2px_var(--dl-input-border)] focus-visible:ring-2 focus-visible:ring-primary"
            >
              <span className="min-w-0 flex-1 truncate text-base leading-[18px] text-content">
                {selectedObjectLabel}
              </span>
              <span
                aria-hidden
                className="flex h-11 w-11 shrink-0 items-center justify-center text-content-tertiary"
              >
                <ChevronDown className="h-4 w-4" />
              </span>
            </button>
            {errorOf('propertyId') !== undefined && (
              <span className="text-[13px] leading-[15px] text-error">
                {errorOf('propertyId')}
              </span>
            )}
          </section>

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

/** Строка-радио страницы «Выбрать объект» (компонент Figma «Row Button»,
 * 936:39347): аватар-круг 48 с фото или сплошным домом, заголовок,
 * подпись, кружок выбора RadioFalse/RadioTrue справа. */
function ObjectRowButton({
  title,
  subtitle,
  photoUrl,
  isGeneral = false,
  checked,
  onCheck,
}: {
  readonly title: string;
  readonly subtitle: string;
  readonly photoUrl?: string;
  readonly isGeneral?: boolean;
  readonly checked: boolean;
  readonly onCheck: () => void;
}): JSX.Element {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={checked}
      onClick={onCheck}
      className="flex w-full cursor-pointer items-center gap-2 py-3 text-left outline-none transition-opacity hover:opacity-80 focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface active:opacity-80"
    >
      <span
        aria-hidden
        className="relative flex h-12 w-12 shrink-0 items-center justify-center overflow-hidden rounded-full bg-surface-muted"
      >
        {photoUrl !== undefined ? (
          <img src={photoUrl} alt="" className="h-full w-full object-cover" />
        ) : (
          <HomeIcon general={isGeneral} />
        )}
      </span>
      <span className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="truncate text-base font-medium leading-[18px] text-content">{title}</span>
        <span className="truncate text-sm text-content-secondary">{subtitle}</span>
      </span>
      {checked ? (
        <RadioTrue className="h-6 w-6 shrink-0" aria-hidden />
      ) : (
        <RadioFalse className="h-6 w-6 shrink-0" aria-hidden />
      )}
    </button>
  );
}

/** Иконка аватара строки: сплошной дом у объектов, «дом со стеной позади»
 * у «Общего контакта» — оба силуэта из макета 1539:83846. */
function HomeIcon({ general = false }: { readonly general?: boolean }): JSX.Element {
  return general ? (
    <HomeCopy className="h-6 w-6 text-content-tertiary" />
  ) : (
    <HomeFilled className="h-6 w-6 text-content-tertiary" />
  );
}

/** Скелет строк объектов на время загрузки списка. */
function ObjectRowsSkeleton(): JSX.Element {
  return (
    <div aria-hidden className="flex flex-col gap-6 py-6">
      {[0, 1, 2, 3].map((row) => (
        <div key={row} className="flex items-center gap-2">
          <div className="h-12 w-12 animate-pulse rounded-full bg-surface-muted" />
          <div className="flex flex-1 flex-col gap-2">
            <div className="h-4 w-2/5 animate-pulse rounded-pill bg-surface-muted" />
            <div className="h-3.5 w-3/5 animate-pulse rounded-pill bg-surface-muted" />
          </div>
        </div>
      ))}
    </div>
  );
}
