'use client';

import type { JSX, ReactNode } from 'react';
import { Button } from './button';
import { Modal, ModalContent } from './modal';

/** Confirm-диалог дизайн-слоя — единственный канон подтверждений
 * (#505, замена легаси HeroUI ConfirmModal ADR 0050; унификация
 * 2026-09-04 поглотила design/confirm-modal). Контролируемый поверх
 * адаптивного Modal: карточка ≥768px, нижний шит с ручкой уже. Заголовок
 * обязателен (a11y: контент ссылается на Title), описание опционально.
 * Кнопки в ряд: отмена (secondary) + подтверждение (primary, для
 * разрушительных действий confirmVariant="danger"); `stacked` ставит их
 * столбиком на всю ширину — подтверждение сверху, отмена под ним
 * (макет удаления объекта #629, Figma 1583:56558; решение владельца
 * 12.09.2026: чекбокс «Удалить все данные» срезан, удаляется всё вместе
 * с объектом, кнопка одна). Закрытие — на
 * потребителе: onConfirm вызывается, диалог закрывает потребитель
 * (обычно в onSuccess мутации, как в задачах) или сам через
 * onOpenChange(false). `pending` — подтверждение в полёте: кнопка
 * в loading, отмена/оверлей/свайп закрытие глушат. */
export type ConfirmDialogProps = {
  readonly open: boolean;
  readonly onOpenChange: (open: boolean) => void;
  readonly title: string;
  /** Дополнение к классу заголовка: каноника — H3 20/24, макетам с H1 28
   * (шит удаления объекта #629) даёт text-[28px] leading-8. */
  readonly titleClassName?: string;
  readonly description?: string;
  /** Дополнение к классу описания: каноника — 14px, макетам с R/400 16
   * (шит завершения аренды #627) даёт text-base — как в ModalContent. */
  readonly descriptionClassName?: string;
  /** Слот между описанием и кнопками: предупреждения о последствиях
   * (красный текст шита удаления #629). */
  readonly children?: ReactNode;
  readonly confirmLabel: string;
  readonly cancelLabel?: string;
  readonly confirmVariant?: 'primary' | 'danger';
  /** Кнопки столбиком на всю ширину вместо ряда (см. докблок). */
  readonly stacked?: boolean;
  /** Подтверждение в полёте: кнопка в loading, закрытие глушится. */
  readonly pending?: boolean;
  readonly onConfirm: () => void;
};

export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  titleClassName,
  description,
  descriptionClassName,
  children,
  confirmLabel,
  cancelLabel = 'Отмена',
  confirmVariant = 'primary',
  stacked = false,
  pending = false,
  onConfirm,
}: ConfirmDialogProps): JSX.Element {
  const cancelButton = (
    <Button variant="secondary" disabled={pending} onClick={() => onOpenChange(false)}>
      {cancelLabel}
    </Button>
  );
  const confirmButton = (
    <Button variant={confirmVariant} loading={pending} onClick={onConfirm}>
      {confirmLabel}
    </Button>
  );
  return (
    <Modal
      open={open}
      onOpenChange={(next) => {
        if (!pending) {
          onOpenChange(next);
        }
      }}
    >
      <ModalContent
        title={title}
        titleClassName={titleClassName}
        description={description}
        descriptionClassName={descriptionClassName}
      >
        {children}
        {stacked ? (
          <div className="flex flex-col gap-2">
            {confirmButton}
            {cancelButton}
          </div>
        ) : (
          <div className="grid grid-cols-2 gap-2">
            {cancelButton}
            {confirmButton}
          </div>
        )}
      </ModalContent>
    </Modal>
  );
}
