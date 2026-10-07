'use client';

import {type JSX, useLayoutEffect, useRef, useState} from 'react';
import {useRouter} from 'next/navigation';
import {
  useProperties,
  usePropertiesWithMeta,
  type SuspendedSharedProperty,
} from '@/features/properties';
import {useSubscription} from '@/features/subscription';
import {Add, Archive, SmallArrowDown, SortingBigSmall, SortingSmallBig} from '@/shared/assets/icons';
import {useUrlParams} from '@/shared/lib/hooks/use-url-params';
import {
  Button,
  HubCollapseAnchor,
  HubTitle,
  IconButton,
  PageContent,
  PickerMenu,
  type PickerMenuGroup,
  SearchPill,
  TopNav,
} from '@/shared/ui/design';
import {ROUTES} from '@/shared/config/routes';
import {resolveFreshSuspendedIds} from '../lib/fresh-suspended';
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
 * Экран-хаб «Объекты» (карта #583; новый первый блок — Figma 3225-77232,
 * хедер 3229-94554, пустые 3229-94647 / 3235-74057): заголовок хаба с «+»
 * создания справа, поисковая пилюля (канон, без «+» — создание в шапке),
 * ряд сортировки (чип PickerMenu + «Архив» → экран архива #587, гейт —
 * archived_count контракта #1233: архивных нет — кнопки нет), список
 * карточек, блюр-карточки подвесших общих объектов (#702). Кнопки создания
 * под списком больше нет (макет 3225-77232): «+» шапки и CTA пустоты.
 * Компакт-бар — канон хаба: лупа на поиск, заголовок, «+».
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

  // Свежая подвеска — blur-in канона C (#880): блюр-карточка, приехавшая
  // живым перечитыванием (кадр property от действий владельца — автор в
  // аудитории, ADR 0062 §4), проявляется из блюра. Снапшот id прошлого
  // рендера: null = доставки ещё не было (холодный вход и фаза загрузки
  // meta-запроса — первая доставка без анимации), дальше дельта снапшотов
  // = живое появление; свёртка — resolveFreshSuspendedIds в ../lib/
  // fresh-suspended. Снапшот — в layout-эффекте: класс freshIn попадает
  // в первый кадр краски (пассивный эффект вешал бы его после коммита —
  // карточка рисовала резкий кадр до старта blur-in, канон LiveValue
  // «переходы до краски»); рендер читает только state.
  const [freshSuspendedIds, setFreshSuspendedIds] = useState<ReadonlySet<string>>(
    () => new Set<string>(),
  );
  const seenSuspendedRef = useRef<ReadonlySet<string> | null>(null);
  useLayoutEffect(() => {
    // undefined-доставка (холодный вход /properties без префетча, уход в
    // refetch) не сеет пустой снапшот и не тратит прежний — null-гард
    // живёт до первой настоящей доставки.
    const resolution = resolveFreshSuspendedIds(
      seenSuspendedRef.current,
      metaQuery.data?.suspendedShared,
    );
    seenSuspendedRef.current = resolution.next;
    if (resolution.fresh.size > 0) {
      setFreshSuspendedIds(resolution.fresh);
    }
  }, [metaQuery.data]);

  // Запись — канон useUrlParams (#786): экран владеет только sort/order,
  // чужие параметры адреса переживают смену сортировки, дефолт снимается.
  const changeSort = (next: PropertySort) => {
    setSort(next);
    write(serializeSortToParams(next), {own: PROPERTY_SORT_PARAMS});
  };

  const isEmpty = !isLoading && !isError && visible.length === 0;
  // Гейт кнопки «Архив» (макет 3229-94647 vs 3235-74057, правило владельца
  // «архивных нет — кнопки нет»): счётчик архивных едет в том же ответе
  // /properties (контракт #1233); до доставки и на ошибке счётчик 0 —
  // кнопки нет.
  const showArchive = (metaQuery.data?.archivedCount ?? 0) > 0;
  // Ряд сортировки — тулбар хаба: рендерится вне фазы загрузки и не
  // подменяется скелетоном (§7, паритет #604). На пустоте живёт только
  // ради «Архива» (замена решения #1051 «вариант 1А»: счётчик #1233 даёт
  // точный гейт, запирать вход в архив без архивных больше не нужно) —
  // без архивных ряда нет вовсе (макет 3235-74057).
  const showSortRow = !isEmpty || showArchive;
  // Список — контент данных, живёт вне фаз загрузки и ошибки.
  const showList = !isLoading && !isError && !isEmpty;

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

  // «+» создания: в ряду заголовка хаба (макет 3225-77232) и в правом
  // слоте компакт-бара (канон сворачивания). На пустоте скрыта —
  // действие там CTA пустого состояния (макеты 3229-94647/3235-74057).
  // Два экземпляра узла: у ряда заголовка свой testid — слоты TopNav
  // рендерят свой узел в трёх местах (крыло, инлайн-компакт, мобайл-клон),
  // общий testid давал бы строгую неоднозначность в e2e.
  const createButton = (
    <IconButton
      icon={<Add/>}
      label={createLabel}
      data-testid="properties-create"
      disabled={isActionLoading}
      onClick={openCreate}
    />
  );
  const createButtonCompact = (
    <IconButton
      icon={<Add/>}
      label={createLabel}
      data-testid="properties-create-compact"
      disabled={isActionLoading}
      onClick={openCreate}
    />
  );

  const content = (
    <div className="flex flex-col gap-4 px-6 pt-6">
      {/* Пилюля поиска — канон SearchPill, «+» в хвосте больше нет
       * (макет 3225-77232: создание в шапке); на подтверждённой пустоте
       * спрятана (#1004): искать нечего, создание остаётся в CTA пустого
       * состояния; вне фазы загрузки и на ошибке видна (§7). */}
      {!isEmpty && (
        <SearchPill
          testId="properties-search-pill"
          label="Найти объект"
          onOpenSearch={() => router.push(ROUTES.propertySearch)}
        />
      )}

      {showSortRow && (
        <PropertiesSortRow
          sort={sort}
          onChange={changeSort}
          showChip={!isEmpty}
          showArchive={showArchive}
          onOpenArchive={() => router.push(ROUTES.propertyArchive)}
        />
      )}

      {isLoading && <PropertiesLoading/>}

      {!isLoading && isError && <PropertiesErrorState onRetry={() => void refetch()} isLoading={isFetching}/>}

      {!isLoading && !isError && isEmpty && (
        <PropertiesEmptyState canAdd={canAdd} isLoading={isActionLoading} onAdd={openCreate}/>
      )}

      {showList && (
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
                fresh={freshSuspendedIds.has(placeholder.propertyId)}
              />
            </li>
          ))}
        </ul>
      )}
    </div>
  );

  return (
    <>
      {/* Хаб-шапка: «крылья» (лого + профиль) и на мобайле; компакт-бар —
       * лупа на поиск, «+» справа (канон сворачивания). На пустоте «+»
       * скрыта в обоих местах. */}
      <TopNav
        mobileWings
        collapse={{
          title: 'Объекты',
          search: {href: ROUTES.propertySearch, label: 'Найти объект'},
          trailing: isEmpty ? undefined : createButtonCompact,
        }}
      />

      <PageContent>
        <HubCollapseAnchor>
          {/* Строка заголовка h-8 (тикет #865): топ заголовка — ровно 24
              от хедера (96), как у хабов без кнопки; кнопка 44 переполняет
              строку симметрично — центрирована против линии заголовка. */}
          <div className="flex h-8 items-center justify-between pr-3.5">
            <HubTitle>Объекты</HubTitle>
            {!isEmpty && createButton}
          </div>
        </HubCollapseAnchor>

        {content}
      </PageContent>

      <SuspendedReasonSheet placeholder={reasonTarget} onClose={() => setReasonTarget(null)} />
    </>
  );
}

/** Ряд сортировки (3225-77232): чип PickerMenu слева и «Архив» справа
 * (ведёт на экран архива, #587). «Архив» — гейт archived_count (#1233):
 * архивных нет — кнопки нет, ряд без неё живёт чипом слева. На
 * подтверждённой пустоте чип скрыт (канон §7 — сортировать нечего), ряд
 * остаётся только ради «Архива» и выровнен вправо; без архивных ряда нет
 * вовсе (3235-74057). Без хуков — вызывается как функция в юнит-тесте
 * (properties-sort-row.test.ts); роутер живёт на странице. Экспорт —
 * для юнит-теста, внутрь слайса наружу ряд не уходит. */
export function PropertiesSortRow({
  sort,
  onChange,
  showChip,
  showArchive,
  onOpenArchive,
}: {
  readonly sort: PropertySort;
  readonly onChange: (sort: PropertySort) => void;
  readonly showChip: boolean;
  readonly showArchive: boolean;
  readonly onOpenArchive: () => void;
}): JSX.Element {
  return (
    <div
      className={`flex items-center ${showChip ? 'justify-between' : 'justify-end'}`}
      data-testid="properties-sort-row"
    >
      {showChip && <PropertiesSortMenu sort={sort} onChange={onChange}/>}
      {showArchive && (
        <Button
          variant="clear"
          size="small"
          leadingIcon={<Archive/>}
          onClick={onOpenArchive}
        >
          Архив
        </Button>
      )}
    </div>
  );
}

/** Чип сортировки + пикер (меню на 561+ / шит на мобайле — канон
 * PickerMenu): поле и направление — два «радио», выбор применяется сразу
 * (1603:93153 / 1603:91341). Экспорт — для юнит-теста ряда. */
export function PropertiesSortMenu({
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
