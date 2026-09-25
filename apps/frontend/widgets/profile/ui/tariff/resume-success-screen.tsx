'use client';

import type { JSX } from 'react';
import { useRouter } from 'next/navigation';
import { CheckNoneLine } from '@/shared/assets/icons';
import { Button, StickyBottomBar } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import type { TariffName } from '@/entities/user';
import { resumeSuccessTitle } from '@/widgets/profile/lib/tariff-about';

/** Полноэкранный успех возобновления (#621, макет 1934-78524): шапки нет,
 * в центре — иконка Color/CheckNoneLine 64 и заголовок H3, внизу Primary
 * «Хорошо» → на главный «Тариф» (goBack снимает «О тарифе» из истории).
 * Рендерится поверх экрана «О тарифе» после успешного бесплатного
 * POST /subscription/resume — отдельного маршрута у состояния нет. */
export function ResumeSuccessScreen({
  tariffName,
}: {
  readonly tariffName: TariffName;
}): JSX.Element {
  const router = useRouter();
  const title = resumeSuccessTitle(tariffName);

  return (
    <div
      className="fullscreen-surface fixed inset-0 z-50 flex flex-col"
      role="dialog"
      aria-label={title}
    >
      <div className="relative flex flex-1 items-center justify-center px-6">
        <div className="flex flex-col items-center gap-4">
          <CheckNoneLine className="h-16 w-16" aria-hidden />
          <p className="m-0 text-xl font-semibold leading-6 text-content">{title}</p>
        </div>
      </div>

      <StickyBottomBar surface>
        <Button onClick={() => goBack(router, ROUTES.profileTariff)}>Хорошо</Button>
      </StickyBottomBar>
    </div>
  );
}
