/**
 * Модель экрана «Настроить уведомления» — матрица состояний и единый
 * красный слот (спека #1028 §1–§2, слайс 3 #1039; решения #738/#746 в
 * основе). Чистые функции — экран только связывает их с хуками; проверяются
 * модульными тестами рядом.
 *
 * Мастерового флага в модели нет (спека #1028 §0): строка подписки есть =
 * устройство включено, строки нет = выключено. Без подписки категории
 * рисуются статично ВЫКЛ — локальных желаний нет, каждый клик = одно
 * намерение, исполняемое немедленно флоу разрешения.
 */

import type {
  NotificationCategoryPreferences,
  NotificationSettingsCategory,
} from '@/entities/notification';
import {
  allCategoriesDisabled,
  allCategoriesEnabled,
} from '@/entities/notification';
import type { PushDevicePreferences } from '@/features/push-notifications';
import { defaultPushDevicePreferences } from '@/features/push-notifications';
import type { RequestPushPermissionOutcome } from '@/features/push-notifications';

/** Причина красного слота (спека #1028 §1): обратная связь «включить
 * нельзя» — unsupported / iOS-не-установлен / denied. Рендерится в
 * единственном месте под мастер-тумблером, только по клику, без тостов. */
export type PushSlotReason = 'unsupported' | 'ios-needs-install' | 'denied';

/** Тексты слота дословно из спеки #1028 §1 (утверждены владельцем 01.10). */
export const PUSH_SLOT_MESSAGES: Record<PushSlotReason, string> = {
  unsupported: 'Ваш браузер не поддерживает push-уведомления.',
  'ios-needs-install':
    'Пуш-уведомления работают только в установленном приложении. Добавьте Рентли на экран „Домой“ (меню „Поделиться“ → „На экран „Домой““) и включите пуши там.',
  denied:
    'Разрешение на уведомления заблокировано в настройках браузера. Разрешите уведомления для этого сайта (значок замка в адресной строке → „Уведомления“ → „Разрешить“) и нажмите тумблер ещё раз.',
};

/** Мир вокруг слота — три заблокированных состояния матрицы. */
export type PushSlotWorld = {
  readonly supported: boolean;
  readonly iosNeedsInstall: boolean;
  readonly permissionDenied: boolean;
};

/** Слот показывается, только пока его причина держится: поздний оптин из
 * Site Settings (denied → granted) гасит слот без перезагрузки — проба
 * разрешения живая через permissions.onchange (спека #1028 §6). После
 * перезагрузки слота нет — снова только по клику. */
export function slotStateHolds(
  reason: PushSlotReason,
  world: PushSlotWorld,
): boolean {
  switch (reason) {
    case 'unsupported':
      return !world.supported;
    case 'ios-needs-install':
      return world.iosNeedsInstall;
    case 'denied':
      return world.permissionDenied;
  }
}

/** Показ пуш-колонки: видна всегда (флаг «скрыть для unsupported» снесён —
 * спека #1028 §1), тумблеры без дизейблов. */
export type PushDisplay = {
  readonly masterOn: boolean;
  readonly categories: NotificationCategoryPreferences;
};

export type PushDisplayInput = {
  /** Endpoint живой подписки браузера; null — подписки нет (№1–№5). */
  readonly endpoint: string | null;
  /** Категории устройства с бэка; undefined — GET ещё в воздухе. */
  readonly server: PushDevicePreferences | undefined;
};

/** Единый источник состояния колонки: с подпиской — сервер (до ответа GET —
 * дефолт «всё включено»), без — статичное «всё выключено». */
export function resolvePushDisplay(input: PushDisplayInput): PushDisplay {
  return {
    masterOn: input.endpoint !== null,
    categories:
      input.endpoint !== null
        ? (input.server ?? defaultPushDevicePreferences()).categories
        : allCategoriesDisabled(),
  };
}

/** Первый POST клика мастера — все категории ВКЛ: включение с чистого
 * листа (спека #1028 §2; старые выборы стираются вместе со строкой — цена
 * варианта Б). */
export function masterEnableCategories(): NotificationCategoryPreferences {
  return allCategoriesEnabled();
}

/** Первый POST клика категории — одна кликнутая ВКЛ, остальные ВЫКЛ
 * (вариант А, спека #1028 §2: клик честен — просил «Совместный доступ»,
 * получаешь только его). */
export function categoryEnableCategories(
  category: NotificationSettingsCategory,
): NotificationCategoryPreferences {
  return {
    ...allCategoriesDisabled(),
    [category]: true,
  };
}

/** Категорийный клик при живой подписке — PUT желаемого состояния целиком,
 * вход не мутирует. */
export function applyCategoryChange(
  current: PushDevicePreferences,
  category: NotificationSettingsCategory,
  value: boolean,
): PushDevicePreferences {
  return {
    ...current,
    categories: { ...current.categories, [category]: value },
  };
}

/** Чем заканчивается флоу включения (клик тумблера → системное окно при
 * `default` / тихая подписка при granted → POST): успех несёт endpoint
 * подписки и флаг created — POST создал строку (тост успеха) или подписка
 * уже была живой (PUT желаемого состояния на её endpoint); блокировка —
 * причину красного слота (тексты в PUSH_SLOT_MESSAGES). */
export type PermissionFlowVerdict =
  | {
      readonly kind: 'subscribe-success';
      readonly endpoint: string;
      readonly created: boolean;
    }
  | { readonly kind: 'blocked'; readonly reason: PushSlotReason }
  | { readonly kind: 'flow-error' };

export function verdictFromOutcome(
  outcome: RequestPushPermissionOutcome,
): PermissionFlowVerdict {
  switch (outcome.outcome) {
    case 'subscribed':
      return {
        kind: 'subscribe-success',
        endpoint: outcome.subscription.endpoint,
        created: true,
      };
    case 'already-subscribed':
      return {
        kind: 'subscribe-success',
        endpoint: outcome.subscription.endpoint,
        created: false,
      };
    case 'denied':
      return { kind: 'blocked', reason: 'denied' };
    case 'ios-needs-install':
      return { kind: 'blocked', reason: 'ios-needs-install' };
    case 'unsupported':
      return { kind: 'blocked', reason: 'unsupported' };
    case 'error':
      return { kind: 'flow-error' };
  }
}
