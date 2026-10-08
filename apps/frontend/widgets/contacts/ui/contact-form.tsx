'use client';

import type { JSX, SubmitEvent } from 'react';
import { SmallArrowDown } from '@/shared/assets/icons';
import { formatPhoneInput } from '@/shared/lib/phone';
import { useProperties } from '@/features/properties';
import { type ContactPhotoStage } from '@/features/contacts';
import { TextField, Textarea } from '@/shared/ui/design';
import type { ContactFormFields } from '@/features/contacts';
import { ContactPhotoSlot } from './contact-photo-slot';

/** Лимиты ввода (молчаливая обрезка сверх — как нативный maxLength, но без
 * счётчика: по макетам #509 у полей нет нижней строки). Именные поля —
 * 255 символов домена contacts (#506), заметка — 1024 из макета. */
const NAME_MAX = 255;
const SHORT_TEXT_MAX = 256;
const NOTE_MAX = 1024;

/**
 * Поля формы контакта — общие для «Создать контакт» (#509) и «Изменить
 * контакт» (#510): фото-слот (круг с BoldUser либо фото; тикет #1229 —
 * решение владельца 2026-09-03 «фото в контракте #507 нет» снято ADR
 * 0065, карта #1217) и группы полей по макету: «Личные данные» (titleIn),
 * «Роль», «Телефон и почта», «Мессенджер», «Привязанный объект» (titleOut,
 * ведёт на страницу выбора), «Заметка». Телефон набирается в маске,
 * обязательность и форматы считаются на экране (contactFormErrors/Ready).
 * Изменение фото живёт в черновике экрана (stage) и применяется кнопками
 * сохранения — форма только показывает слот и гонит выбор вверх.
 */
export function ContactForm({
  form,
  contactId,
  photoUrl,
  photoStage,
  photoBusy,
  onPhotoFile,
  onPhotoRemove,
  errorOf,
  onFieldChange,
  onOpenObjectSelect,
  onSubmit,
}: {
  readonly form: ContactFormFields;
  /** uuid карточки — на создании ещё нет (бастер слота тогда не нужен). */
  readonly contactId?: string;
  /** Серверное фото карточки; на создании карточки нет — null. */
  readonly photoUrl?: string | null;
  /** Незавершённое изменение фото из черновика экрана (ContactPhotoStage). */
  readonly photoStage: ContactPhotoStage | null;
  /** Фото-мутация сохранения в полёте — управление слота глушится. */
  readonly photoBusy: boolean;
  /** Файл выбран (кап уже проверен слотом) — экран кладёт его в stage. */
  readonly onPhotoFile: (file: File) => void;
  /** Удаление фото подтверждено — экран переставляет stage. */
  readonly onPhotoRemove: () => void;
  readonly errorOf: (field: keyof ContactFormFields) => string | undefined;
  readonly onFieldChange: {
    (key: 'propertyId', value: string | null): void;
    (key: Exclude<keyof ContactFormFields, 'propertyId'>, value: string): void;
  };
  readonly onOpenObjectSelect: () => void;
  /** Сабмит формы (Enter в поле); сама форма отправку не выполняет. */
  readonly onSubmit?: (event: SubmitEvent<HTMLFormElement>) => void;
}): JSX.Element {
  const propertiesQuery = useProperties();

  const selectedObjectLabel =
    form.propertyId === null
      ? 'Общий контакт'
      : (propertiesQuery.data?.find((property) => property.id === form.propertyId)?.name ?? '');

  return (
    <form
      className="flex flex-col gap-8 px-6"
      onSubmit={(event) => {
        event.preventDefault();
        onSubmit?.(event);
      }}
    >
      {/* Фото-слот (1281:48439, 1302:58933, тикет #1229): круг с BoldUser
       * либо фото; бейдж-корзина и ссылка по канону 1299:51572/73. */}
      <div className="flex justify-center">
        <ContactPhotoSlot
          contactId={contactId}
          photoUrl={photoUrl}
          stage={photoStage}
          busy={photoBusy}
          onFileChosen={onPhotoFile}
          onRemove={onPhotoRemove}
        />
      </div>

      <section className="flex flex-col gap-2">
        <span className="text-base font-medium leading-[18px] text-content">
          Личные данные
        </span>
        <div className="flex flex-col gap-2">
          <TextField
            variant="titleIn"
            title="Имя"
            required
            value={form.firstName}
            error={errorOf('firstName')}
            onClear={() => onFieldChange('firstName', '')}
            onChange={(event) =>
              onFieldChange('firstName', event.target.value.slice(0, NAME_MAX))
            }
          />
          <TextField
            variant="titleIn"
            title="Фамилия"
            value={form.lastName}
            error={errorOf('lastName')}
            onClear={() => onFieldChange('lastName', '')}
            onChange={(event) =>
              onFieldChange('lastName', event.target.value.slice(0, NAME_MAX))
            }
          />
          <TextField
            variant="titleIn"
            title="Отчество"
            value={form.patronymic}
            error={errorOf('patronymic')}
            onClear={() => onFieldChange('patronymic', '')}
            onChange={(event) =>
              onFieldChange('patronymic', event.target.value.slice(0, NAME_MAX))
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
        onClear={() => onFieldChange('role', '')}
        onChange={(event) =>
          onFieldChange('role', event.target.value.slice(0, SHORT_TEXT_MAX))
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
            onClear={() => onFieldChange('phone', '')}
            onChange={(event) =>
              onFieldChange('phone', formatPhoneInput(event.target.value))
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
            onClear={() => onFieldChange('email', '')}
            onChange={(event) => onFieldChange('email', event.target.value)}
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
            onClear={() => onFieldChange('messengerName', '')}
            onChange={(event) =>
              onFieldChange('messengerName', event.target.value.slice(0, SHORT_TEXT_MAX))
            }
          />
          <TextField
            variant="titleIn"
            title="Имя пользователя"
            value={form.messengerUsername}
            error={errorOf('messengerUsername')}
            onClear={() => onFieldChange('messengerUsername', '')}
            onChange={(event) =>
              onFieldChange('messengerUsername', event.target.value.slice(0, SHORT_TEXT_MAX))
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
          onClick={onOpenObjectSelect}
          className="flex h-14 w-full cursor-pointer items-center rounded-button bg-surface-muted py-0 pl-[18px] pr-2 text-left transition-shadow outline-none hover:shadow-[inset_0_0_0_2px_var(--dl-input-border)] focus-visible:ring-2 focus-visible:ring-primary"
        >
          <span className="min-w-0 flex-1 truncate text-base leading-[18px] text-content">
            {selectedObjectLabel}
          </span>
          <span
            aria-hidden
            className="flex h-11 w-11 shrink-0 items-center justify-center text-content-tertiary"
          >
            <SmallArrowDown className="h-4 w-4" />
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
          onFieldChange('note', event.target.value.slice(0, NOTE_MAX))
        }
      />
    </form>
  );
}
