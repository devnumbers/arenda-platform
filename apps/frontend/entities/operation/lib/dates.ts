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

export type OperationDueInfo = {
  readonly subtitle: string | null;
  readonly trailing: string;
};

/** '1 день', '2 дня', '5 дней'. */
export function formatDaysCount(n: number): string {
  const abs = Math.abs(n);
  const mod10 = abs % 10;
  const mod100 = abs % 100;

  let word: string;
  if (mod10 === 1 && mod100 !== 11) {
    word = 'день';
  } else if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) {
    word = 'дня';
  } else {
    word = 'дней';
  }

  return `${n} ${word}`;
}

export function getOperationDueInfo(
  operation: Pick<OperationResponse, 'status' | 'operation_date'>,
): OperationDueInfo {
  if (operation.status === 'overdue') {
    const diff = diffDays(parseLocalDate(operation.operation_date), new Date());
    return { subtitle: 'Просрочен', trailing: formatDaysCount(Math.max(diff, 0)) };
  }

  if (operation.status === 'pending') {
    const diff = diffDays(new Date(), parseLocalDate(operation.operation_date));
    if (diff === 0) return { subtitle: null, trailing: 'Сегодня' };
    if (diff === 1) return { subtitle: null, trailing: 'Завтра' };
    if (diff > 1) return { subtitle: null, trailing: formatDaysCount(diff) };
    return { subtitle: null, trailing: formatOperationDate(operation.operation_date) };
  }

  return { subtitle: null, trailing: formatOperationDate(operation.operation_date) };
}
