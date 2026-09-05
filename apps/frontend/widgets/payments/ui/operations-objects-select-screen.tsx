'use client';

import { useState, type JSX } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { BoldHome, BoldObjects, Check } from '@/shared/assets/icons';
import { buildReturnUrl } from '@/shared/lib/navigation';
import { clientTodayIso } from '@/entities/payment';
import {
  globalOperationsFiltersParams,
  readGlobalOperationsFilters,
  resolveGlobalFilterReturnPath,
} from '@/features/payments';
import { useProperties } from '@/features/properties';
import {
  Button,
  Checkbox,
  IconButton,
  ListRow,
  PageContent,
  Skeleton,
  StickyBottomBar,
} from '@/shared/ui/design';
import { PageHeader } from '@/shared/ui/page-header';
import { PaymentsStateCard } from './payments-sections';

/**
 * Страница «Выбрать объект» (#542, Figma 1733-26805) — мультивыбор для
 * фильтра «Объект» глобальной ленты (#541). Строгий черновик: тапы меняют
 * только подсветку, применяются «Выбрать»/✓ (возвратом на список,
 * router.replace), назад отбрасывает. Первая строка — «Все объекты»
 * (спец-иконка BoldObjects, фильтр снят), ниже — объекты: фото/плейсхолдер
 * + название + адрес, чекбоксы справа; поиск на странице нет (в макете
 * нет, #539). Кнопка «Выбрать» просто активна всегда (решение владельца
 * 2026-09-05): пустой черновик — легитимное применение «Все объекты».
 * Период и категории при применении сохраняются — страница приходит с
 * полным набором фильтров в query (#541, filterHref). Каркас — кабинетный:
 * PageHeader с «назад» и ✓ в действиях (зона /operations живёт в кабинете,
 * #541), строки — канон ListRow. Книга без объектов сюда не приводит —
 * главная показывает «Операций еще не было» (#478) вместо чипов.
 */
export function OperationsObjectsSelectScreen(): JSX.Element {
  const router = useRouter();
  const searchParams = useSearchParams();

  const filters = readGlobalOperationsFilters(searchParams, clientTodayIso());
  // Черновик живёт от монтирования до монтирования: страница монтируется
  // заново на каждый вход, useState инициализируется применённым выбором.
  const [draft, setDraft] = useState<ReadonlyArray<string>>(filters.propertyIds);

  const propertiesQuery = useProperties();
  const properties = propertiesQuery.data ?? [];

  // Куда возвращаться: список зоны операций, открывший страницу, иначе —
  // главный список.
  const returnTo = resolveGlobalFilterReturnPath(searchParams.get('return'));

  const toggleProperty = (propertyId: string): void => {
    setDraft(
      draft.includes(propertyId)
        ? draft.filter((candidate) => candidate !== propertyId)
        : [...draft, propertyId],
    );
  };

  const apply = (): void => {
    // Единая сериализация фильтров ленты (тот же wire-формат, что пишет
    // useGlobalOperationsFilters) поверх returnTo с сохранением его query —
    // buildReturnUrl мержит параметры без коллизии «?».
    router.replace(
      buildReturnUrl(
        returnTo,
        globalOperationsFiltersParams({ ...filters, propertyIds: draft }),
      ),
    );
  };

  return (
    <>
      <PageHeader
        title="Выбрать объект"
        backHref={returnTo}
        actions={
          <IconButton icon={<Check />} label="Применить выбранные объекты" onClick={apply} />
        }
      />

      <PageContent>
        <div className="-mx-5 min-[1200px]:mx-0">
          <ListRow
            leading={<SelectAvatar fallback={<BoldObjects className="h-6 w-6 text-content-tertiary" />} />}
            title="Все объекты"
            trailing={
              <Checkbox
                aria-label="Все объекты"
                checked={draft.length === 0}
                onCheckedChange={() => setDraft([])}
                onClick={(event) => event.stopPropagation()}
              />
            }
            onSelect={() => setDraft([])}
          />
          {propertiesQuery.isPending && <ObjectRowsSkeleton />}
          {propertiesQuery.isError && (
            <div className="px-6 py-6">
              <PaymentsStateCard
                title="Не удалось загрузить объекты"
                hint="Проверьте подключение и попробуйте еще раз"
                action={
                  <Button
                    variant="secondary"
                    size="small"
                    onClick={() => void propertiesQuery.refetch()}
                  >
                    Повторить
                  </Button>
                }
              />
            </div>
          )}
          {(properties.length > 0) && (
            <div aria-hidden className="mx-6 h-px bg-surface-muted" />
          )}
          {properties.map((property) => (
            <ListRow
              key={property.id}
              leading={
                <SelectAvatar
                  photoUrl={property.photos?.[0]?.url}
                  fallback={<BoldHome className="h-6 w-6 text-content-tertiary" />}
                />
              }
              title={property.name}
              subtitle={property.address}
              trailing={
                <Checkbox
                  aria-label={`Объект ${property.name}`}
                  checked={draft.includes(property.id)}
                  onCheckedChange={() => toggleProperty(property.id)}
                  onClick={(event) => event.stopPropagation()}
                />
              }
              onSelect={() => toggleProperty(property.id)}
            />
          ))}
        </div>
      </PageContent>

      {/* Ниже 1200 в кабинете видна нижняя навигация (80px + safe-area) —
          панель приподнимается над ней, чтобы «Выбрать» не пряталась. */}
      <StickyBottomBar className="max-[1199px]:bottom-[calc(5rem_+_env(safe-area-inset-bottom))]">
        <Button className="w-full" onClick={apply}>
          Выбрать
        </Button>
      </StickyBottomBar>
    </>
  );
}

/** Аватар строки выбора (паттерн contact-object-select): круг 44px
 * #F3F4F6 с белым кольцом 2.5px; фото объекта или иконка-плейсхолдер —
 * серый дом (BoldHome) у объектов, BoldObjects (Figma 208:2994) у
 * «Все объекты». */
function SelectAvatar({ photoUrl, fallback }: {
  readonly photoUrl?: string;
  readonly fallback: JSX.Element;
}): JSX.Element {
  return (
    <span
      aria-hidden
      className="relative flex h-11 w-11 shrink-0 items-center justify-center overflow-hidden rounded-full bg-surface-muted shadow-[0_0_0_2.5px_var(--dl-surface)]"
    >
      {photoUrl !== undefined ? (
        <img src={photoUrl} alt="" className="h-full w-full object-cover" />
      ) : (
        fallback
      )}
    </span>
  );
}

/** Скелет строк объектов на время загрузки списка. */
function ObjectRowsSkeleton(): JSX.Element {
  return (
    <div aria-hidden className="flex flex-col gap-6 py-6">
      {[0, 1, 2, 3].map((row) => (
        <div key={row} className="flex items-center gap-2 px-6">
          <Skeleton className="h-12 w-12 rounded-full" />
          <div className="flex flex-1 flex-col gap-2">
            <Skeleton className="h-4 w-2/5" />
            <Skeleton className="h-3.5 w-3/5" />
          </div>
        </div>
      ))}
    </div>
  );
}
