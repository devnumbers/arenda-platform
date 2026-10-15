import { describe, expect, it } from 'vitest';
import { ApiError } from '@/shared/api/errors';
import type { Subscription, Tariff } from '@/entities/billing';
import {
  classifyTariffChange,
  currentBadgeVisible,
  tariffActionErrorDescription,
  tariffChangeDefaults,
  tariffChangeFooter,
  tariffChangeUrlOverrides,
  tariffPriceLine,
  TARIFF_CHANGE_CARDS,
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

describe('TARIFF_CHANGE_CARDS', () => {
  it('paid cards carry the shortened sharing row and the history row (макет 3322-158610)', () => {
    for (const name of ['pro', 'business'] as const) {
      const card = TARIFF_CHANGE_CARDS.find((entry) => entry.name === name);
      expect(card?.rows.slice(1)).toStrictEqual([
        {
          kind: 'sharing',
          title: 'Совместный доступ',
          description: 'Приглашайте в свои объекты близких и коллег',
        },
        {
          kind: 'history',
          title: 'История действий',
          description: 'Отслеживайте любые изменения в объектах',
        },
      ]);
    }
  });

  it('basic card has the objects row only', () => {
    const card = TARIFF_CHANGE_CARDS.find((entry) => entry.name === 'basic');
    expect(card?.rows.map((row) => row.kind)).toStrictEqual(['objects']);
  });
});

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

describe('tariffChangeUrlOverrides', () => {
  it('deep link from the landing preselects the paid tariff it names', () => {
    expect(tariffChangeUrlOverrides(new URLSearchParams('tariff=pro'))).toStrictEqual({
      tariff: 'pro',
    });
    expect(tariffChangeUrlOverrides(new URLSearchParams('tariff=business'))).toStrictEqual({
      tariff: 'business',
    });
  });

  it('period rides along when valid', () => {
    expect(
      tariffChangeUrlOverrides(new URLSearchParams('tariff=pro&period=month')),
    ).toStrictEqual({ tariff: 'pro', period: 'month' });
  });

  it('unknown tariff, basic and junk are ignored — выбор базового живёт только во флоу отключения', () => {
    expect(tariffChangeUrlOverrides(new URLSearchParams('tariff=basic'))).toStrictEqual({});
    expect(tariffChangeUrlOverrides(new URLSearchParams('tariff=ultimate'))).toStrictEqual({});
    expect(tariffChangeUrlOverrides(new URLSearchParams('period=month'))).toStrictEqual({});
    expect(tariffChangeUrlOverrides(null)).toStrictEqual({});
  });

  it('invalid period does not sink a valid tariff', () => {
    expect(
      tariffChangeUrlOverrides(new URLSearchParams('tariff=pro&period=week')),
    ).toStrictEqual({ tariff: 'pro' });
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
  const now = new Date('2026-09-15T12:00:00Z');

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

  it('cancelled on own tariff and period with a live remainder offers the free resume (#691)', () => {
    expect(
      tariffChangeFooter(
        subscription({ status: 'cancelled', autoRenewEnabled: false }),
        proTariff,
        'month',
        now,
      ),
    ).toStrictEqual({ kind: 'resume', label: 'Возобновить Про' });
  });

  it('cancelled on own tariff and period with an expired period goes through payment (#429)', () => {
    expect(
      tariffChangeFooter(
        subscription({
          status: 'cancelled',
          autoRenewEnabled: false,
          validUntil: '2026-09-01T12:00:00Z',
        }),
        proTariff,
        'month',
        now,
      ),
    ).toStrictEqual({ kind: 'connect', label: 'Подключить за 490 ₽ в месяц' });
  });

  it('cancelled with a live remainder switching the period goes through payment (#691 boundary)', () => {
    expect(
      tariffChangeFooter(
        subscription({ status: 'cancelled', autoRenewEnabled: false }),
        proTariff,
        'year',
        now,
      ),
    ).toStrictEqual({ kind: 'connect', label: 'Подключить за 4\u00A0400 ₽ в год' });
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

describe('classifyTariffChange — зеркало ClassifyTariffChange бэка (#1452)', () => {
  it('лимит решает раньше цен: безлимит бьёт любой конечный лимит', () => {
    const cheapUnlimited: Tariff = {
      ...businessTariff,
      monthlyPriceKopecks: 100,
      yearlyPriceKopecks: 1000,
    };
    expect(classifyTariffChange(proTariff, cheapUnlimited)).toBe('upgrade');
    expect(classifyTariffChange(cheapUnlimited, proTariff)).toBe('downgrade');
  });

  it('равные лимиты различает месячная цена', () => {
    const cheaperSameLimit: Tariff = { ...proTariff, monthlyPriceKopecks: 400 };
    expect(classifyTariffChange(proTariff, cheaperSameLimit)).toBe('downgrade');
    expect(classifyTariffChange(cheaperSameLimit, proTariff)).toBe('upgrade');
  });

  it('равные лимит и месячная цена различаются годовой', () => {
    const cheaperYear: Tariff = { ...proTariff, yearlyPriceKopecks: 400000 };
    expect(classifyTariffChange(proTariff, cheaperYear)).toBe('downgrade');
    expect(classifyTariffChange(cheaperYear, proTariff)).toBe('upgrade');
  });

  it('равные тарифы — Same (вне скоупа #1450, при текущих ценах недостижимо)', () => {
    const twin: Tariff = { ...proTariff, id: 'twin' };
    expect(classifyTariffChange(proTariff, twin)).toBe('same');
  });
});

describe('tariffChangeFooter — отложенный даунгрейд на «Остановлен» и grace (#1452, ADR 0066)', () => {
  const now = new Date('2026-09-15T12:00:00Z');
  const cancelledBusiness = subscription({
    status: 'cancelled',
    tariff: businessTariff,
    autoRenewEnabled: false,
  });

  it('отменённая Бизнес × более дешёвый Про — подтверждение с датой применения', () => {
    expect(tariffChangeFooter(cancelledBusiness, proTariff, 'month', now)).toStrictEqual({
      kind: 'downgrade',
      title: 'Сменить тариф на Про?',
      description:
        'Изменения вступят в силу 10 октября — в конце оплаченного периода. '
        + 'Тогда же спишем 490 ₽ в месяц за тариф Про. Сейчас ничего не списываем.',
      confirmLabel: 'Сменить тариф',
    });
  });

  it('годовой целевой тариф несёт свою цену', () => {
    expect(tariffChangeFooter(cancelledBusiness, proTariff, 'year', now)).toStrictEqual({
      kind: 'downgrade',
      title: 'Сменить тариф на Про?',
      description:
        'Изменения вступят в силу 10 октября — в конце оплаченного периода. '
        + 'Тогда же спишем 4\u00A0400 ₽ в год за тариф Про. Сейчас ничего не списываем.',
      confirmLabel: 'Сменить тариф',
    });
  });

  it('grace × более дешёвый — то же подтверждение с дедлайном окна', () => {
    expect(
      tariffChangeFooter(
        subscription({
          status: 'grace',
          tariff: businessTariff,
          autoRenewEnabled: true,
        }),
        proTariff,
        'month',
        now,
      ),
    ).toStrictEqual({
      kind: 'downgrade',
      title: 'Сменить тариф на Про?',
      description:
        'Изменения вступят в силу 10 октября. '
        + 'Тогда же спишем 490 ₽ в месяц за тариф Про. Сейчас ничего не списываем.',
      confirmLabel: 'Сменить тариф',
    });
  });

  it('свой тариф и период на отменённой — бесплатное возобновление раньше даунгрейда (#691)', () => {
    expect(
      tariffChangeFooter(
        subscription({ status: 'cancelled', tariff: businessTariff, autoRenewEnabled: false }),
        businessTariff,
        'month',
        now,
      ),
    ).toStrictEqual({ kind: 'resume', label: 'Возобновить Бизнес' });
  });

  it('более дорогой на отменённой — платёжный путь реактивации (#429), без подтверждения', () => {
    expect(
      tariffChangeFooter(
        subscription({ status: 'cancelled', tariff: proTariff, autoRenewEnabled: false }),
        businessTariff,
        'month',
        now,
      ),
    ).toStrictEqual({ kind: 'connect', label: 'Подключить за 990 ₽ в месяц' });
  });

  it('активная подписка × более дешёвый — прежнее отложенное планирование, без подтверждения', () => {
    expect(
      tariffChangeFooter(
        subscription({ tariff: businessTariff }),
        proTariff,
        'month',
        now,
      ),
    ).toStrictEqual({ kind: 'connect', label: 'Подключить за 490 ₽ в месяц' });
  });

  it('истёкший остаток отменённой — транзит до тика воркера, платёжный путь', () => {
    expect(
      tariffChangeFooter(
        subscription({
          status: 'cancelled',
          tariff: businessTariff,
          autoRenewEnabled: false,
          validUntil: '2026-09-01T12:00:00Z',
        }),
        proTariff,
        'month',
        now,
      ),
    ).toStrictEqual({ kind: 'connect', label: 'Подключить за 490 ₽ в месяц' });
  });

  it('истёкшее grace-окно планированию не подлежит — платёжный путь до тика воркера', () => {
    expect(
      tariffChangeFooter(
        subscription({
          status: 'grace',
          tariff: businessTariff,
          validUntil: '2026-09-01T12:00:00Z',
        }),
        proTariff,
        'month',
        now,
      ),
    ).toStrictEqual({ kind: 'connect', label: 'Подключить за 490 ₽ в месяц' });
  });
});

describe('tariffActionErrorDescription — человеческие тосты 409 (#1452, край #1450 п.4)', () => {
  const conflictText = 'Сейчас нельзя сменить тариф';

  it('честный 409 без кода показывает человеческий текст, не сырой detail', () => {
    const error = new ApiError(
      'about:blank',
      'Некорректное состояние подписки',
      undefined,
      409,
    );
    expect(tariffActionErrorDescription(error, conflictText)).toBe(conflictText);
  });

  it('закодированные resume-ошибки бэка уже человеческие — проходят как есть', () => {
    const error = new ApiError(
      'resume_required',
      'Период уже оплачен: возобновите подписку бесплатно',
      undefined,
      409,
    );
    expect(tariffActionErrorDescription(error, conflictText)).toBe(
      'Период уже оплачен: возобновите подписку бесплатно',
    );
  });

  it('не-конфликты сохраняют detail бэка', () => {
    const error = new ApiError(
      'network_error',
      'Не удалось выполнить запрос. Проверьте подключение к интернету.',
    );
    expect(tariffActionErrorDescription(error, conflictText)).toBe(
      'Не удалось выполнить запрос. Проверьте подключение к интернету.',
    );
  });
});
