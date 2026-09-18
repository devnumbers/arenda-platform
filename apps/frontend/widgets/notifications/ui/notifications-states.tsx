import type { JSX } from 'react';
import { cn } from '@/shared/lib/cn';
import { Button, Skeleton } from '@/shared/ui/design';

/**
 * Состояния экрана «Уведомления» (#744): скелет ленты с паритетом групп
 * (заголовок + строки, §7), карточка ошибки с повторой. Пустые состояния —
 * канон EmptyState на самом экране (титулы зависят от фильтра).
 */

/** Ширины полей одной строки-заглушки — детерминированный цикл (§7,
 * никакого randomness — гидратация не расходится). */
type RowWidths = readonly [context: string, time: string, title: string, body: string];

/** Строка ленты-заглушка: анатомия NotificationRow (pl-6 pr-4 py-4,
 * круг 44, контекст+время / заголовок / описание). */
function NotificationRowSkeleton({ widths }: { readonly widths: RowWidths }): JSX.Element {
  return (
    <div className="flex items-start gap-4 py-4 pl-6 pr-4" aria-hidden>
      <Skeleton className="h-11 w-11 shrink-0 rounded-pill" />
      <div className="flex min-w-0 flex-1 flex-col gap-2">
        <div className="flex flex-col gap-1.5">
          <div className="flex items-start gap-2">
            <Skeleton className={cn('h-4', widths[0])} />
            <Skeleton className={cn('ml-auto h-4', widths[1])} />
          </div>
          <Skeleton className={cn('h-[18px]', widths[2])} />
        </div>
        <Skeleton className={cn('h-4', widths[3])} />
      </div>
    </div>
  );
}

const ROW_WIDTHS: ReadonlyArray<RowWidths> = [
  ['w-40', 'w-10', 'w-3/5', 'w-11/12'],
  ['w-44', 'w-10', 'w-2/3', 'w-10/12'],
  ['w-36', 'w-10', 'w-1/2', 'w-11/12'],
];

/** Группа ленты-заглушка: заголовок «Сегодня» (14/16, px-6 py-2.5) + строки. */
function FeedGroupSkeleton({ rows }: { readonly rows: number }): JSX.Element {
  return (
    <div aria-hidden>
      <div className="px-6 py-2.5">
        <Skeleton className="h-4 w-16" />
      </div>
      {ROW_WIDTHS.slice(0, rows).map((widths, index) => (
        <NotificationRowSkeleton key={index} widths={widths} />
      ))}
    </div>
  );
}

/** Скелет ленты: паритет финального лейаута — две группы (3 + 2 строки);
 * чип-ряд и кебаб рендерятся вне фазы загрузки (§7). */
export function NotificationsFeedSkeleton(): JSX.Element {
  return (
    <div aria-hidden>
      <FeedGroupSkeleton rows={3} />
      <FeedGroupSkeleton rows={2} />
    </div>
  );
}

/** Ошибка загрузки ленты/страницы уведомления: карточка с повторой (канон
 * состояния, §7). title переопределяется контекстом — 404 страницы
 * уведомления (#745) говорит «Уведомление не найдено». */
export function NotificationsErrorCard({
  onRetry,
  className,
  title = 'Не удалось загрузить уведомления',
}: {
  readonly onRetry: () => void;
  readonly className?: string;
  readonly title?: string;
}): JSX.Element {
  return (
    <section className={cn('mx-6 rounded-card bg-surface-muted px-6 py-6', className)}>
      <h2 className="text-xl font-semibold leading-6 text-content">{title}</h2>
      <p className="mt-2 text-sm leading-4 text-content-secondary">
        Проверьте подключение и попробуйте еще раз
      </p>
      <div className="mt-4">
        <Button size="small" variant="secondary" onClick={onRetry}>
          Повторить
        </Button>
      </div>
    </section>
  );
}

/** Скелет страницы уведомления (#745): паритет анатомии детали — иконка 96,
 * заголовок + тело, карточка сущности, секция категории-даты. */
export function NotificationDetailSkeleton(): JSX.Element {
  return (
    <div className="flex flex-col gap-8 px-8" aria-hidden>
      <Skeleton className="h-24 w-24 rounded-pill" />
      <div className="flex flex-col gap-3">
        <Skeleton className="h-8 w-3/4" />
        <Skeleton className="h-[18px] w-full" />
        <Skeleton className="h-[18px] w-10/12" />
      </div>
      <div className="flex items-center gap-3">
        <Skeleton className="h-11 w-11 shrink-0 rounded-pill" />
        <div className="flex flex-col gap-1">
          <Skeleton className="h-[18px] w-40" />
          <Skeleton className="h-4 w-28" />
        </div>
      </div>
      <div className="flex flex-col gap-2">
        <Skeleton className="h-[18px] w-24" />
        <Skeleton className="h-[18px] w-36" />
      </div>
    </div>
  );
}
