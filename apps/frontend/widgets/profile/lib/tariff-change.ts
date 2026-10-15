import {
  type PaymentPeriod,
  type Subscription,
  type Tariff,
} from '@/entities/billing';
import { getTariffLabel, isPaidTariff, type TariffName } from '@/entities/user';
import { dateToIsoLocal } from '@/shared/lib/calendar';
import { formatMoneyKopecks, ratioToPercent } from '@/shared/lib/format-money';
import type { ApiError } from '@/shared/api/errors';
import { dayMonth } from './tariff-overview';

/** Вид-модель экрана «Выбрать тариф» (#623, макеты 1919-74867 год,
 * 1929-76367 месяц, 1929-75957 pending): цены и подписи строк, дефолт
 * выбора, надпись «Текущий» и состояние футера. Компонент только рисует. */

/** Безлимитный лимит активных объектов тарифа (business) — зеркало
 * UnlimitedPropertyLimit домена бэка. */
const UNLIMITED_PROPERTY_LIMIT = -1;

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

const URL_TARIFFS: ReadonlySet<string> = new Set(['pro', 'business']);
const URL_PERIODS: ReadonlySet<string> = new Set(['year', 'month']);

/** Предвыбор из query глубокой ссылки лендинга: карточка «Подключить»
 * ведёт на экран с ?tariff=pro|business&period=year|month (правка
 * владельца 14.10: клик должен открывать выбор именно этого тарифа).
 * Читается только валидное: выбор базового извне не проводим — это флоу
 * отключения (#622), период без тарифа не применяется (ссылка лендинга
 * всегда шлёт пару), мусор и чужие значения игнорируются. */
export function tariffChangeUrlOverrides(searchParams: {
  readonly get: (name: string) => string | null;
} | null): {
  tariff?: TariffName;
  period?: PaymentPeriod;
} {
  const tariff = searchParams?.get('tariff');
  if (tariff == null || !URL_TARIFFS.has(tariff)) {
    return {};
  }
  const period = searchParams?.get('period');
  return {
    tariff: tariff as TariffName,
    ...(period != null && URL_PERIODS.has(period)
      ? { period: period as PaymentPeriod }
      : {}),
  };
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
 * реактивация (#429) с кнопкой цены; более дешёвый тариф на «Остановлен»
 * и в живом grace — подтверждение отложенного даунгрейда с датой
 * применения (#1452, ADR 0066), после планирования — существующий экран
 * «Тариф изменен» с датой вступления. */
export type TariffChangeFooter =
  | { kind: 'connect'; label: string }
  | { kind: 'connected' }
  | { kind: 'resume'; label: string }
  | { kind: 'downgrade'; title: string; description: string; confirmLabel: string }
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
  if (
    (subscription.status === 'cancelled' || subscription.status === 'grace') &&
    classifyTariffChange(subscription.tariff, selectedTariff) === 'downgrade' &&
    subscription.validUntil !== undefined &&
    new Date(subscription.validUntil).getTime() > now.getTime()
  ) {
    return {
      kind: 'downgrade',
      title: `Сменить тариф на ${getTariffLabel(selectedTariff.name)}?`,
      description: downgradeConfirmDescription(
        subscription.status,
        subscription.validUntil,
        selectedTariff,
        selectedPeriod,
        now,
      ),
      confirmLabel: 'Сменить тариф',
    };
  }

  return {
    kind: 'connect',
    label: `Подключить за ${tariffPriceLine(selectedTariff, selectedPeriod)}`,
  };
}

/** Направление смены тарифа — зеркало ClassifyTariffChange бэка
 * (domain/tariff.go): лимит объектов, при равенстве месячная цена, затем
 * годовая; полное равенство — Same (вне скоупа #1450, при текущих ценах
 * недостижимо). */
export type TariffChangeDirection = 'upgrade' | 'downgrade' | 'same';

export function classifyTariffChange(
  current: Tariff,
  next: Tariff,
): TariffChangeDirection {
  const nextBetter = isTariffBetter(next, current);
  const currentBetter = isTariffBetter(current, next);
  if (nextBetter && !currentBetter) {
    return 'upgrade';
  }
  if (currentBetter && !nextBetter) {
    return 'downgrade';
  }
  return 'same';
}

/** Строгая «лучшесть» тарифа — зеркало isTariffBetter бэка. */
function isTariffBetter(a: Tariff, b: Tariff): boolean {
  const aUnlimited = a.activePropertyLimit === UNLIMITED_PROPERTY_LIMIT;
  const bUnlimited = b.activePropertyLimit === UNLIMITED_PROPERTY_LIMIT;
  if (aUnlimited !== bUnlimited) {
    return aUnlimited;
  }
  if (a.activePropertyLimit !== b.activePropertyLimit) {
    return a.activePropertyLimit > b.activePropertyLimit;
  }
  if (a.monthlyPriceKopecks !== b.monthlyPriceKopecks) {
    return a.monthlyPriceKopecks > b.monthlyPriceKopecks;
  }
  if (a.yearlyPriceKopecks !== b.yearlyPriceKopecks) {
    return a.yearlyPriceKopecks > b.yearlyPriceKopecks;
  }
  return false;
}

/** Описание подтверждения даунгрейда (#1452, ADR 0066): дата применения —
 * конец оплаченного остатка («Остановлен») или дедлайн grace-окна, бэк
 * запишет тот же validUntil в pending_change_at; списание целевого тарифа
 * на границе раскрываем сразу — планирование ничего не списывает, но
 * назначение согласовывает платёж целевого на границе. Язык «Изменения
 * вступят в силу» — как на существующем экране успеха. */
function downgradeConfirmDescription(
  status: Subscription['status'],
  validUntil: string,
  target: Tariff,
  period: PaymentPeriod,
  now: Date,
): string {
  const date = dayMonth(validUntil, dateToIsoLocal(now));
  const appliesAt =
    status === 'cancelled'
      ? `Изменения вступят в силу ${date} — в конце оплаченного периода.`
      : `Изменения вступят в силу ${date}.`;
  return (
    `${appliesAt} Тогда же спишем ${tariffPriceLine(target, period)}`
    + ` за тариф ${getTariffLabel(target.name)}. Сейчас ничего не списываем.`
  );
}

const NO_MACHINE_CODE: ReadonlySet<string> = new Set([
  // Незакодированные 409: у бека это честные конфликты состояния
  // (writeDetailProblem → table detail вроде «Некорректное состояние
  // подписки») без machine-readable code — клиент видит about:blank;
  // unknown — дефолт клиента для ответа без problem+json.
  'about:blank',
  'unknown',
]);

/** Человеческое описание тоста об ошибке действия (#1452, край #1450 п.4):
 * сырой detail честного 409 в тост не показывается — у действия свой
 * человеческий конфликтный текст; закодированные 409 бека
 * (ProblemWithCode) несут уже человеческий текст («Период уже оплачен…»,
 * «Есть неоплаченный платёж…») — проходят как есть; не-409 сохраняют
 * detail бека. */
export function tariffActionErrorDescription(
  error: ApiError,
  conflictText: string,
): string {
  if (error.status === 409 && NO_MACHINE_CODE.has(error.code)) {
    return conflictText;
  }
  return error.detail;
}

/** Карточка возможностей тарифа (#623, макеты 1919-74867 год, обновлены
 * 3322-158509/158610: у платных — сокращённый «Совместный доступ» и ряд
 * «История действий»): заголовок — имя тарифа H1, ряды — статика макета;
 * тексты совпадают с «О тарифе» (#621) и уточняют базовый. */
export type TariffChangeFeatureRow = {
  readonly kind: 'objects' | 'sharing' | 'history';
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
  description: 'Приглашайте в свои объекты близких и коллег',
};

const HISTORY_ROW: TariffChangeFeatureRow = {
  kind: 'history',
  title: 'История действий',
  description: 'Отслеживайте любые изменения в объектах',
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
      HISTORY_ROW,
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
      HISTORY_ROW,
    ],
  },
];
