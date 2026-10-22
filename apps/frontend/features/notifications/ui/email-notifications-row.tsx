'use client';

import type { JSX } from 'react';
import { notify } from '@/shared/lib/notifications';
import { cn } from '@/shared/lib/cn';
import { Skeleton, Switch } from '@/shared/ui/design';
import { useEmailNotificationPreferences, useUpdateEmailPreferences } from '../api/hooks';

/**
 * Строка тумблера «уведомления на почту» на экране создания аренды —
 * последний потребитель шотката глобальной email-настройки категории
 * «Платежи и операции» (канон экрана настроек #746): значение и запись
 * в матрице аккаунта, никаких пер-арендных override. Платёжные экраны
 * переехали на NotifyEmailRow (карта #1551, тикет #1556); аренда —
 * тикет #1557, после которого шоткат живёт только в настройках профиля.
 * Заголовок и подпись — у вызывающего (макет настроек аренды 1428-58757).
 * Пока настройки грузятся — скелетон; не загрузились — тумблер выключен
 * и заглушен (экран не блокируем).
 *
 * disabled (решение владельца 22.10.2026) гасит весь ряд, а не только
 * тумблер (канон ListRow: pointer-events-none + opacity-50 на обёртке,
 * aria-disabled): пер-платёжный выбор «не надо» — уведомлений и напоминаний
 * по платежу нет, глобальной настройке здесь нечего включать; значение
 * матрицы не трогается, вернули уведомления — ряд снова жив.
 */
export function EmailNotificationsRow({
  title,
  caption,
  disabled = false,
}: {
  readonly title: string;
  /** Подпись под заголовком (почта пользователя); undefined — без подписи. */
  readonly caption: string | undefined;
  /** Весь ряд заглушен (пер-платёжный выбор «не надо»); дефолт — жив. */
  readonly disabled?: boolean;
}): JSX.Element {
  const prefsQuery = useEmailNotificationPreferences();
  const updatePrefs = useUpdateEmailPreferences();
  const current = prefsQuery.data;

  const toggle = (value: boolean): void => {
    if (current === undefined || disabled) return;
    updatePrefs.mutate(
      { ...current, payments_operations: value },
      {
        onError: (error) =>
          notify.scenarios.profile.notificationPreferencesSaveError(error),
      },
    );
  };

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
          checked={current?.payments_operations ?? false}
          onCheckedChange={(value) => toggle(value === true)}
          disabled={current === undefined || disabled}
          aria-label={title}
        />
      )}
    </div>
  );
}
