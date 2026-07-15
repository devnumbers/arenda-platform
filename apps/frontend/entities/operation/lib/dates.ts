import type { components } from '@/shared/api/generated';
import { diffDays, parseLocalDate } from '@/shared/lib/lease-payment';

type OperationResponse = components['schemas']['OperationResponse'];

export function formatOperationDate(dateString: string): string {
  const date = new Date(dateString);
  if (Number.isNaN(date.getTime())) return dateString;
  return date.toLocaleDateString('ru-RU', {
    day: 'numeric',
    month: 'long',
    year: 'numeric',
  });
}

export function startOfMonth(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth(), 1);
}

export function endOfMonth(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth() + 1, 0);
}

export function startOfQuarter(date: Date): Date {
  const quarter = Math.floor(date.getMonth() / 3);
  return new Date(date.getFullYear(), quarter * 3, 1);
}

export function endOfQuarter(date: Date): Date {
  const quarter = Math.floor(date.getMonth() / 3);
  return new Date(date.getFullYear(), quarter * 3 + 3, 0);
}

export function startOfYear(date: Date): Date {
  return new Date(date.getFullYear(), 0, 1);
}

export function endOfYear(date: Date): Date {
  return new Date(date.getFullYear(), 11, 31);
}

export function formatDateForApi(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

export function parseDateForApi(dateString: string | null | undefined): Date | undefined {
  if (!dateString) return undefined;

  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(dateString);
  if (!match) return undefined;

  const year = Number(match[1]);
  const month = Number(match[2]);
  const day = Number(match[3]);
  const date = new Date(year, month - 1, day);

  if (formatDateForApi(date) !== dateString) return undefined;

  return date;
}

function pluralizeRu(n: number, one: string, few: string, many: string): string {
  const abs = Math.abs(n);
  const mod10 = abs % 10;
  const mod100 = abs % 100;

  let word: string;
  if (mod10 === 1 && mod100 !== 11) {
    word = one;
  } else if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) {
    word = few;
  } else {
    word = many;
  }

  return `${n} ${word}`;
}

/** Календарная длительность интервала [from, to]: '29 дней', '2 месяца, 4 дня'. */
export function formatCalendarDuration(from: Date, to: Date): string {
  let years = to.getFullYear() - from.getFullYear();
  let months = to.getMonth() - from.getMonth();
  let days = to.getDate() - from.getDate();

  if (days < 0) {
    months -= 1;
    days += new Date(to.getFullYear(), to.getMonth(), 0).getDate();
  }
  if (months < 0) {
    years -= 1;
    months += 12;
  }

  const parts: string[] = [];
  if (years > 0) parts.push(pluralizeRu(years, 'год', 'года', 'лет'));
  if (months > 0) parts.push(pluralizeRu(months, 'месяц', 'месяца', 'месяцев'));
  if (days > 0) parts.push(pluralizeRu(days, 'день', 'дня', 'дней'));

  return parts.length > 0 ? parts.join(', ') : '0 дней';
}

/** '5 июля'; если год даты не текущий — '5 июля 2025'. */
export function formatOperationDateShort(dateString: string): string {
  const date = new Date(dateString);
  if (Number.isNaN(date.getTime())) return dateString;

  const options: Intl.DateTimeFormatOptions = { day: 'numeric', month: 'long' };
  if (date.getFullYear() !== new Date().getFullYear()) {
    options.year = 'numeric';
  }
  return date.toLocaleDateString('ru-RU', options);
}

export function getOperationTrailing(
  operation: Pick<OperationResponse, 'status' | 'operation_date'>,
): string {
  if (operation.status === 'overdue') {
    const dueDate = parseLocalDate(operation.operation_date);
    const today = new Date();
    if (dueDate > today) return 'Сегодня';
    return `на ${formatCalendarDuration(dueDate, today)}`;
  }

  if (operation.status === 'pending') {
    const dueDate = parseLocalDate(operation.operation_date);
    const today = new Date();
    const diff = diffDays(today, dueDate);
    if (diff === 0) return 'Сегодня';
    if (diff === 1) return 'Завтра';
    if (diff > 1) return `через ${formatCalendarDuration(today, dueDate)}`;
    return formatOperationDateShort(operation.operation_date);
  }

  return formatOperationDateShort(operation.operation_date);
}
