import {
  type PaymentPeriod,
  type Subscription,
  type Tariff,
} from '@/entities/billing';
import { getTariffLabel, isPaidTariff, type TariffName } from '@/entities/user';
import { formatMoneyKopecks, ratioToPercent } from '@/shared/lib/format-money';

/** Вид-модель экрана «Выбрать тариф» (#623, макеты 1919-74867 год,
 * 1929-76367 месяц, 1929-75957 pending): цены и подписи строк, дефолт
 * выбора, надпись «Текущий» и состояние футера. Компонент только рисует. */

/** Бейдж «−25%» сегмента «Год» (#623: считать из данных, не константа) —
 * максимальная по платным тарифам выгода года против помесячной оплаты,
 * в процентах; без скидки бейджа нет. Проценты — через канон scale. */
export function yearlyDiscountPercent(tariffs: Tariff[]): number | undefined {
  let best: number | undefined;
  for (const tariff of tariffs) {
    if (!isPaidTariff(tariff.name)) {
      continue;
    }
    const yearly = tariff.yearlyPriceKopecks;
    const monthlyYear = tariff.monthlyPriceKopecks * 12;
    if (monthlyYear <= 0 || yearly >= monthlyYear) {
      continue;
    }
    const percent = Math.round(ratioToPercent(1 - yearly / monthlyYear));
    if (best === undefined || percent > best) {
      best = percent;
    }
  }
  return best;
}

/** Ценовая строка строки тарифа: «4 400 ₽ в год», «490 ₽ в месяц»,
 * базовый — «Бесплатно» при любом периоде. */
export function tariffPriceLine(tariff: Tariff, period: PaymentPeriod): string {
  if (!isPaidTariff(tariff.name)) {
    return 'Бесплатно';
  }
  const price =
    period === 'year'
      ? tariff.yearlyPriceKopecks
      : tariff.monthlyPriceKopecks;
  return `${formatMoneyKopecks(price)} в ${period === 'year' ? 'год' : 'месяц'}`;
}

/** Подстрока годовой строки: «367 ₽ в месяц» — годовая цена в пересчёте
 * на месяц, округление до рубля (макет: 4 400/12 → 367, 8 900/12 → 742). */
export function yearlyPerMonthLine(tariff: Tariff): string {
  return `${formatMoneyKopecks(tariff.yearlyPriceKopecks / 12, { round: true })} в месяц`;
}

/** Дефолт выбора при открытии: платная подписка — свой тариф и период
 * (макет 1929-76367: текущий «Про»-месяц выбран, футер «Подключен»);
 * после регистрации — период «Год» и тариф «Про» (решение владельца). */
export function tariffChangeDefaults(subscription: Subscription): {
  tariff: TariffName;
  period: PaymentPeriod;
} {
  if (isPaidTariff(subscription.tariff.name)) {
    return {
      tariff: subscription.tariff.name,
      period: subscription.currentPeriod ?? 'year',
    };
  }
  return { tariff: 'pro', period: 'year' };
}

/** Надпись «Текущий» (аннотация макета 1929-76367): на платном тарифе —
 * только при совпадении периода, на базовом живёт при любом периоде
 * (правило владельца: базовый вне периодов). */
export function currentBadgeVisible(
  subscription: Subscription,
  tariffName: TariffName,
  period: PaymentPeriod,
): boolean {
  if (subscription.tariff.name !== tariffName) {
    return false;
  }
  return !isPaidTariff(tariffName) || subscription.currentPeriod === period;
}

/** Состояние футера (#623): «Подключен»-дизейбл — только активная
 * подписка на выбранном тарифе и периоде; выбор базового с платной
 * подписки ведёт во флоу отключения (#622); в grace оплата того же
 * тарифа — продление (#250) с кнопкой цены; в «Остановлен» на своём
 * тарифе и периоде при живом остатке — бесплатная «Возобновить» (#691:
 * платная реактивация оплаченного периода закрыта, остаток восстанавливает
 * бесплатное возобновление #617), на истёкшем периоде — платная
 * реактивация (#429) с кнопкой цены. */
export type TariffChangeFooter =
  | { kind: 'connect'; label: string }
  | { kind: 'connected' }
  | { kind: 'resume'; label: string }
  | { kind: 'disable' };

export function tariffChangeFooter(
  subscription: Subscription,
  selectedTariff: Tariff,
  selectedPeriod: PaymentPeriod,
  now: Date = new Date(),
): TariffChangeFooter {
  if (!isPaidTariff(selectedTariff.name)) {
    // Базовый вне периодов и всегда active на бэке (grace/отмена — только
    // у платных), поэтому статус здесь не проверяется.
    return subscription.tariff.name === 'basic' ? { kind: 'connected' } : { kind: 'disable' };
  }

  const isSame =
    subscription.tariff.name === selectedTariff.name &&
    subscription.currentPeriod === selectedPeriod;
  if (isSame && subscription.status === 'active') {
    return { kind: 'connected' };
  }
  if (
    isSame &&
    subscription.status === 'cancelled' &&
    subscription.validUntil !== undefined &&
    new Date(subscription.validUntil).getTime() > now.getTime()
  ) {
    return {
      kind: 'resume',
      label: `Возобновить ${getTariffLabel(selectedTariff.name)}`,
    };
  }

  return {
    kind: 'connect',
    label: `Подключить за ${tariffPriceLine(selectedTariff, selectedPeriod)}`,
  };
}

/** Плашка «Эта функция доступна на платных тарифах» (карта #997): видна,
 * когда пользователь пришёл на выбор тарифа с редиректа платного гейта —
 * query `gate` ставит proxy.ts. Значение само не показывается, важен факт
 * контекста; пустое значение контекстом не считается. */
export function paidGateNoticeVisible(
    gate: string | string[] | undefined,
): boolean {
    const value = Array.isArray(gate) ? gate[0] : gate;
    return typeof value === 'string' && value.length > 0;
}

/** Карточка возможностей тарифа (#623, макет 1919-74867): заголовок —
 * имя тарифа H1, ряды — статика макета; тексты совпадают с «О тарифе»
 * (#621) и уточняют базовый. */
export type TariffChangeFeatureRow = {
  readonly kind: 'objects' | 'sharing';
  readonly title: string;
  readonly description: string;
};

export type TariffChangeCard = {
  readonly name: TariffName;
  readonly title: string;
  readonly rows: ReadonlyArray<TariffChangeFeatureRow>;
};

const SHARING_ROW: TariffChangeFeatureRow = {
  kind: 'sharing',
  title: 'Совместный доступ',
  description:
    'Приглашайте в свои объекты близких и коллег, оплатить подписку нужно только вам',
};

export const TARIFF_CHANGE_CARDS: ReadonlyArray<TariffChangeCard> = [
  {
    name: 'basic',
    title: getTariffLabel('basic'),
    rows: [
      {
        kind: 'objects',
        title: '1 объект',
        description:
          'Идеально, чтобы попробовать сервис или управлять своей недвижимостью: квартирой, домом, гаражом или коммерческой площадью',
      },
    ],
  },
  {
    name: 'pro',
    title: getTariffLabel('pro'),
    rows: [
      {
        kind: 'objects',
        title: 'До 5 объектов',
        description: 'Для небольшого портфеля недвижимости',
      },
      SHARING_ROW,
    ],
  },
  {
    name: 'business',
    title: getTariffLabel('business'),
    rows: [
      {
        kind: 'objects',
        title: 'Без ограничений',
        description: 'Добавляйте любое количество объектов',
      },
      SHARING_ROW,
    ],
  },
];
