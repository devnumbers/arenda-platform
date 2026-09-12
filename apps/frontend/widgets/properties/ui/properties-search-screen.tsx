'use client';

import { useEffect, useRef, useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft, BoldHome } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { useDebounce } from '@/shared/lib/hooks/useDebounce';
import { usePropertiesSearch } from '@/features/properties';
import type { Property } from '@/entities/property';
import {
  Button,
  EmptyState,
  IconButton,
  InfiniteQueryTail,
  ListRow,
  PageContent,
  SearchField,
  SkeletonListRow,
  TopNav,
  skeletonRowWidths,
} from '@/shared/ui/design';

/** Задержка дебаунса поиска (мс) — серверный фильтр по ?search=. */
const SEARCH_DEBOUNCE_MS = 300;

/**
 * Поиск объектов (карта #596, тикет #601, пилюля хаба 1603:89183):
 * поисковая шапка канона (#565) с полем «Найти объект», «←» в левом слоте
 * закрывает поиск (возврат на хаб), поле получает программный фокус.
 * Ввод ищет по серверному ?search= всей видимой срезы — свои объекты плюс
 * разделяемые, без архивных (дебаунс 300 мс, как у книги контактов).
 * Состояния — канон соседей: пустое поле — подсказка, по чему ищем;
 * без совпадений — «Такого объекта нет» (тексты поиска объектов #582);
 * результаты — строки ListRow (фото/плейсхолдер, название, адрес), тап —
 * карточка объекта. Порции по 50 листаются sentinel-скроллом (#600).
 * Кнопок создания в поиске нет — паритет с книгой контактов.
 */
export function PropertiesSearchScreen(): JSX.Element {
  const router = useRouter();

  const [search, setSearch] = useState('');
  const searchInputRef = useRef<HTMLInputElement | null>(null);

  // Открытие поиска сразу делает поле активным (программный фокус —
  // устоявшийся a11y-паттерн вместо autoFocus).
  useEffect(() => {
    searchInputRef.current?.focus();
  }, []);

  const debouncedSearch = useDebounce(search, SEARCH_DEBOUNCE_MS);
  // Сервер матчит подстроку как есть — пробелы по краям срезаем клиентски.
  const trimmedSearch = debouncedSearch.trim();
  const searchQuery = usePropertiesSearch(trimmedSearch);

  const properties = searchQuery.data ?? [];
  const searching = trimmedSearch.length > 0;

  return (
    <>
      <TopNav
        variant="search"
        leading={
          <IconButton
            icon={<ArrowLeft />}
            label="Закрыть поиск"
            onClick={() => goBack(router, ROUTES.properties)}
          />
        }
      >
        <SearchField
          ref={searchInputRef}
          aria-label="Поиск объектов"
          placeholder="Найти объект"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          onClear={() => setSearch('')}
        />
      </TopNav>

      <PageContent>
        {searchQuery.isPending && searching ? (
          <PropertiesSearchSkeleton />
        ) : searchQuery.isError ? (
          <PropertiesSearchErrorCard onRetry={() => void searchQuery.refetch()} />
        ) : !searching ? (
          <EmptyState
            imageSrc="/images/payments/payments-objects-search.png"
            className="py-16"
            description="Введите название или адрес объекта"
          />
        ) : properties.length === 0 ? (
          <EmptyState
            imageSrc="/images/payments/payments-objects-search.png"
            className="py-16"
            description="Такого объекта нет"
          />
        ) : (
          <div
            aria-label="Объекты"
            data-testid="properties-search-results"
            className="flex flex-col"
          >
            {properties.map((property) => (
              <ListRow
                key={property.id}
                leading={<PropertySearchAvatar property={property} />}
                title={property.name}
                subtitle={property.address}
                onSelect={() => router.push(ROUTES.property(property.id))}
              />
            ))}
            {/* Хвост порций (#633): sentinel + индикатор догрузки. */}
            <InfiniteQueryTail query={searchQuery} />
          </div>
        )}
      </PageContent>
    </>
  );
}

/** Аватар строки поиска: фото объекта или плейсхолдер-дом (канон аватара
 * объектов, PaymentObjectAvatar #582 — локальный дубликат, чтобы не тянуть
 * платёжный виджет). */
function PropertySearchAvatar({ property }: { readonly property: Property }): JSX.Element {
  const photoUrl = property.photos?.[0]?.url ?? null;
  return (
    <span
      aria-hidden
      className="relative flex h-11 w-11 shrink-0 items-center justify-center overflow-hidden rounded-full bg-surface-muted shadow-[0_0_0_2.5px_var(--dl-surface)]"
    >
      {photoUrl !== null ? (
        <img src={photoUrl} alt="" className="h-full w-full object-cover" />
      ) : (
        <BoldHome className="h-6 w-6 text-[#D3D7D9]" />
      )}
    </span>
  );
}

/** Скелетон первых совпадений — канон §7: строки SkeletonListRow (круг
 * аватара 44, заголовок + подзаголовок), тот же ритм, что у результатов;
 * показывается только пока данных нет вовсе (первый запрос), правка
 * запроса держит прежнюю выдачу (keepPreviousData). */
function PropertiesSearchSkeleton(): JSX.Element {
  return (
    <div aria-hidden className="flex flex-col py-6">
      {[0, 1, 2, 3].map((row) => (
        <SkeletonListRow key={row} widths={skeletonRowWidths(4)[row % 4]} />
      ))}
    </div>
  );
}

/** Карточка ошибки с повтором (канон поисков). */
function PropertiesSearchErrorCard({ onRetry }: { readonly onRetry: () => void }): JSX.Element {
  return (
    <section className="mx-6 rounded-card bg-surface-muted px-6 py-6">
      <h2 className="text-xl font-semibold leading-6 text-content">
        Не удалось загрузить результаты
      </h2>
      <p className="mt-2 text-sm leading-4 text-content-secondary">
        Проверьте подключение и попробуйте еще раз
      </p>
      <div className="mt-4">
        <Button size="small" variant="secondary" onClick={onRetry}>
          Повторить
        </Button>
      </div>
    </section>
  );
}

/** Route-loading архетип поиска (#609): поисковая шапка и подсказка —
 * тот же кадр, что и пустое состояние экрана. */
export function PropertiesSearchLoading(): JSX.Element {
  return (
    <>
      <TopNav
        variant="search"
        leading={<IconButton icon={<ArrowLeft />} label="Закрыть поиск" />}
      >
        <SearchField aria-label="Поиск объектов" placeholder="Найти объект" />
      </TopNav>

      <PageContent>
        <EmptyState
          imageSrc="/images/payments/payments-objects-search.png"
          className="py-16"
          description="Введите название или адрес объекта"
        />
      </PageContent>
    </>
  );
}
