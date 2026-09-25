'use client';

import type { JSX } from 'react';
import { useRouter } from 'next/navigation';
import { CancelColor } from '@/shared/assets/icons';
import { Button, StickyBottomBar } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import type { TariffName } from '@/entities/user';
import { disableSuccessTitle } from '@/widgets/profile/lib/tariff-disable';

/** Полноэкранный успех отключения (#622, макет 1933-78038): шапки нет,
 * в центре — Icon/Color/Cancel 64 и заголовок «Тариф X отключен», внизу
 * Primary «Хорошо» → на главный «Тариф» в состоянии «отключен». Рендерится
 * поверх экрана «Отключение тарифа» после успешного
 * POST /subscription/cancel — отдельного маршрута у состояния нет. */
export function TariffDisableSuccessScreen({
  tariffName,
}: {
  readonly tariffName: TariffName;
}): JSX.Element {
  const router = useRouter();
  const title = disableSuccessTitle(tariffName);

  return (
    <div
      className="fullscreen-surface fixed inset-0 z-50 flex flex-col"
      role="dialog"
      aria-label={title}
    >
      <div className="relative flex flex-1 items-center justify-center px-6">
        <div className="flex flex-col items-center gap-4">
          <CancelColor className="h-16 w-16" aria-hidden />
          <p className="m-0 text-xl font-semibold leading-6 text-content">{title}</p>
        </div>
      </div>

      <StickyBottomBar surface>
        {/* replace, а не goBack: история несёт «О тарифе» → «Отключение»,
            один назад вернул бы на «О тарифе», а не на главный «Тариф». */}
        <Button onClick={() => router.replace(ROUTES.profileTariff)}>Хорошо</Button>
      </StickyBottomBar>
    </div>
  );
}
