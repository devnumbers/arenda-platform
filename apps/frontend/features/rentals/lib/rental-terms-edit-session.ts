'use client';

import type { RentalEditForm } from './edit-model';

/**
 * Носитель сессии формы «Изменить условия» (решение владельца 28.10.2026,
 * #1623): форма права аренды переживает круговые маршруты секции «Контакт
 * арендатора» — выбор арендатора (?return=terms), создание контакта
 * (?pick=terms), карточка и правка контакта — по той же модели, что и
 * черновик визарда (#1052 D3): модуль-синглтон per объект, умирает вместе
 * со страницей. Перезагрузка = чистый лист, SSR-рассинхрона нет.
 *
 * Роли писателей различены: форму пишет только экран-владелец (полный
 * снимок `RentalEditForm` перед уходом в под-маршрут, восстановление при
 * возврате), контакт пишут гостевые ветви (пикер и создание) — в ожидание
 * `pendingContactId`, которое экран потребляет один раз при возврате.
 * Keep-alive и отложенная чистка — прецедент носителя визарда: флаг ставится
 * синхронно в клике до router.push, закрытие без флага чистит на макротаске
 * (переживает двойное монтирование StrictMode).
 */

type RentalTermsEditSessionEntry = {
  form: RentalEditForm | null;
  pendingContactId: string | null;
  keepAlive: boolean;
};

const EMPTY_ENTRY: RentalTermsEditSessionEntry = {
  form: null,
  pendingContactId: null,
  keepAlive: false,
};

/** Сессия per объект; живёт, пока жива страница. */
const sessions = new Map<string, RentalTermsEditSessionEntry>();

/** Отложенные чистки: закрытие сессии без keep-alive стирает носитель на
 * следующей макротаске, чтобы StrictMode-remount успел отменить её. */
const pendingClears = new Map<string, ReturnType<typeof setTimeout>>();

export type RentalTermsEditSessionDraft = Readonly<{
  form: RentalEditForm | null;
  pendingContactId: string | null;
}>;

/** Снимок носителя; пустой — если форма ещё не писала и гостей не было. */
export function readRentalTermsEditSessionDraft(
  propertyId: string,
): RentalTermsEditSessionDraft {
  const entry = sessions.get(propertyId);
  return entry === undefined
    ? { form: EMPTY_ENTRY.form, pendingContactId: EMPTY_ENTRY.pendingContactId }
    : { form: entry.form, pendingContactId: entry.pendingContactId };
}

/** Полный снимок формы от экрана-владельца: перед уходом в под-маршрут. */
export function setRentalTermsEditSessionForm(
  propertyId: string,
  form: RentalEditForm,
): void {
  sessionEntry(propertyId).form = form;
}

/** Гостевая запись контакта (пикер арендатора, ветвь создания #509): id едет
 * в ожидание — экран-владелец заберёт его при возврате. */
export function setRentalTermsEditSessionContact(
  propertyId: string,
  contactId: string,
): void {
  sessionEntry(propertyId).pendingContactId = contactId;
}

/** Потребить ожидание (владелец, при возврате): id отдаётся один раз. */
export function consumeRentalTermsEditSessionContact(propertyId: string): string | null {
  const entry = sessions.get(propertyId);
  if (entry === undefined || entry.pendingContactId === null) {
    return null;
  }
  const contactId = entry.pendingContactId;
  entry.pendingContactId = null;
  return contactId;
}

/** Явная чистка носителя — после успешного сохранения формы. */
export function clearRentalTermsEditSessionDraft(propertyId: string): void {
  cancelScheduledClear(propertyId);
  sessions.delete(propertyId);
}

/** Пометить сессию живой: уход в под-маршрут не чистит носитель. */
export function keepRentalTermsEditSessionAlive(propertyId: string): void {
  sessionEntry(propertyId).keepAlive = true;
}

/** Открыть сессию (монтирование формы): гасит keep-alive и отменяет
 * отложенную чистку. Возвращает закрытие: без keep-alive оно планирует
 * чистку — браузерный «назад», уход и сохранение дают чистый лист. */
export function openRentalTermsEditSession(propertyId: string): () => void {
  cancelScheduledClear(propertyId);
  const entry = sessions.get(propertyId);
  if (entry !== undefined) {
    entry.keepAlive = false;
  }
  return () => {
    const current = sessions.get(propertyId);
    if (current === undefined || current.keepAlive) {
      return;
    }
    scheduleClear(propertyId);
  };
}

function sessionEntry(propertyId: string): RentalTermsEditSessionEntry {
  let entry = sessions.get(propertyId);
  if (entry === undefined) {
    entry = { form: null, pendingContactId: null, keepAlive: false };
    sessions.set(propertyId, entry);
  }
  return entry;
}

function scheduleClear(propertyId: string): void {
  cancelScheduledClear(propertyId);
  pendingClears.set(
    propertyId,
    setTimeout(() => {
      pendingClears.delete(propertyId);
      sessions.delete(propertyId);
    }, 0),
  );
}

function cancelScheduledClear(propertyId: string): void {
  const pending = pendingClears.get(propertyId);
  if (pending !== undefined) {
    clearTimeout(pending);
    pendingClears.delete(propertyId);
  }
}
