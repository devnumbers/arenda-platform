import type { JSX } from 'react';
import { Skeleton, SubScreenShell } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';

/** Route-loading «Настроить уведомления» (#609): каркас саб-экрана (#568)
 * вне фазы загрузки; контент — скелетон мастер-строки и групп матрицы. */
export default function NotificationsLoading(): JSX.Element {
  return (
    <SubScreenShell title="Настроить уведомления" fallbackHref={ROUTES.profile}>
      <div className="flex flex-col pb-6 pt-1" aria-label="Загрузка настроек">
        <div className="flex items-center justify-between py-3">
          <Skeleton className="h-5 w-56" />
          <Skeleton className="h-7 w-16 rounded-pill" />
        </div>
        {[0, 1, 2, 3].map((group) => (
          <div key={group} className="mt-7 flex flex-col gap-4">
            <Skeleton className="h-7 w-52" />
            <Skeleton className="h-4 w-full" />
            {[0, 1].map((row) => (
              <div key={row} className="flex items-center justify-between py-1">
                <Skeleton className="h-5 w-44" />
                <Skeleton className="h-7 w-16 rounded-pill" />
              </div>
            ))}
          </div>
        ))}
      </div>
    </SubScreenShell>
  );
}
