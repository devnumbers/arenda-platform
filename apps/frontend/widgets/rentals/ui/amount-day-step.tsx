'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import { Calendar } from '@/shared/assets/icons';
import type { RentalPaymentDay } from '@/entities/rental';
import { paymentDayLabel } from '@/features/rentals';
import { kopecksToAmountInputString, parseRublesToKopecks } from '@/shared/lib/format-money';
import { MoneyField, PaymentDayPicker, PickerTriggerBox, WizardHeading } from './wizard-chrome';

/**
 * Шаг 1 «Цена и число оплаты» (Figma 1270:46904/47385): денежный ввод с
 * живой группировкой «56 000» (паттерн компактного поля правки
 * платежа #467) и поле дня оплаты с иконкой Calendar. Пикер дня (Figma
 * 1270:37490 — донор из платежей, выбор одиночный) — полноэкранный
 * оверлей поверх формы: грид чисел 1–30 — 31-е закрывается строкой
 * «Последний день месяца» (решение владельца 2026-09-05), строки
 * взаимоисключимы, «Выбрать» коммитит. Кнопка «Выбрать» скрыта, пока
 * не выбрано ничего (решение владельца 2026-09-05 — скрытие вместо
 * дизейбла, как у канонных пикеров).
 */

export type AmountDayStepProps = {
  /** Копейки; undefined — ещё не задана. */
  readonly amountKopecks: number | undefined;
  readonly onAmountChange: (amountKopecks: number | undefined) => void;
  readonly paymentDay: RentalPaymentDay | undefined;
  readonly onPaymentDayChange: (paymentDay: RentalPaymentDay | undefined) => void;
};

export function AmountDayStep({
  amountKopecks,
  onAmountChange,
  paymentDay,
  onPaymentDayChange,
}: AmountDayStepProps): JSX.Element {
  const [dayPickerOpen, setDayPickerOpen] = useState(false);
  // «Сырое» набранное значение — источник отображения (группировка не
  // должна сбрасывать каретку); копейки едут в черновик для валидации.
  const [amountRaw, setAmountRaw] = useState(() =>
    amountKopecks === undefined ? '' : kopecksToAmountInputString(amountKopecks),
  );

  return (
    <>
      <WizardHeading title="Цена и число оплаты" />
      <div className="flex flex-col gap-8 px-6 pt-6">
        <MoneyField
          title="Арендная плата"
          required
          raw={amountRaw}
          onRawChange={(raw) => {
            setAmountRaw(raw);
            onAmountChange(parseRublesToKopecks(raw, { positive: true }));
          }}
          onClear={() => {
            setAmountRaw('');
            onAmountChange(undefined);
          }}
          ariaLabel="Арендная плата, рублей"
        />
        <PickerTriggerBox
          title="День оплаты"
          required
          value={paymentDay === undefined ? undefined : paymentDayLabel(paymentDay)}
          placeholder="Выбрать день"
          icon={<Calendar className="h-6 w-6" />}
          onClick={() => setDayPickerOpen(true)}
        />
      </div>

      {/* Рендер только в открытом состоянии: черновик выбора живёт, пока
          пикер смонтирован (конвенция канона). */}
      {dayPickerOpen && (
        <PaymentDayPicker
          initial={paymentDay}
          onClose={() => setDayPickerOpen(false)}
          onConfirm={(day) => {
            onPaymentDayChange(day);
            setDayPickerOpen(false);
          }}
        />
      )}
    </>
  );
}
