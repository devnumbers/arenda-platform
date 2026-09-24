'use client';

import type { JSX } from 'react';
import { notify } from '@/shared/lib/notifications';
import { Skeleton, Switch } from '@/shared/ui/design';
import { useEmailNotificationPreferences, useUpdateEmailPreferences } from '../api/hooks';

/**
 * Строка тумблера «уведомления на почту» на экранах создания (карта #822):
 * шоткат глобальной email-настройки категории «Платежи и операции» —
 * значение и запись в матрице аккаунта (канон экрана настроек #746),
 * никаких пер-платёжных override. Заголовок и подпись — у вызывающего
 * (макеты шага 4 платежей 1084-24863/1056-54338 и настроек аренды
 * 1428-58757 ходят разными текстами). Пока настройки грузятся — скелетон;
 * не загрузились — тумблер выключен и заглушен (экран не блокируем).
 */
export function EmailNotificationsRow({
  title,
  caption,
}: {
  readonly title: string;
  /** Подпись под заголовком (почта пользователя); undefined — без подписи. */
  readonly caption: string | undefined;
}): JSX.Element {
  const prefsQuery = useEmailNotificationPreferences();
  const updatePrefs = useUpdateEmailPreferences();
  const current = prefsQuery.data;

  const toggle = (value: boolean): void => {
    if (current === undefined) return;
    updatePrefs.mutate(
      { ...current, payments_operations: value },
      {
        onError: (error) =>
          notify.scenarios.profile.notificationPreferencesSaveError(error),
      },
    );
  };

  return (
    <div className="flex items-center justify-between gap-4 px-6 py-3">
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
          disabled={current === undefined || updatePrefs.isPending}
          aria-label={title}
        />
      )}
    </div>
  );
}
