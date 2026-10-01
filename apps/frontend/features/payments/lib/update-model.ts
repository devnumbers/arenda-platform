import type {
  Payment,
  PaymentUpdateCommand,
  Recurrence,
} from '@/entities/payment';
import { periodicityReady } from './wizard-model';

/**
 * Модель правки платежа (#467): форма одного экрана → частичная команда
 * PATCH (контракт: опущенное поле остаётся без изменений, `since` серверный
 * и не правится). Правка меняет только будущее — пересоздание планового
 * вхождения делает сервер (истории 30–32 спеки #453).
 */

/** Значения формы правки; `categorySlug: undefined` — категория не менялась
 * (важно для платежа с пользовательской категорией: её слага в каталоге нет). */
export type PaymentEditForm = {
  readonly type: Payment['type'];
  readonly title: string;
  readonly amountKopecks: number | undefined;
  readonly categorySlug: string | undefined;
  readonly paymentForm: Payment['paymentForm'];
  readonly recurrence: Recurrence;
  /** Не задано — бессрочный; на сервер уходит tri-state.endDate (null — открыть срок). */
  readonly endDate: string | undefined;
};

/** Содержательное сравнение регулярности (порядок дней недели не значим). */
export function recurrencesEqual(a: Recurrence, b: Recurrence): boolean {
  if (a.kind !== b.kind) return false;
  switch (a.kind) {
    case 'daily':
      return true;
    case 'weekly':
      return (
        b.kind === 'weekly'
        && a.weekdays.length === b.weekdays.length
        && [...a.weekdays].sort((x, y) => x - y).join()
          === [...b.weekdays].sort((x, y) => x - y).join()
      );
    case 'monthly':
      return (
        b.kind === 'monthly'
        && a.lastDay === b.lastDay
        && a.daysOfMonth.length === b.daysOfMonth.length
        && [...a.daysOfMonth].sort((x, y) => x - y).join()
          === [...b.daysOfMonth].sort((x, y) => x - y).join()
      );
    case 'yearly':
      return b.kind === 'yearly' && a.month === b.month && a.day === b.day;
  }
}

/**
 * Действующее название формы: непустая строка или дефолт из каталога
 * категорий. Фолбэк-слаг — выбранная в форме категория, а без выбора —
 * текущая дефолтная категория правила (у пользовательской слага нет).
 */
function resolvedTitle(
  form: PaymentEditForm,
  fallbackCategorySlug: string | undefined,
  resolveTitle: (categorySlug: string) => string | undefined,
): string | undefined {
  const trimmed = form.title.trim();
  if (trimmed.length > 0) return trimmed;
  const slug = form.categorySlug ?? fallbackCategorySlug;
  if (slug !== undefined) return resolveTitle(slug);
  return undefined;
}

/**
 * Готовность формы к сохранению: обязательные поля заполнены; название при
 * пустом вводе заменяется лейблом действующей категории (правила визарда
 * #464) — потому нужен слаг текущей категории правила.
 */
export function editFormReady(
  form: PaymentEditForm,
  fallbackCategorySlug: string | undefined,
  resolveTitle: (categorySlug: string) => string | undefined = () => undefined,
): boolean {
  if (form.amountKopecks === undefined || form.amountKopecks <= 0) return false;
  if (!periodicityReady(form.recurrence)) return false;
  return resolvedTitle(form, fallbackCategorySlug, resolveTitle) !== undefined;
}

/**
 * Частичная команда PATCH из формы: в команду идут только отличия от правила;
 * undefined — изменений нет (сохранение нечем). endDate трисостоянен:
 * опущен (без изменений), null (открыть срок), дата (назначить).
 */
export function buildPaymentUpdateCommand(
  payment: Payment,
  form: PaymentEditForm,
  options: {
    /** Дефолт названия из лейбла каталога — как в визарде создания. */
    readonly resolveTitle?: (categorySlug: string) => string | undefined;
  } = {},
): PaymentUpdateCommand | undefined {
  const command: {
    -readonly [K in keyof PaymentUpdateCommand]: PaymentUpdateCommand[K];
  } = {};

  if (form.type !== payment.type) {
    command.type = form.type;
  }

  const title = resolvedTitle(
    form,
    // Дефолт названия — как в визарде: лейбл действующей категории (новой
    // из формы или текущей дефолтной правила; у пользовательской слага нет).
    payment.category.slug,
    options.resolveTitle ?? (() => undefined),
  );
  if (title !== undefined && title !== payment.title) {
    command.title = title;
  }

  if (form.amountKopecks !== undefined && form.amountKopecks !== payment.amountKopecks) {
    command.amountKopecks = form.amountKopecks;
  }

  if (!recurrencesEqual(form.recurrence, payment.recurrence)) {
    command.recurrence = form.recurrence;
  }

  // Формы оплаты в команде нет (#1008): поле снесено из контракта; форма
  // правки ещё несёт его для чипа экрана — чип уходит в #1009.

  if (form.categorySlug !== undefined && form.categorySlug !== payment.category.slug) {
    command.categorySlug = form.categorySlug;
  }

  // Tri-state.endDate: форма пуста при заполненном правиле — null (открыть
  // срок); расхождение дат — новая дата; совпадение — поле опускается.
  if (form.endDate === undefined) {
    if (payment.endDate !== undefined) {
      command.endDate = null;
    }
  } else if (form.endDate !== payment.endDate) {
    command.endDate = form.endDate;
  }

  if (Object.keys(command).length === 0) {
    return undefined;
  }
  return command;
}
