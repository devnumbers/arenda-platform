'use client';

import { useState, type JSX } from 'react';
import {
  WheelPicker,
  type WheelPickerItem,
  WheelPickerSheet,
  type WheelPickerSheetAction,
} from '@/shared/ui/design';
/** Пикер времени задачи (#500, Figma 1539-82656): нижний шит на общем
 * WheelPickerSheet (редизайн 2026-09-04) — часы 00–23 и минуты 00–59,
 * кнопки «Отменить»/«Выбрать» по макету 1539-82659. Черновик колёс живёт,
 * пока шит открыт (при open=false тело не рендерится): «Выбрать» коммитит
 * HH:MM разом, «Отменить» и свайп вниз закрывают без изменений. Без
 * выбранного времени колёса стоят на текущем времени устройства, точное
 * до минуты (решение владельца 2026-09-03; в поле формы время попадает
 * только по «Выбрать» — симметрично дате). */

function pad2(value: number): string {
  return String(value).padStart(2, '0');
}

/** Локальное время устройства HH:MM — дефолт колёс. */
function currentTimeHHMM(): string {
  const now = new Date();
  return `${pad2(now.getHours())}:${pad2(now.getMinutes())}`;
}

const HOUR_ITEMS: ReadonlyArray<WheelPickerItem> = Array.from({ length: 24 }, (_, value) => {
  const label = pad2(value);
  return { value: label, label };
});

const MINUTE_ITEMS: ReadonlyArray<WheelPickerItem> = Array.from({ length: 60 }, (_, value) => {
  const label = pad2(value);
  return { value: label, label };
});

function timeParts(time: string): { readonly hour: string; readonly minute: string } {
  const [hour = '00', minute = '00'] = time.split(':');
  return { hour, minute };
}

export type TaskTimePickerProps = {
  readonly open: boolean;
  readonly onOpenChange: (open: boolean) => void;
  /** Текущее значение формы (HH:MM); null — шит открывается на текущем
   * времени устройства. */
  readonly value: string | null;
  readonly onConfirm: (time: string) => void;
};

export function TaskTimePicker({
  open,
  onOpenChange,
  value,
  onConfirm,
}: TaskTimePickerProps): JSX.Element | null {
  if (!open) {
    // Черновик колёс не переживает закрытие — тело смонтировано только
    // в открытом шите.
    return null;
  }
  return (
    <TimeSheet
      initial={value ?? currentTimeHHMM()}
      onCancel={() => onOpenChange(false)}
      onConfirm={onConfirm}
    />
  );
}

/** Тело шита: черновик — локальное состояние, живёт при открытом шите. */
function TimeSheet({
  initial,
  onCancel,
  onConfirm,
}: {
  readonly initial: string;
  readonly onCancel: () => void;
  readonly onConfirm: (time: string) => void;
}): JSX.Element {
  const { hour: initialHour, minute: initialMinute } = timeParts(initial);
  const [hour, setHour] = useState(initialHour);
  const [minute, setMinute] = useState(initialMinute);

  const actions: ReadonlyArray<WheelPickerSheetAction> = [
    { label: 'Отменить', variant: 'secondary', onSelect: onCancel },
    { label: 'Выбрать', onSelect: () => onConfirm(`${hour}:${minute}`) },
  ];

  return (
    <WheelPickerSheet
      title="Время"
      open
      onOpenChange={(next) => {
        if (!next) {
          onCancel();
        }
      }}
      actions={actions}
      columns={[
        <WheelPicker
          key="hour"
          label="Часы"
          strip={false}
          items={HOUR_ITEMS}
          value={hour}
          onValueChange={setHour}
        />,
        <WheelPicker
          key="minute"
          label="Минуты"
          strip={false}
          items={MINUTE_ITEMS}
          value={minute}
          onValueChange={setMinute}
        />,
      ]}
    />
  );
}
