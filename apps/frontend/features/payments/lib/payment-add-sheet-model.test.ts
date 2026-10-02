import { describe, expect, it } from 'vitest';
import { paymentAddSheetState } from './payment-add-sheet-model';

describe('paymentAddSheetState', () => {
  it('без фиксированного типа: до загрузки черновиков контент не выбирается', () => {
    expect(paymentAddSheetState(false, undefined)).toEqual({ kind: 'loading' });
    expect(paymentAddSheetState(false, 'payment')).toEqual({ kind: 'loading' });
  });

  it('без фиксированного типа: есть черновик — модалка последнего тронутого', () => {
    expect(paymentAddSheetState(true, 'autopayment')).toEqual({
      kind: 'draft',
      draftType: 'autopayment',
    });
  });

  it('без фиксированного типа: черновиков нет — выбор типа', () => {
    expect(paymentAddSheetState(true, undefined)).toEqual({ kind: 'choice' });
  });

  it('фиксированный тип (каталог, «Объекты»): сразу модалка черновика, шит открывают только при черновике', () => {
    expect(paymentAddSheetState(true, undefined, 'autopayment')).toEqual({
      kind: 'draft',
      draftType: 'autopayment',
    });
    expect(paymentAddSheetState(false, undefined, 'payment')).toEqual({
      kind: 'draft',
      draftType: 'payment',
    });
  });
});
