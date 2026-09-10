'use client';

import type { JSX } from 'react';
import { WizardHeading, AutoPayRow } from './wizard-chrome';

/**
 * Шаг 3 «Настройки аренды» (Figma 1270:37343): единственный тумблер
 * «Сделать платеж автоматическим?» — включённый автоплатёж фиксирует
 * оплату в назначенный день сам, выключенный оставляет отметку владельцу.
 * Тумблеры email-уведомлений макета вырезаны (решение картирования #526,
 * в контракте аренды их нет). Строка тумблера — общая с правкой условий
 * (#532, AutoPayRow).
 */

export type SettingsStepProps = {
  readonly autoPay: boolean;
  readonly onAutoPayChange: (autoPay: boolean) => void;
};

export function SettingsStep({
  autoPay,
  onAutoPayChange,
}: SettingsStepProps): JSX.Element {
  return (
    <>
      <WizardHeading title="Настройки аренды" />
      <div className="flex flex-col gap-8 px-6 pt-6">
        <AutoPayRow checked={autoPay} onCheckedChange={onAutoPayChange} />
      </div>
    </>
  );
}
