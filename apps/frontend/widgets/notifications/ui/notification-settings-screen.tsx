'use client';

import { useState, type JSX } from 'react';
import { Loading } from '@/shared/assets/icons';
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
import { ROUTES } from '@/shared/config/routes';
import { notify } from '@/shared/lib/notifications';
import { cn } from '@/shared/lib/cn';
import { Button, Skeleton, SubScreenShell, Switch } from '@/shared/ui/design';
import {
  PUSH_SLOT_MESSAGES,
  anyFlightFreezes,
  applyCategoryChange,
  categoryEnableCategories,
  masterEnableCategories,
  resolvePushDisplay,
  slotStateHolds,
  verdictFromOutcome,
  type PushMutationFlight,
  type PushSlotReason,
  type PushToggleTarget,
} from '../lib/notification-settings';
import { NotificationSettingsContentSkeleton } from './notification-settings-skeleton';

/** Описания групп — с макета 1789-100250 (карта #734, #746). */
const GROUP_DESCRIPTIONS: Record<NotificationSettingsCategory, string> = {
  rental: 'Завершение срока аренды, напоминания о продлении или окончании аренды',
  payments_operations: 'Напоминания об оплате, просроченные платежи и подтверждения операций',
  tasks: 'Напоминания о задачах и уведомления, если срок выполнения прошел',
  shared_access: 'Приглашения в объект, изменение ваших прав или доступа',
};

/** Спиннер пуш-мутации в правом слоте шапки (макет 3467-80339, тикет
 * #1546): канонический глиф Loading (§10) в цвете primary, вращается ровно
 * пока полёт не снят любым исходом — включая время открытого нативного
 * окна разрешения. role="status" — канон loading-состояний (§7). */
function PushFlightSpinner(): JSX.Element {
  return (
    <span role="status" aria-label="Обновляем пуш-настройки" className="flex items-center">
      <Loading className="h-6 w-6 animate-spin text-primary" aria-hidden />
    </span>
  );
}

/**
 * Экран «Настроить уведомления» (карта #734, тикет #746): матрица 4 группы ×
 * (Электронная почта, Пуш-уведомления); Тариф и Системные всегда включены и
 * в экране отсутствуют. Email-матрица — на аккаунте
 * (GET/PUT /notification-preferences), пуш-колонка — на устройстве
 * (GET/PUT /push/subscriptions/preferences); оба PUT оптимистичны с откатом.
 * Каркас подэкрана собирает сам экран: спиннер пуш-мутации живёт в правом
 * слоте шапки (trailing), а полёт — в состоянии этого же компонента
 * (тикет #1546); до оседания пробы контент держит скелетон, шапка
 * скелетоном не подменяется (§7).
 *
 * Пуш-колонка по матрице состояний (спека #1028 §1, слайс 3 #1039): видна
 * всегда. Обратная связь «включить нельзя» (unsupported / iOS-не-установлен /
 * denied) — единый красный текст-слот под мастер-тумблером: только по клику,
 * без тостов, висит, пока заблокированное состояние держится. Мастерового
 * флага нет — строка подписки есть = включено; выключение — жёсткая отписка
 * pushManager.unsubscribe() + идемпотентный DELETE /push/subscriptions.
 * Включение без подписки — флоу разрешения: системное окно при `default`
 * (in-app шит снесён), тихая подписка при выданном; первый POST несёт
 * желаемое состояние — мастер все категории ВКЛ, категория одну кликнутую
 * ВКЛ.
 *
 * Фризинг на время пуш-мутации (тикет #1546, решения владельца 22.10):
 * флоу включения из мастера и мастер-выключение замораживают всю пуш-
 * колонку (мастер + 4 категорийных, тумблеры `Switch disabled`, подписи
 * строк гаснут), флоу из категорийного тумблера — только этот тумблер;
 * мутации в полёте нет — ничего не задизейблено. Email-колонка остаётся
 * кликабельной при любых пуш-мутациях. Полёт снимается любым исходом:
 * успех (тост/оптимистичный PUT), красный слот, ошибка (тост) — включая
 * already-subscribed и окно, закрытое без ответа (исход denied). Спиннер
 * в шапке виден ровно пока полёт держится.
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

  // Пуш-мутации в полёте: правят фризинг пуш-колонки (anyFlightFreezes)
  // и спиннер в шапке (тикет #1546). Полётов может быть несколько
  // (категорийный флоу висит на нативном окне — соседние тумблеры в это
  // время кликабельны по правилу фризинга), поэтому коллекция, а не флаг:
  // каждый полёт снимает только себя, фриз и спиннер держатся до последнего.
  const [flights, setFlights] = useState<readonly PushMutationFlight[]>([]);

  const startFlight = (flight: PushMutationFlight): void => {
    setFlights((prev) => [...prev, flight]);
  };
  const endFlight = (flight: PushMutationFlight): void => {
    setFlights((prev) => prev.filter((flying) => flying !== flight));
  };

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

  const frozen = (target: PushToggleTarget): boolean =>
    anyFlightFreezes(flights, target);

  // Пуш-состав (мастер, тумблеры) известен только после гидратации: до
  // оседания пробы экран целиком держит контентный скелетон (шелл — на
  // экране, §7 — шапка скелетоном не подменяется); поздняя вставка пуш-
  // состава между секциями сдвигала бы их (аудит #877, CLS 0.15).
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
   * сразу. Полёт держит фриз всей колонки от клика до оседания DELETE —
   * включая время отписки браузера; ошибка — тост (тикет #1546). */
  const turnPushOff = (probeEndpoint: string): void => {
    const flight: PushMutationFlight = { kind: 'disable-master' };
    void (async () => {
      startFlight(flight);
      try {
        const liveEndpoint = await unsubscribeBrowserSubscription();
        await deleteSubscription.mutateAsync(liveEndpoint ?? probeEndpoint);
      } catch (error) {
        notify.scenarios.profile.notificationPreferencesSaveError(error);
      } finally {
        endFlight(flight);
      }
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
   * перезвон пробы; блокировка — красный слот; ошибка — тост. Полёт
   * (origin) замораживает колонку или один тумблер и снимается любым
   * исходом — вердикт обязан прийти и окну, и таймауту (тикет #1546);
   * бросок мимо структурированных исходов (отказ serviceWorker вне
   * request-push) ловится тем же тостом ошибки. */
  const runEnableFlow = async (
    desired: PushDevicePreferences,
    origin: PushMutationFlight,
  ): Promise<void> => {
    startFlight(origin);
    try {
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
    } catch {
      notify.scenarios.profile.pushEnableError(
        new Error('push permission flow failed'),
      );
    } finally {
      endFlight(origin);
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
    void runEnableFlow(
      { categories: masterEnableCategories() },
      { kind: 'enable-flow-master' },
    );
  };

  const togglePushCategory = (
    category: NotificationSettingsCategory,
    value: boolean,
  ): void => {
    if (endpoint !== null) {
      // В подписке — PUT желаемого состояния целиком, оптимистично с
      // откатом (спека #1028 §2); фризинга не даёт — исход виден на самом
      // тумблере (тикет #1546).
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
    void runEnableFlow(
      { categories: categoryEnableCategories(category) },
      { kind: 'enable-flow-category', category },
    );
  };

  const masterFrozen = frozen({ kind: 'master' });

  return (
    <SubScreenShell
      title="Настроить уведомления"
      fallbackHref={ROUTES.profile}
      trailing={flights.length > 0 ? <PushFlightSpinner /> : undefined}
    >
      {!probeSettled ? (
        <NotificationSettingsContentSkeleton />
      ) : (
        <div className="flex flex-col pb-6 pt-1">
          <div className="flex items-center justify-between py-3">
            <span
              className={cn('text-base text-content', masterFrozen && 'opacity-50')}
            >
              Получать пуш-уведомления
            </span>
            {pushLoading ? (
              <Skeleton className="h-7 w-16 rounded-pill" />
            ) : (
              <Switch
                checked={display.masterOn}
                onCheckedChange={toggleMaster}
                disabled={masterFrozen}
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
            NOTIFICATION_SETTINGS_CATEGORIES.map((category) => {
              const categoryFrozen = frozen({ kind: 'category', category });
              return (
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
                        aria-label={`Электронная почта — ${notificationCategoryLabel(category)}`}
                      />
                    )}
                  </div>

                  <div className="flex items-center justify-between py-3">
                    <span
                      className={cn('text-base text-content', categoryFrozen && 'opacity-50')}
                    >
                      Пуш-уведомления
                    </span>
                    {pushLoading ? (
                      <Skeleton className="h-7 w-16 rounded-pill" />
                    ) : (
                      <Switch
                        checked={display.categories[category]}
                        onCheckedChange={(value) => togglePushCategory(category, value)}
                        disabled={categoryFrozen}
                        aria-label={`Пуш-уведомления — ${notificationCategoryLabel(category)}`}
                      />
                    )}
                  </div>
                </section>
              );
            })
          )}
        </div>
      )}
    </SubScreenShell>
  );
}
