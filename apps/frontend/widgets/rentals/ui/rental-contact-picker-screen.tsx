'use client';

import { useEffect, useRef, useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { useDebounce } from '@/shared/lib/hooks/useDebounce';
import { contactSortByRecent, ContactRowButton } from '@/entities/contact';
import { useContacts } from '@/features/contacts';
import { useRentalWizardDraft } from '@/features/rentals';
import {
  Button,
  IconButton,
  InfiniteQueryTail,
  PageContent,
  SearchField,
  TopNav,
  useTabBarSuppression,
} from '@/shared/ui/design';

/** Задержка дебаунса поиска (мс) — серверный фильтр по ?search=, как в
 * поиске книги контактов (#508). */
const SEARCH_DEBOUNCE_MS = 300;

/**
 * Экран выбора арендатора (#807, макет 1855:64129): отдельный маршрут
 * шага «Контакт арендатора» визарда создания аренды. Поисковая шапка
 * («←» возвращает на шаг, поле «Найти контакт» в фокусе), ниже — плоский
 * белый список книги объекта без групп: свежие контакты сверху
 * (created_at DESC — аннотация макета), подзаголовок — роль. Тап по строке
 * выбирает арендатора в черновик визарда (тот же хук черновика, что у
 * ветки создания #509) и возвращает на шаг goBack'ом. Поиск — серверный
 * ?search= с keepPreviousData (канон поиска книги). Экран — ветвь визарда:
 * футер глушится, как на его шагах.
 */
export function RentalContactPickerScreen({
  propertyId,
}: {
  readonly propertyId: string;
}): JSX.Element {
  const router = useRouter();
  // Футер глушится на ветви визарда (#807, P3): шаги визарда его глушат —
  // экран выбора не исключение.
  useTabBarSuppression();

  const [search, setSearch] = useState('');
  const searchInputRef = useRef<HTMLInputElement | null>(null);

  // Открытие экрана сразу делает поле активным (устоявшийся a11y-паттерн
  // поисковых страниц вместо autoFocus).
  useEffect(() => {
    searchInputRef.current?.focus();
  }, []);

  const debouncedSearch = useDebounce(search, SEARCH_DEBOUNCE_MS);
  // Сервер фильтр не нормализует — пробелы по краям срезаем клиентски.
  const trimmedSearch = debouncedSearch.trim();
  const contactsQuery = useContacts(propertyId, trimmedSearch);
  const rentalDraft = useRentalWizardDraft(propertyId);

  const contacts = contactsQuery.data ?? [];

  return (
    <>
      <TopNav
        variant="search"
        leading={
          <IconButton
            icon={<ArrowLeft />}
            label="Назад"
            onClick={() => goBack(router, ROUTES.propertyRentalNew(propertyId))}
          />
        }
      >
        <SearchField
          ref={searchInputRef}
          aria-label="Поиск контактов"
          placeholder="Найти контакт"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          onClear={() => setSearch('')}
        />
      </TopNav>

      <PageContent>
        {contactsQuery.isPending ? null : contactsQuery.isError ? (
          <div className="flex flex-col items-center gap-4 pt-16">
            <p className="text-center text-base leading-[18px] text-content-secondary">
              Не удалось загрузить контакты
            </p>
            <Button
              variant="secondary"
              size="small"
              onClick={() => void contactsQuery.refetch()}
            >
              Повторить
            </Button>
          </div>
        ) : contacts.length === 0 ? (
          <p className="px-6 pt-16 text-center text-base leading-[18px] text-content-secondary">
            Такого контакта нет
          </p>
        ) : (
          /* Плоский список (1855:64129): белые строки без групп, свежие
           * сверху, подзаголовок — роль; тап выбирает арендатора. */
          <div className="flex flex-col px-6">
            {contactSortByRecent(contacts).map((contact) => (
              <ContactRowButton
                key={contact.id}
                contact={contact}
                surface="white"
                onSelect={() => pick(contact.id)}
              />
            ))}
            <InfiniteQueryTail query={contactsQuery} />
          </div>
        )}
      </PageContent>
    </>
  );

  /** Выбор арендатора: id едет в общем черновике визарда (переживёт
   * goBack), запись истории экрана выбора выталкивается. */
  function pick(contactId: string): void {
    rentalDraft.setDraft((prev) => ({ ...prev, contactId }));
    goBack(router, ROUTES.propertyRentalNew(propertyId));
  }
}
