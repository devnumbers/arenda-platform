'use client';

import { useEffect, type JSX } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { useQueryClient } from '@tanstack/react-query';
import { Button, StickyBottomBar } from '@/shared/ui/design';
import { CheckNoneLine, StatusIconDanger, Sync } from '@/shared/assets/icons';
import {
  useSubscription,
  useSubscriptionPayment,
} from '@/features/billing';
import { billingKeys } from '@/shared/api/query-keys';
import { formatDate } from '@/shared/lib/format-date';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { isPaymentFormExpired } from '../lib/payment-history-model';

type TariffChangeSuccessState = {
  readonly icon: JSX.Element;
  readonly title: string;
  readonly descriptions: ReadonlyArray<string>;
  readonly action: JSX.Element;
};

/** Успех смены тарифа (#623, канон полноэкранных успехов #621/#622):
 * шапки нет — в центре иконка 64, заголовок H3 и серое описание, внизу
 * StickyBottomBar с одним действием. Три варианта поверх одного каркаса:
 * платёж в обработке (polling статуса по paymentId из редиректа банка),
 * неуспех («Попробовать снова» на экран выбора) и успех («Хорошо» на
 * главный «Тариф»; отложенный даунгрейд называет дату вступления). */
export function TariffChangeSuccess(): JSX.Element {
  const router = useRouter();
  const searchParams = useSearchParams();
  const paymentId = searchParams.get('paymentId');
  const queryClient = useQueryClient();
  const { data: subscription, isPending } = useSubscription();
  const { data: payment, isPending: isPaymentPending } =
    useSubscriptionPayment(paymentId ?? '', Boolean(paymentId));

  useEffect(() => {
    if (payment?.status === 'succeeded') {
      void queryClient.invalidateQueries({ queryKey: billingKeys.subscription });
    }
  }, [payment?.status, queryClient]);

  let state: TariffChangeSuccessState;

  if (paymentId && (isPaymentPending || payment?.status === 'pending')) {
    // Зависшая pending — по серверному дедлайну формы (expiresAt, #680):
    // экран поллит раз в 5 секунд, время берём на рендере.
    const isStale = payment ? isPaymentFormExpired(payment, new Date()) : false;
    state = {
      icon: <Sync className="h-16 w-16 animate-spin text-error" aria-hidden />,
      title: 'Платёж обрабатывается',
      descriptions: isStale
        ? [
            'Обычно это занимает до минуты, страница обновится автоматически',
            'Проверяем статус у банка, это может занять несколько минут',
          ]
        : ['Обычно это занимает до минуты, страница обновится автоматически'],
      action: (
        <Button onClick={() => goBack(router, ROUTES.profileTariff)}>
          Вернуться к тарифу
        </Button>
      ),
    };
  } else if (paymentId && payment?.status === 'failed') {
    state = {
      icon: <StatusIconDanger className="h-16 w-16" aria-hidden />,
      title: 'Оплата не прошла',
      descriptions: ['Попробуйте сменить тариф ещё раз'],
      action: (
        <Button onClick={() => router.replace(ROUTES.profileTariffChange)}>
          Попробовать снова
        </Button>
      ),
    };
  } else {
    state = {
      icon: <CheckNoneLine className="h-16 w-16" aria-hidden />,
      title: 'Тариф изменен',
      descriptions: [
        isPending
          ? 'Загрузка сведений о подписке...'
          : subscription?.pendingTariff !== undefined &&
              subscription.pendingChangeAt !== undefined
            ? `Изменения вступят в силу ${formatDate(subscription.pendingChangeAt)}.`
            : 'Тариф успешно изменен.',
      ],
      action: (
        <Button onClick={() => goBack(router, ROUTES.profileTariff)}>Хорошо</Button>
      ),
    };
  }

  return (
    <div
      className="fullscreen-surface fixed inset-0 z-50 flex flex-col"
      role="dialog"
      aria-label={state.title}
    >
      <div className="relative flex flex-1 items-center justify-center px-6">
        <div className="flex flex-col items-center gap-4 text-center">
          {state.icon}
          <p className="m-0 text-xl font-semibold leading-6 text-content">{state.title}</p>
          {state.descriptions.map((line) => (
            <p key={line} className="m-0 max-w-72 text-sm leading-4 text-content-secondary">
              {line}
            </p>
          ))}
        </div>
      </div>

      <StickyBottomBar>{state.action}</StickyBottomBar>
    </div>
  );
}
