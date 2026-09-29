'use client';

import { useState, type JSX } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { BoldArchive, BoldHome, BoldObjects, Cancel, Check } from '@/shared/assets/icons';
import { buildReturnUrl, goBack } from '@/shared/lib/navigation';
import { dateToIsoLocal } from '@/shared/lib/calendar';
import {
  globalOperationsFiltersParams,
  readGlobalOperationsFilters,
  resolveGlobalFilterReturnPath,
} from '@/features/payments';
import { useProperties } from '@/features/properties';
import {
  Button,
  Checkbox,
  CircleIcon,
  IconButton,
  ListRow,
  PageContent,
  StickyBottomBar,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import { PaymentsStateCard } from './payments-sections';
import { OperationsObjectsSkeleton } from './operations-skeletons';

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
 * полным набором фильтров в query (#541, filterHref). Шапка — канон
 * выборщика по макету (✕ + заголовок + ✓, Figma 1733-26805): «Закрыть»
 * отбрасывает черновик, ✓ в trailing применяет; единый хром (#564).
 * Строки — канон ListRow. Книга без объектов сюда не приводит —
 * главная показывает «Операций еще не было» (#478) вместо чипов.
 */
export function OperationsObjectsSelectScreen(): JSX.Element {
  const router = useRouter();
  const searchParams = useSearchParams();

  const filters = readGlobalOperationsFilters(searchParams, dateToIsoLocal(new Date()));
  // Черновик живёт от монтирования до монтирования: страница монтируется
  // заново на каждый вход, useState инициализируется применённым выбором.
  const [draft, setDraft] = useState<ReadonlyArray<string>>(filters.propertyIds);
  // Опция архива (#549) — часть того же черновика: «назад» отбрасывает.
  const [archivedDraft, setArchivedDraft] = useState(filters.archived);

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
        globalOperationsFiltersParams({
          ...filters,
          propertyIds: draft,
          archived: archivedDraft,
        }),
      ),
    );
  };

  return (
    <>
      {/* Канон выборщика (Figma 1733-26805): ✕ «Закрыть» отбрасывает
       * черновик (goBack — по истории, прямой загрузке — на returnTo),
       * ✓ применяет; заголовок в TopNav. */}
      <TopNav
        leading={
          <IconButton icon={<Cancel />} label="Закрыть" onClick={() => goBack(router, returnTo)} />
        }
        trailing={
          <IconButton icon={<Check />} label="Применить выбранные объекты" onClick={apply} />
        }
      >
        <TopNavTitle title="Выбрать объект" />
      </TopNav>

      <PageContent>
        <div>
          <ListRow
            leading={<SelectAvatar fallback={<BoldObjects className="h-6 w-6 text-[#D3D7D9]" />} />}
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
          {propertiesQuery.isPending && (
            <>
              {/* Разделитель в фазе загрузки — как после загрузки, чтобы
               * контент занял место скелетона без сдвига (§7). */}
              <div aria-hidden className="mx-6 h-px bg-surface-muted" />
              <OperationsObjectsSkeleton />
            </>
          )}
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
                  fallback={<BoldHome className="h-6 w-6 text-[#D3D7D9]" />}
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
          {/* Опция архива (#549, Figma 1733-26805): за разделителем, после
           * объектов — включение добавляет их операции к текущему выбору. */}
          <div aria-hidden className="mx-6 mt-1 h-px bg-surface-muted" />
          <ListRow
            leading={
              <SelectAvatar
                fallback={<BoldArchive className="h-6 w-6 text-[#D3D7D9]" />}
              />
            }
            title="Объекты в архиве"
            trailing={
              <Checkbox
                aria-label="Объекты в архиве"
                checked={archivedDraft}
                onCheckedChange={() => setArchivedDraft((current) => !current)}
                onClick={(event) => event.stopPropagation()}
              />
            }
            onSelect={() => setArchivedDraft((current) => !current)}
          />
        </div>
      </PageContent>

      {/* TabBar глушится сам, пока смонтирована панель (единый хром,
          #564) — кабинетный лифт над нижней навигацией не нужен. */}
      <StickyBottomBar>
        <Button className="w-full" onClick={apply}>
          Выбрать
        </Button>
      </StickyBottomBar>
    </>
  );
}

/** Аватар строки выбора (паттерн contact-object-select): круг 44px
 * #F3F4F6 с белым кольцом 2.5px; фото объекта или иконка-плейсхолдер —
 * дом (BoldHome) у объектов, BoldObjects (Figma 208:2994) у «Все объекты»,
 * BoldArchive у опции архива (#549). Цвет плейсхолдеров — #D3D7D9 по макету
 * (1733-26805/26831, пиксельная сверка 07.09), не text-content-tertiary.
 * Экспорт для route-loading (#609). */
export function SelectAvatar({ photoUrl, fallback }: {
  readonly photoUrl?: string;
  readonly fallback: JSX.Element;
}): JSX.Element {
  return (
    <CircleIcon variant="white" aria-hidden className="relative overflow-hidden rounded-full">
      {photoUrl !== undefined ? (
        <img src={photoUrl} alt="" className="h-full w-full object-cover" />
      ) : (
        fallback
      )}
    </CircleIcon>
  );
}
