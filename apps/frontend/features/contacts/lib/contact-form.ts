import type { FieldError } from '@/shared/api/errors';
import { isEmailValid } from '@/shared/lib/email';
import { isPhoneValid, normalizePhone } from '@/shared/lib/phone';
import type { ContactCreateCommand, ContactUpdateCommand } from '@/entities/contact';

/**
 * Поля формы создания контакта (#509): структура команды создания
 * (ContactCreateCommand), но значения ещё «сырые» — телефон живёт в маске
 * «+7 (912) 345-67-89», тексты не обрезаны; нормализацию делает
 * buildContactCreateCommand.
 */
export type ContactFormFields = ContactCreateCommand;

/** Форма → команда создания: трим всех текстов, пробельные необязательные
 * поля складываются в пустые строки (сервер трактует их как «не задано»),
 * телефон — в канонический +7XXXXXXXXXX (ADR 0051). Зеркалит нормализацию,
 * которую домен contacts требует от вызывающего (#506). */
export function buildContactCreateCommand(
  fields: ContactFormFields,
): ContactCreateCommand {
  return {
    propertyId: fields.propertyId,
    firstName: fields.firstName.trim(),
    lastName: fields.lastName.trim(),
    patronymic: fields.patronymic.trim(),
    role: fields.role.trim(),
    phone: normalizePhone(fields.phone.trim()),
    email: fields.email.trim(),
    messengerName: fields.messengerName.trim(),
    messengerUsername: fields.messengerUsername.trim(),
    note: fields.note.trim(),
  };
}

/** Клиентская валидация формы: обязателен только Имя; телефон и почта —
 * по формату, когда заполнены (те же правила, что в домене contacts).
 * Ошибки сервера (problem+json fieldErrors) ложатся поверх этих —
 * сегодня бэкенд отдаёт 400 без разбора по полям. */
export function contactFormErrors(
  fields: ContactFormFields,
): Partial<Record<keyof ContactFormFields, string>> {
  const errors: Partial<Record<keyof ContactFormFields, string>> = {};

  if (fields.firstName.trim() === '') {
    errors.firstName = 'Укажите имя';
  }
  const phone = fields.phone.trim();
  if (phone !== '' && !isPhoneValid(phone)) {
    errors.phone = 'Некорректный номер телефона';
  }
  const email = fields.email.trim();
  if (email !== '' && !isEmailValid(email)) {
    errors.email = 'Некорректный адрес почты';
  }
  return errors;
}

/** Форма готова к отправке, когда заполнено имя, — по макету #509
 * (1281:48439) до этого у формы нет нижней кнопки. Форматные ошибки не
 * блокируют показ: кнопка активна, отправка показывает ошибки у полей. */
export function contactFormReady(fields: ContactFormFields): boolean {
  return fields.firstName.trim() !== '';
}

/** Форма → команда правки: та же нормализация, что у создания (трим,
 * канонический телефон). Форма правки #510 отдаёт полный состав полей,
 * поэтому команда структурно совпадает с командой создания — пустая
 * строка очищает поле, propertyId: null снимает привязку. */
export function buildContactUpdateCommand(
  fields: ContactFormFields,
): ContactUpdateCommand {
  return buildContactCreateCommand(fields);
}

/** Поля формы контакта — общий список create/правки (#509/#510) для
 * маппинга серверных fieldErrors на ключи формы. */
const CONTACT_FORM_FIELD_NAMES: readonly (keyof ContactFormFields)[] = [
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

/** Строковый ключ из ответа сервера — ключ формы контакта. */
export function isContactFormField(value: string): value is keyof ContactFormFields {
  return (CONTACT_FORM_FIELD_NAMES as readonly string[]).includes(value);
}

/** Серверные fieldErrors (problem+json) → ошибки по полям формы;
 * неизвестные ключи отбрасываются. Сегодня бэкенд отдаёт 400 без разбора
 * по полям — маппер готов принять их, когда появятся. */
export function contactServerFieldErrors(
  fieldErrors: readonly FieldError[],
): Partial<Record<keyof ContactFormFields, string>> {
  const mapped: Partial<Record<keyof ContactFormFields, string>> = {};
  for (const fieldError of fieldErrors) {
    if (isContactFormField(fieldError.field)) {
      mapped[fieldError.field] = fieldError.detail;
    }
  }
  return mapped;
}
