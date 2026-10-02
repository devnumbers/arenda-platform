/**
 * Модель экрана «Настроить уведомления» (#746, карта #734, решение #738;
 * контракт переписан спекой #1028 — слайс 2 #1038): источник состояния
 * пуш-колонки и вердикты флоу «Разрешите пуши». Чистые функции — экран
 * только связывает их с хуками; проверяются модульными тестами рядом.
 *
 * Мастерового флага в модели больше нет (спека #1028 §0): строка подписки
 * есть = устройство включено, строки нет = выключено. Экран показывает
 * мастер по факту живой подписки (endpoint из пробы), а переключение
 * мастера — это действия подписки/отписки, не правка данных: POST создаёт
 * строку, unsubscribe + DELETE сносит.
 */

import type { NotificationCategoryPreferences, NotificationSettingsCategory } from '@/entities/notification';
import { allCategoriesEnabled } from '@/entities/notification';
import type { PushDevicePreferences } from '@/features/push-notifications';
import { defaultPushDevicePreferences } from '@/features/push-notifications';
import type { RequestPushPermissionOutcome } from '@/features/push-notifications';

/** Желаемое изменение пуш-канала. Мастер в тело PUT не выражается: его клик
 * исполняется подпиской/отпиской, а изменение остаётся в типе ради
 * ожидающего разрешения флоу (шит держит намерение до исхода окна). */
export type PushChange =
  | { readonly kind: 'master'; readonly value: boolean }
  | {
      readonly kind: 'category';
      readonly category: NotificationSettingsCategory;
      readonly value: boolean;
    };

/** Применяет изменение к состоянию устройства, не мутируя вход. Мастер
 * возвращает состояние без изменений — телом PUT он не выражается. */
export function applyPushChange(
  current: PushDevicePreferences,
  change: PushChange,
): PushDevicePreferences {
  if (change.kind === 'master') {
    return current;
  }
  return {
    ...current,
    categories: { ...current.categories, [change.category]: change.value },
  };
}

/** Показ пуш-колонки экрана настроек (макеты 1789-100250 / 2329-150165 /
 * 2333-180696). */
export type PushDisplay = {
  /** Браузер не умеет пушить — мастер и категорийные тумблеры не
   * рендерятся, экран показывает только email-колонку. */
  readonly visible: boolean;
  readonly masterOn: boolean;
  readonly categories: NotificationCategoryPreferences;
  /** Карточка «Разрешите пуши» под мастером (2329-150165): подписки нет и
   * разрешение не выдано — включению нужен флоу разрешения. */
  readonly needsPermission: boolean;
};

export type PushDisplayInput = {
  /** Проба браузера (поддержка/разрешение/подписка) осела. */
  readonly probeSettled: boolean;
  readonly supported: boolean;
  readonly permissionGranted: boolean;
  /** Endpoint живой подписки браузера; null — подписки нет (мастер ВЫКЛ). */
  readonly endpoint: string | null;
  /** Категории устройства с бэка; undefined — GET ещё в воздухе. */
  readonly server: PushDevicePreferences | undefined;
  /** Локальные категории без подписки; null — дефолт «всё включено»
   * (решение #738). Хранит выключатели, сделанные до первой подписки, —
   * флоу разрешения сохранит их первым PUT. */
  readonly local: PushDevicePreferences | null;
};

/** Единый источник категорий пуш-колонки: с подпиской — сервер (до ответа
 * GET — дефолт), без — локальное состояние с тем же дефолтом. */
export function resolvePushState(
  endpoint: string | null,
  server: PushDevicePreferences | undefined,
  local: PushDevicePreferences | null,
): PushDevicePreferences {
  return endpoint !== null
    ? (server ?? defaultPushDevicePreferences())
    : (local ?? defaultPushDevicePreferences());
}

export function resolvePushDisplay(input: PushDisplayInput): PushDisplay {
  if (!input.supported) {
    return {
      visible: false,
      masterOn: false,
      categories: allCategoriesEnabled(),
      needsPermission: false,
    };
  }

  const state = resolvePushState(input.endpoint, input.server, input.local);

  // Мастер — факт живой подписки (строка есть = включено, спека #1028 §0):
  // выключенные категории до подписки мастер не включают.
  return {
    visible: true,
    masterOn: input.endpoint !== null,
    categories: state.categories,
    needsPermission:
      input.endpoint === null &&
      input.probeSettled &&
      !input.permissionGranted,
  };
}

/** Чем заканчивается флоу «Разрешите пуши» (sheet/card →
 * requestPermission → подписка): success несёт endpoint свежей подписки —
 * на него уходит первый PUT желаемого состояния. */
export type PermissionFlowVerdict =
  | { readonly kind: 'subscribe-success'; readonly endpoint: string }
  | { readonly kind: 'denied' }
  | { readonly kind: 'ios-needs-install' }
  | { readonly kind: 'flow-error' }
  | { readonly kind: 'noop' };

export function verdictFromOutcome(
  outcome: RequestPushPermissionOutcome,
): PermissionFlowVerdict {
  switch (outcome.outcome) {
    case 'subscribed':
    case 'already-subscribed':
      return { kind: 'subscribe-success', endpoint: outcome.subscription.endpoint };
    case 'denied':
      return { kind: 'denied' };
    case 'ios-needs-install':
      return { kind: 'ios-needs-install' };
    case 'unsupported':
      return { kind: 'noop' };
    case 'error':
      return { kind: 'flow-error' };
  }
}
