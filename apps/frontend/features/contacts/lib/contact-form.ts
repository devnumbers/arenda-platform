import { isEmailValid } from '@/shared/lib/email';
import { isPhoneValid, normalizePhone } from '@/shared/lib/phone';
import type { ContactCreateCommand } from '@/entities/contact';

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
