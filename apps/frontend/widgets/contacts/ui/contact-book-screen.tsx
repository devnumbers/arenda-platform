'use client';

import { useState, type ComponentProps, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import {
  Add,
  Cancel,
  Search,
  SmallArrowDown,
  SortingBigSmall,
  SortingSmallBig,
} from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { useDebounce } from '@/shared/lib/hooks/useDebounce';
import {
  useContactBook,
  type ContactBookOrder,
  type ContactBookSort,
} from '@/features/contacts';
import {
  ChipButton,
  IconButton,
  PageContent,
  PickerMenu,
  type PickerMenuGroup,
} from '@/shared/ui/design';
import { PageHeader } from '@/shared/ui/page-header';
import {
  contactBookRowSubtitle,
  groupBookByLetter,
  groupBookByProperty,
} from '../lib/contact-book-model';
import { ContactRowButton } from './contact-row-button';
import {
  ContactsEmptyState,
  ContactsErrorCard,
  ContactsNoResults,
  ContactsSkeleton,
} from './contacts-states';

/** Задержка дебаунса поиска (мс) — серверный фильтр по ?search=. */
const SEARCH_DEBOUNCE_MS = 300;

/**
 * Экран «Контакты» — плоская книга владельца (глобальная страница контактов,
 * макеты 1726:65083/65136/85937): заголовок кабинета, поисковая пилюля с
 * «+» (создание контакта книги), чип сортировки и одна серая карточка с
 * группами. Сортировка — серверная: поле «Имя/Объект» × «Возрастание/
 * Убывание» (меню/шит «Сортировать», 1726:65136); при сортировке по объекту
 * группы — «Общие контакты» (без объекта; сервер держит их первыми в обоих
 * направлениях) и имена объектов, при сортировке по имени — алфавитные.
 * Подзаголовок строки — «Роль (Объект)» (1726:85937). Список без пагинации:
 * поиск — серверный подстрочный фильтр (дебаунс), результаты — плоские
 * строки без групп (конвенция книги объекта #508); пустой результат —
 * «Такого контакта нет», пустая книга — EmptyState (служебный чип сортировки
 * прячется вместе со списком, DESIGN.md).
 */
export function ContactBookScreen(): JSX.Element {
  const router = useRouter();

  const [search, setSearch] = useState('');
  const [sortField, setSortField] = useState<ContactBookSort>('name');
  const [sortOrder, setSortOrder] = useState<ContactBookOrder>('asc');

  const debouncedSearch = useDebounce(search, SEARCH_DEBOUNCE_MS);
  // Сервер фильтр не нормализует — пробелы по краям срезаем клиентски.
  const trimmedSearch = debouncedSearch.trim();
  const contactsQuery = useContactBook(trimmedSearch, sortField, sortOrder);

  const contacts = contactsQuery.data ?? [];
  const searching = trimmedSearch.length > 0;

  const groups =
    sortField === 'property' ? groupBookByProperty(contacts) : groupBookByLetter(contacts);

  return (
    <>
      <PageHeader title="Контакты" />

      <PageContent>
        {contactsQuery.isPending ? (
          /* Отступы книги — единые с карточкой списка (решение владельца
           * 2026-09-04: боковой отступ как у основного контента). */
          <ContactsSkeleton className="mx-0" />
        ) : contactsQuery.isError ? (
          <ContactsErrorCard onRetry={() => void contactsQuery.refetch()} className="mx-0" />
        ) : (
          <>
            <div className="mb-6">
              <BookSearchPill
                value={search}
                onChange={setSearch}
                onCreate={() => router.push(ROUTES.contactNew)}
              />
            </div>

            {searching && contacts.length === 0 ? (
              <ContactsNoResults />
            ) : contacts.length === 0 ? (
              <ContactsEmptyState />
            ) : (
              <>
                <div className="mb-4">
                  <PickerMenu
                    title="Сортировать"
                    groups={sortPickerGroups(sortField, sortOrder, setSortField, setSortOrder)}
                  >
                    <BookSortChip field={sortField} order={sortOrder} />
                  </PickerMenu>
                </div>

                {searching ? (
                  /* Результаты поиска: белые строки на белом фоне, без
                   * групп (конвенция результатов #508); края — как у
                   * карточки книги. */
                  <div className="flex flex-col">
                    {contacts.map((contact) => (
                      <ContactRowButton
                        key={contact.id}
                        contact={contact}
                        surface="white"
                        subtitle={contactBookRowSubtitle(contact)}
                        onSelect={() => router.push(ROUTES.contact(contact.id))}
                      />
                    ))}
                  </div>
                ) : (
                  /* Книга (1726:65083/85937): одна серая карточка, группы —
                   * буквы или объекты над своими строками. */
                  <section className="flex flex-col gap-4 rounded-card bg-surface-muted pb-3 pl-5 pr-4 pt-6">
                    {groups.map((group) => (
                      <div key={group.label} className="flex flex-col">
                        <span aria-hidden className="pl-2 text-base font-medium text-content-tertiary">
                          {group.label}
                        </span>
                        <div className="flex flex-col">
                          {group.contacts.map((contact) => (
                            <ContactRowButton
                              key={contact.id}
                              contact={contact}
                              surface="muted"
                              subtitle={contactBookRowSubtitle(contact)}
                              onSelect={() => router.push(ROUTES.contact(contact.id))}
                            />
                          ))}
                        </div>
                      </div>
                    ))}
                  </section>
                )}
              </>
            )}
          </>
        )}
      </PageContent>
    </>
  );
}

/**
 * Поисковая пилюля книги (макет 1726:65083, Search Button 1031:20955):
 * серая пилюля 56px, лупа слева, плейсхолдер «Найти контакт»; правый слот —
 * «+» (создание контакта книги), при непустом запросе — крестик (очистить
 * поиск).
 */
function BookSearchPill({
  value,
  onChange,
  onCreate,
}: {
  readonly value: string;
  readonly onChange: (value: string) => void;
  readonly onCreate: () => void;
}): JSX.Element {
  const hasValue = value.length > 0;

  return (
    <div className="flex h-14 w-full items-center rounded-pill bg-surface-muted pl-[18px] pr-2">
      <Search className="h-6 w-6 shrink-0 text-content" aria-hidden />
      <input
        type="search"
        className="min-w-0 flex-1 border-none bg-transparent px-2 text-base font-medium text-content outline-none placeholder:text-content [&::-webkit-search-cancel-button]:hidden"
        placeholder="Найти контакт"
        aria-label="Поиск контактов"
        value={value}
        onChange={(event) => onChange(event.target.value)}
      />
      {hasValue ? (
        <IconButton icon={<Cancel />} label="Очистить поиск" onClick={() => onChange('')} />
      ) : (
        <IconButton icon={<Add />} label="Добавить контакт" onClick={onCreate} />
      )}
    </div>
  );
}

/** Чип сортировки книги (макеты 1726:65083/85937): подпись — текущее поле
 * («Имя»/«Объект»), ведущая иконка — направление (возрастание —
 * SortingSmallBig 418:4608, убывание — SortingBigSmall 418:4607), хвостовая
 * стрелка всегда вниз (671:7320). Прокидывает все пропсы кнопки: триггер
 * PickerMenu через asChild передаёт ему свои обработчики и aria. */
function BookSortChip({
  field,
  order,
  ...props
}: {
  readonly field: ContactBookSort;
  readonly order: ContactBookOrder;
} & ComponentProps<'button'>): JSX.Element {
  return (
    <ChipButton
      leadingIcon={order === 'asc' ? <SortingSmallBig /> : <SortingBigSmall />}
      trailingIcon={<SmallArrowDown />}
      {...props}
    >
      {field === 'name' ? 'Имя' : 'Объект'}
    </ChipButton>
  );
}

/** Группы опций пикера сортировки книги (макет 1726:65136): поле
 * («Имя»/«Объект») и направление («Возрастание»/«Убывание») — выбор
 * применяется сразу. */
function sortPickerGroups(
  field: ContactBookSort,
  order: ContactBookOrder,
  onFieldChange: (field: ContactBookSort) => void,
  onOrderChange: (order: ContactBookOrder) => void,
): ReadonlyArray<PickerMenuGroup> {
  return [
    {
      options: [
        { label: 'Имя', selected: field === 'name', onSelect: () => onFieldChange('name') },
        {
          label: 'Объект',
          selected: field === 'property',
          onSelect: () => onFieldChange('property'),
        },
      ],
    },
    {
      options: [
        {
          label: 'Возрастание',
          selected: order === 'asc',
          onSelect: () => onOrderChange('asc'),
        },
        {
          label: 'Убывание',
          selected: order === 'desc',
          onSelect: () => onOrderChange('desc'),
        },
      ],
    },
  ];
}
