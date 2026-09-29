'use client';

import { useState, type JSX } from 'react';
import { usePathname, useRouter } from 'next/navigation';
import { Setting, TrashBin, VerticalMenu } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { notify } from '@/shared/lib/notifications';
import {
  groupNotificationsByDay,
  useDeleteAllNotifications,
  useMarkAllNotificationsRead,
  useNotificationsFeed,
  useUnreadNotificationsCount,
} from '@/features/notifications';
import type { Notification } from '@/entities/notification';
import {
  ChipButton,
  ConfirmDialog,
  EmptyState,
  IconButton,
  InfiniteQueryTail,
  Menu,
  MenuContent,
  MenuItem,
  MenuTrigger,
  PageContent,
  SuccessPopup,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import { NotificationRow } from './notification-row';
import { NotificationsErrorCard, NotificationsFeedSkeleton } from './notifications-states';
import { dateToIsoLocal } from '@/shared/lib/calendar';

/**
 * Экран «Уведомления» — центр уведомлений (карта #734, тикет #744, макеты
 * 2329-148674 / 2328-147401 / 2329-148700): хаб-анатомия профиля (#592) —
 * тайтл 16/18 в баре только на ПК (решение владельца 28.09 — ниже ПК он
 * упирался в лого-крыло, раздел подписывает вкладка TabBar); HubTitle в
 * контенте нет, хром с «крыльями» и на мобайле. Чип «Непрочитанные N» —
 * фильтр ленты (выбор в query строки
 * ?unread=1, канон страницы «Контакты»), «Прочитать все» видна при
 * непрочитанных (макет 2333-159051 — при нуле скрыта); кебаб — «Настроить
 * уведомления» и «Удалить все» (подтверждение 2329-152107, успех 2329-
 * 152324); «Прочитать все» подтверждения не требует — успех-попап «Все
 * уведомления прочитаны» (2329-148575). Кебаб заменяется шестерёнкой,
 * когда удалять нечего — лента пуста (2329-152324). Группировка
 * «Сегодня / Вчера / даты» — клиентский день смотрящего; у групп старше
 * вчера время в строках скрыто (решение владельца #744). Тап строки —
 * страница уведомления /notifications/{id} (#745), прочтение — на ней.
 */
export function NotificationsFeedScreen({
  initialUnreadOnly = false,
}: {
  readonly initialUnreadOnly?: boolean;
}): JSX.Element {
  const router = useRouter();
  const pathname = usePathname();

  const [unreadOnly, setUnreadOnly] = useState(initialUnreadOnly);
  const feedQuery = useNotificationsFeed(unreadOnly);
  const unreadCountQuery = useUnreadNotificationsCount();

  const markAll = useMarkAllNotificationsRead();
  const deleteAll = useDeleteAllNotifications();

  const [deleteOpen, setDeleteOpen] = useState(false);
  // Успех-попап — один на оба действия: текст сообщения и есть состояние.
  const [successMessage, setSuccessMessage] = useState<string | null>(null);

  const notifications = feedQuery.data ?? [];
  const unreadCount = unreadCountQuery.data ?? 0;

  // Смена фильтра синхронно переписывает query строки (дефолт не пишется —
  // конвенция «Состояние страницы в адресе», DESIGN.md §3).
  const toggleUnreadOnly = (): void => {
    const next = !unreadOnly;
    setUnreadOnly(next);
    router.replace(next ? `${pathname}?unread=1` : pathname, { scroll: false });
  };

  const markAllRead = (): void => {
    markAll.mutate(undefined, {
      onSuccess: () => setSuccessMessage('Все уведомления прочитаны'),
      onError: (error) => notify.scenarios.notifications.markAllError(error),
    });
  };

  const deleteEverything = (): void => {
    deleteAll.mutate(undefined, {
      onSuccess: () => {
        setDeleteOpen(false);
        setSuccessMessage('Все уведомления удалены');
      },
      onError: (error) => notify.scenarios.notifications.deleteAllError(error),
    });
  };

  const openNotification = (notification: Notification): void => {
    router.push(ROUTES.notification(notification.id));
  };

  // Пустая лента без фильтра — «Уведомлений нет» (макет 2329-152324):
  // служебный чип прячется вместе со списком, кебаб нечего касаться —
  // вместо него шестерёнка настроек. При фильтре чип остаётся (фильтр
  // можно выключить), кебаб — тоже (макет 2333-159051). В pending
  // чип-ряд и кебаб не рисуются: трейлинг зависит от данных (§7,
  // прецедент «Ваших участников» #697 — иначе иконка мелькает до
  // ответа), кадр совпадает с loading-архетипом без trailing.
  const feedEmpty = feedQuery.isSuccess && notifications.length === 0;
  const feedPending = feedQuery.isPending;
  const showEmptyAll = feedEmpty && !unreadOnly;
  const showEmptyUnread = feedEmpty && unreadOnly;
  const showToolbar = !showEmptyAll && !feedPending && !feedQuery.isError;

  const kebab = (
    <Menu>
      <MenuTrigger asChild>
        <IconButton icon={<VerticalMenu />} label="Действия с уведомлениями" />
      </MenuTrigger>
      <MenuContent>
        <MenuItem
          icon={<Setting />}
          onSelect={() => router.push(ROUTES.profileNotifications)}
        >
          Настроить уведомления
        </MenuItem>
        <MenuItem icon={<TrashBin />} onSelect={() => setDeleteOpen(true)}>
          Удалить все уведомления
        </MenuItem>
      </MenuContent>
    </Menu>
  );
  const trailing = showEmptyAll ? (
    <IconButton
      icon={<Setting />}
      label="Настроить уведомления"
      onClick={() => router.push(ROUTES.profileNotifications)}
    />
  ) : feedPending ? (
    undefined
  ) : (
    kebab
  );

  const today = dateToIsoLocal(new Date());
  const groups = groupNotificationsByDay(notifications, today);

  return (
    <>
      {/* Хаб-анатомия профиля (#592): компакт при скролле не подключается —
       * сворачивать нечего. Кебаб/шестерёнка — постоянный правый слот бара
       * (barTrailing, аудит #876): на ПК — правый край колонки 560, на
       * мобайле/планшете — левее крыла-аватара. Тайтл в баре — только на
       * ПК (макет 2329-148700): на мобайле/планшете он упирался в лого-
       * крыло (решение владельца 28.09 — тайтл из бара убрать), вкладка
       * TabBar подписывает раздел. */}
      <TopNav mobileWings barTrailing={trailing}>
        <TopNavTitle title="Уведомления" className="hidden desktop:flex" />
      </TopNav>

      <PageContent>
        {showToolbar && (
          <div className="mb-2 mt-3 flex items-center gap-1 px-6">
            <ChipButton selected={unreadOnly} onClick={toggleUnreadOnly}>
              Непрочитанные{unreadCount > 0 ? ` ${unreadCount}` : ''}
            </ChipButton>
            {unreadCount > 0 && (
              <button
                type="button"
                onClick={markAllRead}
                disabled={markAll.isPending}
                className="inline-flex h-11 shrink-0 cursor-pointer items-center justify-center rounded-pill px-5 text-sm font-medium text-content outline-none transition-colors hover:bg-surface-muted focus-visible:ring-4 focus-visible:ring-primary disabled:pointer-events-none disabled:opacity-50"
              >
                Прочитать все
              </button>
            )}
          </div>
        )}

        <div className="flex flex-col pb-6">
          {feedQuery.isPending ? (
            <NotificationsFeedSkeleton />
          ) : feedQuery.isError ? (
            <NotificationsErrorCard onRetry={() => void feedQuery.refetch()} />
          ) : showEmptyAll ? (
            <EmptyState
              imageSrc="/images/notifications/empty-bell.png"
              title="Уведомлений нет"
            />
          ) : showEmptyUnread ? (
            <EmptyState
              imageSrc="/images/notifications/empty-bell.png"
              title="Все уведомления прочитаны"
            />
          ) : (
            <>
              {groups.map((group) => (
                <section key={group.day}>
                  <h2 className="px-6 py-2.5 text-sm leading-4 text-content-secondary">
                    {group.label}
                  </h2>
                  {group.notifications.map((notification) => (
                    <NotificationRow
                      key={notification.id}
                      notification={notification}
                      showTime={group.showTime}
                      onOpen={openNotification}
                    />
                  ))}
                </section>
              ))}
              <InfiniteQueryTail query={feedQuery} />
            </>
          )}
        </div>
      </PageContent>

      <ConfirmDeleteAllDialog
        open={deleteOpen}
        pending={deleteAll.isPending}
        onOpenChange={setDeleteOpen}
        onConfirm={deleteEverything}
      />
      <SuccessPopup
        open={successMessage !== null}
        onOpenChange={(open) => {
          if (!open) {
            setSuccessMessage(null);
          }
        }}
        message={successMessage ?? ''}
      />
    </>
  );
}

/** Шит подтверждения «Удалить все уведомления?» (макет 2329-152107):
 * канон ConfirmDialog — «Отменить» слева, деструктивное «Да» справа. */
function ConfirmDeleteAllDialog({
  open,
  pending,
  onOpenChange,
  onConfirm,
}: {
  readonly open: boolean;
  readonly pending: boolean;
  readonly onOpenChange: (open: boolean) => void;
  readonly onConfirm: () => void;
}): JSX.Element {
  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      title="Удалить все уведомления?"
      confirmLabel="Да"
      cancelLabel="Отменить"
      confirmVariant="danger"
      pending={pending}
      onConfirm={onConfirm}
    />
  );
}
