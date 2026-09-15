import { describe, expect, it } from 'vitest';
import type { Subscription, Tariff } from '@/entities/billing';
import {
  currentBadgeVisible,
  tariffChangeDefaults,
  tariffChangeFooter,
  tariffPriceLine,
  yearlyDiscountPercent,
  yearlyPerMonthLine,
} from './tariff-change';

const basicTariff: Tariff = {
  id: 'basic',
  name: 'basic',
  monthlyPriceKopecks: 0,
  yearlyPriceKopecks: 0,
  activePropertyLimit: 1,
};

const proTariff: Tariff = {
  id: 'pro',
  name: 'pro',
  monthlyPriceKopecks: 49000,
  yearlyPriceKopecks: 440000,
  activePropertyLimit: 5,
};

const businessTariff: Tariff = {
  id: 'business',
  name: 'business',
  monthlyPriceKopecks: 99000,
  yearlyPriceKopecks: 890000,
  activePropertyLimit: -1,
};

function subscription(overrides: Partial<Subscription>): Subscription {
  return {
    id: 'sub-1',
    status: 'active',
    tariff: proTariff,
    autoRenewEnabled: true,
    validUntil: '2026-10-10T21:00:00Z',
    currentPeriod: 'month',
    ...overrides,
  };
}

describe('yearlyDiscountPercent', () => {
  it('25% for mockup prices, computed from data not a constant', () => {
    expect(yearlyDiscountPercent([basicTariff, proTariff, businessTariff])).toBe(25);
  });

  it('undefined when no paid tariff is discounted yearly', () => {
    const flat: Tariff = {
      ...proTariff,
      monthlyPriceKopecks: 40000,
      yearlyPriceKopecks: 480000,
    };
    expect(yearlyDiscountPercent([flat])).toBeUndefined();
  });

  it('picks the largest discount across paid tariffs', () => {
    const deeper: Tariff = {
      ...businessTariff,
      monthlyPriceKopecks: 99000,
      yearlyPriceKopecks: 594000,
    };
    expect(yearlyDiscountPercent([proTariff, deeper])).toBe(50);
  });
});

describe('tariffPriceLine', () => {
  it('year price with period word', () => {
    expect(tariffPriceLine(proTariff, 'year')).toBe('4\u00A0400 ₽ в год');
  });

  it('month price with period word', () => {
    expect(tariffPriceLine(businessTariff, 'month')).toBe('990 ₽ в месяц');
  });

  it('basic is free for either period', () => {
    expect(tariffPriceLine(basicTariff, 'year')).toBe('Бесплатно');
    expect(tariffPriceLine(basicTariff, 'month')).toBe('Бесплатно');
  });
});

describe('yearlyPerMonthLine', () => {
  it('recalculates the year price as a monthly one, rounded to rubles', () => {
    expect(yearlyPerMonthLine(proTariff)).toBe('367 ₽ в месяц');
    expect(yearlyPerMonthLine(businessTariff)).toBe('742 ₽ в месяц');
  });
});

describe('tariffChangeDefaults', () => {
  it('paid subscription opens on its own tariff and period', () => {
    expect(tariffChangeDefaults(subscription({ currentPeriod: 'month' }))).toStrictEqual({
      tariff: 'pro',
      period: 'month',
    });
    expect(tariffChangeDefaults(subscription({ currentPeriod: 'year' }))).toStrictEqual({
      tariff: 'pro',
      period: 'year',
    });
  });

  it('fresh account opens on year Pro (owner decision)', () => {
    expect(
      tariffChangeDefaults(
        subscription({
          tariff: basicTariff,
          currentPeriod: undefined,
          validUntil: undefined,
        }),
      ),
    ).toStrictEqual({ tariff: 'pro', period: 'year' });
  });
});

describe('currentBadgeVisible', () => {
  it('paid tariff: only when the selected period matches the current one', () => {
    expect(currentBadgeVisible(subscription({}), 'pro', 'month')).toBe(true);
    expect(currentBadgeVisible(subscription({}), 'pro', 'year')).toBe(false);
  });

  it('basic: the label survives a period switch (owner rule)', () => {
    const onBasic = subscription({ tariff: basicTariff, currentPeriod: undefined });
    expect(currentBadgeVisible(onBasic, 'basic', 'year')).toBe(true);
    expect(currentBadgeVisible(onBasic, 'basic', 'month')).toBe(true);
  });

  it('other tariffs never carry the label', () => {
    expect(currentBadgeVisible(subscription({}), 'business', 'month')).toBe(false);
  });
});

describe('tariffChangeFooter', () => {
  it('active + same tariff and period is connected', () => {
    expect(tariffChangeFooter(subscription({}), proTariff, 'month')).toStrictEqual({
      kind: 'connected',
    });
  });

  it('active + same tariff other period offers the year price', () => {
    expect(tariffChangeFooter(subscription({}), proTariff, 'year')).toStrictEqual({
      kind: 'connect',
      label: 'Подключить за 4\u00A0400 ₽ в год',
    });
  });

  it('grace renewal of the same tariff goes through payment (owner decision)', () => {
    expect(
      tariffChangeFooter(subscription({ status: 'grace' }), proTariff, 'month'),
    ).toStrictEqual({ kind: 'connect', label: 'Подключить за 490 ₽ в месяц' });
  });

  it('cancelled reactivation of the same tariff goes through payment (owner decision)', () => {
    expect(
      tariffChangeFooter(subscription({ status: 'cancelled' }), proTariff, 'month'),
    ).toStrictEqual({ kind: 'connect', label: 'Подключить за 490 ₽ в месяц' });
  });

  it('another paid tariff offers its price', () => {
    expect(tariffChangeFooter(subscription({}), businessTariff, 'month')).toStrictEqual({
      kind: 'connect',
      label: 'Подключить за 990 ₽ в месяц',
    });
  });

  it('basic selection on a paid subscription leads to the disable flow', () => {
    expect(tariffChangeFooter(subscription({}), basicTariff, 'year')).toStrictEqual({
      kind: 'disable',
    });
  });

  it('basic selection on a basic account is connected', () => {
    expect(
      tariffChangeFooter(
        subscription({ tariff: basicTariff, currentPeriod: undefined }),
        basicTariff,
        'month',
      ),
    ).toStrictEqual({ kind: 'connected' });
  });
});
