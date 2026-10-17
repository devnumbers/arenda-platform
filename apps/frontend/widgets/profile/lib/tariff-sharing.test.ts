import { describe, expect, it } from 'vitest';
import { SHARING_TITLE, SHARING_UPGRADE_TEXT } from './tariff-sharing';

describe('интерстишал «Совместный доступ недоступен» (#1470, макет 3518-80047)', () => {
  it('заголовок H2 — по макету', () => {
    expect(SHARING_TITLE).toBe('Совместный доступ недоступен');
  });

  it('текст описания — один статичный, называет оба платных тарифа (в тикете; adaptivity #1469 сюда не перенесена)', () => {
    expect(SHARING_UPGRADE_TEXT).toBe(
      'Повысьте тариф до «Про» или «Бизнес», чтобы пользоваться совместным доступом',
    );
  });
});
