import { describe, expect, it } from 'vitest';
import { paymentDetailActions } from './payment-detail-actions-model';

describe('paymentDetailActions — состав круглых действий (#818)', () => {
  it('обычное правило: пауза, правка, оплата', () => {
    expect(paymentDetailActions({ isRentalManaged: false, isRentalCompleted: false }, false, false, false, false))
      .toStrictEqual(['pause', 'edit', 'pay']);
  });

  it('на паузе: возобновление вместо паузы — серверный isCompleted не решает (у бессрочной паузы он true, багфикс «Домофона» полного e2e)', () => {
    expect(paymentDetailActions({ isRentalManaged: false, isRentalCompleted: false }, true, false, false, false))
      .toStrictEqual(['resume', 'edit', 'pay']);
  });

  it('завершённое правило (клиентский предикат, история 44 спеки #453): паузы нет', () => {
    expect(paymentDetailActions({ isRentalManaged: false, isRentalCompleted: false }, false, true, false, false))
      .toStrictEqual(['edit', 'pay']);
  });

  it('управляемый платёж незавершённой аренды — «Изменить аренду» + «Оплатить» (#1158): правка правила даёт 409, условия правятся через аренду', () => {
    expect(paymentDetailActions({ isRentalManaged: true, isRentalCompleted: false }, false, false, false, false))
      .toStrictEqual(['editRental', 'pay']);
  });

  it('управляемый платёж завершённой аренды — только «Оплатить»: «Завершена» — финал, правки условий нет (#1158)', () => {
    expect(paymentDetailActions({ isRentalManaged: true, isRentalCompleted: true }, false, false, false, false))
      .toStrictEqual(['pay']);
  });

  it('состояние аренды решает, не клиентский предикат завершённости правила: «Ожидает действия» (isCompleted-истина) ещё правится', () => {
    expect(paymentDetailActions({ isRentalManaged: true, isRentalCompleted: false }, false, true, true, false))
      .toStrictEqual(['editRental', 'pay']);
  });
});

describe('paymentDetailActions — вычисляемая завершённость: «Оплатить» → «Завершить» (#1341)', () => {
  it('расписание закончилось (endDate прошла) и всё оплачено: вместо «Оплатить» — «Завершить», «Изменить» остаётся', () => {
    expect(paymentDetailActions({ isRentalManaged: false, isRentalCompleted: false }, false, true, true, false))
      .toStrictEqual(['edit', 'complete']);
  });

  it('досрочное исчерпание (всё оплачено при endDate впереди): флаг гонит «Завершить», пауза и правка остаются — гейт мутации на сервере', () => {
    expect(paymentDetailActions({ isRentalManaged: false, isRentalCompleted: false }, false, false, true, false))
      .toStrictEqual(['pause', 'edit', 'complete']);
  });

  it('открытая пауза запрещает «Завершить» (сервер отвечает 409 до снятия): у паузного правила слот оплаты не заменяется', () => {
    expect(paymentDetailActions({ isRentalManaged: false, isRentalCompleted: false }, true, false, true, false))
      .toStrictEqual(['resume', 'edit', 'pay']);
  });

  it('пауза при закончившемся расписании (флаг завершённости, endDate прошла): паузы нет, «Завершить» не появляется — мутация закрыта паузой', () => {
    expect(paymentDetailActions({ isRentalManaged: false, isRentalCompleted: false }, true, true, true, false))
      .toStrictEqual(['edit', 'pay']);
  });

  it('у арендно-управляемого платежа «Завершить» нет: complete гейтится арендой (409)', () => {
    expect(paymentDetailActions({ isRentalManaged: true, isRentalCompleted: false }, false, true, true, false))
      .toStrictEqual(['editRental', 'pay']);
  });
});

describe('paymentDetailActions — архивный объект (#1342, правка владельца 13.10)', () => {
  it('архив + системная автопауза: строка пуста — ни кнопок, ни пояснения (состояние несут бейдж карточки и списки)', () => {
    expect(paymentDetailActions({ isRentalManaged: false, isRentalCompleted: false }, true, false, false, true))
      .toStrictEqual([]);
  });

  it('архив без паузы (расписание закончилось): строка пуста', () => {
    expect(paymentDetailActions({ isRentalManaged: false, isRentalCompleted: false }, false, true, false, true))
      .toStrictEqual([]);
  });

  it('архив + активное правило: строка пуста', () => {
    expect(paymentDetailActions({ isRentalManaged: false, isRentalCompleted: false }, false, false, false, true))
      .toStrictEqual([]);
  });

  it('архив + ручная пауза: строка пуста и здесь — ручная пауза переживает архив, возобновлять можно только после возврата', () => {
    expect(paymentDetailActions({ isRentalManaged: false, isRentalCompleted: false }, true, true, true, true))
      .toStrictEqual([]);
  });

  it('архив + управляемый арендой платёж: строка пуста — на архивном объекте состав не зависит от аренды', () => {
    expect(paymentDetailActions({ isRentalManaged: true, isRentalCompleted: false }, true, false, false, true))
      .toStrictEqual([]);
  });
});
