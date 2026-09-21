'use client';

import { type JSX } from 'react';
import { NotificationStreamProvider } from '@/features/notifications';
import {
  usePushDevicePreferences,
  usePushSubscriptionStatus,
} from '@/features/push-notifications';
import { toastAllowedByPush } from '../lib/toast-gate';

/**
 * Точка монтирования живого слоя уведомлений (#747) с тост-гейтом
 * push-настройки устройства (#790, решение #737): тост — живое отображение
 * событий, разрешённых push-настройкой категории; категория, выключенная
 * на устройстве (или весь канал мастер-тумблером), тоста не получает.
 * Лента и бейдж обновляются по тем же кадрам всегда — лента пишется
 * независимо от настроек (ADR 0058). Состояние устройства — подписка
 * браузера (endpoint из пробы usePushSubscriptionStatus) и её настройки
 * (GET по ключу endpoint); подписки нет — дефолт «всё включено» (решение
 * #738), тосты не глушатся.
 */
export function NotificationStreamGate(): JSX.Element | null {
  const { endpoint } = usePushSubscriptionStatus();
  const { data: preferences } = usePushDevicePreferences(endpoint ?? undefined);
  return (
    <NotificationStreamProvider
      shouldToast={(category) => toastAllowedByPush(preferences, category)}
    />
  );
}
