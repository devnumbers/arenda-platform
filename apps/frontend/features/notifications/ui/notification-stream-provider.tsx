"use client";

import { useEffect, useRef, type JSX } from "react";
import { useQueryClient } from "@tanstack/react-query";
import {
  notificationHandlers,
  type ToastGate,
} from "@/features/notifications/api/notification-handlers";
import { connectNotificationStream } from "@/features/notifications/api/notification-stream";
import {
  notifyNotificationCreated,
  type ResolvePaymentCategoryIcon,
} from "./notification-toast";

/**
 * Живой слой уведомлений (#747): одно SSE-соединение на вкладку
 * (EventSource на /api/notifications/stream, cookie-сессия), кадры —
 * инвалидации react-query и тосты о новых уведомлениях. Кадровая логика
 * живёт в api/notification-handlers (симметрия realtime-близнеца), тост —
 * DOM-эффект, инъекция сюда. Тосты проходят через необязательный
 * тост-гейт (#790): без него показываются все — прежнее поведение. Бэк
 * рассчитан на соединение на вкладку (лимит хаба 8 на пользователя). Рвется
 * при unmount; 401 до старта стрима — fail-соединение, ручной бэкофф
 * соединения повторяет попытки, живой слой не мешает работе приложения.
 */
export function NotificationStreamProvider({
  shouldToast,
  resolvePaymentCategoryIcon,
}: {
  readonly shouldToast?: ToastGate;
  /** Резолв снимка категории платежа кадра в кругляш тоста (#1348):
   * инъекция из слоя выше — слайс не импортирует соседний
   * features/payment-categories. Нет резолвера — тост на стандартной
   * иконке (прежнее поведение). Должен быть СТАБИЛЬНОЙ ссылкой
   * (модульная функция, не инлайн) — входит в зависимости эффекта
   * соединения: новая ссылка переоткрывает SSE. */
  readonly resolvePaymentCategoryIcon?: ResolvePaymentCategoryIcon;
} = {}): JSX.Element | null {
  const queryClient = useQueryClient();
  // Гейт читается в момент кадра и обновляется эффектом: настройки
  // устройства прилетают асинхронно (проба подписки + GET), а соединение
  // при их смене не переоткрывается.
  const shouldToastRef = useRef<ToastGate | undefined>(undefined);
  useEffect(() => {
    shouldToastRef.current = shouldToast;
  }, [shouldToast]);

  useEffect(() => {
    const dispose = connectNotificationStream({
      createSource: () => new EventSource("/api/notifications/stream"),
      handlers: notificationHandlers({
        queryClient,
        // Кругляш категории решается в момент кадра из снимка (#1348);
        // нет снимка/резолвера — undefined, тост рисует стандартную.
        showToast: (frame) =>
          notifyNotificationCreated(
            frame,
            resolvePaymentCategoryIcon?.(frame.paymentCategory),
          ),
        toastGate: (category) => shouldToastRef.current?.(category) ?? true,
      }),
    });
    return dispose;
  }, [queryClient, resolvePaymentCategoryIcon]);

  return null;
}
