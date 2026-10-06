import { cmp, type IsoDate } from '@/shared/lib/calendar';
import type {
  Rental,
  RentalPaymentDay,
  RentalUpdateCommand,
  RentalUtilities,
} from '@/entities/rental';
import { endCoversSchedule, RENTAL_AMOUNT_MAX_KOPECKS } from './wizard-model';

/**
 * Чистая логика правки условий аренды (#532, Figma 1302:53055): форма,
 * предзаполненная арендой, валидация окончания и дифф в частичный PATCH
 * (прецедент правки платежа #467). Начало не правится (ADR 0053 §3) —
 * на экране read-only; сумма, день оплаты, автоплатёж и окончание сервер
 * синхронно переносит на Платёж арендной платы.
 */

/** Лимит комментария контракта (ADR 0053 §4) — и в поле, и в проверке. */
export const RENTAL_COMMENT_MAX = 2000;

/** Рабочая форма правки: nullable-поля — null = «не задано/очищено».
 * Дифф с арендой решает, что уходит в PATCH: нетронутое поле совпадает
 * с исходным и опускается (omitted = без изменений, tri-state ADR 0053). */
export type RentalEditForm = {
  readonly amountKopecks: number | undefined;
  readonly paymentDay: RentalPaymentDay | undefined;
  readonly autoPay: boolean;
  readonly plannedEndDate: IsoDate | null;
  readonly utilities: RentalUtilities;
  readonly depositKopecks: number | null;
  readonly commissionKopecks: number | null;
  readonly comment: string;
};

/** Форма, предзаполненная текущими условиями. */
export function rentalEditFormFromRental(rental: Rental): RentalEditForm {
  return {
    amountKopecks: rental.rentPayment.amountKopecks,
    paymentDay: rental.rentPayment.paymentDay,
    autoPay: rental.rentPayment.autoPay,
    plannedEndDate: rental.plannedEndDate,
    utilities: rental.utilities,
    depositKopecks: rental.depositKopecks,
    commissionKopecks: rental.commissionKopecks,
    comment: rental.comment,
  };
}

/** Окончание в правке (ADR 0053 §3: «вперёд и назад, но не в прошлое»):
 * бессрочная (null) валидна всегда — это очистка; дата — строго позже
 * начала и не раньше сегодняшнего дня по TZ собственника. */
export function rentalPlannedEndDateEditError(
  plannedEndDate: IsoDate | null,
  startDate: IsoDate,
  today: IsoDate,
): string | undefined {
  if (plannedEndDate === null) {
    return undefined;
  }
  if (cmp(plannedEndDate, startDate) <= 0) {
    return 'Окончание должно быть позже начала';
  }
  return plannedEndDate < today ? 'Окончание не может быть в прошлом' : undefined;
}

/** Форма после смены дня оплаты (#1156, канон молчаливого сброса): стоящее
 * окончание, оказавшееся раньше первого вхождения нового дня, очищается
 * в бессрочную — дата, которую пикер окончания больше не даёт выбрать,
 * в форме не хранится (иначе сохранение ловило бы 400 «окна графика»,
 * #1154; тот же паттерн, что в визарде и в правке платежа #1155). */
export function formAfterPaymentDayChange(
  rental: Rental,
  form: RentalEditForm,
  paymentDay: RentalPaymentDay,
): RentalEditForm {
  const next = { ...form, paymentDay };
  if (next.plannedEndDate === null || endCoversSchedule(next.plannedEndDate, rental.startDate, paymentDay)) {
    return next;
  }
  return { ...next, plannedEndDate: null };
}

/** Дифф формы с арендой → команда PATCH; пустой дифф или невалидное
 * изменённое поле команды не дают (кнопки сохранения притушены).
 * Валидируется только то, что уходит в PATCH: нетронутое предзаполненное
 * значение серверу не отправляется и форму не блокирует — окончание
 * «needs_attention»-аренды может уже быть в прошлом, контракт его не
 * проверял бы (ADR 0053 §3: правятся только отправленные поля). */
export function buildRentalUpdateCommand(
  rental: Rental,
  form: RentalEditForm,
): RentalUpdateCommand | undefined {
  const { amountKopecks, paymentDay } = form;
  if (
    amountKopecks === undefined
    || amountKopecks <= 0
    || amountKopecks > RENTAL_AMOUNT_MAX_KOPECKS
    || paymentDay === undefined
  ) {
    return undefined;
  }
  const endChanged = form.plannedEndDate !== rental.plannedEndDate;
  if (
    (endChanged
      && rentalPlannedEndDateEditError(form.plannedEndDate, rental.startDate, rental.today)
        !== undefined)
    || !amountWithinContract(form.depositKopecks)
    || !amountWithinContract(form.commissionKopecks)
    || form.comment.length > RENTAL_COMMENT_MAX
  ) {
    return undefined;
  }

  // Мутируемая копия команды: собираем дифф поле за полем.
  const patch: { -readonly [K in keyof RentalUpdateCommand]: RentalUpdateCommand[K] } = {};

  if (amountKopecks !== rental.rentPayment.amountKopecks) {
    patch.amountKopecks = amountKopecks;
  }
  if (paymentDay !== rental.rentPayment.paymentDay) {
    patch.paymentDay = paymentDay;
  }
  if (form.autoPay !== rental.rentPayment.autoPay) {
    patch.autoPay = form.autoPay;
  }
  if (form.plannedEndDate !== rental.plannedEndDate) {
    patch.plannedEndDate = form.plannedEndDate;
  }
  if (form.utilities !== rental.utilities) {
    patch.utilities = form.utilities;
  }
  if (form.depositKopecks !== rental.depositKopecks) {
    patch.depositKopecks = form.depositKopecks;
  }
  if (form.commissionKopecks !== rental.commissionKopecks) {
    patch.commissionKopecks = form.commissionKopecks;
  }
  // Пустой комментарий — очистка: явный null (tri-state контракта).
  if (form.comment !== rental.comment) {
    patch.comment = form.comment === '' ? null : form.comment;
  }

  return Object.keys(patch).length === 0 ? undefined : patch;
}

/** Залог/комиссия в границах контракта: 0…10⁹ копеек либо null. */
function amountWithinContract(kopecks: number | null): boolean {
  return kopecks === null || (kopecks >= 0 && kopecks <= RENTAL_AMOUNT_MAX_KOPECKS);
}
