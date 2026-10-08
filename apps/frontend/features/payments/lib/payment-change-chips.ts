import type {
  PaymentChangeCategoryRef,
  PaymentChangeEntry,
  PaymentReminderOffset,
} from '@/entities/payment';
import { paymentReminderOptionLabel, recurrenceLabel } from '@/entities/payment';
import { formatDayMonthYear } from '@/shared/lib/date-format';
import { formatMoneyKopecks } from '@/shared/lib/format-money';

/**
 * Тексты чипов режима «изменения» экрана «История платежа» (макеты
 * 3214-73216/73857, тикет #1195; ADR 0065 §5): сервер отдаёт структурный
 * диф типизированных значений, фразы собирает фронт из канонической
 * лексики — recurrenceLabel (резолюция #452), formatMoneyKopecks,
 * метки пикера напоминаний. Чип ведёт новым значением; у окончания и
 * напоминания переход «отсутствовало → появилось» читается «добавлено»,
 * у типа направления — фраза без значения. Пауза и возобновление — строки
 * журнала с пустым дифом; их тексты — канон тостов мутаций #452.
 */

/** Порядок чипов — порядок строк дифа (словарный, ADR 0065 §2): ответ
 * сервера уже отсортирован, функция сохраняет вход. */
export function paymentChangeChips(entry: PaymentChangeEntry): ReadonlyArray<string> {
  switch (entry.action) {
    case 'paused':
      return ['Платеж поставлен на паузу'];
    case 'resumed':
      return ['Платеж возобновлен'];
    case 'updated':
      return entry.changes.map(chipForChange);
  }
}

function chipForChange(change: PaymentChangeEntry['changes'][number]): string {
  switch (change.field) {
    case 'amount_kopecks':
      return `Сумма изменена: ${formatMoneyKopecks(change.new ?? 0)}`;
    case 'title':
      return `Название изменено: «${change.new ?? ''}»`;
    case 'category_slug':
      return `Категория изменена: «${categoryLabel(change.new)}»`;
    case 'recurrence':
      return change.new === null
        ? 'Регулярность изменена'
        : `Регулярность изменена: ${recurrenceLabel(change.new)}`;
    case 'end_date':
      return endDateChip(change.old, change.new);
    case 'type':
      return typeChip(change.new);
    case 'auto_pay':
      return change.new ? 'Автоплатеж включен' : 'Автоплатеж выключен';
    case 'reminder_offset_days':
      return reminderChip(change.old, change.new);
  }
}

/** Лейбл-снапшот правки; контракт гарантирует лейбл у ссылки (ADR 0065 §2)
 * — защита от частичного значения на случай расхождения контрактов. */
function categoryLabel(ref: PaymentChangeCategoryRef | null): string {
  return ref?.label ?? '';
}

function endDateChip(old: string | null, next: string | null): string {
  if (next === null) {
    return 'Окончание платежа изменено: Бессрочно';
  }
  if (old === null) {
    return `Окончание платежа добавлено: ${formatDayMonthYear(next)}`;
  }
  return `Окончание платежа изменено: ${formatDayMonthYear(next)}`;
}

function typeChip(next: string | null): string {
  if (next === 'expense') {
    return 'Доход изменен на расход';
  }
  if (next === 'income') {
    return 'Расход изменен на доход';
  }
  return 'Направление изменено';
}

function reminderChip(old: PaymentReminderOffset | null, next: PaymentReminderOffset | null): string {
  if (next === null) {
    return 'Напоминание изменено: Не напоминать';
  }
  if (old === null) {
    return `Напоминание добавлено: ${paymentReminderOptionLabel(next)}`;
  }
  return `Напоминание изменено: ${paymentReminderOptionLabel(next)}`;
}
