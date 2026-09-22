'use client';

import {type JSX, useState} from 'react';
import {useRouter} from 'next/navigation';
import {
  useProperties,
  usePropertiesWithMeta,
  type SuspendedSharedProperty,
} from '@/features/properties';
import {useSubscription} from '@/features/subscription';
import {Add, Archive, Search, SmallArrowDown, SortingBigSmall, SortingSmallBig} from '@/shared/assets/icons';
import {useKeyboardActivation} from '@/shared/lib/hooks/useKeyboardActivation';
import {useUrlParams} from '@/shared/lib/hooks/use-url-params';
import {
  Button,
  HubCollapseAnchor,
  HubTitle,
  IconButton,
  PageContent,
  PickerMenu,
  type PickerMenuGroup,
  TopNav,
} from '@/shared/ui/design';
import {ROUTES} from '@/shared/config/routes';
import {
  DEFAULT_PROPERTY_SORT,
  PROPERTY_SORT_PARAMS,
  SORT_FIELD_CHIP_LABEL,
  SORT_FIELD_OPTIONS,
  serializeSortToParams,
  sortProperties,
  type PropertySort,
  type PropertySortDirection,
  type PropertySortField,
} from '../lib/property-sort';
import {PropertyCard} from './PropertyCard';
import {SuspendedPropertyCard} from './SuspendedPropertyCard';
import {SuspendedReasonSheet} from './SuspendedReasonSheet';
import {PropertiesEmptyState} from './PropertiesEmptyState';
import {PropertiesLoading} from './PropertiesLoading';
import {PropertiesErrorState} from './PropertiesErrorState';
import styles from './PropertiesPage.module.css';

export type PropertiesPageProps = {
  readonly initialSort?: PropertySort;
};

/**
 * Экран-хаб «Объекты» (карта #583, тикет #586; Figma 1603:89079 — ПК,
 * 1603:88972 — планшет, 1590:88756 — мобайл, 1603:90604 — пустое): заголовок
 * хаба, поисковая пилюля с «+» создания, ряд сортировки (чип PickerMenu +
 * ссылка «Архив» → экран архива #587; пустой список прячет ряд — DESIGN.md
 * §7), список карточек, блюр-карточки подвесших общих объектов (#702),
 * «+ Создать объект» под списком. Компакт-бар — канон хаба: лупа на поиск,
 * заголовок, «+».
 * Сортировки — резолюция #584 (4 поля × возрастание/убывание, основной
 * всегда первый), персистентность в URL (?sort=&order=). Фильтров по
 * типу/статусу в хабе нет; поиск объектов — серверный на отдельной
 * странице (#601), вход — пилюля. Архивные объекты живут на отдельном
 * экране /properties/archive (#587); бэк /properties возвращает только
 * активные (сервис #585).
 */
export function PropertiesPage({initialSort}: PropertiesPageProps): JSX.Element {
  const {data, isLoading, isFetching, isError, refetch} = useProperties();
  // Та же запись кэша /properties, что и у useProperties (один ключ — один
  // запрос): подвесшие общие объекты получателя (suspended_shared —
  // блюр-карточки suspendedList ниже) и «сегодня владельца» (ADR 0048) для
  // бейджей аренды.
  const metaQuery = usePropertiesWithMeta();
  const subscriptionQuery = useSubscription();
  const router = useRouter();
  const {write} = useUrlParams();

  const [sort, setSort] = useState<PropertySort>(initialSort ?? DEFAULT_PROPERTY_SORT);
  const [reasonTarget, setReasonTarget] = useState<SuspendedSharedProperty | null>(null);

  const visible = sortProperties(data ?? [], sort);
  const suspendedShared = metaQuery.data?.suspendedShared ?? [];
  const showSuspended = !isLoading && !isError && suspendedShared.length > 0;

  // Запись — канон useUrlParams (#786): экран владеет только sort/order,
  // чужие параметры адреса переживают смену сортировки, дефолт снимается.
  const changeSort = (next: PropertySort) => {
    setSort(next);
    write(serializeSortToParams(next), {own: PROPERTY_SORT_PARAMS});
  };

  const isEmpty = !isLoading && !isError && visible.length === 0;
  // Служебный ряд (сортировка + «Архив») и список живут только вместе (§7).
  const showControls = !isLoading && !isError && !isEmpty;

  // 404 подписки хук отдаёт null (#768) — это не pending и не ошибка:
  // canAdd=false ниже ведёт на смену тарифа, кнопки не висят в disabled.
  const isActionLoading = subscriptionQuery.isPending || data === undefined;

  const canAdd = (() => {
    if (!subscriptionQuery.data || data === undefined) return false;
    const limit = subscriptionQuery.data.tariff.activePropertyLimit;
    if (limit < 0) return true;
    return data.length < limit;
  })();

  const openCreate = () => {
    if (isActionLoading) return;
    router.push(canAdd ? ROUTES.propertyNew : ROUTES.profileTariffChange);
  };

  const createLabel = canAdd
    ? 'Создать объект'
    : 'Достигнут лимит объектов по тарифу — сменить тариф';

  // «+» создания: в хаб-шапке нового макета его нет — в хвосте поисковой
  // пилюли (1590:88756) и в правом слоте компакт-бара (канон сворачивания).
  const createButton = (
    <IconButton icon={<Add/>} label={createLabel} disabled={isActionLoading} onClick={openCreate}/>
  );

  const content = (
    <div className="flex flex-col gap-4 px-6 pt-6">
      <PropertiesSearchPill onCreate={openCreate} createLabel={createLabel} createDisabled={isActionLoading}/>

      {isLoading && <PropertiesLoading/>}

      {!isLoading && isError && <PropertiesErrorState onRetry={() => void refetch()} isLoading={isFetching}/>}

      {!isLoading && !isError && isEmpty && (
        <PropertiesEmptyState canAdd={canAdd} isLoading={isActionLoading} onAdd={openCreate}/>
      )}

      {showControls && (
        <>
          <PropertiesSortRow sort={sort} onChange={changeSort}/>
          <ul className={styles.list} data-testid="properties-list">
            {visible.map((property) => (
              <li key={property.id}>
                <PropertyCard
                  property={property}
                  today={metaQuery.data?.today}
                />
              </li>
            ))}
          </ul>
        </>
      )}

      {/* Подвесшие общие объекты (#702): блюр-карточки вместо сноски
       * hidden_shared_count — после обычных карточек, в серверном
       * FIFO-порядке. */}
      {showSuspended && (
        <ul className={styles.suspendedList} data-testid="suspended-properties-list">
          {suspendedShared.map((placeholder) => (
            <li key={placeholder.propertyId}>
              <SuspendedPropertyCard
                placeholder={placeholder}
                onReason={setReasonTarget}
              />
            </li>
          ))}
        </ul>
      )}

      {showControls && (
        <Button
          variant="white"
          size="small"
          leadingIcon={<Add/>}
          className="w-full"
          onClick={openCreate}
          disabled={isActionLoading}
        >
          Создать объект
        </Button>
      )}
    </div>
  );

  return (
    <>
      {/* Хаб-шапка: «крылья» (лого + профиль) и на мобайле; компакт-бар —
       * лупа на поиск, «+» справа (канон сворачивания хаба). */}
      <TopNav
        mobileWings
        collapse={{
          title: 'Объекты',
          search: {href: ROUTES.propertySearch, label: 'Найти объект'},
          trailing: createButton,
        }}
      />

      <PageContent>
        <HubCollapseAnchor>
          <HubTitle>Объекты</HubTitle>
        </HubCollapseAnchor>

        {content}
      </PageContent>

      <SuspendedReasonSheet placeholder={reasonTarget} onClose={() => setReasonTarget(null)} />
    </>
  );
}

/** Поисковая пилюля хаба (Figma 1031:20955 в 1590:88756): лупа, «Найти
 * объект», в хвосте — «+» создания (кнопка внутри пилюли). Тап по пилюле
 * открывает страницу серверного поиска (#601, канон поисковых хабов). */
function PropertiesSearchPill({
  onCreate,
  createLabel,
  createDisabled,
}: {
  readonly onCreate: () => void;
  readonly createLabel: string;
  readonly createDisabled: boolean;
}): JSX.Element {
  const router = useRouter();
  const openSearch = () => router.push(ROUTES.propertySearch);
  const activatorProps = useKeyboardActivation({onSelect: openSearch});

  return (
    <div
      {...activatorProps}
      data-testid="properties-search-pill"
      className="flex h-14 w-full cursor-pointer items-center rounded-pill bg-surface-muted pr-1.5 pl-4 text-left outline-none transition-opacity hover:opacity-90 focus-visible:ring-4 focus-visible:ring-primary active:opacity-90"
    >
      <Search className="h-6 w-6 shrink-0 text-content" aria-hidden/>
      <span className="min-w-0 flex-1 truncate px-2 text-base font-medium text-content">
        Найти объект
      </span>
      <IconButton
        icon={<Add/>}
        label={createLabel}
        disabled={createDisabled}
        onClick={(event) => {
          event.stopPropagation();
          onCreate();
        }}
      />
    </div>
  );
}

/** Ряд сортировки (1590:88756): чип PickerMenu слева и «Архив» справа
 * (ведёт на экран архива, #587). */
function PropertiesSortRow({
  sort,
  onChange,
}: {
  readonly sort: PropertySort;
  readonly onChange: (sort: PropertySort) => void;
}): JSX.Element {
  const router = useRouter();
  return (
    <div className="flex items-center justify-between" data-testid="properties-sort-row">
      <PropertiesSortMenu sort={sort} onChange={onChange}/>
      <Button
        variant="clear"
        size="small"
        leadingIcon={<Archive/>}
        onClick={() => router.push(ROUTES.propertyArchive)}
      >
        Архив
      </Button>
    </div>
  );
}

/** Чип сортировки + пикер (меню на 561+ / шит на мобайле — канон
 * PickerMenu): поле и направление — два «радио», выбор применяется сразу
 * (1603:93153 / 1603:91341). */
function PropertiesSortMenu({
  sort,
  onChange,
}: {
  readonly sort: PropertySort;
  readonly onChange: (sort: PropertySort) => void;
}): JSX.Element {
  const fieldLabels: Record<PropertySortField, string> = {
    name: 'По названию',
    created: 'По дате создания',
    type: 'По типу объекта',
    status: 'По статусу',
  };
  const directions: ReadonlyArray<{ readonly value: PropertySortDirection; readonly label: string }> = [
    {value: 'asc', label: 'Возрастание'},
    {value: 'desc', label: 'Убывание'},
  ];

  const groups: ReadonlyArray<PickerMenuGroup> = [
    {
      options: SORT_FIELD_OPTIONS.map((field) => ({
        label: fieldLabels[field],
        selected: sort.field === field,
        onSelect: () => onChange({...sort, field}),
      })),
    },
    {
      options: directions.map(({value, label}) => ({
        label,
        selected: sort.direction === value,
        onSelect: () => onChange({...sort, direction: value}),
      })),
    },
  ];

  return (
    <PickerMenu title="Сортировать" groups={groups}>
      <Button
        variant="secondary"
        size="small"
        leadingIcon={sort.direction === 'asc' ? <SortingSmallBig/> : <SortingBigSmall/>}
        trailingIcon={<SmallArrowDown/>}
      >
        {SORT_FIELD_CHIP_LABEL[sort.field]}
      </Button>
    </PickerMenu>
  );
}
