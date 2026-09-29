'use client';

import type { JSX } from 'react';
import { useRouter } from 'next/navigation';
import { CancelColor } from '@/shared/assets/icons';
import { Button } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import type { TariffName } from '@/entities/user';
import { disableSuccessTitle } from '@/widgets/profile/lib/tariff-disable';
import { TariffSuccessScreen } from './tariff-success-screen';

/** Полноэкранный успех отключения (#622, макет 1933-78038) на общем
 * каркасе TariffSuccessScreen: иконка Icon/Color/Cancel 64, заголовок
 * «Тариф X отключен», внизу Primary «Хорошо» → на главный «Тариф» в
 * состоянии «отключен». Рендерится поверх экрана «Отключение тарифа» после
 * успешного POST /subscription/cancel — отдельного маршрута у состояния
 * нет. */
export function TariffDisableSuccessScreen({
  tariffName,
}: {
  readonly tariffName: TariffName;
}): JSX.Element {
  const router = useRouter();
  const title = disableSuccessTitle(tariffName);

  return (
    <TariffSuccessScreen
      icon={<CancelColor className="h-16 w-16" aria-hidden />}
      title={title}
      action={
        // replace, а не goBack: история несёт «О тарифе» → «Отключение»,
        // один назад вернул бы на «О тарифе», а не на главный «Тариф».
        <Button onClick={() => router.replace(ROUTES.profileTariff)}>Хорошо</Button>
      }
    />
  );
}
