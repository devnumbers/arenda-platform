'use client';

import { Fragment, useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import Image from 'next/image';
import { useQueryClient } from '@tanstack/react-query';
import {
  Button,
  RadioGroup,
  RadioGroupItem,
  Skeleton,
  StickyBottomBar,
  buttonVariants,
} from '@/shared/ui/design';
import { notify } from '@/shared/lib/notifications';
import { billingKeys } from '@/shared/api/query-keys';
import {
  useChangeTariff,
  useSubscription,
  useTariffs,
} from '@/features/billing';
import { getTariffLabel, isPaidTariff, type TariffName } from '@/entities/user';
import type {
  PaymentPeriod,
  PendingPayment,
  Subscription,
  Tariff,
} from '@/entities/billing';
import { ROUTES } from '@/shared/config/routes';
import type { ApiError } from '@/shared/api/errors';
import {
  currentBadgeVisible,
  tariffChangeDefaults,
  tariffChangeFooter,
  tariffPriceLine,
  TARIFF_CHANGE_CARDS,
  yearlyDiscountPercent,
  yearlyPerMonthLine,
} from '@/widgets/profile/lib/tariff-change';
import { formatPaymentCountdown } from '@/widgets/profile/lib/tariff-overview';
import { TariffFaq } from './tariff-faq';
import { FEATURE_IMAGES } from './tariff-about-cards';
import { usePaymentTimer } from './use-payment-timer';

/** Экран «Выбрать тариф» (#623, макеты 1919-74867 год, 1929-76367 месяц,
 * 1929-75957 pending) — редизайн TariffChangeForm на канон: сегмент
 * «Год/Месяц» с бейджем скидки из данных, радио-строки всех трёх тарифов
 * (базовый — «Бесплатно»), карточки возможностей, общий с главным экраном
 * FAQ и липкий футер трёх состояний: «Подключить за X ₽» (апгрейд →
 * confirmUrl банка; grace-продление #250 и реактивация #429 — тоже
 * платёжный путь), disabled «Подключен» (активная подписка на выбранном
 * тарифе и периоде), выбор базового — во флоу отключения (#622). Живая
 * pending-оплата заменяет футер блоком «Вернуться к оплате» с таймером:
 * плашка не меняется при переключениях (аннотация макета), по истечении
 * срока refetch разблокирует выбор. Дефолт выбора: своя подписка — свой
 * тариф и период, после регистрации — «Год» + «Про» (решение владельца). */
export function TariffChangeScreen(): JSX.Element {
  const {
    data: tariffs,
    isPending: isTariffsPending,
    isError: isTariffsError,
    refetch: refetchTariffs,
  } = useTariffs();
  const {
    data: subscription,
    isPending: isSubscriptionPending,
    isError: isSubscriptionError,
    refetch: refetchSubscription,
  } = useSubscription();

  if (isTariffsError || isSubscriptionError) {
    return (
      <div className="flex flex-col items-center gap-4 pt-6">
        <p className="text-center text-base leading-[18px] text-content-secondary">
          Не удалось загрузить данные тарифов
        </p>
        <Button
          variant="secondary"
          size="small"
          onClick={() => {
            if (isTariffsError) {
              void refetchTariffs();
            }
            if (isSubscriptionError) {
              void refetchSubscription();
            }
          }}
        >
          Повторить
        </Button>
      </div>
    );
  }

  if (isTariffsPending || isSubscriptionPending) {
    return <TariffChangeSkeleton />;
  }

  return <TariffChangeContent tariffs={tariffs} subscription={subscription} />;
}

type TariffChangeContentProps = {
  readonly tariffs: Tariff[];
  readonly subscription: Subscription;
};

const PERIOD_OPTIONS: ReadonlyArray<{
  readonly value: PaymentPeriod;
  readonly label: string;
}> = [
  { value: 'year', label: 'Год' },
  { value: 'month', label: 'Месяц' },
];

function TariffChangeContent({
  tariffs,
  subscription,
}: TariffChangeContentProps): JSX.Element {
  const pendingPayment = subscription.pendingPayment;
  const router = useRouter();
  const queryClient = useQueryClient();
  const changeTariff = useChangeTariff();

  const defaults = tariffChangeDefaults(subscription);
  const [period, setPeriod] = useState<PaymentPeriod>(defaults.period);
  const [selectedName, setSelectedName] = useState<TariffName>(defaults.tariff);

  const selectedTariff: Tariff | undefined =
    tariffs.find((tariff) => tariff.name === selectedName) ?? tariffs[0];
  const footer =
    pendingPayment === undefined && selectedTariff !== undefined
      ? tariffChangeFooter(subscription, selectedTariff, period)
      : undefined;

  const handleConnect = () => {
    if (selectedTariff === undefined) {
      return;
    }
    const loadingToastId = notify.scenarios.tariff.changeLoading();

    changeTariff
      .mutateAsync({ tariffName: selectedTariff.name, period })
      .then((data) => {
        notify.close(loadingToastId);

        if (data.confirmUrl) {
          // Оплата создана (pending) — редирект в банк без success-тоста;
          // подписку применит вебхук после подтверждения.
          window.location.href = data.confirmUrl;
          return;
        }

        // Даунгрейд без оплаты применён отложенно — успех с датой.
        notify.scenarios.tariff.changed();
        router.replace(ROUTES.profileTariffChangeSuccess);
      })
      .catch((error: ApiError) => {
        notify.close(loadingToastId);
        notify.scenarios.tariff.changeError({ description: error.detail });
      });
  };

  const handlePendingExpired = () => {
    void queryClient.invalidateQueries({ queryKey: billingKeys.subscription });
  };

  const discountPercent = yearlyDiscountPercent(tariffs);

  return (
    <>
      <div className="flex flex-col gap-4 pb-4">
        <PeriodSegment
          value={period}
          onChange={setPeriod}
          discountPercent={discountPercent}
        />

        <RadioGroup
          value={selectedName}
          onValueChange={(value) => setSelectedName(value as TariffName)}
          className="rounded-3xl bg-surface-muted py-2"
          aria-label="Тариф"
        >
          {tariffs.map((tariff, index) => (
            <Fragment key={tariff.name}>
              {index > 0 && (
                <div aria-hidden className="pl-16">
                  <div className="h-px bg-[var(--gray-second,#d3d7d9)]" />
                </div>
              )}
              <TariffRow
                tariff={tariff}
                period={period}
                showCurrentBadge={currentBadgeVisible(subscription, tariff.name, period)}
              />
            </Fragment>
          ))}
        </RadioGroup>

        {TARIFF_CHANGE_CARDS.map((card) => (
          <TariffFeatureCard key={card.name} card={card} />
        ))}

        <TariffFaq />
      </div>

      <StickyBottomBar>
        {pendingPayment !== undefined ? (
          <PendingFooter pending={pendingPayment} onExpired={handlePendingExpired} />
        ) : (
          footer !== undefined && (
            <ChangeFooter
              footer={footer}
              onConnect={handleConnect}
              onDisable={() => router.push(ROUTES.profileTariffDisable)}
              loading={changeTariff.isPending}
            />
          )
        )}
      </StickyBottomBar>
    </>
  );
}

type PeriodSegmentProps = {
  readonly value: PaymentPeriod;
  readonly onChange: (period: PaymentPeriod) => void;
  readonly discountPercent: number | undefined;
};

/** Сегмент «Год/Месяц» (макет: серый контейнер radius 16, паддинг 2,
 * опции 40 высотой radius 14; выбранная — белая с тенью). Бейдж «−N%» —
 * на «Годе» при любом выборе, считается из цен (#623). */
function PeriodSegment({
  value,
  onChange,
  discountPercent,
}: PeriodSegmentProps): JSX.Element {
  return (
    <div
      role="radiogroup"
      aria-label="Период оплаты"
      className="flex gap-0.5 rounded-2xl bg-surface-muted p-0.5"
    >
      {PERIOD_OPTIONS.map((option) => {
        const isSelected = option.value === value;
        return (
          <button
            key={option.value}
            type="button"
            role="radio"
            aria-checked={isSelected}
            onClick={() => onChange(option.value)}
            className={`flex min-h-10 flex-1 cursor-pointer items-center justify-center gap-2 rounded-[14px] px-4 text-sm font-medium outline-none transition-colors focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface ${
              isSelected
                ? 'bg-surface text-content shadow-[0_2px_4px_rgba(0,0,0,0.16)]'
                : 'text-content-secondary'
            }`}
          >
            {option.label}
            {option.value === 'year' && discountPercent !== undefined && (
              <span className="rounded-md bg-primary py-0.5 pl-0.5 pr-1 text-sm font-medium leading-4 text-white">
                −{discountPercent}%
              </span>
            )}
          </button>
        );
      })}
    </div>
  );
}

type TariffRowProps = {
  readonly tariff: Tariff;
  readonly period: PaymentPeriod;
  readonly showCurrentBadge: boolean;
};

/** Строка тарифа (макет: ряд 70 высотой, радио 24, делитель с отступом
 * 64; «Текущий» — серая надпись 14/16 под именем; у годовой цены —
 * пересчёт «X ₽ в месяц»). Вся строка — кликабельный label (#622). */
function TariffRow({
  tariff,
  period,
  showCurrentBadge,
}: TariffRowProps): JSX.Element {
  const paid = isPaidTariff(tariff.name);
  return (
    <label className="flex min-h-[70px] cursor-pointer items-center gap-4 px-6 py-4">
      <RadioGroupItem value={tariff.name} className="h-6 w-6" />
      <span className="flex min-w-0 flex-1 flex-col gap-1">
        <span className="text-base font-medium leading-[18px] text-content">
          {getTariffLabel(tariff.name)}
        </span>
        {showCurrentBadge && (
          <span className="text-sm leading-4 text-content-secondary">Текущий</span>
        )}
      </span>
      <span className="flex flex-col items-end gap-1 text-right">
        <span className="whitespace-nowrap text-base font-medium leading-[18px] text-content">
          {tariffPriceLine(tariff, period)}
        </span>
        {paid && period === 'year' && (
          <span className="whitespace-nowrap text-sm leading-4 text-content-secondary">
            {yearlyPerMonthLine(tariff)}
          </span>
        )}
      </span>
    </label>
  );
}

/** Карточка возможностей тарифа (макет 1919-74867: серая карточка 32/32,
 * радиус 32, заголовок H1 28/32, ряды — картинка 48 и колонка 18/600 +
 * 14/16; тексты — статика макета). */
function TariffFeatureCard({
  card,
}: {
  readonly card: (typeof TARIFF_CHANGE_CARDS)[number];
}): JSX.Element {
  return (
    <section className="flex flex-col gap-6 rounded-[32px] bg-surface-muted p-8">
      <h2 className="m-0 text-[28px] font-semibold leading-8 text-content">{card.title}</h2>
      {card.rows.map((row) => (
        <div key={row.kind} className="flex gap-4">
          <Image
            src={FEATURE_IMAGES[row.kind]}
            alt=""
            width={48}
            height={48}
            className="h-12 w-12 shrink-0"
          />
          <div className="flex min-w-0 flex-1 flex-col gap-2">
            <p className="m-0 text-lg font-semibold leading-[18px] text-content">{row.title}</p>
            <p className="m-0 text-sm leading-4 text-content-secondary">{row.description}</p>
          </div>
        </div>
      ))}
    </section>
  );
}

function ChangeFooter({
  footer,
  onConnect,
  onDisable,
  loading,
}: {
  readonly footer: ReturnType<typeof tariffChangeFooter>;
  readonly onConnect: () => void;
  readonly onDisable: () => void;
  readonly loading: boolean;
}): JSX.Element {
  if (footer.kind === 'connected') {
    return <Button disabled>Подключен</Button>;
  }
  if (footer.kind === 'disable') {
    // Выбор Базового с платной подписки — во флоу отключения (#622).
    return <Button onClick={onDisable}>Отключить тариф</Button>;
  }
  return (
    <Button loading={loading} onClick={onConnect}>
      {footer.label}
    </Button>
  );
}

/** Блок живой pending-оплаты (макет 1929-75957): кнопка «Вернуться к
 * оплате» на confirmUrl банка, отсчёт «Время на оплату MM:SS» и серый
 * дисклеймер; кнопки подключения нет, пока платёж жив. Ссылка, а не
 * Button — confirmUrl ведёт на форму банка в новой вкладке (обёртка
 * text-white против легаси-сброса цвета ссылок, #620). */
function PendingFooter({
  pending,
  onExpired,
}: {
  readonly pending: PendingPayment;
  readonly onExpired: () => void;
}): JSX.Element {
  const now = usePaymentTimer(pending.expiresAt, true, onExpired);

  return (
    <div className="flex flex-col items-center gap-4">
      <div className="w-full text-white">
        <a
          href={pending.confirmUrl}
          target="_blank"
          rel="noopener noreferrer"
          className={buttonVariants({ className: 'w-full' })}
        >
          Вернуться к оплате
        </a>
      </div>
      <p className="m-0 flex items-center gap-1 text-sm font-medium leading-4 text-primary">
        Время на оплату
        <span className="font-mono">{formatPaymentCountdown(pending.expiresAt, now)}</span>
      </p>
      <p className="m-0 text-center text-sm leading-4 text-content-tertiary">
        Чтобы выбрать другой тариф, дождитесь завершения времени на оплату
      </p>
    </div>
  );
}

/** Скелетон геометрии экрана (DESIGN.md §7, гейт Loading stability):
 * сегмент, радио-список, три карточки, FAQ. */
function TariffChangeSkeleton(): JSX.Element {
  return (
    <div className="flex flex-col gap-4" role="status" aria-label="Загрузка тарифов">
      <Skeleton className="h-11 rounded-2xl" />
      <div className="flex flex-col rounded-3xl bg-surface-muted py-2">
        {[0, 1, 2].map((index) => (
          <Fragment key={index}>
            {index > 0 && (
              <div aria-hidden className="pl-16">
                <div className="h-px bg-[var(--gray-second,#d3d7d9)]" />
              </div>
            )}
            <div className="flex min-h-[70px] items-center gap-4 px-6 py-4">
              <Skeleton className="h-6 w-6 rounded-full bg-surface-muted-hover" />
              <Skeleton className="h-[18px] w-24 bg-surface-muted-hover" />
              <Skeleton className="ml-auto h-[18px] w-28 bg-surface-muted-hover" />
            </div>
          </Fragment>
        ))}
      </div>
      <Skeleton className="h-[226px] rounded-[32px]" />
      <Skeleton className="h-[276px] rounded-[32px]" />
      <Skeleton className="h-[276px] rounded-[32px]" />
      <div className="flex flex-col rounded-3xl bg-surface-muted pb-3">
        <div className="px-6 pb-2 pt-6">
          <Skeleton className="h-6 w-44 bg-surface-muted-hover" />
        </div>
        {Array.from({ length: 7 }, (_, index) => (
          <div key={index} className="flex items-center justify-between gap-4 px-6 py-3">
            <Skeleton
              className="h-[18px] bg-surface-muted-hover"
              style={{ width: `${64 - (index % 3) * 12}%` }}
            />
            <Skeleton className="h-6 w-6 shrink-0 bg-surface-muted-hover" />
          </div>
        ))}
      </div>
    </div>
  );
}
