'use client';

import { useEffect, useRef, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { useSearchQueryState } from '@/shared/lib/hooks/useSearchQueryState';
import { useProperties } from '@/features/properties';
import { filterPropertiesByQuery } from '../lib/property-sort';
import { filterPropertiesByMode } from '../lib/mode-filter';
import {
  Button,
  EmptyState,
  IconButton,
  ListRow,
  PageContent,
  SearchField,
  Skeleton,
} from '@/shared/ui/design';
import { PropertyAvatar } from '@/entities/property';

/**
 * Поиск по объектам (карта #583, тикет #586): поисковая шапка с полем
 * «Найти объект», вход — пилюля хаба «Объекты» и лупа компакт-бара. Фильтр
 * клиентский — подстрока по названию и адресу активной книги (тикет #586),
 * архивных в поиске нет. Старт — «Введите название или адрес объекта»,
 * пустой результат — «Такого объекта нет» (одна иллюстрация). Результаты —
 * строки канона ListRow (аватар-фото/плейсхолдер, название, адрес); тап —
 * на страницу объекта. Ввод живёт в адресе (?q=) — useSearchQueryState.
 */
export function PropertiesSearchScreen(): JSX.Element {
  const router = useRouter();
  const { value, setValue, clear } = useSearchQueryState();
  const inputRef = useRef<HTMLInputElement>(null);
  const propertiesQuery = useProperties();

  // Полевой фокус при входе на экран (поисковая шапка — каретка в поле);
  // jsx-a11y/no-autofocus запрещает сам проп.
  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  const active = filterPropertiesByMode(propertiesQuery.data ?? [], 'active');
  const items = filterPropertiesByQuery(active, value);

  const emptyQuery = value.trim() === '';
  const pending = propertiesQuery.isLoading;
  const showError = propertiesQuery.isError;

  const openProperty = (propertyId: string): void =>
    router.push(ROUTES.property(propertyId));

  return (
    <PageContent>
      {/* Шапка поиска держит вставку 24 сама; строки ListRow ниже — полный
       * вылет (их px-6 и есть 24, вторая вставка удвоила бы отступ). */}
      <div className="flex items-center gap-1 px-6 pt-1">
        <IconButton
          icon={<ArrowLeft />}
          label="Назад"
          onClick={() => goBack(router, ROUTES.properties)}
        />
        <SearchField
          ref={inputRef}
          placeholder="Найти объект"
          aria-label="Найти объект"
          value={value}
          onChange={(event) => setValue(event.target.value)}
          onClear={clear}
        />
      </div>

      {emptyQuery && !pending && (
        <EmptyState
          imageSrc="/images/properties/properties-empty.png"
          className="py-16"
          description="Введите название или адрес объекта"
        />
      )}

      {pending && (
        <div aria-hidden className="flex flex-col gap-6 px-6 py-6">
          {[0, 1, 2, 3].map((row) => (
            <div key={row} className="flex items-center gap-3">
              <Skeleton className="h-11 w-11 rounded-full" />
              <div className="flex flex-1 flex-col gap-2">
                <Skeleton className="h-4 w-2/5" />
                <Skeleton className="h-3.5 w-3/5" />
              </div>
            </div>
          ))}
        </div>
      )}

      {!emptyQuery && !pending && showError && (
        <div className="px-6 pt-6">
          <Button
            variant="secondary"
            size="small"
            onClick={() => void propertiesQuery.refetch()}
          >
            Повторить
          </Button>
        </div>
      )}

      {!emptyQuery && !pending && !showError &&
        (items.length === 0 ? (
          <EmptyState
            imageSrc="/images/properties/properties-empty.png"
            className="py-16"
            description="Такого объекта нет"
          />
        ) : (
          <div aria-label="Объекты" data-testid="properties-search-results">
            {items.map((property) => (
              <ListRow
                key={property.id}
                leading={
                  <PropertyAvatar
                    photoUrl={property.photos?.[0]?.url ?? null}
                    surface="row"
                  />
                }
                title={property.name}
                subtitle={property.address}
                onSelect={() => openProperty(property.id)}
              />
            ))}
          </div>
        ))}
    </PageContent>
  );
}
