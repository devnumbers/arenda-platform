import { describe, expect, it } from 'vitest';
import type { Rental } from '@/entities/rental';
import {
  buildRentalCompleteCommand,
  completePlannedEndDate,
  rentalDurationLine,
  type RentalCompleteDraft,
} from './complete-model';

describe('buildRentalCompleteCommand', () => {
  const base: RentalCompleteDraft = {
    completedDate: '2029-01-10',
    depositRaw: '56000',
    comment: '',
  };

  it('собирает команду с суммой залога в копейках и датой', () => {
    expect(buildRentalCompleteCommand(base)).toEqual({
      completedDate: '2029-01-10',
      depositReturn: { amountKopecks: 5_600_000 },
    });
  });

  it('ноль валиден — «не вернул» (ADR 0053)', () => {
    expect(buildRentalCompleteCommand({ ...base, depositRaw: '0' })).toEqual({
      completedDate: '2029-01-10',
      depositReturn: { amountKopecks: 0 },
    });
  });

  it('дробная сумма округляется до копейки', () => {
    expect(buildRentalCompleteCommand({ ...base, depositRaw: '1234,5' })).toEqual({
      completedDate: '2029-01-10',
      depositReturn: { amountKopecks: 123_450 },
    });
  });

  it('непустой комментарий стримится и попадает в команду', () => {
    expect(buildRentalCompleteCommand({ ...base, comment: '  Удержали за счётчики  ' })).toEqual({
      completedDate: '2029-01-10',
      depositReturn: { amountKopecks: 5_600_000, comment: 'Удержали за счётчики' },
    });
  });

  it('без даты завершения команды нет', () => {
    expect(buildRentalCompleteCommand({ ...base, completedDate: undefined })).toBeUndefined();
  });

  it('с нечитаемой суммой команды нет', () => {
    expect(buildRentalCompleteCommand({ ...base, depositRaw: '' })).toBeUndefined();
    expect(buildRentalCompleteCommand({ ...base, depositRaw: 'abc' })).toBeUndefined();
  });
});

describe('rentalDurationLine', () => {
  it('складывает годы и месяцы как в макете', () => {
    expect(rentalDurationLine('2026-05-10', '2029-01-10')).toBe('2 года, 8 месяцев');
  });

  it('ровные годы без хвоста месяцев', () => {
    expect(rentalDurationLine('2026-05-10', '2027-05-10')).toBe('1 год');
    expect(rentalDurationLine('2026-05-10', '2028-05-10')).toBe('2 года');
    expect(rentalDurationLine('2026-05-10', '2031-05-10')).toBe('5 лет');
  });

  it('меньше года — только месяцы', () => {
    expect(rentalDurationLine('2026-05-10', '2026-06-10')).toBe('1 месяц');
    expect(rentalDurationLine('2026-05-10', '2027-04-10')).toBe('11 месяцев');
  });

  it('смешанный срок согласует оба слова', () => {
    expect(rentalDurationLine('2026-05-10', '2028-02-10')).toBe('1 год, 9 месяцев');
    expect(rentalDurationLine('2026-05-10', '2027-06-10')).toBe('1 год, 1 месяц');
    expect(rentalDurationLine('2026-05-10', '2029-03-10')).toBe('2 года, 10 месяцев');
  });

  it('неполный месяц — «Меньше месяца»', () => {
    expect(rentalDurationLine('2026-05-10', '2026-05-24')).toBe('Меньше месяца');
    expect(rentalDurationLine('2026-05-10', '2026-06-09')).toBe('Меньше месяца');
  });
});

describe('completePlannedEndDate', () => {
  const rental = (overrides: Partial<Rental>): Rental =>
    ({
      id: 'r1',
      propertyId: 'p1',
      status: 'active',
      startDate: '2026-05-10',
      plannedEndDate: '2027-05-10',
      completedDate: null,
      utilities: 'included',
      depositKopecks: 5_600_000,
      commissionKopecks: null,
      depositReturnKopecks: null,
      depositReturnComment: null,
      tenant: null,
      comment: '',
      rentPayment: {
        paymentId: 'pay1',
        amountKopecks: 56_000_00,
        paymentDay: 10,
        autoPay: false,
        nextPayment: null,
      },
      progress: { paidMonths: 0, totalMonths: 12, monthsRemaining: 12 },
      today: '2027-06-01',
      createdAt: '2026-05-10T00:00:00Z',
      ...overrides,
    });

  it('план в прошлом доступен — аренда ждёт завершения', () => {
    expect(completePlannedEndDate(rental({ today: '2027-06-01' }))).toBe('2027-05-10');
  });

  it('план в будущем недоступен — завершить «по плану» нельзя до его наступления', () => {
    expect(completePlannedEndDate(rental({ today: '2027-01-01' }))).toBeUndefined();
  });

  it('план сегодня — доступен', () => {
    expect(completePlannedEndDate(rental({ today: '2027-05-10' }))).toBe('2027-05-10');
  });

  it('у бессрочной подсказки нет', () => {
    expect(completePlannedEndDate(rental({ plannedEndDate: null }))).toBeUndefined();
  });
});
