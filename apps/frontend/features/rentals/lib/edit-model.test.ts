import { describe, expect, it } from 'vitest';
import type { Rental } from '@/entities/rental';
import {
  buildRentalUpdateCommand,
  formAfterPaymentDayChange,
  rentalEditFormFromRental,
  rentalPlannedEndDateEditError,
  RENTAL_COMMENT_MAX,
} from './edit-model';

const TODAY = '2026-09-07';

const RENTAL: Rental = {
  id: '0198c7a2-0000-7000-8000-000000000001',
  propertyId: '0198c7a2-0000-7000-8000-000000000002',
  status: 'active',
  startDate: '2026-05-10',
  plannedEndDate: '2028-05-10',
  completedDate: null,
  utilities: 'meters_only',
  depositKopecks: 5600000,
  commissionKopecks: null,
  depositReturnKopecks: null,
  depositReturnComment: null,
  tenant: null,
  comment: 'Предыдущий жилец съехал',
  rentPayment: {
    paymentId: '0198c7a2-0000-7000-8000-000000000003',
    amountKopecks: 5600000,
    paymentDay: 10,
    autoPay: true,
    reminderOffsetDays: 3,
    nextPayment: null,
  },
  progress: { paidMonths: 3, totalMonths: 24, monthsRemaining: 17, overdueMonths: null },
  today: TODAY,
  createdAt: '2026-09-05T10:00:00Z',
};

describe('rentalEditFormFromRental', () => {
  it('предзаполняет форму условиями аренды и её платежа', () => {
    expect(rentalEditFormFromRental(RENTAL)).toStrictEqual({
      amountKopecks: 5600000,
      paymentDay: 10,
      autoPay: true,
      reminderOffsetDays: 3,
      plannedEndDate: '2028-05-10',
      utilities: 'meters_only',
      depositKopecks: 5600000,
      commissionKopecks: null,
      comment: 'Предыдущий жилец съехал',
    });
  });

  it('бессрочная аренда без залога и с пустым комментарием предзаполняется нулями и строкой', () => {
    const endless: Rental = {
      ...RENTAL,
      plannedEndDate: null,
      depositKopecks: null,
      comment: '',
    };
    expect(rentalEditFormFromRental(endless)).toStrictEqual({
      amountKopecks: 5600000,
      paymentDay: 10,
      autoPay: true,
      reminderOffsetDays: 3,
      plannedEndDate: null,
      utilities: 'meters_only',
      depositKopecks: null,
      commissionKopecks: null,
      comment: '',
    });
  });
});

describe('rentalPlannedEndDateEditError', () => {
  it('null — бессрочная, валидна всегда', () => {
    expect(rentalPlannedEndDateEditError(null, '2026-05-10', TODAY)).toBeUndefined();
  });

  it('дата строго позже начала', () => {
    expect(rentalPlannedEndDateEditError('2026-05-10', '2026-05-10', TODAY)).toBe(
      'Окончание должно быть позже начала',
    );
    expect(rentalPlannedEndDateEditError('2026-05-09', '2026-05-10', TODAY)).toBe(
      'Окончание должно быть позже начала',
    );
  });

  it('дата не в прошлом: ≥ сегодня', () => {
    expect(rentalPlannedEndDateEditError('2026-09-06', '2026-05-10', TODAY)).toBe(
      'Окончание не может быть в прошлом',
    );
    expect(rentalPlannedEndDateEditError(TODAY, '2026-05-10', TODAY)).toBeUndefined();
  });

  it('завтрашняя и дальняя даты валидны', () => {
    expect(rentalPlannedEndDateEditError('2026-09-08', '2026-05-10', TODAY)).toBeUndefined();
    expect(rentalPlannedEndDateEditError('2028-05-10', '2026-05-10', TODAY)).toBeUndefined();
  });
});

describe('buildRentalUpdateCommand', () => {
  it('форма без изменений команды не даёт', () => {
    expect(buildRentalUpdateCommand(RENTAL, rentalEditFormFromRental(RENTAL))).toBeUndefined();
  });

  it('изменённые поля уходят, нетронутые опущены (частичный PATCH)', () => {
    const form = {
      ...rentalEditFormFromRental(RENTAL),
      amountKopecks: 6000000,
      paymentDay: 15 as const,
      autoPay: false,
      utilities: 'included' as const,
    };
    expect(buildRentalUpdateCommand(RENTAL, form)).toStrictEqual({
      amountKopecks: 6000000,
      paymentDay: 15,
      autoPay: false,
      utilities: 'included',
    });
  });

  it('очищенное nullable-поле уходит явным null (tri-state)', () => {
    const clearedDeposit = {
      ...rentalEditFormFromRental(RENTAL),
      depositKopecks: null,
    };
    expect(buildRentalUpdateCommand(RENTAL, clearedDeposit)).toStrictEqual({
      depositKopecks: null,
    });

    const clearedEnd = {
      ...rentalEditFormFromRental(RENTAL),
      plannedEndDate: null,
    };
    expect(buildRentalUpdateCommand(RENTAL, clearedEnd)).toStrictEqual({
      plannedEndDate: null,
    });

    const clearedComment = {
      ...rentalEditFormFromRental(RENTAL),
      comment: '',
    };
    expect(buildRentalUpdateCommand(RENTAL, clearedComment)).toStrictEqual({
      comment: null,
    });
  });

  it('смена напоминания уходит значением, «Не напоминать» — явным null (#1208)', () => {
    const base = rentalEditFormFromRental(RENTAL);
    expect(buildRentalUpdateCommand(RENTAL, { ...base, reminderOffsetDays: 7 })).toStrictEqual({
      reminderOffsetDays: 7,
    });
    expect(buildRentalUpdateCommand(RENTAL, { ...base, reminderOffsetDays: undefined })).toStrictEqual({
      reminderOffsetDays: null,
    });

    // Аренда без напоминаний (null) → выбор оффсета едет значением.
    const silent: Rental = { ...RENTAL, rentPayment: { ...RENTAL.rentPayment, reminderOffsetDays: null } };
    const silentForm = rentalEditFormFromRental(silent);
    expect(buildRentalUpdateCommand(silent, { ...silentForm, reminderOffsetDays: 1 })).toStrictEqual({
      reminderOffsetDays: 1,
    });
    expect(buildRentalUpdateCommand(silent, silentForm)).toBeUndefined();
  });

  it('заполнение пустого поля уходит значением', () => {
    const filled = {
      ...rentalEditFormFromRental(RENTAL),
      commissionKopecks: 500000,
      plannedEndDate: null,
    };
    expect(buildRentalUpdateCommand(RENTAL, filled)).toStrictEqual({
      commissionKopecks: 500000,
      plannedEndDate: null,
    });
  });

  it('невалидная сумма (пустая, ноль, сверх лимита) команду роняет', () => {
    const base = rentalEditFormFromRental(RENTAL);
    expect(
      buildRentalUpdateCommand(RENTAL, { ...base, amountKopecks: undefined }),
    ).toBeUndefined();
    expect(buildRentalUpdateCommand(RENTAL, { ...base, amountKopecks: 0 })).toBeUndefined();
    expect(
      buildRentalUpdateCommand(RENTAL, { ...base, amountKopecks: 1_000_000_001 }),
    ).toBeUndefined();
  });

  it('неизвестный день оплаты команду роняет', () => {
    const base = rentalEditFormFromRental(RENTAL);
    expect(
      buildRentalUpdateCommand(RENTAL, { ...base, paymentDay: undefined }),
    ).toBeUndefined();
  });

  it('невалидное изменённое окончание (не позже начала, в прошлом) команду роняет', () => {
    const base = rentalEditFormFromRental(RENTAL);
    expect(
      buildRentalUpdateCommand(RENTAL, { ...base, plannedEndDate: '2026-05-10' }),
    ).toBeUndefined();
    expect(
      buildRentalUpdateCommand(RENTAL, { ...base, plannedEndDate: '2026-09-06' }),
    ).toBeUndefined();
  });

  it('нетронутое прошедшее окончание needs_attention-аренды правку прочих полей не блокирует', () => {
    // Окончание уже прошло (плановая дата наступила и прошла, ADR 0053 §2):
    // сервер его принял, PATCH-контракт проверяет только отправленное —
    // сумма/день должны правиться без починки даты.
    const overdue: Rental = { ...RENTAL, plannedEndDate: '2026-09-01', status: 'needs_attention' };
    const form = { ...rentalEditFormFromRental(overdue), amountKopecks: 6100000 };
    expect(buildRentalUpdateCommand(overdue, form)).toStrictEqual({
      amountKopecks: 6100000,
    });
  });

  it('залог/комиссия сверх лимита команду роняют, ноль валиден', () => {
    const base = rentalEditFormFromRental(RENTAL);
    expect(
      buildRentalUpdateCommand(RENTAL, { ...base, depositKopecks: 1_000_000_001 }),
    ).toBeUndefined();
    expect(
      buildRentalUpdateCommand(RENTAL, { ...base, commissionKopecks: 1_000_000_001 }),
    ).toBeUndefined();
    expect(
      buildRentalUpdateCommand(RENTAL, { ...base, depositKopecks: 0 })?.depositKopecks,
    ).toBe(0);
  });

  it('комментарий длиннее лимита команду роняет; лимит экспортируется для поля', () => {
    const base = rentalEditFormFromRental(RENTAL);
    expect(RENTAL_COMMENT_MAX).toBe(2000);
    expect(
      buildRentalUpdateCommand(RENTAL, { ...base, comment: 'а'.repeat(2001) }),
    ).toBeUndefined();
    expect(
      buildRentalUpdateCommand(RENTAL, { ...base, comment: 'а'.repeat(2000) })?.comment,
    ).toBe('а'.repeat(2000));
  });
});

describe('formAfterPaymentDayChange', () => {
  it('день оплаты, сдвинувший первое вхождение за стоящее окончание, очищает его в бессрочную (#1156)', () => {
    // Аренда с 10-го, день 10-е, окончание 20-го — валидно; смена дня
    // на 25-е делает пару невалидной: первое вхождение 25-го позже конца.
    const form = rentalEditFormFromRental(RENTAL);
    const next = formAfterPaymentDayChange(
      { ...RENTAL, startDate: '2026-10-10', plannedEndDate: '2026-10-20' },
      { ...form, plannedEndDate: '2026-10-20' },
      25,
    );
    expect(next.paymentDay).toBe(25);
    expect(next.plannedEndDate).toBeNull();
  });

  it('окончание, покрывающее новое первое вхождение, остаётся', () => {
    const form = rentalEditFormFromRental(RENTAL);
    const next = formAfterPaymentDayChange(
      { ...RENTAL, startDate: '2026-10-10', plannedEndDate: '2026-11-20' },
      { ...form, plannedEndDate: '2026-11-20' },
      25,
    );
    expect(next.plannedEndDate).toBe('2026-11-20');
  });

  it('бессрочная (null) остаётся бессрочной при любом дне', () => {
    const form = rentalEditFormFromRental(RENTAL);
    const next = formAfterPaymentDayChange(RENTAL, { ...form, plannedEndDate: null }, 25);
    expect(next.plannedEndDate).toBeNull();
  });
});
