import { describe, expect, it } from 'vitest';
import { paddedItems } from './wheel-items';

describe('paddedItems', () => {
  it('часы 24: 00–23 с двузначным паддингом', () => {
    const items = paddedItems(24);
    expect(items).toHaveLength(24);
    expect(items[0]).toStrictEqual({ value: '00', label: '00' });
    expect(items[9]).toStrictEqual({ value: '09', label: '09' });
    expect(items[10]).toStrictEqual({ value: '10', label: '10' });
    expect(items[23]).toStrictEqual({ value: '23', label: '23' });
  });

  it('минуты 60: 00–59 с двузначным паддингом', () => {
    const items = paddedItems(60);
    expect(items).toHaveLength(60);
    expect(items[0]).toStrictEqual({ value: '00', label: '00' });
    expect(items[59]).toStrictEqual({ value: '59', label: '59' });
  });

  it('value совпадает с меткой у каждого предмета — контракт WheelPicker: выбор по value', () => {
    for (const length of [24, 60]) {
      const items = paddedItems(length);
      for (const [index, item] of items.entries()) {
        expect(item.value, `length=${length} index=${index}`).toBe(item.label);
      }
    }
  });
});
