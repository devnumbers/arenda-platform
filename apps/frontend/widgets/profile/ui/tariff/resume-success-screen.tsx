'use client';

import type { JSX } from 'react';
import { useRouter } from 'next/navigation';
import { CheckNoneLine } from '@/shared/assets/icons';
import { Button } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import type { TariffName } from '@/entities/user';
import { resumeSuccessTitle } from '@/widgets/profile/lib/tariff-about';
import { TariffSuccessScreen } from './tariff-success-screen';

/** Полноэкранный успех возобновления (#621, макет 1934-78524) на общем
 * каркасе TariffSuccessScreen: иконка Color/CheckNoneLine 64, заголовок H3,
 * внизу Primary «Хорошо» → на главный «Тариф» (goBack снимает «О тарифе»
 * из истории). Рендерится поверх экрана «О тарифе» после успешного
 * бесплатного POST /subscription/resume — отдельного маршрута у состояния
 * нет. */
export function ResumeSuccessScreen({
  tariffName,
}: {
  readonly tariffName: TariffName;
}): JSX.Element {
  const router = useRouter();
  const title = resumeSuccessTitle(tariffName);

  return (
    <TariffSuccessScreen
      icon={<CheckNoneLine className="h-16 w-16" aria-hidden />}
      title={title}
      action={
        <Button onClick={() => goBack(router, ROUTES.profileTariff)}>Хорошо</Button>
      }
    />
  );
}
