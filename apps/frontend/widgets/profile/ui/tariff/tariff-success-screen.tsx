'use client';

import type { JSX, ReactNode } from 'react';
import { cn } from '@/shared/lib/cn';
import { fullscreenSurfaceClass, StickyBottomBar } from '@/shared/ui/design';

type TariffSuccessScreenProps = {
  /** Иконка 64 в центре (CheckNoneLine/CancelColor/…), aria-hidden у
   * вызывающего; может отсутствовать — каркас центрирует заголовок один
   * (платёж в обработке живёт без иконки, решение владельца 30.09). */
  readonly icon?: ReactNode;
  /** Заголовок H3 — он же aria-label диалога (хелперы tariff-about/tariff-disable). */
  readonly title: string;
  /** Серые строки описания под заголовком (успех смены тарифа #623);
   * без пропа — каркас #621/#622 с одной парой «иконка + заголовок». */
  readonly descriptions?: ReadonlyArray<string>;
  /** Нижнее действие целиком — переход (goBack/router.replace) остаётся
   * у вызывающего, архетип знает только раскладку. */
  readonly action?: ReactNode;
};

/** Общий каркас полноэкранных успехов тарифа (#621 возобновление, #622
 * отключение, #623 смена): шапки нет, в центре — иконка 64 и заголовок H3
 * (с описаниями — они центрируются, серые 14/16), внизу StickyBottomBar
 * с одним действием. Поверхность — fullscreenSurfaceClass (полноэкранный
 * диалог без собственного фона). */
export function TariffSuccessScreen({
  icon,
  title,
  descriptions,
  action,
}: TariffSuccessScreenProps): JSX.Element {
  return (
    <div className={fullscreenSurfaceClass} role="dialog" aria-label={title}>
      <div className="relative flex flex-1 items-center justify-center px-6">
        <div
          className={cn(
            'flex flex-col items-center gap-4',
            // Многострочные описания центрируются; каркас #621/#622 без
            // описаний остаётся как в макетах — без text-center.
            descriptions !== undefined && 'text-center',
          )}
        >
          {icon}
          <p className="m-0 text-xl font-semibold leading-6 text-content">{title}</p>
          {descriptions?.map((line) => (
            <p key={line} className="m-0 max-w-72 text-sm leading-4 text-content-secondary">
              {line}
            </p>
          ))}
        </div>
      </div>

      <StickyBottomBar>{action}</StickyBottomBar>
    </div>
  );
}
