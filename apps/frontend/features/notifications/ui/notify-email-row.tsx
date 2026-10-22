'use client';

import type { JSX } from 'react';
import { cn } from '@/shared/lib/cn';
import { Skeleton, Switch } from '@/shared/ui/design';
import type { NotificationSettingsCategory } from '@/entities/notification';
import { emailNotificationsRowVisible } from '../lib/email-notifications-row-visibility';
import { useEmailNotificationPreferences } from '../api/hooks';

/**
 * Ряд «Уведомления на почту» двухуровневой модели (карта #1551, канон
 * #1552): свитч — персональная галочка отправки писем этого платежа
 * (notifyEmail), управляется формой вызывающего экрана; глобальная матрица
 * профиля здесь только читается и решает видимость ряда. Шоткат-запись
 * матрицы с платёжных/арендных экранов снята — пишут матрицу только
 * настройки профиля (#746). GET в полёте — рендер как сейчас: заголовок
 * и подпись сразу, скелетон свитча; мастер категории off (или не
 * подтвердился из-за сбоя чтения) — ряд скрыт вовсе, команда сохранения
 * галочку не трогает (дифф без поля — забота вызывающего). Запоминание
 * выбора при гейте («не надо» показывает off, значение формы не стирает)
 * — на вызывающем: действующее значение считает
 * emailNotificationsRowValue (features/payments).
 *
 * Тексты всех рядов — прежние (решение владельца 22.10.2026 «текста
 * оставь как есть»): заголовок и подпись с почтой — у вызывающего.
 * Гейт «не надо» (заглушенный ряд) тоже у вызывающего — disabled.
 */
export function NotifyEmailRow({
  category,
  title,
  caption,
  checked,
  onCheckedChange,
  disabled = false,
}: {
  /** Категория матрицы, чей мастер решает видимость ряда: «Платежи и
   * операции» у платёжных экранов, «Аренда» у арендных (тикет #1557). */
  readonly category: NotificationSettingsCategory;
  readonly title: string;
  /** Подпись под заголовком (почта пользователя); undefined — без подписи. */
  readonly caption: string | undefined;
  /** Значение свитча — персональная галочка формы (уже свёрнутая гейтом). */
  readonly checked: boolean;
  readonly onCheckedChange: (checked: boolean) => void;
  /** Весь ряд заглушен (пер-платёжный выбор «не надо»); дефолт — жив. */
  readonly disabled?: boolean;
}): JSX.Element | null {
  const prefsQuery = useEmailNotificationPreferences();
  // Мастер решает видимость: off или нечитаемый (сбой) — ряда нет вовсе,
  // свитч-форме тут включаться нечего. Пока GET в полёте — скелетон.
  if (
    !emailNotificationsRowVisible(
      prefsQuery.isPending,
      prefsQuery.data?.[category],
    )
  ) {
    return null;
  }

  return (
    <div
      aria-disabled={disabled || undefined}
      className={cn(
        'flex items-center justify-between gap-4 px-6 py-3',
        disabled && 'pointer-events-none opacity-50',
      )}
    >
      <div className="flex min-w-0 flex-col gap-1">
        <span className="text-base font-medium leading-[18px] text-content">{title}</span>
        {caption !== undefined && (
          <span className="text-sm leading-4 text-content-tertiary">{caption}</span>
        )}
      </div>
      {prefsQuery.isPending ? (
        <Skeleton className="h-7 w-16 rounded-pill" />
      ) : (
        <Switch
          checked={disabled ? false : checked}
          onCheckedChange={(value) => onCheckedChange(value === true)}
          disabled={disabled}
          aria-label={title}
        />
      )}
    </div>
  );
}
