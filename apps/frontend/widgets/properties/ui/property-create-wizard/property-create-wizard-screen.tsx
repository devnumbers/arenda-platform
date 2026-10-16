'use client';

import type { JSX } from 'react';
import { PropertyCreateWizardFlow } from './property-create-wizard-flow';

/**
 * Экран визарда создания объекта (#480): маршрут /properties/new, шаги —
 * клиентское состояние потока (черновика нет — карта #1052, Q2=В),
 * рендерится сразу: мутационного входа у экрана нет, на первом кадре нет
 * асинхронных данных (чипы категорий — статический реестр).
 */

export type PropertyCreateWizardScreenProps = {
  /** Санитизированный ?returnTo= маршрута (sanitizeReturnTo на серверной
   * странице): внутренний абсолютный путь возврата после создания. */
  readonly returnTo?: string;
};

export function PropertyCreateWizardScreen({ returnTo }: PropertyCreateWizardScreenProps): JSX.Element {
  return <PropertyCreateWizardFlow returnTo={returnTo} />;
}
