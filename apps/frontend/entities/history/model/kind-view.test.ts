import { describe, expect, it } from 'vitest';
import { HISTORY_BASE_ACTIONS, HISTORY_KINDS, kindLabel } from './kind-view';

describe('kind-view', () => {
  it('канонические подписи видов действий — группы чекбоксов фильтра (#711)', () => {
    expect(kindLabel('property')).toBe('Объект');
    expect(kindLabel('rental')).toBe('Аренда');
    expect(kindLabel('payment')).toBe('Платежи');
    expect(kindLabel('operation')).toBe('Операции');
    expect(kindLabel('contact')).toBe('Контакты');
    expect(kindLabel('task')).toBe('Задачи');
    expect(kindLabel('member')).toBe('Участники');
  });

  it('HISTORY_KINDS — все семь видов словаря (ADR 0061 §4) в порядке макета', () => {
    expect([...HISTORY_KINDS]).toEqual([
      'property',
      'rental',
      'payment',
      'operation',
      'contact',
      'task',
      'member',
    ]);
  });

  it('HISTORY_BASE_ACTIONS — все четыре основных действия (ADR 0061 §4) в порядке макета', () => {
    expect([...HISTORY_BASE_ACTIONS]).toEqual(['added', 'changed', 'completed', 'deleted']);
  });
});
