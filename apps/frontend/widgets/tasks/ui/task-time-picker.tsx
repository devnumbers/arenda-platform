'use client';

import { useState, type JSX } from 'react';
import { Button, Modal, ModalContent, WheelPicker } from '@/shared/ui/design';
// Именованные константы грида — прямой импорт модуля shared (общие слои —
// точки входа сами по себе); в индекс слоя тип не выведен.
import type { WheelPickerItem } from '@/shared/ui/design/wheel-picker';

/** Пикер времени задачи (#500, Figma 1539-82656): нижний шит с двумя
 * колёсами WheelPicker — часы 00–23 и минуты 00–59 — и кнопками
 * «Отменить»/«Выбрать». Черновик колёс живёт, пока шит открыт (контент
 * модалки размонтируется при закрытии): «Выбрать» коммитит HH:MM разом,
 * «Отменить» и свайп вниз закрывают без изменений. Без выбранного времени
 * колёса стоят на текущем времени устройства, точное до минуты (решение
 * владельца 2026-09-03; в поле формы время попадает только по «Выбрать» —
 * симметрично дате). */

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
}: TaskTimePickerProps): JSX.Element {
  return (
    <Modal open={open} onOpenChange={onOpenChange}>
      <ModalContent title="Время" titleSrOnly>
        <TimeWheelBody
          initial={value ?? currentTimeHHMM()}
          onCancel={() => onOpenChange(false)}
          onConfirm={onConfirm}
        />
      </ModalContent>
    </Modal>
  );
}

/** Колёса с кнопками: черновик — локальное состояние тела шита. */
function TimeWheelBody({
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

  return (
    <>
      <div className="flex items-stretch">
        <WheelPicker
          className="min-w-0 flex-1"
          label="Часы"
          items={HOUR_ITEMS}
          value={hour}
          onValueChange={setHour}
        />
        <WheelPicker
          className="min-w-0 flex-1"
          label="Минуты"
          items={MINUTE_ITEMS}
          value={minute}
          onValueChange={setMinute}
        />
      </div>
      <div className="grid grid-cols-2 gap-2">
        <Button variant="secondary" className="w-full" onClick={onCancel}>
          Отменить
        </Button>
        <Button className="w-full" onClick={() => onConfirm(`${hour}:${minute}`)}>
          Выбрать
        </Button>
      </div>
    </>
  );
}
