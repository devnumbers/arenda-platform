'use client';

import { useState, type JSX } from 'react';
import {
  NOTIFICATION_SETTINGS_CATEGORIES,
  notificationCategoryLabel,
  type NotificationSettingsCategory,
} from '@/entities/notification';
import {
  defaultPushDevicePreferences,
  requiresInstallOnIos,
  useDeletePushSubscription,
  usePushDevicePreferences,
  usePushSubscriptionStatus,
  useSubscribePush,
  useUpdatePushDevicePreferences,
  unsubscribeBrowserSubscription,
  type PushDevicePreferences,
} from '@/features/push-notifications';
import {
  useEmailNotificationPreferences,
  useUpdateEmailPreferences,
} from '@/features/notifications';
import { notify } from '@/shared/lib/notifications';
import { Button, Skeleton, Switch } from '@/shared/ui/design';
import {
  PUSH_SLOT_MESSAGES,
  applyCategoryChange,
  categoryEnableCategories,
  masterEnableCategories,
  resolvePushDisplay,
  slotStateHolds,
  verdictFromOutcome,
  type PushSlotReason,
} from '../lib/notification-settings';
import { NotificationSettingsContentSkeleton } from './notification-settings-skeleton';

/** Описания групп — с макета 1789-100250 (карта #734, #746). */
const GROUP_DESCRIPTIONS: Record<NotificationSettingsCategory, string> = {
  rental: 'Завершение срока аренды, напоминания о продлении или окончании аренды',
  payments_operations: 'Напоминания об оплате, просроченные платежи и подтверждения операций',
  tasks: 'Напоминания о задачах и уведомления, если срок выполнения прошел',
  shared_access: 'Приглашения в объект, изменение ваших прав или доступа',
};

/**
 * Экран «Настроить уведомления» (карта #734, тикет #746): матрица 4 группы ×
 * (Электронная почта, Пуш-уведомления); Тариф и Системные всегда включены и
 * в экране отсутствуют. Email-матрица — на аккаунте
 * (GET/PUT /notification-preferences), пуш-колонка — на устройстве
 * (GET/PUT /push/subscriptions/preferences); оба PUT оптимистичны с откатом.
 *
 * Пуш-колонка по матрице состояний (спека #1028 §1, слайс 3 #1039): видна
 * всегда, тумблеры всегда кликабельны — дизейблов нет. Обратная связь
 * «включить нельзя» (unsupported / iOS-не-установлен / denied) — единый
 * красный текст-слот под мастер-тумблером: только по клику, без тостов,
 * висит, пока заблокированное состояние держится. Мастерового флага нет —
 * строка подписки есть = включено; выключение — жёсткая отписка
 * pushManager.unsubscribe() + идемпотентный DELETE /push/subscriptions.
 * Включение без подписки — флоу разрешения: системное окно при `default`
 * (in-app шит снесён), тихая подписка при выданном; первый POST несёт
 * желаемое состояние — мастер все категории ВКЛ, категория одну кликнутую
 * ВКЛ.
 */
export function NotificationSettingsScreen(): JSX.Element {
  const { refresh, ...pushStatus } = usePushSubscriptionStatus();
  const probeSettled = !pushStatus.isPending;
  const endpoint = pushStatus.endpoint;

  const pushPrefsQuery = usePushDevicePreferences(endpoint ?? undefined);
  const emailQuery = useEmailNotificationPreferences();

  const updatePush = useUpdatePushDevicePreferences();
  const deleteSubscription = useDeletePushSubscription();
  const updateEmail = useUpdateEmailPreferences();
  const { subscribe } = useSubscribePush();

  // Красный слот: причина последнего клика, попавшего в заблокированное
  // состояние. Показ — пока состояние держится (slotStateHolds): поздний
  // оптин из Site Settings гасит слот без перезагрузки (проба разрешения
  // живая, спека #1028 §6); после перезагрузки слота нет — снова по клику.
  const [slotReason, setSlotReason] = useState<PushSlotReason | null>(null);

  const display = resolvePushDisplay({ endpoint, server: pushPrefsQuery.data });
  const shownSlot =
    slotReason !== null &&
    slotStateHolds(slotReason, {
      supported: !pushStatus.isUnsupported,
      iosNeedsInstall: requiresInstallOnIos(),
      permissionDenied: pushStatus.permissionDenied,
    })
      ? slotReason
      : null;

  // Пуш-состав (мастер, тумблеры) известен только после гидратации: до
  // оседания пробы экран целиком держит контентный скелетон (шелл — на
  // странице, §7 — шапка скелетоном не подменяется); поздняя вставка пуш-
  // состава между секциями сдвигала бы их (аудит #877, CLS 0.15).
  if (!probeSettled) {
    return <NotificationSettingsContentSkeleton />;
  }
  // Пуш-тумблеры рисуем скелетоном, пока неясно состояние (GET устройства
  // при живой подписке).
  const pushLoading = endpoint !== null && pushPrefsQuery.isPending;

  const savePush = (next: PushDevicePreferences, saveEndpoint: string): void => {
    updatePush.mutate(
      { endpoint: saveEndpoint, ...next },
      {
        onError: (error) =>
          notify.scenarios.profile.notificationPreferencesSaveError(error),
      },
    );
  };

  /** Мастер-выключение (№6 → №5): жёсткая отписка — браузерная подписка +
   * строка БД (спека #1028 §2). DELETE идемпотентный (204 и без строки);
   * пробу подписки и кэш настроек обновляет мутация — UI видит «выключено»
   * сразу. */
  const turnPushOff = (probeEndpoint: string): void => {
    void (async () => {
      const liveEndpoint = await unsubscribeBrowserSubscription();
      deleteSubscription.mutate(liveEndpoint ?? probeEndpoint, {
        onError: (error) =>
          notify.scenarios.profile.notificationPreferencesSaveError(error),
      });
    })();
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

  /** Флоу включения (спека #1028 §2): системное окно при `default`, тихая
   * подписка при выданном разрешении, POST желаемого состояния. Созданная
   * подписка — тост «Пуши включены» (POST уже посадил endpoint в пробу);
   * уже-живая подписка браузера — PUT желаемого состояния на её endpoint и
   * перезвон пробы; блокировка — красный слот; ошибка — тост. */
  const runEnableFlow = async (desired: PushDevicePreferences): Promise<void> => {
    const outcome = await subscribe(desired.categories);
    const verdict = verdictFromOutcome(outcome);

    switch (verdict.kind) {
      case 'subscribe-success': {
        if (verdict.created) {
          notify.scenarios.profile.pushEnabled();
        } else {
          savePush(desired, verdict.endpoint);
          refresh();
        }
        break;
      }
      case 'blocked':
        // Окно, закрытое без ответа (разрешение осталось `default`), слот
        // не рисует: slotStateHolds держит denied-слот только при настоящем
        // denied — по спеке §2 «иной исход/таймаут» остаётся в №3.
        setSlotReason(verdict.reason);
        break;
      case 'flow-error':
        notify.scenarios.profile.pushEnableError(
          new Error('push permission flow failed'),
        );
        break;
    }
  };

  const toggleMaster = (value: boolean): void => {
    if (!value) {
      // Выключение = жёсткая отписка (строки нет = выключено); без подписки
      // выключать нечего — и так выключено.
      if (endpoint !== null) turnPushOff(endpoint);
      return;
    }
    // Строка есть = уже включено; включение без подписки — флоу разрешения.
    // Категории первого POST — все ВКЛ: включение мастера с чистого листа
    // (спека #1028 §2).
    if (endpoint !== null) return;
    void runEnableFlow({ categories: masterEnableCategories() });
  };

  const togglePushCategory = (
    category: NotificationSettingsCategory,
    value: boolean,
  ): void => {
    if (endpoint !== null) {
      // В подписке — PUT желаемого состояния целиком, оптимистично с
      // откатом (спека #1028 §2).
      savePush(
        applyCategoryChange(
          pushPrefsQuery.data ?? defaultPushDevicePreferences(),
          category,
          value,
        ),
        endpoint,
      );
      return;
    }
    // Без подписки категории статично ВЫКЛ — выключать нечего; включение
    // исполняет тот же флоу, POST несёт одну кликнутую категорию ВКЛ
    // (вариант А: клик честен).
    if (!value) return;
    void runEnableFlow({ categories: categoryEnableCategories(category) });
  };

  return (
    <div className="flex flex-col pb-6 pt-1">
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
      {shownSlot !== null && (
        <p role="alert" className="m-0 text-sm leading-[18px] text-error">
          {PUSH_SLOT_MESSAGES[shownSlot]}
        </p>
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
          <section key={category} className="mt-6">
            <h2 className="m-0 text-2xl font-semibold leading-8 text-content">
              {notificationCategoryLabel(category)}
            </h2>
            <p className="mb-3 mt-2 text-base leading-[18px] text-content-secondary">
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

            <div className="flex items-center justify-between py-3">
              <span className="text-base text-content">Пуш-уведомления</span>
              {pushLoading ? (
                <Skeleton className="h-7 w-16 rounded-pill" />
              ) : (
                <Switch
                  checked={display.categories[category]}
                  onCheckedChange={(value) => togglePushCategory(category, value)}
                  aria-label={`Пуш-уведомления — ${notificationCategoryLabel(category)}`}
                />
              )}
            </div>
          </section>
        ))
      )}
    </div>
  );
}
