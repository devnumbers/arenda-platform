import { describe, expect, it } from 'vitest';
import { makePayment as payment } from '@/entities/payment';
import { catalogRules } from './payments-catalog-model';

describe('catalogRules', () => {
  const manual = payment({ id: 'manual', title: 'Арендная плата' });
  const auto = payment({ id: 'auto', title: 'Электроэнергия', autoPay: true });

  it('срез «Платежи» — только ручные правила, автоплатежи не протекают (#1071)', () => {
    expect(catalogRules([manual, auto], 'payments')).toStrictEqual([manual]);
  });

  it('срез «Автоплатежи» — только автоплатежи', () => {
    expect(catalogRules([manual, auto], 'auto')).toStrictEqual([auto]);
  });

  it('объект с одними автоплатежами: срез «Платежи» пуст — пустое состояние', () => {
    expect(catalogRules([auto], 'payments')).toStrictEqual([]);
  });

  it('порядок входного списка сохраняется (сортировка — снаружи)', () => {
    const second = payment({ id: 'second' });
    const first = payment({ id: 'first', autoPay: true });

    expect(catalogRules([second, first], 'auto').map((row) => row.id)).toEqual(['first']);
    expect(catalogRules([second, first], 'payments').map((row) => row.id)).toEqual(['second']);
  });
});
