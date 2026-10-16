'use client';

import type { JSX } from 'react';
import { ConfirmDialog } from '@/shared/ui/design';

/**
 * Подтверждение «Удалить все выполненные задачи?» — общий для объектного
 * экрана (#499) и глобальной ленты (#523); мутацию и её скоуп приносит
 * экран (на объекте — все выполненные задачи объекта, в ленте — вся книга
 * читателя; горизонт истории на бэке не даёт тику воскресить снесённое,
 * ADR 0051 §3 в правке 2026-10-01 / #536).
 */
export function TasksDeleteCompletedDialog({
  open,
  onOpenChange,
  pending,
  onConfirm,
}: {
  readonly open: boolean;
  readonly onOpenChange: (open: boolean) => void;
  readonly pending: boolean;
  readonly onConfirm: () => void;
}): JSX.Element {
  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      title="Удалить все выполненные задачи?"
      description="Все выполненные задачи будут навсегда удалены"
      cancelLabel="Отменить"
      confirmLabel="Удалить"
      confirmVariant="danger"
      pending={pending}
      onConfirm={onConfirm}
    />
  );
}
