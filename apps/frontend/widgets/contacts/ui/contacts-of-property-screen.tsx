'use client';

import { useEffect, useRef, useState } from 'react';
import type { ComponentProps, JSX } from 'react';
import { useRouter } from 'next/navigation';
import {
  ArrowLeft,
  Search,
  SmallArrowDown,
  SortingBigSmall,
  SortingSmallBig,
} from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { useDebounce } from '@/shared/lib/hooks/useDebounce';
import {
  ContactRowButton,
  contactSortByName,
  groupContactsByLetter,
  type ContactSortOrder,
} from '@/entities/contact';
import { useContacts } from '@/features/contacts';
import { canMutateProperty, useProperty } from '@/features/properties';
import {
  Button,
  ChipButton,
  IconButton,
  InfiniteQueryTail,
  PageContent,
  PickerMenu,
  SearchField,
  StickyBottomBar,
  TopNav,
  TopNavTitle,
  type PickerMenuGroup,
} from '@/shared/ui/design';
import {
  ContactsEmptyState,
  ContactsErrorCard,
  ContactsNoResults,
  ContactsSearchHint,
  ContactsSkeleton,
} from './contacts-states';

/** Задержка дебаунса поиска (мс) — серверный фильтр по ?search=. */
const SEARCH_DEBOUNCE_MS = 300;

/**
 * Экран «Контакты объекта» (#508, переделан по макетам 1527:74479/74138/
 * 74139, 1539:85395, 1527:74813/74837/74825): шапка «Контакты объекта» с
 * лупой в правом слоте — по тапу шапка переключается в поисковый режим
 * (вариант search, конвенция платёжного поиска, fixme-спека #491), «назад»
 * в нём закрывает поиск. Список — серая карточка с алфавитными группами,
 * строка: имя + роль (телефона в строке нет). Чип «Имя»: на десктопе
 * открывает меню сортировки (механика меню задач, Figma 1603-94487 —
 * выбор применяет и закрывает), на мобильной ширине — шит А→Я / Я→А
 * (клиентская, макет 1539:85395; выбор применяет, шит остаётся открытым —
 * как в задачах). Пустой список — иллюстрация
 * «Контактов нет»; в поиске — подсказка по началу и «Такого контакта нет».
 * Полноширинная кнопка «Добавить контакт» — тому, кто может мутировать
 * (как у платежей: не смотрящий и не архив, #446); ведёт на создание —
 * #509 по порядку приёмки карты.
 */
export function ContactsOfPropertyScreen({
  propertyId,
}: {
  readonly propertyId: string;
}): JSX.Element {
  const router = useRouter();
  const propertyQuery = useProperty(propertyId);

  const [searchOpen, setSearchOpen] = useState(false);
  const [search, setSearch] = useState('');
  const [sortOrder, setSortOrder] = useState<ContactSortOrder>('asc');
  const searchInputRef = useRef<HTMLInputElement | null>(null);

  // Открытие поиска сразу делает поле активным (программный фокус —
  // устоявшийся a11y-паттерн вместо autoFocus, как на платёжных экранах).
  useEffect(() => {
    if (searchOpen) {
      searchInputRef.current?.focus();
    }
  }, [searchOpen]);

  const debouncedSearch = useDebounce(search, SEARCH_DEBOUNCE_MS);
  // Сервер фильтр не нормализует — пробелы по краям срезаем клиентски;
  // закрытый поиск не фильтрует вовсе.
  const trimmedSearch = debouncedSearch.trim();
  const contactsQuery = useContacts(propertyId, searchOpen ? trimmedSearch : '');

  const contacts = contactsQuery.data ?? [];
  const searching = trimmedSearch.length > 0;

  const sorted = contactSortByName(contacts, sortOrder);
  const groups = groupContactsByLetter(sorted);

  // Мутационный вход — тому, кому можно мутировать: смотрящий читает без
  // кнопок (как у платежей, история 47), архив read-only (#446). Пока
  // объект не загружен или не загрузился — без кнопки.
  const property = propertyQuery.isSuccess ? propertyQuery.data : undefined;
  // Мутационный вход — общий предикат (платежи #446/история 47, контакты #508).
  const canMutate = canMutateProperty(property);

  const closeSearch = (): void => {
    setSearchOpen(false);
    setSearch('');
  };

  return (
    <>
      {searchOpen ? (
        <TopNav
          variant="search"
          leading={
            <IconButton icon={<ArrowLeft />} label="Закрыть поиск" onClick={closeSearch} />
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
      ) : (
        <TopNav
          leading={
            <IconButton
              icon={<ArrowLeft />}
              label="Назад"
              onClick={() => goBack(router, ROUTES.property(propertyId))}
            />
          }
          trailing={<IconButton icon={<Search />} label="Поиск" onClick={() => setSearchOpen(true)} />}
        >
          <TopNavTitle title="Контакты объекта" />
        </TopNav>
      )}

      <PageContent>
        {contactsQuery.isPending ? (
          <ContactsSkeleton />
        ) : contactsQuery.isError ? (
          <ContactsErrorCard onRetry={() => void contactsQuery.refetch()} />
        ) : searchOpen && !searching ? (
          <ContactsSearchHint />
        ) : searching && contacts.length === 0 ? (
          <ContactsNoResults />
        ) : searchOpen ? (
          /* Результаты поиска (1527:74837): белые строки на белом фоне,
           * без алфавитных групп. */
          <div className="flex flex-col px-6">
            {sorted.map((contact) => (
              <ContactRowButton
                key={contact.id}
                contact={contact}
                surface="white"
                onSelect={() => router.push(ROUTES.propertyContact(propertyId, contact.id))}
              />
            ))}
            {/* Хвост порций (#633): sentinel + индикатор догрузки — белые
             * строки поиска. */}
            <InfiniteQueryTail query={contactsQuery} />
          </div>
        ) : contacts.length === 0 ? (
          <ContactsEmptyState />
        ) : (
          <>
            <div className="mx-6 mb-6">
              <PickerMenu title="Сортировать" groups={orderPickerGroups(sortOrder, setSortOrder)}>
                <ContactsSortChip order={sortOrder} />
              </PickerMenu>
            </div>
            {/* Книга (1527:74139): одна серая карточка с алфавитными
             * группами; буква — над своими строками. */}
            <section className="mx-6 flex flex-col gap-4 rounded-card bg-surface-muted pb-3 pl-5 pr-4 pt-6">
              {groups.map((group) => (
                <div key={group.letter} className="flex flex-col">
                  <span aria-hidden className="pl-2 text-base font-medium text-content-tertiary">
                    {group.letter}
                  </span>
                  <div className="flex flex-col">
                    {group.contacts.map((contact) => (
                      <ContactRowButton
                        key={contact.id}
                        contact={contact}
                        surface="muted"
                        onSelect={() => router.push(ROUTES.propertyContact(propertyId, contact.id))}
                      />
                    ))}
                  </div>
                </div>
              ))}
              {/* Хвост порций (#633): sentinel + индикатор догрузки —
               * серая карточка книги, тон muted. */}
              <InfiniteQueryTail query={contactsQuery} tone="muted" />
            </section>
          </>
        )}
      </PageContent>

      {canMutate && !searchOpen && (
        <StickyBottomBar>
          <Button className="w-full" onClick={() => router.push(ROUTES.propertyContactNew(propertyId))}>
            Добавить контакт
          </Button>
        </StickyBottomBar>
      )}
    </>
  );
}

/** Чип сортировки книги (макет 1527:74139, механика чипа задач
 * 1535:76222): «Имя»; ведущая иконка — направление (возрастание —
 * SortingSmallBig 418:4608, от меньшего к большему; убывание —
 * SortingBigSmall 418:4607, от большего к меньшему), хвостовая стрелка
 * всегда вниз (671:7320). Прокидывает все пропсы кнопки: триггер
 * PickerMenu через asChild передаёт ему свои обработчики и aria. */
function ContactsSortChip({
  order,
  ...props
}: {
  readonly order: ContactSortOrder;
} & ComponentProps<'button'>): JSX.Element {
  return (
    <ChipButton
      leadingIcon={order === 'asc' ? <SortingSmallBig /> : <SortingBigSmall />}
      trailingIcon={<SmallArrowDown />}
      {...props}
    >
      Имя
    </ChipButton>
  );
}

/** Группы опций пикера сортировки книги (макет 1539:85395): книга
 * сортируется только по имени (#508). */
function orderPickerGroups(
  order: ContactSortOrder,
  onOrderChange: (order: ContactSortOrder) => void,
): ReadonlyArray<PickerMenuGroup> {
  return [
    {
      options: [
        {
          label: 'Имя от А до Я',
          selected: order === 'asc',
          onSelect: () => onOrderChange('asc'),
        },
        {
          label: 'Имя от Я до А',
          selected: order === 'desc',
          onSelect: () => onOrderChange('desc'),
        },
      ],
    },
  ];
}
