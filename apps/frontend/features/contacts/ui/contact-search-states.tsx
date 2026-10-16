import type { JSX } from 'react';
import { ErrorCard } from '@/shared/ui/design';

/**
 * Состояния поиска контактов — единый источник для поиска книги (#508) и
 * пикера арендатора визарда аренды (#807): карточка ошибки загрузки,
 * пустой результат поиска (1527:74825) и задержка дебаунса серверного
 * фильтра ?search=.
 */

/** Подсказка поиска и пустого результата — те же кегль и цвет (1527:74813/74825). */
const searchNoteClass = 'text-base leading-[18px] text-content-secondary';

/** Задержка дебаунса поиска (мс) — серверный фильтр по ?search=. */
export const CONTACTS_SEARCH_DEBOUNCE_MS = 300;

/** Карточка ошибки загрузки с повтором — канон design ErrorCard (#697). */
export function ContactsErrorCard({
  onRetry,
  className,
}: {
  readonly onRetry: () => void;
  readonly className?: string;
}): JSX.Element {
  return (
    <ErrorCard title="Не удалось загрузить контакты" onRetry={onRetry} className={className} />
  );
}

/** Поиск без совпадений (1527:74825): «Такого контакта нет», по центру. */
export function ContactsNoResults(): JSX.Element {
  return <p className={`${searchNoteClass} px-6 pt-16 text-center`}>Такого контакта нет</p>;
}
