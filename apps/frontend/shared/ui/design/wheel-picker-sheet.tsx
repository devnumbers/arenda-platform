'use client';

import type { JSX, ReactNode } from 'react';
import { Button, type ButtonProps } from './button';
import { Modal, ModalContent } from './modal';

/** Универсальный шит колёс-«крутилок» (Figma 1539-82659): оболочка пикера
 * значений — любое число колёс WheelPicker колонками + ряд кнопок снизу.
 * Один и тот же шит собирает выбор месяца/года, времени (часы/минуты) и
 * любых будущих значений — решение владельца 2026-09-04 («колесо не только
 * для даты»). Колёса передаются готовыми элементами (каждая колонка сама
 * владеет items/value/черновиком), кнопки — конфигурацией: не всегда
 * «Отменить»/«Выбрать», набор и варианты задаёт потребитель. Одна кнопка —
 * во всю ширину, две — пополам (grid-cols-2, зазор 8 — по макету).
 * Заголовок — sr-only: в макете заголовка нет, a11y-имя диалога остаётся;
 * drag-ручка, свайп вниз и переход карточка/шит — внутри Modal (vaul на
 * мобиле). Черновики колёс живут, пока шит смонтирован: рендерите шит
 * только в открытом состоянии, если черновик должен сбрасываться. */
export type WheelPickerSheetAction = {
  readonly label: string;
  /** Вариант design Button; по умолчанию primary. */
  readonly variant?: ButtonProps['variant'];
  readonly onSelect: () => void;
};

export type WheelPickerSheetProps = {
  /** A11y-имя шита: в макете заголовка нет, скринридерам нужен заголовок. */
  readonly title: string;
  /** Колонки-колёса: готовые <WheelPicker/> — без своего flex-класса,
   * колонки раскладывает шит. */
  readonly columns: ReadonlyArray<ReactNode>;
  /** Кнопки шита снизу. */
  readonly actions: ReadonlyArray<WheelPickerSheetAction>;
  readonly open: boolean;
  readonly onOpenChange: (open: boolean) => void;
  readonly className?: string;
};

export function WheelPickerSheet({
  title,
  columns,
  actions,
  open,
  onOpenChange,
  className,
}: WheelPickerSheetProps): JSX.Element {
  return (
    <Modal open={open} onOpenChange={onOpenChange}>
      <ModalContent title={title} titleSrOnly className={className}>
        <div className="flex flex-col gap-6">
          {/* Полоса выбора одна на все колонки: скруглены только внешние
              края — у левой колонки слева, у правой справа (решение
              владельца 2026-09-04, Figma 1539-82659); ul колёс —
              position:relative и рисуется поверх полосы. */}
          <div className="relative flex items-stretch">
            <div
              aria-hidden
              className="pointer-events-none absolute inset-x-0 top-1/2 h-12 -translate-y-1/2 rounded-2xl bg-surface-muted"
            />
            {columns.map((column, index) => (
              <div key={index} className="min-w-0 flex-1">
                {column}
              </div>
            ))}
          </div>
          <div className="grid grid-cols-2 gap-2">
            {actions.map((action) => (
              <Button
                key={action.label}
                variant={action.variant ?? 'primary'}
                className={actions.length === 1 ? 'col-span-full' : 'w-full'}
                onClick={action.onSelect}
              >
                {action.label}
              </Button>
            ))}
          </div>
        </div>
      </ModalContent>
    </Modal>
  );
}
