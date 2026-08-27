import { describe, expect, it } from 'vitest';
import {
  paymentCategories,
  userCategoryDefault,
} from '@/features/payment-categories/lib/generated/categories';
import { categoryIconComponents } from './icon-registry';

describe('реестр иконок покрывает каталог #447 целиком', () => {
  it('у каждой категории каталога есть компонент иконки', () => {
    const missing = paymentCategories
      .map((entry) => entry.icon)
      .filter((icon) => categoryIconComponents[icon] === undefined);
    expect(missing).toStrictEqual([]);
  });

  it('дефолт пользовательской категории (bold-other) есть в реестре', () => {
    expect(categoryIconComponents[userCategoryDefault.icon]).toBeDefined();
  });
});
