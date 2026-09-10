import type { JSX } from 'react';
import { cn } from '@/shared/lib/cn';
import {
  Button,
  EmptyState,
  Skeleton,
  SkeletonListRow,
  skeletonBlockClass,
  skeletonRowWidths,
} from '@/shared/ui/design';

const headingClass = 'text-xl font-semibold leading-6 text-content';
/** Пояснение состояния (16/18, серый #6F787C — макет 1527:74479). */
const hintClass = 'text-base leading-[18px] text-content-secondary';
/** Подсказка поиска и пустого результата — те же кегль и цвет (1527:74813/74825). */
const searchNoteClass = 'text-base leading-[18px] text-content-secondary';

/**
 * Состояния экрана «Контакты объекта» (#508): скелет загрузки, ошибка с
 * действием, пустой список с иллюстрацией (1527:74479) и поисковые
 * подсказки (1527:74813/74825).
 */

export function ContactsSkeleton({ className }: { readonly className?: string }): JSX.Element {
  return (
    <section className={cn('mx-6 rounded-card bg-surface-muted px-6 py-6', className)} aria-hidden>
      <div className="flex flex-col gap-4">
        <Skeleton className="h-11 bg-surface-muted-hover" />
        <Skeleton className="h-11 w-4/5 bg-surface-muted-hover" />
        <Skeleton className="h-11 w-3/5 bg-surface-muted-hover" />
      </div>
    </section>
  );
}

/** Группа книги-заглушка: метка буквы/объекта (16/500, pl-2) и строки
 * контактов без своей вставки — поля приносит карточка (pl-5 pr-4). */
function ContactsBookGroupSkeleton({ rows }: { readonly rows: number }): JSX.Element {
  const widths = skeletonRowWidths(rows);
  return (
    <div className="flex flex-col" aria-hidden>
      <div className="pl-2">
        <Skeleton className={`h-6 w-16 ${skeletonBlockClass('muted')}`} />
      </div>
      <div className="flex flex-col">
        {widths.map((rowWidths, index) => (
          <SkeletonListRow
            key={index}
            tone="muted"
            widths={rowWidths}
            className="px-0 py-2"
          />
        ))}
      </div>
    </div>
  );
}

/**
 * Скелетон книги контактов (#605, паритет — §7 DESIGN.md): каркас карточки
 * книги (1726:65083 — одна серая карточка, группы с зазором 16) со
 * строками канона ContactRowButton (аватар 44, имя + подзаголовок).
 * Пилюля поиска и чип сортировки — вне фазы загрузки, скелетоном не
 * подменяются.
 */
export function ContactsBookSkeleton(): JSX.Element {
  return (
    <section
      aria-hidden
      className="mx-6 flex flex-col gap-4 rounded-card bg-surface-muted pb-3 pl-5 pr-4 pt-6"
    >
      <ContactsBookGroupSkeleton rows={2} />
      <ContactsBookGroupSkeleton rows={3} />
    </section>
  );
}

/** Карточка ошибки загрузки с повтором. */
export function ContactsErrorCard({
  onRetry,
  className,
}: {
  readonly onRetry: () => void;
  readonly className?: string;
}): JSX.Element {
  return (
    <section className={cn('mx-6 rounded-card bg-surface-muted px-6 py-6', className)}>
      <h2 className={headingClass}>Не удалось загрузить контакты</h2>
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

/** Пустой список (1527:74479): иллюстрация 128, «Контактов нет», пояснение —
 * на каноне EmptyState дизайн-слоя. */
export function ContactsEmptyState(): JSX.Element {
  return (
    <EmptyState
      imageSrc="/images/contacts/empty-contacts.png"
      title="Контактов нет"
      description="Добавьте контакты арендатора, мастеров и других специалистов"
    />
  );
}

/** Поиск открыт, поле пустое (1527:74813): подсказка, по чему ищем — текст
 * по центру (textStyle Mobile/Text/R/400, textAlignHorizontal CENTER). */
export function ContactsSearchHint(): JSX.Element {
  return (
    <p className={`${searchNoteClass} px-6 pt-16 text-center`}>
      Начните искать по имени, номеру телефона, электронной почте, имени пользователя или по роли
    </p>
  );
}

/** Поиск без совпадений (1527:74825): «Такого контакта нет», по центру. */
export function ContactsNoResults(): JSX.Element {
  return <p className={`${searchNoteClass} px-6 pt-16 text-center`}>Такого контакта нет</p>;
}

/** Правка недоступна (гейт ADR 0028, тексты — как у правки платежей):
 * смотрящий или архив. */
export function ContactsUnavailableCard({
  hint,
}: {
  readonly hint: string;
}): JSX.Element {
  return (
    <div className="flex flex-col items-center gap-3 px-6 pt-16 text-center">
      <h2 className={headingClass}>Правка недоступна</h2>
      <p className={`${hintClass} max-w-[360px]`}>{hint}</p>
    </div>
  );
}
