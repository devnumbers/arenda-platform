import type { JSX } from 'react';
import { Button } from '@/shared/ui/design';

const headingClass = 'text-xl font-semibold leading-6 text-content';
/** Пояснение состояния (14/16, серый) — как в секциях «Платежей объекта». */
const hintClass = 'text-sm leading-4 text-content-secondary';

/**
 * Состояния экрана «Контакты объекта» (#508) — та же серая карточка, что у
 * секций платежей: скелет загрузки, ошибка с действием, пустые состояния
 * (нет контактов / ничего не нашлось) — по центру колонки, без секций.
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

/** Карточка ошибки загрузки с повтором (образец PaymentsStateCard). */
export function ContactsErrorCard({
  onRetry,
}: {
  readonly onRetry: () => void;
}): JSX.Element {
  return (
    <section className="mx-6 rounded-card bg-surface-muted px-6 py-6">
      <h2 className={headingClass}>Не удалось загрузить контакты</h2>
      <p className={`${hintClass} mt-2`}>Проверьте подключение и попробуйте еще раз</p>
      <div className="mt-4">
        <Button size="small" variant="secondary" onClick={onRetry}>
          Повторить
        </Button>
      </div>
    </section>
  );
}

/** Пустое состояние по центру колонки: без иллюстраций — текстовые
 * заголовок и пояснение (иллюстраций контактов в канве нет). */
export function ContactsEmptyState({
  title,
  hint,
}: {
  readonly title: string;
  readonly hint: string;
}): JSX.Element {
  return (
    <div className="flex flex-col items-center gap-3 px-6 pt-16 text-center">
      <h2 className={headingClass}>{title}</h2>
      <p className={`${hintClass} max-w-[360px]`}>{hint}</p>
    </div>
  );
}
