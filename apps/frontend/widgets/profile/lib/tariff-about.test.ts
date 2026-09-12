import { describe, expect, it } from 'vitest';
import type { Subscription, Tariff } from '@/entities/billing';
import {
  resumeSuccessTitle,
  tariffAboutCard,
  tariffFeatureRows,
} from './tariff-about';

const proTariff: Tariff = {
  id: 'pro',
  name: 'pro',
  monthlyPriceKopecks: 49000,
  yearlyPriceKopecks: 490000,
  activePropertyLimit: 5,
};

const businessTariff: Tariff = {
  id: 'business',
  name: 'business',
  monthlyPriceKopecks: 99000,
  yearlyPriceKopecks: 890000,
  activePropertyLimit: -1,
};

const basicTariff: Tariff = {
  id: 'basic',
  name: 'basic',
  monthlyPriceKopecks: 0,
  yearlyPriceKopecks: 0,
  activePropertyLimit: 1,
};

function subscription(overrides: Partial<Subscription>): Subscription {
  return {
    id: 'active-pro-2026-09-10',
    status: 'active',
    tariff: proTariff,
    autoRenewEnabled: true,
    validUntil: '2026-09-10T21:00:00Z',
    currentPeriod: 'month',
    ...overrides,
  };
}

describe('tariffAboutCard', () => {
  it('active paid: price, next charge date, payment method', () => {
    expect(
      tariffAboutCard(subscription({
        activePaymentMethod: {
          id: 'pm-1',
          displayMask: '4111********1111',
          provider: 'tkassa',
          isActive: true,
          createdAt: '2026-08-01T10:00:00Z',
        },
      })),
    ).toStrictEqual({
      title: 'Про',
      rows: [
        { label: 'Стоимость', value: 'Вы платите 490 ₽ в месяц' },
        { label: 'Следующее списание', value: '10 сентября' },
        { label: 'Способ оплаты', value: '•••• 1111' },
      ],
    });
  });

  it('active paid year: price follows the current period', () => {
    const card = tariffAboutCard(subscription({ currentPeriod: 'year' }));
    expect(card.rows[0]).toStrictEqual({
      label: 'Стоимость',
      value: 'Вы платите 4\u00A0900 ₽ в год',
    });
  });

  it('active paid without a payment method omits the method row', () => {
    const card = tariffAboutCard(subscription({}));
    expect(card.rows.map((row) => row.label)).toStrictEqual([
      'Стоимость',
      'Следующее списание',
    ]);
  });

  it('payment method: mask without a system name keeps only the last-4 tail', () => {
    const card = tariffAboutCard(subscription({
      activePaymentMethod: {
        id: 'pm-1',
        displayMask: 'Мир •••• 0700',
        provider: 'tkassa',
        isActive: true,
        createdAt: '2026-08-01T10:00:00Z',
      },
    }));
    expect(card.rows.at(-1)).toStrictEqual({
      label: 'Способ оплаты',
      value: '•••• 0700',
    });
  });

  it('payment method: an unparseable mask is shown as is', () => {
    const card = tariffAboutCard(subscription({
      activePaymentMethod: {
        id: 'pm-1',
        displayMask: '**',
        provider: 'tkassa',
        isActive: true,
        createdAt: '2026-08-01T10:00:00Z',
      },
    }));
    expect(card.rows.at(-1)).toStrictEqual({
      label: 'Способ оплаты',
      value: '**',
    });
  });

  it('grace: pay-by row replaces the charge date', () => {
    expect(
      tariffAboutCard(subscription({
        status: 'grace',
        validUntil: '2026-09-17T21:00:00Z',
      })),
    ).toStrictEqual({
      title: 'Про',
      rows: [
        { label: 'Стоимость', value: 'Вы платите 490 ₽ в месяц' },
        { label: 'Оплатите тариф', value: 'До 17 сентября' },
      ],
    });
  });

  it('cancelled: stopped title with the valid-until row only', () => {
    expect(tariffAboutCard(subscription({ status: 'cancelled' }))).toStrictEqual({
      title: 'Про отключен',
      rows: [{ label: 'Действует до', value: '10 сентября' }],
    });
  });

  it('basic: free price row', () => {
    expect(
      tariffAboutCard(subscription({
        tariff: basicTariff,
        validUntil: undefined,
        currentPeriod: undefined,
      })),
    ).toStrictEqual({
      title: 'Базовый',
      rows: [{ label: 'Стоимость', value: 'Бесплатно' }],
    });
  });
});

describe('tariffFeatureRows', () => {
  it('paid tariff: object limit from API plus sharing', () => {
    expect(tariffFeatureRows(proTariff)).toStrictEqual([
      {
        kind: 'objects',
        title: 'До 5 объектов',
        description: 'Для небольшого портфеля недвижимости',
      },
      {
        kind: 'sharing',
        title: 'Совместный доступ',
        description:
          'Приглашайте в свои объекты близких и коллег, оплатить подписку нужно только вам',
      },
    ]);
  });

  it('basic: object limit row only', () => {
    expect(tariffFeatureRows(basicTariff)).toStrictEqual([
      {
        kind: 'objects',
        title: 'До 1 объекта',
        description: 'Для небольшого портфеля недвижимости',
      },
    ]);
  });

  it('business: unlimited limit keeps the sharing row', () => {
    const rows = tariffFeatureRows(businessTariff);
    expect(rows[0]?.title).toBe('Неограниченно');
    expect(rows.map((row) => row.kind)).toStrictEqual(['objects', 'sharing']);
  });

  it('pluralizes 2-4 limits', () => {
    const rows = tariffFeatureRows({ ...proTariff, activePropertyLimit: 2 });
    expect(rows[0]?.title).toBe('До 2 объектов');
  });
});

describe('resumeSuccessTitle', () => {
  it('names the resumed tariff', () => {
    expect(resumeSuccessTitle('pro')).toBe('Тариф Про возобновлен');
    expect(resumeSuccessTitle('business')).toBe('Тариф Бизнес возобновлен');
  });
});
