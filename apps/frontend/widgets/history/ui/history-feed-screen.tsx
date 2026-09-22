'use client';

import { useLayoutEffect, useRef, type JSX } from 'react';
import { actorRoleLabel, type HistoryFilterOptions } from '@/entities/history';
import { PropertyAvatar } from '@/entities/property';
import { groupHistoryByDay, useHistoryFeed, useHistoryFilters } from '@/features/history';
import { dateToIsoLocal } from '@/shared/lib/calendar';
import {
  Button,
  ErrorCard,
  InfiniteQueryHead,
  SubScreenShell,
  useTabBarSuppression,
} from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { HistoryRow } from './history-row';
import { HistoryFeedSkeleton } from './history-states';

/**
 * Экран «История действий» (карта #704, тикет #709; макеты 2157-56786 —
 * лента, 2050-158499 — пустая): мессенджерская лента по всем доступным
 * объектам — новые снизу, прокрутка вверх догружает старое (двусторонний
 * keyset #708, head-сентинел). Группировка «дата → объект → актёр →
 * строки» (ADR 0061 §6); фото шапок объектов — опции /history/filters
 * (item фото не несёт), нет опции — дом-плейсхолдер. Вход — хаб
 * «Совместный доступ» (решение владельца 22.09), фолбэк «Назад» — туда же.
 *
 * Элементы макета, приходящие со своими тикетами: поиск в шапке (#710),
 * контент шита «Настройки» (#711), действия участника (#712), переходы по
 * ссылкам сегментов (#713). Кнопка «Настройки» стоит на месте по макету
 * (в т.ч. на пустой ленте), пока без действия.
 */
export function HistoryFeedScreen(): JSX.Element {
  const feedQuery = useHistoryFeed();
  const filtersQuery = useHistoryFilters();
  // Экран с плавающей нижней кнопкой — TabBar глушится (канон
  // StickyBottomBar, без белого шита: макет оставляет контент видимым).
  useTabBarSuppression();

  const entries = feedQuery.data ?? [];
  const today = dateToIsoLocal(new Date());
  const days = groupHistoryByDay(entries, today);
  const photos = propertyPhotos(filtersQuery.data);

  // Мессенджерская прокрутка: на первой загрузке окно встаёт на низ
  // (видны самые новые), prepend старых при прокрутке вверх удерживает
  // позицию компенсацией дельты высоты (iOS overflow-anchor не умеем).
  const entryCount = entries.length;
  const previous = useRef<{ count: number | null; height: number }>({ count: null, height: 0 });
  useLayoutEffect(() => {
    const height = document.documentElement.scrollHeight;
    const { count: prevCount, height: prevHeight } = previous.current;
    previous.current = { count: entryCount, height };
    if (entryCount === 0) {
      return;
    }
    if (prevCount === null || prevCount === 0 || entryCount < prevCount) {
      window.scrollTo({ top: height });
      return;
    }
    if (entryCount > prevCount) {
      const delta = height - prevHeight;
      if (delta > 0) {
        window.scrollBy({ top: delta });
      }
    }
  }, [entryCount]);

  const feedEmpty = feedQuery.isSuccess && entries.length === 0;

  return (
    <>
      <SubScreenShell title="История действий" fallbackHref={ROUTES.participants}>
        {feedQuery.isPending ? (
          <HistoryFeedSkeleton />
        ) : feedQuery.isError ? (
          <ErrorCard title="Не удалось загрузить историю" onRetry={() => void feedQuery.refetch()} className="mt-6" />
        ) : feedEmpty ? (
          <div className="flex min-h-[60vh] items-center justify-center">
            <p className="text-base leading-[18px] text-content-secondary">Действий не было</p>
          </div>
        ) : (
          <>
            <InfiniteQueryHead query={feedQuery} />
            {days.map((day) => (
              <section key={day.day}>
                <div className="flex justify-center py-2.5">
                  <span className="rounded-pill bg-surface-muted px-3.5 py-1.5 text-sm leading-4 text-content-secondary">
                    {day.label}
                  </span>
                </div>
                {day.objects.map((object_, objectIndex) => (
                  <section key={`${object_.propertyId}-${objectIndex}`} className="pb-2">
                    <div className="flex items-center gap-3 pb-1 pt-4">
                      <PropertyAvatar
                        photoUrl={photos.get(object_.propertyId) ?? ''}
                        surface="row"
                        className="h-7 w-7"
                      />
                      <h2 className="text-base font-semibold leading-[18px] text-content">
                        {object_.propertyName}
                      </h2>
                    </div>
                    {object_.actors.map((actor, actorIndex) => (
                      <div key={`${actor.key}-${actorIndex}`} className="pt-3">
                        <div className="pl-[36px]">
                          <h3 className="text-[15px] font-medium leading-[18px] text-content">
                            {actor.name}
                          </h3>
                          <p className="text-[13px] leading-4 text-content-secondary">
                            {actorRoleLabel(actor.role)}
                          </p>
                        </div>
                        <div className="divide-y divide-dashed divide-surface-muted">
                          {actor.entries.map((entry) => (
                            <HistoryRow key={entry.id} entry={entry} />
                          ))}
                        </div>
                      </div>
                    ))}
                  </section>
                ))}
              </section>
            ))}
          </>
        )}
      </SubScreenShell>

      {/* Шит «Настройки» (#711) — на месте по макету 2157-56786/2050-158499,
        * пока без действия. */}
      <div className="fixed inset-x-0 bottom-[max(1.5rem,env(safe-area-inset-bottom))] z-40 flex justify-center">
        <Button type="button" className="w-auto rounded-pill px-12">
          Настройки
        </Button>
      </div>
    </>
  );
}

/** Фото объектов области: id → URL первого фото ('' — плейсхолдер). */
function propertyPhotos(options: HistoryFilterOptions | undefined): Map<string, string> {
  const photos = new Map<string, string>();
  for (const object_ of options?.objects ?? []) {
    photos.set(object_.id, object_.photoUrl);
  }
  return photos;
}
