import { describe, expect, it } from 'vitest';
import { paymentCategoryVisualFromSnapshot } from './category-visual';
import { categoryIconComponents } from './icon-registry';
import { paymentCategoryBySlug, userCategoryDefault } from './generated/categories';

/** Снимок категории платежа из payload/кадра (#1348) → кругляш: слаг
 * резолвится каталогом, id пользовательской — дефолтом пользовательских;
 * приоритет кругляша (иконка категории главнее фото) решает потребитель. */
describe('paymentCategoryVisualFromSnapshot — снимок категории платежа (#1348)', () => {
  it('слаг каталога даёт иконку и цвет каталога', () => {
    const entry = paymentCategoryBySlug('water-supply');
    if (entry === undefined) throw new Error('water-supply отсутствует в каталоге');
    const visual = paymentCategoryVisualFromSnapshot({ slug: 'water-supply' });
    expect(visual?.component).toBe(categoryIconComponents[entry.icon]);
    expect(visual?.color).toBe(entry.color);
  });

  it('пользовательская категория — дефолт пользовательских (bold-other на сером)', () => {
    const visual = paymentCategoryVisualFromSnapshot({
      userCategoryId: '0194a3f8-0000-7000-8000-000000000009',
    });
    expect(visual?.component).toBe(categoryIconComponents[userCategoryDefault.icon]);
    expect(visual?.color).toBe(userCategoryDefault.color);
  });

  it('приоритет внутри снимка: id пользовательской главнее слага', () => {
    const visual = paymentCategoryVisualFromSnapshot({
      slug: 'water-supply',
      userCategoryId: '0194a3f8-0000-7000-8000-000000000009',
    });
    expect(visual?.component).toBe(categoryIconComponents[userCategoryDefault.icon]);
  });

  it('неизвестный слаг деградирует в дефолт пользовательских', () => {
    const visual = paymentCategoryVisualFromSnapshot({ slug: 'removed-from-catalog' });
    expect(visual?.component).toBe(categoryIconComponents[userCategoryDefault.icon]);
    expect(visual?.color).toBe(userCategoryDefault.color);
  });

  it('нет снимка — нет кругляша (фото/стандартная иконка потребителя)', () => {
    expect(paymentCategoryVisualFromSnapshot(undefined)).toBeUndefined();
  });
});
