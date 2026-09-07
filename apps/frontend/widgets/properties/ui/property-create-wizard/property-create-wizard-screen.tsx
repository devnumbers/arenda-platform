'use client';

import type { JSX } from 'react';
import { PageContent, TopNav } from '@/shared/ui/design';
import { usePropertyCreateDraft } from '@/features/properties';
import { PropertyCreateWizardFlow } from './property-create-wizard-flow';

/**
 * Экран визарда создания объекта (#480): маршрут /properties/new, шаги —
 * клиентское состояние. Черновик читается только после гидрации
 * хранилища (useSyncExternalStore), поэтому до гидрации экран показывает
 * скелет шага категории — мутационного входа у экрана нет (аналог
 * визарда платежей #464).
 */

/** Ширины чипов скелета — по меткам категорий шага 1. */
const SKELETON_CHIP_WIDTHS = [104, 104, 72, 208, 72, 88, 88, 136, 160] as const;

export type PropertyCreateWizardScreenProps = {
  /** Санитизированный ?returnTo= маршрута (sanitizeReturnTo на серверной
   * странице): внутренний абсолютный путь возврата после создания. */
  readonly returnTo?: string;
};

export function PropertyCreateWizardScreen({ returnTo }: PropertyCreateWizardScreenProps): JSX.Element {
  const { isLoaded } = usePropertyCreateDraft();

  if (!isLoaded) {
    return (
      <>
        <TopNav />
        <PageContent>
          <div className="flex flex-col gap-3 px-6 pt-6" aria-hidden>
            <div className="h-8 w-72 max-w-full rounded bg-surface-muted" />
            <div className="mt-3 flex flex-wrap gap-2">
              {/* Список декоративный и статичный (aria-hidden, никогда не
                  переупорядочивается) — ключ по позиции честен. */}
              {SKELETON_CHIP_WIDTHS.map((width, index) => (
                <div key={index} className="h-11 rounded-pill bg-surface-muted" style={{ width }} />
              ))}
            </div>
          </div>
        </PageContent>
      </>
    );
  }

  return <PropertyCreateWizardFlow returnTo={returnTo} />;
}
