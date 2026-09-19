'use client';

import { useState, type JSX } from 'react';
import {
  NOTIFICATION_SETTINGS_CATEGORIES,
  notificationCategoryLabel,
  type NotificationSettingsCategory,
} from '@/entities/notification';
import {
  defaultPushDevicePreferences,
  usePushDevicePreferences,
  usePushSubscriptionStatus,
  useSubscribePush,
  useUpdatePushDevicePreferences,
  type PushDevicePreferences,
} from '@/features/push-notifications';
import {
  useEmailNotificationPreferences,
  useUpdateEmailPreferences,
} from '@/features/notifications';
import { notify } from '@/shared/lib/notifications';
import { Button, Skeleton, Switch } from '@/shared/ui/design';
import {
  applyPushChange,
  resolvePushDisplay,
  verdictFromOutcome,
  type PushChange,
} from '../lib/notification-settings';
import { PushPermissionCard } from './settings-permission-card';
import { NotificationPermissionSheet } from './settings-permission-sheet';

/** Описания групп — с макета 1789-100250 (карта #734, #746). */
const GROUP_DESCRIPTIONS: Record<NotificationSettingsCategory, string> = {
  rental: 'Завершение срока аренды, напоминания о продлении или окончании аренды',
  payments_operations: 'Напоминания об оплате, просроченные платежи и подтверждения операций',
  tasks: 'Напоминания о задачах и уведомления, если срок выполнения прошел',
  shared_access: 'Приглашения в объект, изменение ваших прав или доступа',
};

/**
 * Экран «Настроить уведомления» (карта #734, тикет #746): мастер-тумблер
 * «Получать пуш-уведомления» per-device (решение #738) и матрица 4 группы ×
 * (Электронная почта, Пуш-уведомления); Тариф и Системные всегда включены и
 * в экране отсутствуют. Email-матрица — на аккаунте
 * (GET/PUT /notification-preferences), пуш-колонка — на подписке браузера
 * (GET/PUT /push/subscriptions/preferences); оба PUT оптимистичны с откатом.
 *
 * Пуш-флоу «Разрешите пуши»: включение пуш-тумблера без разрешения браузера
 * открывает шит (2329-151632), мастер включён без разрешения — inline-карту
 * под мастером (2329-150165); после разрешения пуш включён (2329-151164).
 * Мастер выключен — пуш-тумблеры групп затемнены (2333-180696), значения
 * хранятся. Браузер без Web Push — пуш-колонка не рендерится.
 */
export function NotificationSettingsScreen(): JSX.Element {
  const { refresh, ...pushStatus } = usePushSubscriptionStatus();
  const probeSettled = !pushStatus.isPending;
  const supported = !pushStatus.isUnsupported;
  const permissionGranted = probeSettled && !pushStatus.needsPermission;
  const endpoint = pushStatus.endpoint;

  const pushPrefsQuery = usePushDevicePreferences(endpoint ?? undefined);
  const emailQuery = useEmailNotificationPreferences();

  const updatePush = useUpdatePushDevicePreferences();
  const updateEmail = useUpdateEmailPreferences();
  const { subscribe, isPending: flowPending } = useSubscribePush();

  // Локальное состояние пуш-колонки до первой подписки: выключатели здесь
  // нечему хранить на сервере, флоу разрешения сохранит их первым PUT.
  const [localPush, setLocalPush] = useState<PushDevicePreferences | null>(null);
  // Изменение, ждущее разрешения (шит открыт); карточка идёт без изменения —
  // сохраняет текущее состояние как есть.
  const [pendingChange, setPendingChange] = useState<PushChange | null>(null);
  const [sheetOpen, setSheetOpen] = useState(false);

  const display = resolvePushDisplay({
    probeSettled,
    supported,
    permissionGranted,
    endpoint,
    server: pushPrefsQuery.data,
    local: localPush,
  });
  // Пуш-тумблеры рисуем скелетоном, пока неясно состояние (проба браузера
  // или GET устройства при живой подписке).
  const pushLoading = !probeSettled || (endpoint !== null && pushPrefsQuery.isPending);

  const currentPushState = (): PushDevicePreferences =>
    endpoint !== null
      ? (pushPrefsQuery.data ?? defaultPushDevicePreferences())
      : (localPush ?? defaultPushDevicePreferences());

  const savePush = (next: PushDevicePreferences, saveEndpoint: string): void => {
    updatePush.mutate(
      { endpoint: saveEndpoint, ...next },
      {
        onError: (error) =>
          notify.scenarios.profile.notificationPreferencesSaveError(error),
      },
    );
  };

  const toggleEmail = (category: NotificationSettingsCategory, value: boolean): void => {
    const current = emailQuery.data;
    if (current === undefined) return;
    updateEmail.mutate(
      { ...current, [category]: value },
      {
        onError: (error) =>
          notify.scenarios.profile.notificationPreferencesSaveError(error),
      },
    );
  };

  /** Флоу разрешения: системный промпт (если нужно) → подписка → первый PUT
   * желаемого состояния (локальные выключатели + ожидающее изменение).
   * Исходы, кроме подписки, тумблер не двигают — только тост. */
  const runPermissionFlow = async (change: PushChange | null): Promise<void> => {
    const outcome = await subscribe();
    const verdict = verdictFromOutcome(outcome);

    switch (verdict.kind) {
      case 'subscribe-success': {
        notify.scenarios.profile.pushEnabled();
        const desired = change
          ? applyPushChange(currentPushState(), change)
          : currentPushState();
        savePush(desired, verdict.endpoint);
        refresh();
        break;
      }
      case 'denied':
        notify.scenarios.profile.pushPermissionDenied();
        break;
      case 'ios-needs-install':
        notify.scenarios.profile.pushIosNeedsInstall();
        break;
      case 'flow-error':
        notify.scenarios.profile.pushEnableError(
          new Error('push permission flow failed'),
        );
        break;
      case 'noop':
        break;
    }

    setPendingChange(null);
    setSheetOpen(false);
  };

  const toggleMaster = (value: boolean): void => {
    if (!value) {
      // Выключение в разрешении не нуждается: с подпиской — PUT, без — локально.
      if (endpoint !== null) {
        savePush(
          applyPushChange(currentPushState(), { kind: 'master', value: false }),
          endpoint,
        );
      } else {
        setLocalPush((prev) =>
          applyPushChange(prev ?? defaultPushDevicePreferences(), {
            kind: 'master',
            value: false,
          }),
        );
      }
      return;
    }
    if (endpoint !== null && permissionGranted) {
      savePush(
        applyPushChange(currentPushState(), { kind: 'master', value: true }),
        endpoint,
      );
      return;
    }
    if (permissionGranted) {
      // Разрешение есть, подписки нет — тихая подписка без промпта.
      void runPermissionFlow({ kind: 'master', value: true });
      return;
    }
    setPendingChange({ kind: 'master', value: true });
    setSheetOpen(true);
  };

  const togglePushCategory = (
    category: NotificationSettingsCategory,
    value: boolean,
  ): void => {
    if (value && !permissionGranted) {
      setPendingChange({ kind: 'category', category, value: true });
      setSheetOpen(true);
      return;
    }
    if (value && endpoint === null) {
      void runPermissionFlow({ kind: 'category', category, value: true });
      return;
    }
    if (endpoint !== null) {
      savePush(
        applyPushChange(currentPushState(), { kind: 'category', category, value }),
        endpoint,
      );
    } else {
      setLocalPush((prev) =>
        applyPushChange(prev ?? defaultPushDevicePreferences(), {
          kind: 'category',
          category,
          value,
        }),
      );
    }
  };

  return (
    <div className="flex flex-col pb-6 pt-1">
      {display.visible && (
        <>
          <div className="flex items-center justify-between py-3">
            <span className="text-base text-content">Получать пуш-уведомления</span>
            {pushLoading ? (
              <Skeleton className="h-7 w-16 rounded-pill" />
            ) : (
              <Switch
                checked={display.masterOn}
                onCheckedChange={toggleMaster}
                aria-label="Получать пуш-уведомления на этом устройстве"
              />
            )}
          </div>
          {display.needsPermission && (
            <PushPermissionCard onAllow={() => void runPermissionFlow(null)} />
          )}
        </>
      )}

      {emailQuery.isError ? (
        <div className="mt-6 flex flex-col items-center gap-4 rounded-3xl bg-surface-muted px-6 py-8">
          <p className="m-0 text-sm text-content-secondary">
            Не удалось загрузить настройки уведомлений
          </p>
          <Button variant="white" onClick={() => void emailQuery.refetch()}>
            Повторить
          </Button>
        </div>
      ) : (
        NOTIFICATION_SETTINGS_CATEGORIES.map((category) => (
          <section key={category} className="mt-7">
            <h2 className="m-0 text-[26px] font-semibold leading-8 text-content">
              {notificationCategoryLabel(category)}
            </h2>
            <p className="mb-2 mt-1 text-sm leading-[18px] text-content-tertiary">
              {GROUP_DESCRIPTIONS[category]}
            </p>

            <div className="flex items-center justify-between py-3">
              <span className="text-base text-content">Электронная почта</span>
              {emailQuery.isPending ? (
                <Skeleton className="h-7 w-16 rounded-pill" />
              ) : (
                <Switch
                  checked={emailQuery.data[category]}
                  onCheckedChange={(value) => toggleEmail(category, value)}
                  disabled={updateEmail.isPending}
                  aria-label={`Электронная почта — ${notificationCategoryLabel(category)}`}
                />
              )}
            </div>

            {display.visible && (
              <div
                className={
                  display.categoriesInteractive
                    ? 'flex items-center justify-between py-3'
                    : 'flex items-center justify-between py-3 opacity-50'
                }
              >
                <span className="text-base text-content">Пуш-уведомления</span>
                {pushLoading ? (
                  <Skeleton className="h-7 w-16 rounded-pill" />
                ) : (
                  <Switch
                    checked={display.categories[category]}
                    onCheckedChange={(value) => togglePushCategory(category, value)}
                    disabled={!display.categoriesInteractive}
                    aria-label={`Пуш-уведомления — ${notificationCategoryLabel(category)}`}
                  />
                )}
              </div>
            )}
          </section>
        ))
      )}

      <NotificationPermissionSheet
        open={sheetOpen}
        pending={flowPending}
        onAllow={() => void runPermissionFlow(pendingChange)}
        onDecline={() => {
          setPendingChange(null);
          setSheetOpen(false);
        }}
      />
    </div>
  );
}
