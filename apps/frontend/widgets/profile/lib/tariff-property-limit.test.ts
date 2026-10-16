import { describe, expect, it } from 'vitest';
import { propertyLimitUpgradeText } from './tariff-property-limit';

describe('propertyLimitUpgradeText', () => {
  it('basic: называет оба платных тарифа', () => {
    expect(propertyLimitUpgradeText('basic')).toBe(
      'Повысьте тариф до «Про» или «Бизнес», чтобы добавить больше объектов',
    );
  });

  it('pro: называет только «Бизнес»', () => {
    expect(propertyLimitUpgradeText('pro')).toBe(
      'Повысьте тариф до «Бизнес», чтобы добавить больше объектов',
    );
  });

  it('подписки нет (null) — читается как basic', () => {
    expect(propertyLimitUpgradeText(null)).toBe(
      'Повысьте тариф до «Про» или «Бизнес», чтобы добавить больше объектов',
    );
  });

  it('подписка ещё не загрузилась (undefined) — читается как basic', () => {
    expect(propertyLimitUpgradeText(undefined)).toBe(
      'Повысьте тариф до «Про» или «Бизнес», чтобы добавить больше объектов',
    );
  });

  it('defensive-фолбэк: business (безлимит) и неизвестные имена тарифа', () => {
    const fallback = 'Повысьте тариф, чтобы добавить больше объектов';
    expect(propertyLimitUpgradeText('business')).toBe(fallback);
    expect(propertyLimitUpgradeText('ultra')).toBe(fallback);
  });
});
