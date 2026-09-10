import { describe, expect, it } from 'vitest';
import { validateOperationWizardDraft } from './use-operation-wizard-draft';

describe('validateOperationWizardDraft', () => {
  it('полный черновик читается как сохранён', () => {
    expect(
      validateOperationWizardDraft({
        type: 'income',
        title: 'Аренда мебели',
        categorySlug: 'rent',
        amountKopecks: 600_000,
        propertyId: 'property-1',
      }),
    ).toStrictEqual({
      type: 'income',
      title: 'Аренда мебели',
      categorySlug: 'rent',
      amountKopecks: 600_000,
      propertyId: 'property-1',
    });
  });

  it('не объект и массив роняют черновик в пустой', () => {
    expect(validateOperationWizardDraft(null)).toStrictEqual({});
    expect(validateOperationWizardDraft('draft')).toStrictEqual({});
    expect(validateOperationWizardDraft([])).toStrictEqual({});
  });

  it('неизвестное направление роняет черновик целиком', () => {
    expect(
      validateOperationWizardDraft({ type: 'transfer', amountKopecks: 100 }),
    ).toStrictEqual({});
  });

  it('битая сумма или категория роняют черновик целиком', () => {
    expect(validateOperationWizardDraft({ amountKopecks: -5 })).toStrictEqual({});
    expect(validateOperationWizardDraft({ amountKopecks: 10.5 })).toStrictEqual({});
    expect(validateOperationWizardDraft({ categorySlug: '' })).toStrictEqual({});
    expect(validateOperationWizardDraft({ propertyId: 42 })).toStrictEqual({});
  });

  it('пустое название отбрасывается по полю, остальное живёт', () => {
    expect(
      validateOperationWizardDraft({ title: '', amountKopecks: 100 }),
    ).toStrictEqual({ amountKopecks: 100 });
  });
});
