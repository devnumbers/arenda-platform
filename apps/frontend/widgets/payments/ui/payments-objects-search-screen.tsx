'use client';

import { useEffect, useRef, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { useSearchQueryState } from '@/shared/lib/hooks/useSearchQueryState';
import { useGlobalPaymentObjects } from '@/features/payments';
import {
  Button,
  EmptyState,
  IconButton,
  ListRow,
  PageContent,
  SearchField,
  Skeleton,
} from '@/shared/ui/design';
import { PaymentObjectAvatar, PaymentsStateCard } from './payments-sections';

/**
 * Поиск объектов (карта #573, тикет #582, Figma 888:19356/19364): поисковая
 * шапка соседей (#581) с полем «Найти объект». Старт — «Введите название
 * или адрес объекта», пустой результат — «Такого объекта нет» (одна
 * иллюстрация). Результаты — строки канона ListRow (фото/плейсхолдер-дом,
 * название, адрес); тап — платежи объекта. Серверная область —
 * GET /payments/objects?search= (#575): фильтр по названию/адресу; ввод
 * живёт в адресе (?q=) и догоняется дебаунсом — useSearchQueryState.
 * Архивные в поиске не участвуют (фильтр сервера). Каркас кабинетный,
 * как поиск платежей: шапка в потоке страницы, «назад» возвращает на
 * «Объекты».
 */
export function PaymentsObjectsSearchScreen(): JSX.Element {
  const router = useRouter();
  const { value, debounced, setValue, clear } = useSearchQueryState();
  const inputRef = useRef<HTMLInputElement>(null);

  // Полевой фокус при входе на экран (поисковая шапка — каретка в поле);
  // jsx-a11y/no-autofocus запрещает сам проп.
  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  const objectsQuery = useGlobalPaymentObjects(debounced, {
    enabled: debounced.trim() !== '',
  });

  const items = objectsQuery.data ?? [];

  const emptyQuery = value.trim() === '';
  // Скелетон — только пока данных нет вовсе (первый запрос): правка запроса
  // держит прежние результаты (keepPreviousData) и не дёргает экран; ошибка
  // без данных показывает карточку повтора, не скелетон.
  const pending =
    !emptyQuery && objectsQuery.data === undefined && !objectsQuery.isError;
  const showResults = !emptyQuery && !pending && !objectsQuery.isError;

  const openObject = (propertyId: string): void =>
    router.push(ROUTES.propertyPayments(propertyId));

  return (
    <PageContent>
      {/* Ритм страницы — ровно 24px по бокам, как на «Объектах» и поиске
       * платежей: шапка держит вставку сама, строки ListRow — полный
       * вылет (их px-6 и есть 24px, вторая вставка удвоила бы отступ). */}
      <div className="-mx-5 min-[1200px]:mx-0">
        <div className="flex items-center gap-1 px-6 pt-1">
          <IconButton
            icon={<ArrowLeft />}
            label="Назад"
            onClick={() => goBack(router, ROUTES.paymentsObjects)}
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

        {emptyQuery && (
          <EmptyState
            imageSrc="/images/payments/payments-objects-search.png"
            className="py-16"
            description="Введите название или адрес объекта"
          />
        )}

        {!emptyQuery && objectsQuery.isError && (
          <PaymentsStateCard
            title="Не удалось загрузить результаты"
            hint="Проверьте подключение и попробуйте еще раз"
            action={
              <Button
                variant="secondary"
                size="small"
                onClick={() => void objectsQuery.refetch()}
              >
                Повторить
              </Button>
            }
          />
        )}

        {pending && (
          <div aria-hidden className="flex flex-col gap-6 py-6">
            {[0, 1, 2, 3].map((row) => (
              <div key={row} className="flex items-center gap-3 px-6">
                <Skeleton className="h-11 w-11 rounded-full" />
                <div className="flex flex-1 flex-col gap-2">
                  <Skeleton className="h-4 w-2/5" />
                  <Skeleton className="h-3.5 w-3/5" />
                </div>
              </div>
            ))}
          </div>
        )}

        {showResults &&
          (items.length === 0 ? (
            <EmptyState
              imageSrc="/images/payments/payments-objects-search.png"
              className="py-16"
              description="Такого объекта нет"
            />
          ) : (
            <div
              aria-label="Объекты"
              data-testid="payments-objects-search-results"
            >
              {items.map((object) => (
                <ListRow
                  key={object.propertyId}
                  leading={
                    <PaymentObjectAvatar
                      photoUrl={object.photoUrl}
                      surface="row"
                    />
                  }
                  title={object.name}
                  subtitle={object.address}
                  onSelect={() => openObject(object.propertyId)}
                />
              ))}
            </div>
          ))}
      </div>
    </PageContent>
  );
}
