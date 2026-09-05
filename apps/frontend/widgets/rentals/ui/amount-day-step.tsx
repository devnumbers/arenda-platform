'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import { ArrowLeft, Calendar } from '@/shared/assets/icons';
import type { RentalPaymentDay } from '@/entities/rental';
import { paymentDayFromPicker, paymentDayLabel } from '@/features/rentals';
import {
  Button,
  Checkbox,
  IconButton,
  ListRow,
  MonthDaysGrid,
  PageContent,
  StickyBottomBar,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import { kopecksToAmountInputString, parseRublesToKopecks } from '@/shared/lib/format-money';
import { MoneyField, PickerTriggerBox, WizardBottomBar, WizardHeading } from './wizard-chrome';

/**
 * Шаг 1 «Цена и число оплаты» (Figma 1270:46904/47385): денежный ввод с
 * живой группировкой «56 000 ₽» (паттерн компактного поля правки
 * платежа #467) и поле дня оплаты с иконкой Calendar. Пикер дня (Figma
 * 1270:37490 — донор из платежей, выбор одиночный) — полноэкранный
 * оверлей поверх формы: грид чисел месяца и взаимоисключимая строка
 * «Последний день месяца», «Выбрать» коммитит.
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

/** Полноэкранный оверлей выбора дня оплаты (поверхность «временный пикер
 * поверх формы»): одиночный выбор числа либо «последний день месяца» —
 * они взаимоисключимы, тап по выбранному числу снимает его. */
function PaymentDayPicker({
  initial,
  onClose,
  onConfirm,
}: {
  readonly initial: RentalPaymentDay | undefined;
  readonly onClose: () => void;
  readonly onConfirm: (paymentDay: RentalPaymentDay | undefined) => void;
}): JSX.Element {
  const [draftDay, setDraftDay] = useState<number | undefined>(
    typeof initial === 'number' ? initial : undefined,
  );
  const [draftLast, setDraftLast] = useState<boolean>(initial === 'last');

  const handleDay = (day: number): void => {
    setDraftLast(false);
    setDraftDay((prev) => (prev === day ? undefined : day));
  };

  const handleLast = (): void => {
    if (draftLast) {
      setDraftLast(false);
      return;
    }
    setDraftLast(true);
    setDraftDay(undefined);
  };

  const ready = draftLast || draftDay !== undefined;

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Выбор дня оплаты"
      className="fixed inset-0 z-50 flex flex-col bg-surface"
    >
      <TopNav
        leading={
          <IconButton icon={<ArrowLeft />} label="Назад" onClick={onClose} />
        }
      >
        <TopNavTitle title="Выберите день" />
      </TopNav>
      <PageContent className="flex h-[calc(100dvh-72px)] flex-col pb-0">
        <div className="flex-1 min-h-0 overflow-y-auto pb-6">
          <div className="pt-6">
            <MonthDaysGrid
              days={31}
              selectedDays={
                draftDay === undefined || draftLast ? undefined : new Set([draftDay])
              }
              onDayToggle={handleDay}
            />
          </div>
          <div className="pt-6">
            {/* Строка — сам переключатель (Enter/Space/клик по ListRow);
                чекбокс — его зрительный индикатор, некликабельный: тап
                не должен тонуть дважды (строка + чекбокс). */}
            <ListRow
              title="Последний день месяца"
              onSelect={handleLast}
              trailing={
                <Checkbox
                  checked={draftLast}
                  tabIndex={-1}
                  aria-hidden
                  className="pointer-events-none"
                />
              }
            />
          </div>
        </div>
        <StickyBottomBar>
          <WizardBottomBar>
            <Button className="w-full" disabled={!ready} onClick={() => onConfirm(paymentDayFromPicker({ day: draftDay, last: draftLast }))}>
              Выбрать
            </Button>
          </WizardBottomBar>
        </StickyBottomBar>
      </PageContent>
    </div>
  );
}
