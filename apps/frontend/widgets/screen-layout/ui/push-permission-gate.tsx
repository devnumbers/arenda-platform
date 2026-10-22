'use client';

import { type JSX } from 'react';
import { usePushStartup } from '@/features/push-notifications';

/**
 * Невидимый стартовый гейт пушей (маунтится в ScreenLayout, слайс 2 #1038,
 * спека #1028 §3–§4 в редакции аменды #1548). Всю работу делает
 * {@link usePushStartup}:
 *
 * 1. heal — браузерная подписка без строки в БД молча отписывается;
 * 2. авто-промпт варианта (c) — системное окно разрешения без жеста
 *    **при каждом заходе**, пока разрешение `default` (решение #1548,
 *    отмена «авто-промпт один раз»); grant → тихая подписка + POST + тост.
 *
 * Фонового ресабскрайба больше нет (ensureActiveSubscription снесён):
 * подписка возникает только из явного действия. Ожидаемые состояния
 * (unsupported, нет разрешения, сетевые сбои) молчаливы.
 */
export function PushPermissionGate(): JSX.Element | null {
  usePushStartup();
  return null;
}
