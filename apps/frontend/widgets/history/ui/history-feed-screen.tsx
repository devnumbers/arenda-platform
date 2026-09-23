'use client';

import { useLayoutEffect, useRef, type JSX } from 'react';
import { BoldUser } from '@/shared/assets/icons';
import { type HistoryFilterOptions } from '@/entities/history';
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
 * keyset #708, head-сентинел). Анатомия по макету 2157-56876 (правка
 * дизайна, решение владельца 23.09): секция дня открывается белой плашкой
 * даты с тенью — она прилипает к верхнему краю при прокрутке, как в
 * мессенджерах (на мобиле шапка уезжает — плашка встаёт к краю с
 * safe-area, на планшете/ПК — под закреплённой шапкой 72); шапка объекта —
 * аватар 24 (feed) + название + адрес из опций /history/filters; действия
 * актёра — серые карточки (surface-muted, radius m): белый кружок с
 * пользователем + имя, внутри строки «полоска тона + S-иконка + текст»
 * (см. HistoryRow). Роль актёра экран не показывает (как в макете) —
 * словарь роли живёт в entity (ADR 0061), действия участника — #712.
 * Вход — хаб «Совместный доступ» (решение владельца 22.09), фолбэк «Назад»
 * — туда же.
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
  const options = propertyOptions(filtersQuery.data);

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
      <SubScreenShell
        title="История действий"
        fallbackHref={ROUTES.participants}
        contentClassName="px-4"
      >
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
              <section key={day.day} className="pb-4">
                {/* Плашка дня липнет к верхнему краю (макет 2157-56876 —
                  * sticky, как в мессенджерах): на мобиле шапка уезжает —
                  * встаёт к краю с safe-area, на планшете/ПК — под
                  * закреплённой шапкой 72. */}
                <div className="sticky top-[env(safe-area-inset-top)] z-20 mb-3 flex justify-center tablet:top-[72px]">
                  <span className="rounded-pill bg-white px-3.5 py-2 text-xs leading-[15px] text-content shadow-[0_8px_24px_rgba(0,0,0,0.12)]">
                    {day.label}
                  </span>
                </div>
                <div className="flex flex-col gap-3">
                  {day.objects.map((object_, objectIndex) => (
                    <section
                      key={`${object_.propertyId}-${objectIndex}`}
                      className="flex flex-col gap-3"
                    >
                      <div className="flex items-center gap-2">
                        <PropertyAvatar
                          photoUrl={options.get(object_.propertyId)?.photoUrl ?? ''}
                          surface="feed"
                        />
                        <div className="min-w-0">
                          <h2 className="truncate text-xs font-medium leading-[15px] text-content">
                            {object_.propertyName}
                          </h2>
                          {options.get(object_.propertyId)?.address && (
                            <p className="truncate text-xs leading-[15px] text-content-secondary">
                              {options.get(object_.propertyId)?.address}
                            </p>
                          )}
                        </div>
                      </div>
                      <div className="flex flex-col gap-1.5">
                        {object_.actors.map((actor, actorIndex) => (
                          <div
                            key={`${actor.key}-${actorIndex}`}
                            className="flex flex-col gap-2 rounded-m bg-surface-muted p-3"
                          >
                            <div className="flex items-center gap-2">
                              <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-pill bg-white">
                                <BoldUser className="h-3.5 w-3.5" aria-hidden />
                              </span>
                              <h3 className="min-w-0 truncate text-xs font-medium leading-[15px] text-content">
                                {actor.name}
                              </h3>
                            </div>
                            <div className="flex flex-col gap-2">
                              {actor.entries.map((entry) => (
                                <HistoryRow key={entry.id} entry={entry} />
                              ))}
                            </div>
                          </div>
                        ))}
                      </div>
                    </section>
                  ))}
                </div>
              </section>
            ))}
          </>
        )}
      </SubScreenShell>

      {/* Шит «Настройки» (#711) — на месте по макету 2157-56786/56887,
        * пока без действия; кнопка — primary дизайн-слоя на всю колонку. */}
      <div className="fixed inset-x-0 bottom-[max(1.5rem,env(safe-area-inset-bottom))] z-40 flex justify-center px-4">
        <Button type="button" className="w-full max-w-[560px]">
          Настройки
        </Button>
      </div>
    </>
  );
}

/** Опции объектов области: id → фото и адрес шапки группы ('' — плейсхолдер). */
function propertyOptions(
  options: HistoryFilterOptions | undefined,
): Map<string, { photoUrl: string; address: string }> {
  const map = new Map<string, { photoUrl: string; address: string }>();
  for (const object_ of options?.objects ?? []) {
    map.set(object_.id, { photoUrl: object_.photoUrl, address: object_.address });
  }
  return map;
}
