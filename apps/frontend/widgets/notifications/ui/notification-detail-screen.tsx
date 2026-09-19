'use client';

import { useEffect, useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import {
  notificationCategoryLabel,
  NotificationCategoryIcon,
} from '@/entities/notification';
import type { NotificationDetail } from '@/entities/notification';
import {
  notificationActionView,
  useDeleteNotification,
  useMarkNotificationRead,
  useNotificationDetail,
} from '@/features/notifications';
import { BoldHome, BoldUser, Cancel, TrashBin } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { formatDayMonthTime } from '@/shared/lib/date-format';
import { goBack } from '@/shared/lib/navigation';
import { notify } from '@/shared/lib/notifications';
import {
  Button,
  ConfirmDialog,
  IconButton,
  PageContent,
  TopNav,
} from '@/shared/ui/design';
import { NotificationDetailSkeleton, NotificationsErrorCard } from './notifications-states';

/**
 * Страница уведомления (карта #734, тикет #745, макеты 2333-184048 /
 * 2316-143297 / 2333-184060 / 2333-184485): X-закрыть и корзина-удалить
 * в баре (подтверждение — канон ConfirmDialog, на мобайле нижний шит),
 * 3D-иконка категории 96, заголовок 28/32, тело, карточки сущностей из
 * payload-снимков (объект, у приглашения — приглашающий; адрес и email
 * контрактом #743 не приходят — карточка рендерит снимок имени), секция
 * «Категория + дата-время» (время остаётся всегда — решение владельца
 * 17.09.2026, чарт карты #734). Кнопки действий — живое состояние из
 * contract actions (вычислены при чтении): после выполнения действия
 * возврат перемонтирует экран,
 * ремаунт перечитывает — кнопки пропадают (аннотация макета). Прочтение —
 * обязанность фронта: GET /{id} не мутирует read (#743), при открытии
 * непрочитанного уходит POST read.
 */
export function NotificationDetailScreen({
  notificationId,
}: {
  readonly notificationId: string;
}): JSX.Element {
  const router = useRouter();
  const detailQuery = useNotificationDetail(notificationId);
  const { mutate: markRead } = useMarkNotificationRead();
  const { mutate: removeNotification, isPending: removePending } = useDeleteNotification();

  const [deleteOpen, setDeleteOpen] = useState(false);

  // GET не мутирует прочитанность (#743) — открытая непрочитанная страница
  // читается сама; идемпотентный POST, повтор не приходит: после
  // инвалидации read_at уже не null.
  useEffect(() => {
    if (detailQuery.data?.readAt === null) {
      markRead(notificationId, {
        onError: (error) => notify.scenarios.notifications.markReadError(error),
      });
    }
  }, [detailQuery.data, notificationId, markRead]);

  const close = (): void => goBack(router, ROUTES.notifications);

  const deleteNotification = (): void => {
    removeNotification(notificationId, {
      onSuccess: () => {
        setDeleteOpen(false);
        goBack(router, ROUTES.notifications);
      },
      onError: (error) => notify.scenarios.notifications.deleteError(error),
    });
  };

  const detail = detailQuery.data;

  return (
    <>
      <TopNav
        leading={
          <IconButton icon={<Cancel />} label="Закрыть" onClick={close} />
        }
        trailing={
          <IconButton
            icon={<TrashBin />}
            label="Удалить"
            onClick={() => setDeleteOpen(true)}
          />
        }
      />

      <PageContent>
        {detailQuery.isPending ? (
          <NotificationDetailSkeleton />
        ) : detailQuery.isError || !detail ? (
          <NotificationsErrorCard
            onRetry={() => void detailQuery.refetch()}
            title={
              detailQuery.error.status === 404
                ? 'Уведомление не найдено'
                : 'Не удалось загрузить уведомление'
            }
            className="mx-0"
          />
        ) : (
          <NotificationDetailBody detail={detail} onAction={(href) => router.push(href)} />
        )}
      </PageContent>

      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title="Удалить уведомление?"
        confirmLabel="Да"
        cancelLabel="Отменить"
        confirmVariant="danger"
        pending={removePending}
        onConfirm={deleteNotification}
      />
    </>
  );
}

/** Тело страницы (макет 2333:184048): колонка px-8 gap-8 — иконка 96,
 * заголовок 28/32 + тело 16/18, карточки сущностей, «Категория + дата»
 * 16/18 третичным серым, кнопки действий в потоке после секции. */
function NotificationDetailBody({
  detail,
  onAction,
}: {
  readonly detail: NotificationDetail;
  readonly onAction: (href: string) => void;
}): JSX.Element {
  const actions = detail.actions
    .map((action) => notificationActionView(action, detail.payload))
    .filter((view) => view !== null);
  const singleAction = actions.length === 1 ? actions[0] : undefined;

  return (
    <div className="flex flex-col gap-8 px-8">
      <NotificationCategoryIcon
        category={detail.category}
        unread={false}
        variant="page"
      />
      <div className="flex flex-col gap-3 [word-break:break-word]">
        <h1 className="text-balance text-[28px] font-semibold leading-8 text-content">
          {detail.title}
        </h1>
        <p className="text-balance text-base leading-[18px] text-content">{detail.body}</p>
      </div>
      {detail.payload.property && (
        <NotificationEntityCard
          icon={<BoldHome className="h-6 w-6 text-white" />}
          name={detail.payload.property.name}
        />
      )}
      {detail.payload.actor && (
        <NotificationEntityCard
          icon={<BoldUser className="h-6 w-6 text-white" />}
          name={detail.payload.actor.name}
        />
      )}
      <div className="flex flex-col gap-2 text-base leading-[18px] text-content-tertiary">
        <p>{notificationCategoryLabel(detail.category)}</p>
        <p>{formatDayMonthTime(detail.createdAt)}</p>
      </div>
      {singleAction ? (
        <Button
          variant={singleAction.variant}
          className="w-full"
          onClick={() => onAction(singleAction.href)}
        >
          {singleAction.label}
        </Button>
      ) : actions.length > 1 ? (
        <div className="grid grid-cols-2 gap-2">
          {actions.map((view) => (
            <Button
              key={`${view.label}:${view.href}`}
              variant={view.variant}
              onClick={() => onAction(view.href)}
            >
              {view.label}
            </Button>
          ))}
        </div>
      ) : (
        detail.category === 'system' && (
          // Системные: статическая навигация в Поддержку (макет
          // 2328:147773); контрактом действий у категории нет.
          <Button
            variant="secondary"
            className="w-full"
            onClick={() => onAction(ROUTES.support)}
          >
            Поддержка
          </Button>
        )
      )}
    </div>
  );
}

/** Карточка сущности (Row Button макета 2333:184048): круг 44 на
 * surface-muted с bold-глифом, имя-снимок 16/18. Адрес объекта и email
 * приглашающего контрактом не приходят — рендерится снимок имени. */
function NotificationEntityCard({
  icon,
  name,
}: {
  readonly icon: JSX.Element;
  readonly name: string;
}): JSX.Element {
  return (
    <div className="flex min-h-[52px] items-center gap-3">
      <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-pill bg-surface-muted ring-[2.5px] ring-white">
        {icon}
      </div>
      <p className="min-w-0 flex-1 truncate text-base font-medium leading-[18px] text-content">
        {name}
      </p>
    </div>
  );
}
