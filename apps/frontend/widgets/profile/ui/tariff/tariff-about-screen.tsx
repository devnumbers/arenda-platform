'use client';

import { useState, type JSX } from 'react';
import NextLink from 'next/link';
import { useRouter } from 'next/navigation';
import { useQueryClient } from '@tanstack/react-query';
import {
  Button,
  buttonVariants,
  Skeleton,
  StickyBottomBar,
} from '@/shared/ui/design';
import { billingKeys } from '@/shared/api/query-keys';
import { ROUTES } from '@/shared/config/routes';
import { notify } from '@/shared/lib/notifications';
import type { ApiError } from '@/shared/api/errors';
import {
  useChangeTariff,
  useResumeSubscription,
  useSubscription,
} from '@/features/billing';
import { getTariffLabel, isPaidTariff } from '@/entities/user';
import type { Subscription } from '@/entities/billing';
import {
  tariffAboutCard,
  tariffFeatureRows,
} from '@/widgets/profile/lib/tariff-about';
import { DisableGuardSheet } from './disable-guard-sheet';
import { GracePlaque } from './grace-plaque';
import { ResumeSuccessScreen } from './resume-success-screen';
import { TariffAboutCard, TariffFeaturesCard } from './tariff-about-cards';

/** Экран «О тарифе» (#621, макеты 1918-73255 активен, 2036-84420 grace,
 * 1933-77851 отключён, 1929-76198 гард pending, 1934-78524 успех
 * возобновления): карточка тарифа, «Возможности», «Отключить тариф»
 * (гард при живой pending-оплате), бесплатное «Возобновить» для
 * отключённого, футер «Выбрать другой тариф» (в grace — ещё и «Оплатить
 * тариф» ручной оплатой того же тарифа). Данные — кэш GET /subscription
 * (staleTime 30с), скелетон повторяет геометрию (DESIGN.md §7). */
export function TariffAboutScreen(): JSX.Element {
  const { data: subscription, isPending, isError, refetch } = useSubscription();

  if (isPending) {
    return <TariffAboutSkeleton />;
  }

  if (isError) {
    return (
      <div className="flex flex-col items-center gap-4 pt-6">
        <p className="text-center text-base leading-[18px] text-content-secondary">
          Не удалось загрузить данные тарифа
        </p>
        <Button variant="secondary" size="small" onClick={() => void refetch()}>
          Повторить
        </Button>
      </div>
    );
  }

  return <TariffAboutContent subscription={subscription} />;
}

function TariffAboutContent({
  subscription,
}: {
  readonly subscription: Subscription;
}): JSX.Element {
  const router = useRouter();
  const queryClient = useQueryClient();
  const resume = useResumeSubscription();
  const changeTariff = useChangeTariff();
  const [guardOpen, setGuardOpen] = useState(false);
  const [resumed, setResumed] = useState(false);

  const cancelled = subscription.status === 'cancelled';
  const grace = subscription.status === 'grace';
  const paid = isPaidTariff(subscription.tariff.name);
  const pending = subscription.pendingPayment;

  if (resumed) {
    return <ResumeSuccessScreen tariffName={subscription.tariff.name} />;
  }

  const handleDisable = (): void => {
    if (pending !== undefined) {
      setGuardOpen(true);
      return;
    }
    router.push(ROUTES.profileTariffDisable);
  };

  const handleResume = (): void => {
    resume
      .mutateAsync()
      .then(() => setResumed(true))
      .catch((error: ApiError) =>
        notify.scenarios.tariff.resumeError({ description: error.detail }),
      );
  };

  const handleGracePayment = (): void => {
    changeTariff
      .mutateAsync({
        tariffName: subscription.tariff.name,
        period: subscription.currentPeriod ?? 'month',
      })
      .then((data) => {
        if (data.confirmUrl !== undefined) {
          // Ручная оплата того же тарифа в grace: платёж создан (pending),
          // подписку применит вебхук после подтверждения — редирект в банк
          // без success-тоста (как в смене тарифа, #250).
          window.location.href = data.confirmUrl;
          return;
        }
        notify.scenarios.tariff.gracePaymentError();
      })
      .catch((error: ApiError) =>
        notify.scenarios.tariff.gracePaymentError({ description: error.detail }),
      );
  };

  return (
    <>
      {/* В grace футер несёт две кнопки (168px) — канонного клиренса
          PageContent pb-136 не хватает, последняя кнопка контента остаётся
          под панелью; добор 48px возвращает зазор (приёмка #621). */}
      <div className={grace ? 'flex flex-col gap-4 pb-12' : 'flex flex-col gap-4'}>
        {grace && <GracePlaque validUntil={subscription.validUntil} />}

        <TariffAboutCard view={tariffAboutCard(subscription)} />

        {!cancelled && (
          <TariffFeaturesCard rows={tariffFeatureRows(subscription.tariff)} />
        )}

        {cancelled ? (
          <Button
            variant="secondary"
            loading={resume.isPending}
            onClick={handleResume}
          >
            Возобновить {getTariffLabel(subscription.tariff.name)}
          </Button>
        ) : (
          paid && (
            <Button variant="secondary" onClick={handleDisable}>
              Отключить тариф
            </Button>
          )
        )}
      </div>

      <StickyBottomBar>
        {grace ? (
          <div className="flex flex-col gap-2">
            <Button loading={changeTariff.isPending} onClick={handleGracePayment}>
              Оплатить тариф
            </Button>
            <NextLink
              href={ROUTES.profileTariffChange}
              className={buttonVariants({ variant: 'secondary' })}
            >
              Выбрать другой тариф
            </NextLink>
          </div>
        ) : (
          <NextLink
            href={ROUTES.profileTariffChange}
            className={buttonVariants({ variant: 'primary' })}
          >
            Выбрать другой тариф
          </NextLink>
        )}
      </StickyBottomBar>

      {pending !== undefined && (
        <DisableGuardSheet
          pending={pending}
          open={guardOpen}
          onOpenChange={setGuardOpen}
          onExpired={() => {
            // Закрываем гард вместе с истечением: оставили бы sheet с
            // мёртвым «Вернуться к оплате», а guardOpen=true при появлении
            // следующей pending открыл бы модалку сам собой (ревью #621).
            setGuardOpen(false);
            void queryClient.invalidateQueries({ queryKey: billingKeys.subscription });
          }}
        />
      )}
    </>
  );
}

function TariffAboutSkeleton(): JSX.Element {
  return (
    <div className="flex flex-col gap-4" role="status" aria-label="Загрузка данных тарифа">
      <div className="flex flex-col gap-6 rounded-[32px] bg-surface-muted p-8">
        <Skeleton className="h-8 w-32 bg-surface-muted-hover" />
        {Array.from({ length: 3 }, (_, index) => (
          <div key={index} className="flex flex-col gap-2">
            <Skeleton className="h-[18px] w-28 bg-surface-muted-hover" />
            <Skeleton
              className="h-6 bg-surface-muted-hover"
              style={{ width: `${72 - index * 12}%` }}
            />
          </div>
        ))}
      </div>
      <div className="flex flex-col gap-6 rounded-[32px] bg-surface-muted p-8">
        <Skeleton className="h-8 w-44 bg-surface-muted-hover" />
        {Array.from({ length: 2 }, (_, index) => (
          <div key={index} className="flex gap-4">
            <Skeleton className="h-12 w-12 shrink-0 bg-surface-muted-hover" />
            <div className="flex flex-col gap-2">
              <Skeleton className="h-[18px] w-40 bg-surface-muted-hover" />
              <Skeleton className="h-8 w-56 bg-surface-muted-hover" />
            </div>
          </div>
        ))}
      </div>
      <Skeleton className="h-14 rounded-button" />
    </div>
  );
}
