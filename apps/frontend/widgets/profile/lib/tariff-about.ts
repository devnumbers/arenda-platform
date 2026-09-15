import { cardNumberTail, type Subscription, type Tariff } from '@/entities/billing';
import { getTariffLabel, isPaidTariff } from '@/entities/user';
import { pluralize } from '@/shared/lib/pluralize';
import type { TariffName } from '@/entities/user';
import { dayMonth, priceLine } from './tariff-overview';

/** Ряд карточки тарифа: серый лейбл и значение H3 (макеты #621). */
export type TariffAboutRow = {
  label: string;
  value: string;
};

/** Карточка тарифа экрана «О тарифе» (#621, макеты 1918-73255 активен,
 * 2036-84420 grace, 1933-77851 отключён): состояние выбирает заголовок и
 * набор рядов. Опечатка макета «Следующие списание» исправлена
 * («Следующее списание»). */
export type TariffAboutCardView = {
  title: string;
  rows: TariffAboutRow[];
};

function methodRow(
  subscription: Subscription,
): TariffAboutRow | undefined {
  const mask = subscription.activePaymentMethod?.displayMask;
  return mask === undefined ? undefined : { label: 'Способ оплаты', value: cardNumberTail(mask) };
}

function withMethod(
  rows: TariffAboutRow[],
  subscription: Subscription,
): TariffAboutRow[] {
  const method = methodRow(subscription);
  return method === undefined ? rows : [...rows, method];
}

export function tariffAboutCard(subscription: Subscription): TariffAboutCardView {
  const title = getTariffLabel(subscription.tariff.name);

  if (subscription.status === 'cancelled') {
    return {
      title: `${title} отключен`,
      rows: subscription.validUntil !== undefined
        ? [
            { label: 'Действует до', value: dayMonth(subscription.validUntil) },
          ]
        : [],
    };
  }

  if (!isPaidTariff(subscription.tariff.name)) {
    return {
      title,
      rows: [{ label: 'Стоимость', value: 'Бесплатно' }],
    };
  }

  const price = { label: 'Стоимость', value: priceLine(subscription) };

  if (subscription.status === 'grace') {
    return {
      title,
      rows: withMethod(
        subscription.validUntil !== undefined
          ? [price, { label: 'Оплатите тариф', value: `До ${dayMonth(subscription.validUntil)}` }]
          : [price],
        subscription,
      ),
    };
  }

  return {
    title,
    rows: withMethod(
      subscription.validUntil !== undefined
        ? [price, { label: 'Следующее списание', value: dayMonth(subscription.validUntil) }]
        : [price],
      subscription,
    ),
  };
}

/** Ряд карточки «Возможности» (#621): лимит объектов из API + статические
 * описания; «Совместный доступ» — только у платных тарифов. */
export type TariffFeatureRow = {
  kind: 'objects' | 'sharing';
  title: string;
  description: string;
};

const OBJECTS_LIMIT_DESCRIPTION = 'Для небольшого портфеля недвижимости';
const SHARING_DESCRIPTION =
  'Приглашайте в свои объекты близких и коллег, оплатить подписку нужно только вам';

export function tariffFeatureRows(tariff: Tariff): TariffFeatureRow[] {
  const limitTitle = tariff.activePropertyLimit < 0
    ? 'Неограниченно'
    : `До ${tariff.activePropertyLimit} ${pluralize(tariff.activePropertyLimit, 'объекта', 'объектов', 'объектов')}`;

  const rows: TariffFeatureRow[] = [
    { kind: 'objects', title: limitTitle, description: OBJECTS_LIMIT_DESCRIPTION },
  ];

  if (isPaidTariff(tariff.name)) {
    rows.push({ kind: 'sharing', title: 'Совместный доступ', description: SHARING_DESCRIPTION });
  }

  return rows;
}

/** Заголовок экрана успеха возобновления (#621, макет 1934-78524). */
export function resumeSuccessTitle(tariffName: TariffName): string {
  return `Тариф ${getTariffLabel(tariffName)} возобновлен`;
}
