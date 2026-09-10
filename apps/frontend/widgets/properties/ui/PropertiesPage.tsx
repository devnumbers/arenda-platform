'use client';

import {type JSX, useCallback, useMemo, useState} from 'react';
import {usePathname, useRouter} from 'next/navigation';
import {
  useArchivedProperties,
  useProperties,
  usePropertiesWithMeta,
} from '@/features/properties';
import {useSubscription} from '@/features/subscription';
import {Add, Archive, ArrowLeft, Search, SmallArrowDown, SortingBigSmall, SortingSmallBig} from '@/shared/assets/icons';
import {goBack} from '@/shared/lib/navigation';
import {useKeyboardActivation} from '@/shared/lib/hooks/useKeyboardActivation';
import {
  Button,
  HubCollapseAnchor,
  HubTitle,
  IconButton,
  PageContent,
  PickerMenu,
  type PickerMenuGroup,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import {ROUTES} from '@/shared/config/routes';
import {
  DEFAULT_PROPERTY_SORT,
  SORT_FIELD_CHIP_LABEL,
  SORT_FIELD_OPTIONS,
  serializeSortToParams,
  sortProperties,
  type PropertySort,
  type PropertySortDirection,
  type PropertySortField,
} from '../lib/property-sort';
import {filterPropertiesByMode, type PropertiesViewMode} from '../lib/mode-filter';
import {formatHiddenSharedFootnote} from '../lib/format-hidden-shared-footnote';
import {PropertyCard} from './PropertyCard';
import {PropertiesEmptyState} from './PropertiesEmptyState';
import {PropertiesLoading} from './PropertiesLoading';
import {PropertiesErrorState} from './PropertiesErrorState';
import styles from './PropertiesPage.module.css';

export type PropertiesPageProps = {
  readonly mode?: PropertiesViewMode;
  readonly initialSort?: PropertySort;
};

/**
 * Экран-хаб «Объекты» (карта #583, тикет #586; Figma 1603:89079 — ПК,
 * 1603:88972 — планшет, 1590:88756 — мобайл, 1603:90604 — пустое): заголовок
 * хаба, поисковая пилюля с «+» создания, ряд сортировки (чип PickerMenu +
 * ссылка «Архив»; пустой список прячет ряд — DESIGN.md §7), список карточек,
 * сноска скрытых шаренных объектов, «+ Создать объект» под списком.
 * Компакт-бар — канон хаба: лупа на поиск, заголовок, «+». Сортировки —
 * резолюция #584 (4 поля × возрастание/убывание, основной всегда первый),
 * персистентность в URL (?sort=&order=). Фильтров по типу/статусу в новом
 * хабе нет — остался клиентский поиск на отдельной странице.
 */
export function PropertiesPage({mode = 'active', initialSort}: PropertiesPageProps): JSX.Element {
  const propertiesQuery = useProperties({enabled: mode === 'active'});
  const archivedPropertiesQuery = useArchivedProperties({enabled: mode === 'archived'});
  const listQuery = mode === 'archived' ? archivedPropertiesQuery : propertiesQuery;
  const {data, isLoading, isFetching, isError, refetch} = listQuery;
  const {data: activeProperties} = useProperties();
  // Shares the /properties request with useProperties via the shared
  // propertyKeys.list prefix; surfaces how many shared objects are hidden
  // from the recipient by a tariff slot shortage and the actor's today
  // (ADR 0048) for the rental badges.
  const metaQuery = usePropertiesWithMeta({enabled: mode === 'active'});
  const subscriptionQuery = useSubscription();
  const router = useRouter();
  const pathname = usePathname();

  const [sort, setSort] = useState<PropertySort>(initialSort ?? DEFAULT_PROPERTY_SORT);

  const visible = useMemo(
    () => sortProperties(filterPropertiesByMode(data ?? [], mode), sort),
    [data, mode, sort],
  );

  const changeSort = useCallback(
    (next: PropertySort) => {
      setSort(next);
      const query = new URLSearchParams(serializeSortToParams(next)).toString();
      router.replace(query ? `${pathname}?${query}` : pathname, {scroll: false});
    },
    [pathname, router],
  );

  const isEmpty = !isLoading && !isError && visible.length === 0;
  // Служебный ряд (сортировка + «Архив») и список живут только вместе (§7).
  const showControls = !isLoading && !isError && !isEmpty;

  const hiddenSharedCount = metaQuery.data?.hiddenSharedCount ?? 0;
  const showHiddenSharedNote =
    mode === 'active'
    && showControls
    && !metaQuery.isLoading
    && !metaQuery.isError
    && hiddenSharedCount > 0;

  const isActionLoading = subscriptionQuery.isPending || activeProperties === undefined;

  const canAdd = useMemo(() => {
    if (!subscriptionQuery.data || activeProperties === undefined) return false;
    const limit = subscriptionQuery.data.tariff.activePropertyLimit;
    if (limit < 0) return true;
    return activeProperties.filter((property) => property.status !== 'archived').length < limit;
  }, [subscriptionQuery.data, activeProperties]);

  const openCreate = useCallback(() => {
    if (isActionLoading) return;
    router.push(canAdd ? ROUTES.propertyNew : ROUTES.profileTariffChange);
  }, [canAdd, isActionLoading, router]);

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
      {mode === 'active' && (
        <PropertiesSearchPill onCreate={openCreate} createLabel={createLabel} createDisabled={isActionLoading}/>
      )}

      {isLoading && <PropertiesLoading/>}

      {!isLoading && isError && <PropertiesErrorState onRetry={() => void refetch()} isLoading={isFetching}/>}

      {!isLoading && !isError && isEmpty && (
        <PropertiesEmptyState canAdd={canAdd} isLoading={isActionLoading} onAdd={openCreate}/>
      )}

      {showControls && (
        <>
          <PropertiesSortRow mode={mode} sort={sort} onChange={changeSort}/>
          <ul className={styles.list} data-testid="properties-list">
            {visible.map((property) => (
              <li key={property.id}>
                <PropertyCard
                  property={property}
                  today={mode === 'active' ? metaQuery.data?.today : undefined}
                />
              </li>
            ))}
          </ul>
        </>
      )}

      {showHiddenSharedNote && (
        <p className={styles.hiddenSharedNote}>
          {formatHiddenSharedFootnote(hiddenSharedCount)}
        </p>
      )}

      {showControls && mode === 'active' && (
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

  if (mode === 'archived') {
    return (
      <>
        <TopNav
          leading={
            <IconButton
              icon={<ArrowLeft/>}
              label="Назад"
              onClick={() => goBack(router, ROUTES.properties)}
            />
          }
          trailing={createButton}
        >
          <TopNavTitle title="Архивные объекты"/>
        </TopNav>

        <PageContent>{content}</PageContent>
      </>
    );
  }

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
    </>
  );
}

/** Поисковая пилюля хаба (Figma 1031:20955 в 1590:88756): лупа, «Найти
 * объект», в хвосте — «+» создания (кнопка внутри пилюли). Тап по пилюле
 * открывает страницу поиска (#586, канон поисковых хабов). */
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
  const openSearch = useCallback(() => router.push(ROUTES.propertySearch), [router]);
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
 * (только в активном режиме; в архиве — один чип). */
function PropertiesSortRow({
  mode,
  sort,
  onChange,
}: {
  readonly mode: PropertiesViewMode;
  readonly sort: PropertySort;
  readonly onChange: (sort: PropertySort) => void;
}): JSX.Element {
  const router = useRouter();
  return (
    <div className="flex items-center justify-between" data-testid="properties-sort-row">
      <PropertiesSortMenu sort={sort} onChange={onChange}/>
      {mode === 'active' && (
        <Button
          variant="clear"
          size="small"
          leadingIcon={<Archive/>}
          onClick={() => router.push(ROUTES.propertyArchive)}
        >
          Архив
        </Button>
      )}
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
