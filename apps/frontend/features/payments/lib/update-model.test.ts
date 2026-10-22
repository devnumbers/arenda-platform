import { describe, expect, it } from 'vitest';
import { makePayment, type Payment } from '@/entities/payment';
import {
  buildPaymentUpdateCommand,
  editFormReady,
  formAfterRecurrenceChange,
  type PaymentEditForm,
} from './update-model';

/** Правило-база: все поля заполнены, категория из дефолтного каталога;
 * значения согласованы с baseForm ниже. Остальное — канон makePayment. */
function basePayment(overrides: Partial<Payment> = {}): Payment {
  return makePayment({
    id: '019abcde-0000-7000-8000-000000000001',
    propertyId: '019abcde-0000-7000-8000-000000000002',
    title: 'Страхование',
    amountKopecks: 320_000,
    recurrence: { kind: 'monthly', daysOfMonth: [15], lastDay: false },
    since: '2026-08-01',
    category: { source: 'default', slug: 'insurance', label: 'Страхование' },
    createdAt: '2026-08-01T10:00:00Z',
    updatedAt: '2026-08-01T10:00:00Z',
    ...overrides,
  });
}

/** Форма-база: значения совпадают с правилом-базой. */
function baseForm(overrides: Partial<PaymentEditForm> = {}): PaymentEditForm {
  return {
    type: 'expense',
    title: 'Страхование',
    amountKopecks: 320_000,
    categorySlug: undefined,
    recurrence: { kind: 'monthly', daysOfMonth: [15], lastDay: false },
    endDate: undefined,
    reminderOffsetDays: undefined,
    notifyAutoPaid: false,
    notifyEmail: true,
    ...overrides,
  };
}

describe('buildPaymentUpdateCommand', () => {
  it('без изменений возвращает undefined — PATCH не отправляется', () => {
    expect(buildPaymentUpdateCommand(basePayment(), baseForm())).toBeUndefined();
  });

  it('изменённая сумма попадает в команду, остальное опущено', () => {
    const command = buildPaymentUpdateCommand(basePayment(), baseForm({ amountKopecks: 400_000 }));
    expect(command).toEqual({ amountKopecks: 400_000 });
  });

  it('изменённые тип и название идут одним PATCH', () => {
    const command = buildPaymentUpdateCommand(
      basePayment(),
      baseForm({ type: 'income', title: '  Страхование квартиры  ' }),
    );
    expect(command).toEqual({
      type: 'income',
      title: 'Страхование квартиры',
    });
  });

  it('изменённая регулярность сравнивается по содержимому', () => {
    const command = buildPaymentUpdateCommand(
      basePayment(),
      baseForm({ recurrence: { kind: 'monthly', daysOfMonth: [20], lastDay: false } }),
    );
    expect(command).toEqual({ recurrence: { kind: 'monthly', daysOfMonth: [20], lastDay: false } });
  });

  it('порядок дней недели не ломает сравнение еженедельной регулярности', () => {
    const payment = basePayment({
      recurrence: { kind: 'weekly', weekdays: [1, 3] },
    });
    expect(
      buildPaymentUpdateCommand(payment, baseForm({
        recurrence: { kind: 'weekly', weekdays: [3, 1] },
      })),
    ).toBeUndefined();
    expect(
      buildPaymentUpdateCommand(payment, baseForm({
        recurrence: { kind: 'weekly', weekdays: [1] },
      })),
    ).toEqual({ recurrence: { kind: 'weekly', weekdays: [1] } });
  });

  describe('endDate трисостоянен', () => {
    it('пусто и было пусто — опущено', () => {
      expect(buildPaymentUpdateCommand(basePayment(), baseForm())).toBeUndefined();
    });

    it('дата назначена — в команде дата', () => {
      const command = buildPaymentUpdateCommand(
        basePayment(),
        baseForm({ endDate: '2027-08-15' }),
      );
      expect(command).toEqual({ endDate: '2027-08-15' });
    });

    it('была дата, стало пусто — null открывает срок', () => {
      const payment = basePayment({ endDate: '2027-08-15' });
      const command = buildPaymentUpdateCommand(payment, baseForm());
      expect(command).toEqual({ endDate: null });
    });

    it('та же дата — опущено', () => {
      const payment = basePayment({ endDate: '2027-08-15' });
      expect(
        buildPaymentUpdateCommand(payment, baseForm({ endDate: '2027-08-15' })),
      ).toBeUndefined();
    });
  });

  describe('категория', () => {
    it('выбор нового слага дефолтного каталога попадает в команду', () => {
      const command = buildPaymentUpdateCommand(
        basePayment(),
        baseForm({ categorySlug: 'internet' }),
      );
      expect(command).toEqual({ categorySlug: 'internet' });
    });

    it('тот же слаг — опущен', () => {
      expect(
        buildPaymentUpdateCommand(basePayment(), baseForm({ categorySlug: 'insurance' })),
      ).toBeUndefined();
    });

    it('пользовательская категория без выбора — категория не уходит в PATCH', () => {
      const payment = basePayment({
        category: { source: 'custom', id: '019abcde-0000-7000-8000-000000000003', label: 'Моя' },
      });
      expect(buildPaymentUpdateCommand(payment, baseForm())).toBeUndefined();
    });
  });

  describe('напоминание — tri-state (#1197, макет 1130-35022)', () => {
    it('выбранный оффсет попадает в команду', () => {
      const command = buildPaymentUpdateCommand(
        basePayment(),
        baseForm({ reminderOffsetDays: 3 }),
      );
      // Оживление ряда почты («не надо» → «За 3 дня») везёт галочку:
      // действующее on против заглушенного исходного (карта #1551).
      expect(command).toEqual({ reminderOffsetDays: 3, notifyEmail: true });
    });

    it('тот же оффсет — опущен', () => {
      const payment = basePayment({ reminderOffsetDays: 3 });
      expect(
        buildPaymentUpdateCommand(payment, baseForm({ reminderOffsetDays: 3 })),
      ).toBeUndefined();
    });

    it('смена оффсета 1 → 7 едет числом', () => {
      const payment = basePayment({ reminderOffsetDays: 1 });
      const command = buildPaymentUpdateCommand(
        payment,
        baseForm({ reminderOffsetDays: 7 }),
      );
      expect(command).toEqual({ reminderOffsetDays: 7 });
    });

    it('«Не напоминать» у правила с напоминанием — null снимает его', () => {
      const payment = basePayment({ reminderOffsetDays: 7 });
      const command = buildPaymentUpdateCommand(payment, baseForm());
      // Радио в «не надо» гасит и галочку почты (канон #1552).
      expect(command).toEqual({ reminderOffsetDays: null, notifyEmail: false });
    });

    it('«Не напоминать» у правила без напоминания — опущено', () => {
      expect(buildPaymentUpdateCommand(basePayment(), baseForm())).toBeUndefined();
    });

    it('правка напоминания едет в одном PATCH с другими полями', () => {
      const command = buildPaymentUpdateCommand(
        basePayment(),
        baseForm({ amountKopecks: 400_000, reminderOffsetDays: 1 }),
      );
      // Появление напоминания оживляет ряд почты: действующее значение
      // (запомненное on) расходится с заглушенным исходным — булево в команде
      // (карта #1551, сервер хранит тот же true — запись идемпотентна).
      expect(command).toEqual({
        amountKopecks: 400_000,
        reminderOffsetDays: 1,
        notifyEmail: true,
      });
    });
  });

  describe('флаг «уведомлять об автоплатеже» (#1189, #1197)', () => {
    it('у автоплатёжного правила явное «да» едет true', () => {
      const payment = basePayment({ autoPay: true, notifyAutoPaid: false });
      const command = buildPaymentUpdateCommand(
        payment,
        baseForm({ notifyAutoPaid: true }),
      );
      // Ряд почты оживает («Не уведомлять» → «Да») — галочка едёт действующим
      // on против заглушенного исходного (карта #1551; сервер хранит тот же
      // true — запись идемпотентна).
      expect(command).toEqual({ notifyAutoPaid: true, notifyEmail: true });
    });

    it('у автоплатёжного правила «не уведомлять» едет false', () => {
      const payment = basePayment({ autoPay: true, notifyAutoPaid: true });
      const command = buildPaymentUpdateCommand(
        payment,
        baseForm({ notifyAutoPaid: false }),
      );
      // Радио в «Не уведомлять» гасит и ряд почты — off в команде
      // (карта #1551, канон #1552: заглушенный ряд отправляет off).
      expect(command).toEqual({ notifyAutoPaid: false, notifyEmail: false });
    });

    it('то же значение — опущено', () => {
      const payment = basePayment({ autoPay: true, notifyAutoPaid: true });
      expect(
        buildPaymentUpdateCommand(payment, baseForm({ notifyAutoPaid: true })),
      ).toBeUndefined();
    });

    it('у ручного правила флаг не трогается никогда', () => {
      const command = buildPaymentUpdateCommand(
        basePayment({ autoPay: false, notifyAutoPaid: false }),
        baseForm({ notifyAutoPaid: true }),
      );
      expect(command).toBeUndefined();
    });
  });

  describe('персональная галочка почты (карта #1551, канон #1552)', () => {
    it('переключённый свитч живого ряда едет булевым', () => {
      const command = buildPaymentUpdateCommand(
        basePayment({ reminderOffsetDays: 3, notifyEmail: true }),
        baseForm({ reminderOffsetDays: 3, notifyEmail: false }),
      );
      expect(command).toEqual({ notifyEmail: false });
    });

    it('то же действующее значение — опущено', () => {
      expect(
        buildPaymentUpdateCommand(
          basePayment({ reminderOffsetDays: 3, notifyEmail: true }),
          baseForm({ reminderOffsetDays: 3 }),
        ),
      ).toBeUndefined();
    });

    it('снятие напоминания («не надо») гасит галочку в команде', () => {
      const command = buildPaymentUpdateCommand(
        basePayment({ reminderOffsetDays: 3, notifyEmail: true }),
        baseForm(),
      );
      expect(command).toEqual({ reminderOffsetDays: null, notifyEmail: false });
    });

    it('правило уже «не надо» (бэкфилл true): нетронутая форма не пишет галочку', () => {
      // Дифф по действующим значениям: исходная сторона тоже под гейтом —
      // правка чужого поля не оборачивается тихой записью notify_email.
      expect(buildPaymentUpdateCommand(basePayment(), baseForm())).toBeUndefined();
    });

    it('автоплатёж: перевод радио в «Не уведомлять» гасит галочку', () => {
      const command = buildPaymentUpdateCommand(
        basePayment({ autoPay: true, notifyAutoPaid: true, notifyEmail: true }),
        baseForm({ notifyAutoPaid: false }),
      );
      expect(command).toEqual({ notifyAutoPaid: false, notifyEmail: false });
    });

    it('ряд скрыт мастером — галочка в команде не едёт (дифф без поля)', () => {
      const command = buildPaymentUpdateCommand(
        basePayment({ reminderOffsetDays: 3, notifyEmail: true }),
        baseForm({ reminderOffsetDays: 3, notifyEmail: false }),
        { notifyEmailRowVisible: false },
      );
      expect(command).toBeUndefined();
    });
  });

  it('пустое название откатывается к лейблу категории резолвером', () => {
    const resolveTitle = (slug: string) => (slug === 'insurance' ? 'Страхование' : undefined);
    const command = buildPaymentUpdateCommand(
      basePayment({ title: 'Страхование' }),
      baseForm({ title: '   ' }),
      { resolveTitle },
    );
    expect(command).toBeUndefined();

    const renamed = buildPaymentUpdateCommand(
      basePayment({ title: 'Кредит' }),
      baseForm({ title: '' }),
      { resolveTitle },
    );
    expect(renamed).toEqual({ title: 'Страхование' });
  });

  it('пустое название с новой категорией берёт лейбл новой категории', () => {
    const command = buildPaymentUpdateCommand(
      basePayment({ title: 'Кредит' }),
      baseForm({ title: '', categorySlug: 'internet' }),
      { resolveTitle: (slug) => (slug === 'internet' ? 'Интернет' : undefined) },
    );
    expect(command).toEqual({
      title: 'Интернет',
      categorySlug: 'internet',
    });
  });

  it('пользовательская категория без выбора: пустое название не резолвится', () => {
    const payment = basePayment({
      title: 'Моя категория',
      category: { source: 'custom', id: '019abcde-0000-7000-8000-000000000003', label: 'Моя' },
    });
    expect(buildPaymentUpdateCommand(payment, baseForm({ title: '' }))).toBeUndefined();
  });
});

describe('editFormReady', () => {
  it('заполненная форма готова', () => {
    expect(editFormReady(baseForm(), 'insurance')).toBe(true);
  });

  it('не готова без суммы или с нулевой суммой', () => {
    expect(editFormReady(baseForm({ amountKopecks: undefined }), 'insurance')).toBe(false);
    expect(editFormReady(baseForm({ amountKopecks: 0 }), 'insurance')).toBe(false);
  });

  it('не готова, когда название пусто и категории-фолбэка нет', () => {
    expect(
      editFormReady(baseForm({ title: '' }), undefined, () => 'Лейбл'),
    ).toBe(false);
    expect(
      editFormReady(baseForm({ title: '' }), 'insurance', () => 'Лейбл из каталога'),
    ).toBe(true);
  });

  it('неготовая регулярность закрывает сохранение', () => {
    expect(
      editFormReady(baseForm({ recurrence: { kind: 'weekly', weekdays: [] } }), 'insurance'),
    ).toBe(false);
  });
});

describe('formAfterRecurrenceChange — молчаливый сброс endDate (#1155)', () => {
  it('окончание раньше первого вхождения нового правила сбрасывается', () => {
    // Правило monthly-15 (since 01.08), окончание 10.08; применили
    // monthly-20 — первое вхождение нового графика 20.08 позже окончания:
    // в форме endDate нет, сохранение уйдёт с tri-state endDate: null
    // (открыть срок).
    const next = formAfterRecurrenceChange(
      baseForm({ endDate: '2026-08-10' }),
      { kind: 'monthly', daysOfMonth: [20], lastDay: false },
      basePayment(),
    );
    expect(next.recurrence).toStrictEqual({ kind: 'monthly', daysOfMonth: [20], lastDay: false });
    expect(next.endDate).toBeUndefined();
    // Остальные поля формы не тронуты.
    expect(next.title).toBe('Страхование');
    expect(next.amountKopecks).toBe(320_000);
  });

  it('сброс даёт команду PATCH с endDate: null вместе с новой регулярностью', () => {
    const payment = basePayment({ endDate: '2026-08-10' });
    const form = formAfterRecurrenceChange(
      baseForm({ endDate: '2026-08-10' }),
      { kind: 'monthly', daysOfMonth: [20], lastDay: false },
      payment,
    );
    expect(buildPaymentUpdateCommand(payment, form)).toEqual({
      recurrence: { kind: 'monthly', daysOfMonth: [20], lastDay: false },
      endDate: null,
    });
  });

  it('окончание в день первого вхождения валидно — сохраняется', () => {
    const next = formAfterRecurrenceChange(
      baseForm({ endDate: '2026-10-20' }),
      { kind: 'monthly', daysOfMonth: [20], lastDay: false },
      basePayment(),
    );
    expect(next.endDate).toBe('2026-10-20');
  });

  it('смена на более раннее правило окно не ломает — окончание остаётся', () => {
    const next = formAfterRecurrenceChange(
      baseForm({ endDate: '2026-10-20' }),
      { kind: 'monthly', daysOfMonth: [10], lastDay: false },
      basePayment(),
    );
    expect(next.endDate).toBe('2026-10-20');
  });

  it('daily: первое вхождение = since, стоящее окончание после него валидно', () => {
    const next = formAfterRecurrenceChange(
      baseForm({ endDate: '2026-08-01' }),
      { kind: 'daily' },
      basePayment({ since: '2026-08-01' }),
    );
    expect(next.endDate).toBe('2026-08-01');
  });

  it('пауза сдвигает первое вхождение — окончание внутри паузы сбрасывается', () => {
    // Первое вхождение monthly-15 = 15.08, но пауза [15.08, 01.09) его
    // вырезает: график начинается 15.09 — окончание 20.08 раньше него.
    const payment = basePayment({ pauses: [{ from: '2026-08-15', to: '2026-09-01' }] });
    const next = formAfterRecurrenceChange(
      baseForm({ endDate: '2026-08-20' }),
      { kind: 'monthly', daysOfMonth: [15], lastDay: false },
      payment,
    );
    expect(next.endDate).toBeUndefined();
  });

  it('без окончания сбросить нечего — форма возвращается как есть', () => {
    const next = formAfterRecurrenceChange(
      baseForm(),
      { kind: 'monthly', daysOfMonth: [20], lastDay: false },
      basePayment(),
    );
    expect(next.endDate).toBeUndefined();
  });
});
