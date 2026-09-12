'use client';

import { useEffect, useRef, useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { useDebounce } from '@/shared/lib/hooks/useDebounce';
import { useInfiniteScroll } from '@/shared/lib/hooks/useInfiniteScroll';
import { useContactBook } from '@/features/contacts';
import { IconButton, PageContent, SearchField, TopNav } from '@/shared/ui/design';
import { contactBookRowSubtitle } from '../lib/contact-book-model';
import { ContactRowButton } from '@/entities/contact';
import {
  ContactsErrorCard,
  ContactsLoadingMore,
  ContactsNoResults,
  ContactsSearchHint,
  ContactsSkeleton,
} from './contacts-states';

/** Задержка дебаунса поиска (мс) — серверный фильтр по ?search=. */
const SEARCH_DEBOUNCE_MS = 300;

/**
 * Поиск по плоской книге (глобальная страница контактов) — отдельная
 * страница с поисковой шапкой 1:1 как у книги объекта (#508; решение
 * владельца 2026-09-05): «←» в левом слоте закрывает поиск (возврат в
 * книгу), поле получает программный фокус, ввод ищет по серверному
 * ?search= всей видимой книги (дебаунс 300 мс). Состояния — конвенция
 * #508: пустое поле — подсказка, по чему ищем; без совпадений — «Такого
 * контакта нет»; результаты — плоские белые строки без групп, подзаголовок
 * «Роль (Объект)»; тап — карточка книги. Кнопки создания в поиске нет —
 * паритет с объектом.
 */
export function ContactBookSearchScreen(): JSX.Element {
  const router = useRouter();

  const [search, setSearch] = useState('');
  const searchInputRef = useRef<HTMLInputElement | null>(null);

  // Открытие поиска сразу делает поле активным (программный фокус —
  // устоявшийся a11y-паттерн вместо autoFocus, как на платёжных экранах).
  useEffect(() => {
    searchInputRef.current?.focus();
  }, []);

  const debouncedSearch = useDebounce(search, SEARCH_DEBOUNCE_MS);
  // Сервер фильтр не нормализует — пробелы по краям срезаем клиентски.
  const trimmedSearch = debouncedSearch.trim();
  const contactsQuery = useContactBook(trimmedSearch);

  const contacts = contactsQuery.data ?? [];
  const searching = trimmedSearch.length > 0;

  // Порции по 50 листаются sentinel-скроллом (#600) — смена поискового
  // запроса начинает свежий обход с пустого курсора.
  const sentinelRef = useInfiniteScroll(() => {
    if (contactsQuery.hasNextPage && !contactsQuery.isFetchingNextPage) {
      void contactsQuery.fetchNextPage();
    }
  }, contactsQuery.hasNextPage === true);

  return (
    <>
      <TopNav
        variant="search"
        leading={
          <IconButton
            icon={<ArrowLeft />}
            label="Закрыть поиск"
            onClick={() => goBack(router, ROUTES.contacts)}
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
        {contactsQuery.isPending && searching ? (
          <ContactsSkeleton />
        ) : contactsQuery.isError ? (
          <ContactsErrorCard onRetry={() => void contactsQuery.refetch()} />
        ) : !searching ? (
          <ContactsSearchHint />
        ) : contacts.length === 0 ? (
          <ContactsNoResults />
        ) : (
          /* Результаты (конвенция #508): белые строки на белом фоне,
           * без групп. */
          <div className="flex flex-col px-6">
            {contacts.map((contact) => (
              <ContactRowButton
                key={contact.id}
                contact={contact}
                surface="white"
                subtitle={contactBookRowSubtitle(contact)}
                onSelect={() => router.push(ROUTES.contact(contact.id))}
              />
            ))}
            {/* Хвост порций (#600): sentinel дозагрузки и индикатор
             * едущей следующей порции. */}
            {contactsQuery.hasNextPage === true && (
              <div ref={sentinelRef} aria-hidden />
            )}
            {contactsQuery.isFetchingNextPage && <ContactsLoadingMore />}
          </div>
        )}
      </PageContent>
    </>
  );
}
