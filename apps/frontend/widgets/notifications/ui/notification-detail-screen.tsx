'use client';

import { useEffect, useState, type JSX } from 'react';
import Link from 'next/link';
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
import { propertyTypeIcons, type PropertyType } from '@/entities/property';
import {
  Button,
  CircleIcon,
  ConfirmDialog,
  IconButton,
  PageContent,
  SupportModal,
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

/** Кнопка «Поддержка» системных уведомлений (макет 2328:147773;
 * контрактом действий у категории нет): у страницы /support после #766
 * маршрута нет — поддержка открывается модалкой «Связаться с нами»,
 * как остальные триггеры хрома. */
function SystemSupportButton(): JSX.Element {
  const [supportOpen, setSupportOpen] = useState(false);

  return (
    <>
      <Button variant="secondary" className="w-full" onClick={() => setSupportOpen(true)}>
        Поддержка
      </Button>
      <SupportModal open={supportOpen} onOpenChange={setSupportOpen} />
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
        eventType={detail.eventType}
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
        // Карточка объекта — ссылка на объект (решение владельца 19.09.2026,
        // #745): имя и адрес — снимки payload, у снесённого объекта остаётся
        // снимок, ссылка уводит на 404 объектного экрана. Глиф — по типу из
        // снимка (карта #1217, #1244); payload free-form — словарь значения
        // проверяет реестр, неизвестное/старое (до #1244) — дом-фолбэк.
        <NotificationEntityLink
          href={ROUTES.property(detail.payload.property.id)}
          icon={
            <PropertyCardAvatar
              photo={detail.payload.property.photo}
              type={detail.payload.property.type}
            />
          }
          name={detail.payload.property.name}
          detail={detail.payload.property.address}
        />
      )}
      {detail.payload.actor && (
        <NotificationEntityCard
          icon={<ActorAvatar photo={detail.payload.actor.photo} />}
          name={detail.payload.actor.name}
          detail={detail.payload.actor.email}
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
        detail.category === 'system' && <SystemSupportButton />
      )}
    </div>
  );
}

/** Карточка сущности (Row Button макета 2316:143298): круг 44 на
 * surface-muted с серым bold-глифом (канонный серый иконок #D3D7D9),
 * имя-снимок 16/18 и серая строка карточки 14/16 (#6F787C) — адрес объекта
 * / email приглашающего, снимки payload (решение владельца 19.09.2026,
 * #745); строки, которых в снимке нет, не рендерятся. */
function NotificationEntityCardBody({
  icon,
  name,
  detail,
}: {
  readonly icon: JSX.Element;
  readonly name: string;
  readonly detail?: string;
}): JSX.Element {
  return (
    <>
      {/* overflow-hidden: фото-аватар (#1275) заполняет круг целиком, клип
       * нужен по скруглению; глифам он безразличен — они меньше круга. */}
      <CircleIcon variant="white" className="relative overflow-hidden">
        {icon}
      </CircleIcon>
      <div className="min-w-0 flex-1">
        <p className="truncate text-base font-medium leading-[18px] text-content">{name}</p>
        {detail && (
          <p className="mt-1 truncate text-sm leading-4 text-content-secondary">{detail}</p>
        )}
      </div>
    </>
  );
}

/** Аватар актёра на карточке payload-снимка (решение #1286): фото профиля
 * из снимка публикации; битое (фото удалили, доступ отозван — 404 стрима)
 * или отсутствующее — заглушка BoldUser, канон фолбэка #1275. */
function ActorAvatar({ photo }: { readonly photo?: string }): JSX.Element {
  const [photoBroken, setPhotoBroken] = useState(false);
  if (photo !== undefined && photo !== '' && !photoBroken) {
    return (
      <img
        src={photo}
        alt=""
        className="h-full w-full object-cover"
        onError={() => setPhotoBroken(true)}
      />
    );
  }
  return <BoldUser className="h-6 w-6 text-[#d3d7d9]" />;
}

/** Карточка приглашающего — не ссылка. */
function NotificationEntityCard({
  icon,
  name,
  detail,
}: {
  readonly icon: JSX.Element;
  readonly name: string;
  readonly detail?: string;
}): JSX.Element {
  return (
    <div className="flex min-h-[52px] items-center gap-3">
      <NotificationEntityCardBody icon={icon} name={name} detail={detail} />
    </div>
  );
}

/** Карточка объекта-ссылка: та же анатомия, вся карточка ведёт на объект
 * (решение владельца 19.09.2026, #745); ховер — как у строк ленты (#744). */
function NotificationEntityLink({
  href,
  icon,
  name,
  detail,
}: {
  readonly href: string;
  readonly icon: JSX.Element;
  readonly name: string;
  readonly detail?: string;
}): JSX.Element {
  return (
    <Link
      href={href}
      className="flex min-h-[52px] items-center gap-3 rounded-button outline-none transition-opacity hover:opacity-80 focus-visible:ring-4 focus-visible:ring-primary active:opacity-80"
    >
      <NotificationEntityCardBody icon={icon} name={name} detail={detail} />
    </Link>
  );
}

/** Глиф аватара объекта на карточке payload-снимка: по типу из снимка
 * (карта #1217, #1244). Payload путешествует free-form — словарь значения
 * проверяет реестр; снимки до #1244 и неизвестные значения рисует
 * дом-фолбэком. Выборка из статичного реестра, не вызов:
 * react-hooks/static-components. */
function PropertyGlyph({ type }: { readonly type: string | undefined }): JSX.Element {
  const Glyph = type !== undefined && type in propertyTypeIcons ? propertyTypeIcons[type as PropertyType] : BoldHome;
  return <Glyph className="h-6 w-6 text-[#d3d7d9]" />;
}

/** Аватар объекта на карточке payload-снимка (#1275): фото из снимка
 * заполняет круг; битое (фото или объект удалили после публикации — 404
 * стрима) откатывается к глифу типа. */
function PropertyCardAvatar({ photo, type }: { readonly photo?: string; readonly type?: string }): JSX.Element {
  const [photoBroken, setPhotoBroken] = useState(false);
  if (photo !== undefined && !photoBroken) {
    return (
      <img
        src={photo}
        alt=""
        className="h-full w-full object-cover"
        onError={() => setPhotoBroken(true)}
      />
    );
  }
  return <PropertyGlyph type={type} />;
}
