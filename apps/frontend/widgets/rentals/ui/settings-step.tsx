'use client';

import type { JSX } from 'react';
import { Switch } from '@/shared/ui/design';
import { WizardHeading } from './wizard-chrome';

/**
 * Шаг 3 «Настройки аренды» (Figma 1270:37343): единственный тумблер
 * «Сделать платеж автоматическим?» — включённый автоплатёж фиксирует
 * оплату в назначенный день сам, выключенный оставляет отметку владельцу.
 * Тумблеры email-уведомлений макета вырезаны (решение картирования #526,
 * в контракте аренды их нет). Опечатки подписи макета не воспроизводятся.
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
        <div className="flex items-center justify-between gap-4">
          <div className="flex min-w-0 flex-col gap-1">
            <span className="text-base font-medium leading-[18px] text-content">
              Сделать платеж автоматическим?
            </span>
            <span className="text-sm leading-4 text-content-tertiary">
              Оплата аренды автоматически зафиксируется в назначенный день.
              Либо отмечайте её вручную, а мы пришлём напоминание
            </span>
          </div>
          <Switch
            checked={autoPay}
            onCheckedChange={(checked) => onAutoPayChange(checked === true)}
            aria-label="Сделать платеж автоматическим"
          />
        </div>
      </div>
    </>
  );
}
