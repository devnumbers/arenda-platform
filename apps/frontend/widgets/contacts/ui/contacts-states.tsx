import type { JSX } from 'react';
import Image from 'next/image';
import { Button } from '@/shared/ui/design';

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

export function ContactsSkeleton(): JSX.Element {
  return (
    <section className="mx-6 rounded-card bg-surface-muted px-6 py-6" aria-hidden>
      <div className="flex flex-col gap-4">
        <div className="h-11 animate-pulse rounded-pill bg-surface-muted-hover" />
        <div className="h-11 w-4/5 animate-pulse rounded-pill bg-surface-muted-hover" />
        <div className="h-11 w-3/5 animate-pulse rounded-pill bg-surface-muted-hover" />
      </div>
    </section>
  );
}

/** Карточка ошибки загрузки с повтором. */
export function ContactsErrorCard({ onRetry }: { readonly onRetry: () => void }): JSX.Element {
  return (
    <section className="mx-6 rounded-card bg-surface-muted px-6 py-6">
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

/** Пустой список (1527:74479): иллюстрация 128, «Контактов нет», пояснение. */
export function ContactsEmptyState(): JSX.Element {
  return (
    <div className="flex flex-col items-center gap-3 px-6 pt-16 text-center">
      <Image
        src="/images/contacts/empty-contacts.png"
        alt=""
        width={128}
        height={128}
        className="h-32 w-32 object-cover"
      />
      <h2 className={headingClass}>Контактов нет</h2>
      <p className={`${hintClass} max-w-[360px]`}>
        Добавьте контакты арендатора, мастеров и других специалистов
      </p>
    </div>
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
