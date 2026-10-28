'use client';

import { useRef, useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { useDebounce } from '@/shared/lib/hooks/useDebounce';
import { contactSortByRecent, ContactRowButton } from '@/entities/contact';
import {
  CONTACTS_SEARCH_DEBOUNCE_MS,
  ContactsErrorCard,
  ContactsNoResults,
  useContacts,
} from '@/features/contacts';
import {
  setRentalTermsEditSessionContact,
  setRentalWizardSessionDraft,
} from '@/features/rentals';
import {
  IconButton,
  InfiniteQueryTail,
  PageContent,
  SearchField,
  SkeletonListRow,
  TopNav,
  useTabBarSuppression,
} from '@/shared/ui/design';

/**
 * Экран выбора арендатора (#807, макет 1855:64129): отдельный маршрут
 * шага «Контакт арендатора» визарда создания аренды. Поисковая шапка
 * («←» возвращает на шаг, поле «Найти контакт» в фокусе), ниже — плоский
 * белый список книги объекта без групп: свежие контакты сверху —
 * серверная ось sort=created (#847), подзаголовок — роль. Тап по строке
 * выбирает арендатора гостевой записью в носитель сессии визарда (та же
 * запись у ветки создания #509) и возвращает на шаг goBack'ом. Поиск —
 * серверный ?search= с keepPreviousData (канон поиска книги). Экран —
 * ветвь визарда: футер глушится, как на его шагах.
 */
export function RentalContactPickerScreen({
  propertyId,
  returnTo = 'wizard',
}: {
  readonly propertyId: string;
  /** Куда возвращать выбор (решение владельца 28.10.2026, #1623):
   * 'wizard' — шаг 4 визарда создания (носитель сессии визарда),
   * 'terms' — форма «Изменить условия» (?return=terms, носитель её
   * сессии). */
  readonly returnTo?: 'wizard' | 'terms';
}): JSX.Element {
  const router = useRouter();
  // Футер глушится на ветви визарда (#807, P3): шаги визарда его глушат —
  // экран выбора не исключение.
  useTabBarSuppression();

  const backFallback =
    returnTo === 'terms'
      ? ROUTES.propertyRentalTermsEdit(propertyId)
      : ROUTES.propertyRentalNew(propertyId);

  const [search, setSearch] = useState('');
  const searchInputRef = useRef<HTMLInputElement | null>(null);


  const debouncedSearch = useDebounce(search, CONTACTS_SEARCH_DEBOUNCE_MS);
  // Сервер фильтр не нормализует — пробелы по краям срезаем клиентски.
  const trimmedSearch = debouncedSearch.trim();
  // Свежие сверху — серверная ось sort=created + order=desc (#847, макет
  // 1855:64129): свежая карточка наверху при книге любой длины, а не только
  // в первой порции по имени. Клиентская contactSortByRecent ниже остаётся
  // деградацией для тёплого кэша.
  const contactsQuery = useContacts(propertyId, trimmedSearch, {
    sort: 'created',
    order: 'desc',
  });

  const contacts = contactsQuery.data ?? [];

  return (
    <>
      <TopNav
        variant="search"
        leading={
          <IconButton
            icon={<ArrowLeft />}
            label="Назад"
            onClick={() => goBack(router, backFallback)}
          />
        }
      >
        <SearchField
            focusOnMount
          ref={searchInputRef}
          aria-label="Поиск контактов"
          placeholder="Найти контакт"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          onClear={() => setSearch('')}
        />
      </TopNav>

      <PageContent>
        {(contactsQuery.isPending || (contactsQuery.isFetching && contacts.length === 0)) ? (
          /* Каркас списка строк на время первой порции (паритет скелетонов
           * #607; SearchField в шапке уже показывает активность). */
          <div aria-hidden className="flex flex-col px-6">
            {Array.from({ length: 4 }, (_, index) => (
              <SkeletonListRow key={index} className="py-2" />
            ))}
          </div>
        ) : contactsQuery.isError ? (
          <ContactsErrorCard onRetry={() => void contactsQuery.refetch()} loading={contactsQuery.isFetching} />
        ) : contacts.length === 0 ? (
          <ContactsNoResults />
        ) : (
          /* Плоский список (1855:64129): белые строки без групп, свежие
           * сверху (sort=created + order=desc, #847), подзаголовок — роль;
           * тап выбирает арендатора. */
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

  /** Выбор арендатора: id едет гостевой записью в носитель соответствующей
   * сессии (переживёт goBack), запись истории экрана выбора выталкивается. */
  function pick(contactId: string): void {
    if (returnTo === 'terms') {
      setRentalTermsEditSessionContact(propertyId, contactId);
      goBack(router, ROUTES.propertyRentalTermsEdit(propertyId));
      return;
    }
    setRentalWizardSessionDraft(propertyId, (prev) => ({ ...prev, contactId }));
    goBack(router, ROUTES.propertyRentalNew(propertyId));
  }
}
