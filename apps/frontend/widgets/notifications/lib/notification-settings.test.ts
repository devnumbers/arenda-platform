import { describe, expect, it } from 'vitest';
import type { PushDevicePreferences } from '@/features/push-notifications';
import {
  allCategoriesEnabled,
  NOTIFICATION_SETTINGS_CATEGORIES,
} from '@/entities/notification';
import {
  PUSH_SLOT_MESSAGES,
  applyCategoryChange,
  categoryEnableCategories,
  masterEnableCategories,
  resolvePushDisplay,
  slotStateHolds,
  verdictFromOutcome,
  type PushSlotReason,
} from './notification-settings';

const ALL_ON: PushDevicePreferences = {
  categories: { rental: true, payments_operations: true, tasks: true, shared_access: true },
};

const ALL_OFF_CATEGORIES: PushDevicePreferences['categories'] = {
  rental: false,
  payments_operations: false,
  tasks: false,
  shared_access: false,
};

describe('PUSH_SLOT_MESSAGES — тексты красного слота дословно из спеки #1028 §1', () => {
  it('три сообщения: unsupported, iOS-не-установлен, denied', () => {
    expect(PUSH_SLOT_MESSAGES.unsupported).toBe(
      'Ваш браузер не поддерживает push-уведомления.',
    );
    expect(PUSH_SLOT_MESSAGES['ios-needs-install']).toBe(
      'Пуш-уведомления работают только в установленном приложении. Добавьте Рентли на экран „Домой“ (меню „Поделиться“ → „На экран „Домой““) и включите пуши там.',
    );
    expect(PUSH_SLOT_MESSAGES.denied).toBe(
      'Разрешение на уведомления заблокировано в настройках браузера. Разрешите уведомления для этого сайта (значок замка в адресной строке → „Уведомления“ → „Разрешить“) и нажмите тумблер ещё раз.',
    );
  });

  it('слот замкнут по трём причинам — ключи совпадают с типом', () => {
    const reasons: readonly PushSlotReason[] = [
      'unsupported',
      'ios-needs-install',
      'denied',
    ];
    expect(Object.keys(PUSH_SLOT_MESSAGES).sort()).toStrictEqual(
      [...reasons].sort(),
    );
  });
});

describe('slotStateHolds — слот висит, пока заблокированное состояние держится', () => {
  const world = {
    supported: true,
    iosNeedsInstall: false,
    permissionDenied: false,
  };

  it('unsupported держится, только пока браузер не умеет пушить', () => {
    expect(slotStateHolds('unsupported', { ...world, supported: false })).toBe(true);
    expect(slotStateHolds('unsupported', world)).toBe(false);
  });

  it('iOS-не-установлен держится, пока PWA не установлен', () => {
    expect(
      slotStateHolds('ios-needs-install', { ...world, iosNeedsInstall: true }),
    ).toBe(true);
    expect(slotStateHolds('ios-needs-install', world)).toBe(false);
  });

  it('denied держится, пока разрешение заблокировано (оптин из Site Settings гасит слот)', () => {
    expect(
      slotStateHolds('denied', { ...world, permissionDenied: true }),
    ).toBe(true);
    expect(slotStateHolds('denied', world)).toBe(false);
  });
});

describe('resolvePushDisplay — матрица состояний (спека #1028 §1)', () => {
  it('№6 подписка есть: мастер ВКЛ, категории с сервера', () => {
    const display = resolvePushDisplay({
      endpoint: 'https://push.example/endpoint-1',
      server: { categories: { ...ALL_ON.categories, rental: false } },
    });
    expect(display.masterOn).toBe(true);
    expect(display.categories.rental).toBe(false);
    expect(display.categories.tasks).toBe(true);
  });

  it('подписка есть, GET ещё в воздухе — категории по умолчанию «всё включено»', () => {
    const display = resolvePushDisplay({
      endpoint: 'https://push.example/endpoint-1',
      server: undefined,
    });
    expect(display.masterOn).toBe(true);
    expect(display.categories).toStrictEqual(ALL_ON.categories);
  });

  it('№1–№5 подписки нет: мастера ВКЛ нет, категории статично ВЫКЛ — локальных желаний нет', () => {
    const display = resolvePushDisplay({ endpoint: null, server: undefined });
    expect(display.masterOn).toBe(false);
    expect(display.categories).toStrictEqual(ALL_OFF_CATEGORIES);
  });
});

describe('категории первого POST (спека #1028 §2)', () => {
  it('клик мастера — все категории ВКЛ (включение с чистого листа)', () => {
    expect(masterEnableCategories()).toStrictEqual(allCategoriesEnabled());
  });

  it('клик категории — одна кликнутая ВКЛ, остальные ВЫКЛ (вариант А: клик честен)', () => {
    for (const category of NOTIFICATION_SETTINGS_CATEGORIES) {
      const desired = categoryEnableCategories(category);
      expect(desired[category]).toBe(true);
      for (const other of NOTIFICATION_SETTINGS_CATEGORIES) {
        if (other !== category) expect(desired[other]).toBe(false);
      }
    }
  });
});

describe('applyCategoryChange — категорийный клик с подпиской', () => {
  it('двигает одну категорию, вход не мутирует', () => {
    const next = applyCategoryChange(ALL_ON, 'tasks', false);
    expect(next.categories.tasks).toBe(false);
    expect(next.categories.rental).toBe(true);
    expect(ALL_ON.categories.tasks).toBe(true);
  });
});

describe('verdictFromOutcome — исходы флоу включения (спека #1028 §2)', () => {
  const subscription = { endpoint: 'https://push.example/endpoint-9' } as PushSubscription;

  it('подписка создана — успех с endpoint и created', () => {
    expect(verdictFromOutcome({ outcome: 'subscribed', subscription })).toStrictEqual({
      kind: 'subscribe-success',
      endpoint: 'https://push.example/endpoint-9',
      created: true,
    });
  });

  it('подписка уже была живой — успех с created: false (PUT вместо тоста)', () => {
    expect(
      verdictFromOutcome({ outcome: 'already-subscribed', subscription }),
    ).toStrictEqual({
      kind: 'subscribe-success',
      endpoint: 'https://push.example/endpoint-9',
      created: false,
    });
  });

  it('отказ, iOS-не-установлен и unsupported — красный слот своей причиной', () => {
    expect(verdictFromOutcome({ outcome: 'denied' })).toStrictEqual({
      kind: 'blocked',
      reason: 'denied',
    });
    expect(verdictFromOutcome({ outcome: 'ios-needs-install' })).toStrictEqual({
      kind: 'blocked',
      reason: 'ios-needs-install',
    });
    expect(verdictFromOutcome({ outcome: 'unsupported' })).toStrictEqual({
      kind: 'blocked',
      reason: 'unsupported',
    });
  });

  it('ошибка флоу — свой вердикт (тост остаётся)', () => {
    expect(verdictFromOutcome({ outcome: 'error', reason: 'no-vapid-key' })).toStrictEqual({
      kind: 'flow-error',
    });
  });
});
