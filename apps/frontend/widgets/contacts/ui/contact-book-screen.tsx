'use client';

import { useState, type ComponentProps, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import {
  Add,
  SmallArrowDown,
  SortingBigSmall,
  SortingSmallBig,
} from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { useUrlParams } from '@/shared/lib/hooks/use-url-params';
import {
  useContactBook,
  type ContactBookOrder,
  type ContactBookSort,
} from '@/features/contacts';
import {
  ChipButton,
  HubCollapseAnchor,
  HubTitle,
  IconButton,
  InfiniteQueryTail,
  PageContent,
  PickerMenu,
  SearchPill,
  TopNav,
  type PickerMenuGroup,
} from '@/shared/ui/design';
import {
  CONTACT_BOOK_SORT_PARAMS,
  DEFAULT_CONTACT_BOOK_ORDER,
  DEFAULT_CONTACT_BOOK_SORT,
  contactBookRowSubtitle,
  groupBookByLetter,
  groupBookByProperty,
  serializeContactBookSortToParams,
} from '../lib/contact-book-model';
import { ContactRowButton } from '@/entities/contact';
import {
  ContactsBookSkeleton,
  ContactsEmptyState,
  ContactsErrorCard,
} from './contacts-states';

/**
 * Экран «Контакты» — плоская книга владельца (глобальная страница контактов,
 * макеты 1726:65083/65136/85937): хаб-шапка нового хрома (#565), заголовок
 * раздела 28, поисковая пилюля с «+» (создание контакта книги), чип
 * сортировки и одна серая карточка с группами. Сортировка — серверная:
 * поле «Имя/Объект» × «Возрастание/Убывание» (меню/шит «Сортировать»,
 * 1726:65136) уходит в ?sort/order GET /contacts; выбор живёт в query
 * строки (?sort=&order=, конвенция состояния в адресе) — переживает
 * перезагрузку и назад/вперёд. При сортировке по объекту группы — «Общие
 * контакты» (без объекта; сервер держит их первыми в обоих направлениях —
 * решение владельца 2026-09-04) и имена объектов, при сортировке по имени —
 * алфавитные. Подзаголовок строки — «Роль (Объект)» (1726:85937).
 *
 * Поиск — не здесь: пилюля — кнопка, тап открывает отдельную поисковую
 * страницу /contacts/search с поисковой шапкой 1:1 как у книги объекта
 * (#508; решение владельца 2026-09-05). «+» в пилюле ведёт на создание.
 * Пустая книга — EmptyState (служебный чип сортировки прячется вместе со
 * списком, DESIGN.md).
 */

export function ContactBookScreen({
  initialSort,
  initialOrder,
}: {
  readonly initialSort?: ContactBookSort;
  readonly initialOrder?: ContactBookOrder;
}): JSX.Element {
  const router = useRouter();
  const { write } = useUrlParams();

  const [sortField, setSortField] = useState<ContactBookSort>(
    initialSort ?? DEFAULT_CONTACT_BOOK_SORT,
  );
  const [sortOrder, setSortOrder] = useState<ContactBookOrder>(
    initialOrder ?? DEFAULT_CONTACT_BOOK_ORDER,
  );

  // Серверная сортировка книги: ключ и направление уходят в запрос —
  // без них данные всегда приходят в дефолтном порядке (name asc).
  const contactsQuery = useContactBook('', sortField, sortOrder);

  const contacts = contactsQuery.data ?? [];

  // Смена сортировки синхронно переписывает query строки — канон
  // useUrlParams (#786): экран владеет только sort/order, дефолтные
  // значения не пишутся (как на странице «Объекты»).
  const changeSort = (field: ContactBookSort, order: ContactBookOrder): void => {
    setSortField(field);
    setSortOrder(order);
    write(serializeContactBookSortToParams(field, order), { own: CONTACT_BOOK_SORT_PARAMS });
  };

  const groups =
    sortField === 'property' ? groupBookByProperty(contacts) : groupBookByLetter(contacts);

  return (
    <>
      {/* Хаб-шапка: «крылья» (лого + профиль) и на мобайле, поведение
       * стандартное — в потоке на мобайле, закреплена на планшете и ПК. */}
      <TopNav
        mobileWings
        collapse={{
          title: 'Контакты',
          search: { href: ROUTES.contactSearch, label: 'Найти контакт' },
        }}
      />

      <PageContent>
        <HubCollapseAnchor>
          <HubTitle>Контакты</HubTitle>
          <div className="mt-4 mb-6 px-6">
            {/* Пилюля видна всегда — в ней «+» создания (вид пустой книги
             * по макету 1726:65083 согласован владельцем); вне фазы
             * загрузки — контент встаёт на её место без сдвига (§7). */}
            <BookSearchPill
              onOpenSearch={() => router.push(ROUTES.contactSearch)}
              onCreate={() => router.push(ROUTES.contactNew)}
            />
          </div>
        </HubCollapseAnchor>

        {contactsQuery.isPending ? (
          <>
            {/* Паритет §7: чип сортировки реальный — вне фазы загрузки
             * (переключение сортировки во время загрузки безвредно: запрос
             * уходит с новым ключом); прячется вместе с пустым списком. */}
            <div className="mb-4 px-6">
              <PickerMenu
                title="Сортировать"
                groups={sortPickerGroups(sortField, sortOrder, changeSort)}
              >
                <BookSortChip field={sortField} order={sortOrder} />
              </PickerMenu>
            </div>
            <ContactsBookSkeleton />
          </>
        ) : contactsQuery.isError ? (
          <ContactsErrorCard onRetry={() => void contactsQuery.refetch()} />
        ) : (
          <>
            {contacts.length === 0 ? (
              <ContactsEmptyState />
            ) : (
              <>
                <div className="mb-4 px-6">
                  <PickerMenu
                    title="Сортировать"
                    groups={sortPickerGroups(sortField, sortOrder, changeSort)}
                  >
                    <BookSortChip field={sortField} order={sortOrder} />
                  </PickerMenu>
                </div>

                {/* Книга (1726:65083/85937): одна серая карточка, группы —
                 * буквы или объекты над своими строками. */}
                <section className="mx-6 flex flex-col gap-4 rounded-card bg-surface-muted pb-3 pl-5 pr-4 pt-6">
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
                  {/* Хвост порций (#633): sentinel + индикатор догрузки —
                   * серая карточка книги, тон muted. */}
                  <InfiniteQueryTail query={contactsQuery} tone="muted" />
                </section>
              </>
            )}
          </>
        )}
      </PageContent>
    </>
  );
}

/** Поисковая пилюля книги (макет 1726:65083, Search Button 1031:20955) —
 * адаптер канона SearchPill с подписью «Найти контакт»; тап открывает
 * поисковую страницу, «+» справа — создание контакта (кнопка хвоста глушит
 * всплытие клика, активация строки — паттерн useKeyboardActivation,
 * DESIGN.md §6). Экспорт для route-loading (#609). */
export function BookSearchPill({
  onOpenSearch,
  onCreate,
}: {
  readonly onOpenSearch: () => void;
  readonly onCreate: () => void;
}): JSX.Element {
  return (
    <SearchPill
      onOpenSearch={onOpenSearch}
      label="Найти контакт"
      className="pr-2"
      trailing={
        <IconButton
          icon={<Add />}
          label="Добавить контакт"
          onClick={(event) => {
            event.stopPropagation();
            onCreate();
          }}
        />
      }
    />
  );
}

/** Чип сортировки книги (макеты 1726:65083/85937): подпись — текущее поле
 * («Имя»/«Объект»), ведущая иконка — направление (возрастание —
 * SortingSmallBig 418:4608, убывание — SortingBigSmall 418:4607), хвостовая
 * стрелка всегда вниз (671:7320). Прокидывает все пропсы кнопки: триггер
 * PickerMenu через asChild передаёт ему свои обработчики и aria. Экспорт
 * для route-loading (#609). */
export function BookSortChip({
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
 * применяется сразу и синхронно переписывает query строки. */
function sortPickerGroups(
  field: ContactBookSort,
  order: ContactBookOrder,
  onChange: (field: ContactBookSort, order: ContactBookOrder) => void,
): ReadonlyArray<PickerMenuGroup> {
  return [
    {
      options: [
        { label: 'Имя', selected: field === 'name', onSelect: () => onChange('name', order) },
        {
          label: 'Объект',
          selected: field === 'property',
          onSelect: () => onChange('property', order),
        },
      ],
    },
    {
      options: [
        {
          label: 'Возрастание',
          selected: order === 'asc',
          onSelect: () => onChange(field, 'asc'),
        },
        {
          label: 'Убывание',
          selected: order === 'desc',
          onSelect: () => onChange(field, 'desc'),
        },
      ],
    },
  ];
}
