'use client';

import { useEffect, useSyncExternalStore } from 'react';
import type { Dispatch, SetStateAction } from 'react';
import type { IsoDate } from '@/shared/lib/calendar';
import { PAYMENT_REMINDER_OPTIONS } from '@/entities/payment';
import type { RentalPaymentDay, RentalUtilities } from '@/entities/rental';
import type { RentalWizardDraft } from './wizard-model';

/**
 * Носитель сессии визарда создания аренды (карта #1052, D3): модуль-синглтон
 * вместо localStorage-черновика. Состояние шагов едет через круговые маршруты
 * шага 4 — выбор арендатора (/rentals/new/contact), создание контакта
 * (?pick=rental), карточка и правка контакта (#1159) — и умирает вместе со
 * страницей: перезагрузка, закрытие вкладки и уход из визарда вне этих ветвей
 * (крестик «Закрыть», выход назад, финал) дают чистый лист.
 *
 * Keep-alive: перед router.push в под-маршрут (кнопки шага 4) визард
 * ставит флаг `keepRentalWizardSessionAlive` синхронно, в клике; открытие
 * сессии (`openRentalWizardSession` — жизненный цикл потока) гасит флаг,
 * закрытие чистит носитель без флага — отложенно, на макротаску. Отложка
 * переживает двойное монтирование React StrictMode в dev (setup → cleanup →
 * setup: повторный setup отменяет чистку, запланированную симулированным
 * cleanup'ом), а реальному возврату в визард она не мешает — переход по
 * маршруту заведомо дольше макротаски. Перезагрузка = память умерла = чистый
 * лист, поэтому и гидратационного рассинхрона нет: на SSR носитель всегда
 * пуст.
 *
 * Пикер и создание контакта пишут contactId гостевой записью
 * (`setRentalWizardSessionDraft`) — подписка на черновик им не нужна.
 * Глубокий вход `?pick=rental` без сессии визарда после создания открывает
 * чистый визард с id контакта в носителе — краевой случай, осознан (спека
 * #1054 §2.3).
 */

type RentalWizardSessionEntry = {
  draft: RentalWizardDraft;
  keepAlive: boolean;
};

const EMPTY_DRAFT: RentalWizardDraft = {};

/** Сессия per объект; живёт, пока жива страница. */
const sessions = new Map<string, RentalWizardSessionEntry>();

const listeners = new Map<string, Set<() => void>>();

/** Отложенные чистки: закрытие сессии без keep-alive стирает носитель на
 * следующей макротаске, чтобы StrictMode-remount успел отменить её. */
const pendingClears = new Map<string, ReturnType<typeof setTimeout>>();

/** Черновик сессии объекта; пустой (общая ссылка) — если визард ещё не
 * писал ничего или сессия уже закрыта. */
export function readRentalWizardSessionDraft(propertyId: string): RentalWizardDraft {
  return sessions.get(propertyId)?.draft ?? EMPTY_DRAFT;
}

/** Гостевая запись в носитель (пикер арендатора, создание контакта) и путь
 * `setDraft` хука. Значение прогоняется через валидатор — контракты формы
 * те же, что у черновика-хранилища раньше: мусорное поле отбрасывается,
 * остальные едут дальше. */
export function setRentalWizardSessionDraft(
  propertyId: string,
  update: SetStateAction<RentalWizardDraft>,
): void {
  const entry = sessionEntry(propertyId);
  entry.draft = validateRentalWizardDraft(
    typeof update === 'function' ? update(entry.draft) : update,
  );
  notify(propertyId);
}

/** Явная чистка носителя — после успешного POST создания аренды. */
export function clearRentalWizardSessionDraft(propertyId: string): void {
  if (!sessions.delete(propertyId)) {
    return;
  }
  notify(propertyId);
}

/** Пометить сессию живой: уход в под-маршрут не чистит носитель. Флаг
 * ставится синхронно в клике, до router.push, и гасится следующим
 * открытием сессии — возвратом в визард. */
export function keepRentalWizardSessionAlive(propertyId: string): void {
  sessionEntry(propertyId).keepAlive = true;
}

/** Открыть сессию (монтирование визарда): гасит keep-alive — возврат из
 * под-маршрута состояние сохранил — и отменяет отложенную чистку (двойное
 * монтирование StrictMode). Возвращает закрытие: без keep-alive оно
 * планирует чистку носителя — браузерный «назад», уход и финал дают
 * чистый лист. */
export function openRentalWizardSession(propertyId: string): () => void {
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

function sessionEntry(propertyId: string): RentalWizardSessionEntry {
  let entry = sessions.get(propertyId);
  if (entry === undefined) {
    entry = { draft: EMPTY_DRAFT, keepAlive: false };
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
      notify(propertyId);
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

function notify(propertyId: string): void {
  for (const listener of listeners.get(propertyId) ?? []) {
    listener();
  }
}

function listenersOf(propertyId: string): Set<() => void> {
  let set = listeners.get(propertyId);
  if (set === undefined) {
    set = new Set();
    listeners.set(propertyId, set);
  }
  return set;
}

/** Сессия визарда — ручки потока создания аренды: реактивный черновик,
 * запись, чистка после успеха и keep-alive перед уходом в под-маршрут.
 * Жизненный цикл (открытие/закрытие сессии) живёт в самом хуке — у потока,
 * единственного владельца сессии. */
export type RentalWizardSession = {
  readonly draft: RentalWizardDraft;
  readonly setDraft: Dispatch<SetStateAction<RentalWizardDraft>>;
  readonly clearDraft: () => void;
  readonly keepAlive: () => void;
};

export function useRentalWizardSession(propertyId: string): RentalWizardSession {
  // Ручной мемоизации нет (React Compiler, CODING_STANDARDS): снимок носителя
  // стабильен по построению — общая ссылка до следующей записи.
  const draft = useSyncExternalStore(
    (listener) => {
      const set = listenersOf(propertyId);
      set.add(listener);
      return () => {
        set.delete(listener);
      };
    },
    () => readRentalWizardSessionDraft(propertyId),
    () => readRentalWizardSessionDraft(propertyId),
  );

  useEffect(() => openRentalWizardSession(propertyId), [propertyId]);

  return {
    draft,
    setDraft: (update) => {
      setRentalWizardSessionDraft(propertyId, update);
    },
    clearDraft: () => {
      clearRentalWizardSessionDraft(propertyId);
    },
    keepAlive: () => {
      keepRentalWizardSessionAlive(propertyId);
    },
  };
}

function isIsoDate(value: unknown): value is IsoDate {
  return typeof value === 'string' && /^\d{4}-\d{2}-\d{2}$/.test(value);
}

function isNonNegativeInt(value: unknown): value is number {
  return typeof value === 'number' && Number.isInteger(value) && value >= 0;
}

function isFilledString(value: unknown): value is string {
  return typeof value === 'string' && value.length > 0;
}

function validatePaymentDay(value: unknown): RentalPaymentDay | undefined {
  if (value === 'last') return 'last';
  return typeof value === 'number' && Number.isInteger(value) && value >= 1 && value <= 31
    ? value
    : undefined;
}

function validateUtilities(value: unknown): RentalUtilities | undefined {
  return value === 'included' || value === 'meters_only' || value === 'full_receipt'
    ? value
    : undefined;
}

/** Форма-проверка записанного payload: неизвестные значения роняют поле
 * (мусор не всплывает в визарде), остальные сохраняются. */
export function validateRentalWizardDraft(parsed: unknown): RentalWizardDraft {
  if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) {
    return EMPTY_DRAFT;
  }

  const record = parsed as Record<string, unknown>;

  const amountKopecks =
    isNonNegativeInt(record.amountKopecks) && record.amountKopecks > 0
      ? record.amountKopecks
      : undefined;
  const paymentDay =
    record.paymentDay !== undefined ? validatePaymentDay(record.paymentDay) : undefined;
  const autoPay = typeof record.autoPay === 'boolean' ? record.autoPay : undefined;
  const startDate = isIsoDate(record.startDate) ? record.startDate : undefined;
  const plannedEndDate = isIsoDate(record.plannedEndDate) ? record.plannedEndDate : undefined;
  const utilities =
    record.utilities !== undefined ? validateUtilities(record.utilities) : undefined;
  const depositKopecks = isNonNegativeInt(record.depositKopecks)
    ? record.depositKopecks
    : undefined;
  const commissionKopecks = isNonNegativeInt(record.commissionKopecks)
    ? record.commissionKopecks
    : undefined;
  const contactId = isFilledString(record.contactId) ? record.contactId : undefined;
  // Напоминание — контрактный оффсет из того же кортежа, что и селект.
  const reminderOffsetDays = PAYMENT_REMINDER_OPTIONS.find(
    (option) => option.offset === record.reminderOffsetDays,
  )?.offset;

  return {
    ...(amountKopecks !== undefined && { amountKopecks }),
    ...(paymentDay !== undefined && { paymentDay }),
    ...(autoPay !== undefined && { autoPay }),
    ...(reminderOffsetDays !== undefined && { reminderOffsetDays }),
    ...(startDate !== undefined && { startDate }),
    ...(plannedEndDate !== undefined && { plannedEndDate }),
    ...(utilities !== undefined && { utilities }),
    ...(depositKopecks !== undefined && { depositKopecks }),
    ...(commissionKopecks !== undefined && { commissionKopecks }),
    ...(contactId !== undefined && { contactId }),
  };
}
