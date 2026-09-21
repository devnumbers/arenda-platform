'use client';

import type { JSX } from 'react';
import NextLink from 'next/link';
import { useQueryClient } from '@tanstack/react-query';
import { CreditCard, Support, TimeHistory } from '@/shared/assets/icons';
import { Button, buttonVariants, Skeleton } from '@/shared/ui/design';
import { billingKeys } from '@/shared/api/query-keys';
import { ROUTES } from '@/shared/config/routes';
import { useSubscription } from '@/features/billing';
import { NO_SUBSCRIPTION } from '@/widgets/profile/lib/no-subscription';
import { tariffHero } from '@/widgets/profile/lib/tariff-overview';
import { GracePlaque } from './grace-plaque';
import { PendingPaymentPlaque } from './pending-payment-plaque';
import { TariffFaq } from './tariff-faq';
import { TariffHero } from './tariff-hero';

/** Главный экран «Тариф» (#620): hero-карточка вариантов GET /subscription,
 * синяя плашка живой pending-оплаты (#616), красная grace-плашка (#615),
 * плитки «Оплата»/«Операции», статический FAQ и «Поддержка». Данные —
 * клиентский react-query с кэшем (staleTime 30с), поэтому плашки переживают
 * уход/возврат на экран без миганий (правило владельца). Скелетон повторяет
 * геометрию загруженного экрана (DESIGN.md §7, гейт Loading stability). */
export function TariffScreen(): JSX.Element {
  const { data, isPending, isError, refetch } = useSubscription();
  const subscription = data ?? NO_SUBSCRIPTION;
  const queryClient = useQueryClient();

  if (isPending) {
    return <TariffScreenSkeleton />;
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

  return (
    <div className="flex flex-col gap-4">
      <TariffHero hero={tariffHero(subscription)} />

      {subscription.pendingPayment !== undefined && (
        <PendingPaymentPlaque
          pending={subscription.pendingPayment}
          onExpired={() => {
            void queryClient.invalidateQueries({ queryKey: billingKeys.subscription });
          }}
        />
      )}

      {subscription.status === 'grace' && (
        <GracePlaque validUntil={subscription.validUntil} />
      )}

      <nav aria-label="Управление подпиской" className="flex items-stretch gap-3">
        <Tile href={ROUTES.profilePaymentMethods} icon={<CreditCard />} label="Оплата" />
        <Tile href={ROUTES.profilePayments} icon={<TimeHistory />} label="Операции" />
      </nav>

      <TariffFaq />

      <NextLink href={ROUTES.support} className={buttonVariants({ variant: 'secondary' })}>
        <Support className="h-6 w-6 shrink-0" aria-hidden />
        Поддержка
      </NextLink>
    </div>
  );
}

function Tile({
  href,
  icon,
  label,
}: {
  readonly href: string;
  readonly icon: JSX.Element;
  readonly label: string;
}): JSX.Element {
  return (
    <NextLink
      href={href}
      className="flex flex-1 basis-0 flex-col gap-4 rounded-card bg-surface-muted p-6 text-content outline-none transition-colors hover:bg-surface-muted-hover active:bg-surface-muted-active focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface"
    >
      {icon}
      <span className="text-base font-medium leading-[18px]">{label}</span>
    </NextLink>
  );
}

function TariffScreenSkeleton(): JSX.Element {
  return (
    <div className="flex flex-col gap-4" role="status" aria-label="Загрузка тарифа">
      <Skeleton className="h-[212px] rounded-[32px]" />
      <div className="flex items-stretch gap-3">
        <Skeleton className="h-[106px] flex-1 rounded-card" />
        <Skeleton className="h-[106px] flex-1 rounded-card" />
      </div>
      <div className="flex flex-col rounded-card bg-surface-muted pb-3">
        <div className="px-6 pt-6 pb-2">
          <Skeleton className="h-6 w-44 bg-surface-muted-hover" />
        </div>
        {Array.from({ length: 7 }, (_, index) => (
          <div key={index} className="flex items-center justify-between gap-4 px-6 py-3">
            <Skeleton className="h-[18px] bg-surface-muted-hover" style={{ width: `${64 - (index % 3) * 12}%` }} />
            <Skeleton className="h-6 w-6 shrink-0 bg-surface-muted-hover" />
          </div>
        ))}
      </div>
      <Skeleton className="h-14 rounded-button" />
    </div>
  );
}
