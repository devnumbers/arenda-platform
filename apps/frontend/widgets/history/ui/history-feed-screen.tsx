'use client';

import { useEffect, useLayoutEffect, useRef, useState, type JSX, type ReactNode } from 'react';
import Link from 'next/link';
import { ArrowLeft, BoldUser, Search } from '@/shared/assets/icons';
import { type HistoryFilterOptions } from '@/entities/history';
import { PropertyAvatar } from '@/entities/property';
import {
  groupHistoryByDay,
  historyFeedScope,
  historyMemberFeedScope,
  isDefaultHistoryFilters,
  memberHistoryFilters,
  useHistoryFeed,
  useHistoryFilters,
  useHistoryFiltersState,
  type HistoryFilters,
} from '@/features/history';
import { dateToIsoLocal } from '@/shared/lib/calendar';
import { useDebounce } from '@/shared/lib/hooks/useDebounce';
import {
  Button,
  ErrorCard,
  InfiniteQueryHead,
  IconButton,
  PageContent,
  SearchField,
  TopNav,
  TopNavBackButton,
  TopNavTitle,
  useTabBarSuppression,
} from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { HistoryRow } from './history-row';
import { HistoryFeedSkeleton } from './history-states';
import { HistoryFiltersSheet } from './history-filters-sheet';

/** Задержка дебаунса поиска (мс) — канон поисков (#601). */
const SEARCH_DEBOUNCE_MS = 300;

/** Канон строки-ссылки (хаб «Совместный доступ»): подложка hover/active
 * не меняется, только прозрачность; фокус-ринг с клавиатуры. */
const HEADER_LINK =
  'flex min-w-0 items-center gap-2 rounded-m outline-none transition-opacity hover:opacity-80 active:opacity-80 focus-visible:ring-4 focus-visible:ring-primary';

/**
 * Экран «История действий» (карта #704, тикеты #709+#710; макеты
 * 2157-56876 — лента, 2050-158499 — пустая, 2089-165788 и
 * 2092-166284/166621/166866 — поиск): мессенджерская лента по всем
 * доступным объектам — новые снизу, прокрутка вверх догружает старое
 * (двусторонний keyset #708, head-сентинел). Анатомия по макету
 * 2157-56876 (правка дизайна, решение владельца 23.09): секция дня
 * открывается белой плашкой даты с тенью — она прилипает к верхнему краю
 * при прокрутке, как в мессенджерах, с отступом 24 от хедера (на мобиле
 * шапка уезжает — safe-area + 24, на планшете/ПК — под закреплённой
 * шапкой 72+24); шапка объекта — аватар 24 (feed) + название + адрес из
 * опций /history/filters, кликабельна — ведёт на страницу объекта;
 * действия актёра — серые карточки (surface-muted, radius m): белый
 * кружок с пользователем + имя, внутри строки «полоска тона + S-иконка +
 * текст» (см. HistoryRow). Шапка актёра кликабельна при известном
 * actor_id — тап ведёт на «Действия участника» (#712, ниже; решение
 * #709 о ссылке на страницу участника заменено: владельцы и своя шапка
 * тоже кликабельны — страница действий не читает участников, 404 не
 * бывает; обезличенные записи, actor_id null, не ссылки). Роль на экране
 * не показывается (как в макете) — словарь роли живёт в entity
 * (ADR 0061). Вход — хаб «Совместный доступ» (решение владельца 22.09),
 * фолбэк «Назад» — туда же.
 *
 * Поиск по истории (#710) — иконка в шапке ленты (макет 2157-56876),
 * шапка меняется на поисковую поверх того же экрана (прецедент «Ваших
 * участников» #697; серверный поиск — канон #601: дебаунс 300 мс, трим,
 * пустой ввод после трима запроса не порождает — под полем остаётся
 * сама лента, макет 2089-165788). Ввод ищет серверно по q (маршрутизация
 * trgm/fts #708); смена запроса держит прежнюю выдачу до ответа
 * (keepPreviousData в useHistoryFeed). Найденное группируется как лента —
 * чипы дней сохраняются (макеты 2092-166284/166621); без совпадений —
 * серая строка «Ничего не найдено» без иллюстрации (макет 2092-166866,
 * канон «пусто без иллюстрации»); выход — «Назад» возвращает ленту.
 *
 * Элементы макета, приходящие со своими тикетами: переходы по ссылкам
 * сегментов (#713). Кнопка «Настройки» стоит
 * на месте по макету (в т.ч. на пустой ленте и в поиске — 2092-166284):
 * primary по контенту (не на всю ширину, макет 2184-94734), по центру.
 *
 * Фильтры (#711) живут в адресе ленты (?from=&to=&actions=&kinds=&actors=
 * &objects=, канон §3; применение — push, «назад» возвращает без
 * фильтров) и открываются шитом «Настройки» (HistoryFiltersSheet) поверх
 * той же ленты. Группа фильтров, выбранная «в ноль», — пустой результат
 * без запроса (historyFeedScope → null); активные фильтры с пустой
 * выдачей показывают «Ничего не найдено» — «Действий не было» остаётся
 * только у чистой ленты.
 *
 * ## Действия участника (#712; макет 2184-94731)
 *
 * Та же лента, прибитая к одному человеку: маршрут
 * /history/participants/[participantId] несёт uuid юзера (actor_id
 * журнала), скоуп — actor_ids = один (ADR 0061 §7: тот же GET /history),
 * группы адреса действуют поверх (historyMemberFeedScope). Группировка
 * «день → объект → актёр → строки» переиспользуется как есть — в рамках
 * одного человека объекты внутри дня остаются секциями (макет 2184-94731:
 * шапки объектов и серые карточки). Поиск и «Настройки» переиспользуются;
 * группы «Участники» в шите нет — человек прибит страницей, а не фильтр
 * (memberHistoryFilters: и URL-actors не читается, и «Сбросить»/«Применить»
 * её не пишут). Вход — тап по актёру в общей ленте (все записи с
 * actor_id — включая владельца: страница не читает участников, 404 не
 * бывает; решение #709 о ссылке на страницу участника заменено: она
 * доступна из списка «Ваши участники») и кебаб страницы участника (#698).
 * Шапки актёров на самой странице статичны — человек уже её предмет.
 * Фолбэк «Назад» — общая лента.
 */
export type HistoryFeedScreenProps = {
  /** Действия участника (#712): uuid юзера, прибивающий ленту
   * (actor_ids = один); undefined — общая лента по всем объектам. */
  readonly participantId?: string;
};

export function HistoryFeedScreen({ participantId }: HistoryFeedScreenProps = {}): JSX.Element {
  // Личность экрана: страница участника прибивает человека, заголовок и
  // фолбэк «Назад»; группа «Участники» шита и URL-actors на ней не живут.
  const isMemberPage = participantId !== undefined;
  const [searchMode, setSearchMode] = useState(false);
  const [search, setSearch] = useState('');
  const [filtersOpen, setFiltersOpen] = useState(false);
  const searchInputRef = useRef<HTMLInputElement | null>(null);

  // Фильтры (#711) живут в адресе ленты; чтение и запись — через канон
  // useUrlParams (DESIGN.md §3), применение — push («назад» возвращает
  // без фильтров). На странице участника группа «Участники» не читается
  // (человек прибит путём) — и в шит, и в скоуп идёт нормализованная
  // модель (memberHistoryFilters), «Применить» заодно вычищает ?actors=.
  const { filters: urlFilters, applyFilters } = useHistoryFiltersState();
  const filters = isMemberPage ? memberHistoryFilters(urlFilters) : urlFilters;

  // Открытие поиска сразу делает поле активным (программный фокус —
  // устоявшийся a11y-паттерн вместо autoFocus).
  useEffect(() => {
    if (searchMode) {
      searchInputRef.current?.focus();
    }
  }, [searchMode]);

  // Канон поиска #601: серверный запрос догоняет ввод с дебаунсом, в
  // запрос идёт трим; пустой трим — не поиск, а лента (запроса нет).
  // Гард по searchMode закрывает окно дебаунса: «Назад» из поиска
  // возвращает ленту тем же кадром, не держа 300 мс старый ключ.
  const debounced = useDebounce(search, SEARCH_DEBOUNCE_MS);
  const q = debounced.trim();
  const searching = q.length > 0;

  // Скоуп ленты = фильтры адреса (#711) + живой поиск (#710); на странице
  // участника поверх — прибитый человек (historyMemberFeedScope). Группа
  // фильтров, выбранная «в ноль», означает пустой результат — скоупа нет
  // (null), запрос не делается.
  const scope = isMemberPage
    ? historyMemberFeedScope(filters, participantId)
    : historyFeedScope(filters);
  const feedQuery = useHistoryFeed(
    scope === null ? {} : searchMode && searching ? { ...scope, q } : scope,
    { enabled: scope !== null },
  );
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
  // позицию компенсацией дельты высоты (iOS overflow-anchor не умеем);
  // сужение выдачи поиском (макет 2092-166284 — «Сегодня» под шапкой)
  // ведёт себя как первая загрузка: окно на самых свежих найденных.
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

  const closeSearch = (): void => {
    setSearch('');
    setSearchMode(false);
  };

  return (
    <>
      {searchMode ? (
        <TopNav
          variant="search"
          leading={
            <IconButton icon={<ArrowLeft />} label="Закрыть поиск" onClick={closeSearch} />
          }
        >
          <SearchField
            ref={searchInputRef}
            aria-label="Поиск по истории"
            placeholder="Поиск действий"
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            onClear={() => setSearch('')}
          />
        </TopNav>
      ) : (
        <TopNav
          leading={
            <TopNavBackButton fallbackHref={isMemberPage ? ROUTES.history : ROUTES.participants} />
          }
          // Лупа — только на загруженной непустой ленте: пустая книга
          // прячет иконки шапки (§7), pending/error — тоже (прецедент
          // «Ваших участников» #697), иначе иконка мелькает до ответа.
          trailing={
            feedQuery.isSuccess && entries.length > 0 ? (
              <IconButton
                icon={<Search className="h-6 w-6" />}
                label="Поиск по истории"
                onClick={() => setSearchMode(true)}
              />
            ) : undefined
          }
        >
          <TopNavTitle title={isMemberPage ? 'Действия участника' : 'История действий'} />
        </TopNav>
      )}

      <PageContent className="px-4">
        {scope === null ? (
          /* Фильтры выбраны «в ноль» — результат пуст по семантике
            * (запроса нет, #711). */
          <p className="pt-16 text-center text-base leading-[18px] text-content-secondary">
            Ничего не найдено
          </p>
        ) : feedQuery.isPending ? (
          <HistoryFeedSkeleton />
        ) : feedQuery.isError ? (
          <ErrorCard title="Не удалось загрузить историю" onRetry={() => void feedQuery.refetch()} className="mt-6" />
        ) : entries.length === 0 ? (
          searching || !isDefaultHistoryFilters(filters) ? (
            /* Без совпадений — серая строка без иллюстрации (макет
              * 2092-166866; канон «пусто без иллюстрации» #697); активные
              * фильтры с пустой выдачей — тот же канон (#711: действия
              * были, но не подходят под фильтр). */
            <p className="pt-16 text-center text-base leading-[18px] text-content-secondary">
              Ничего не найдено
            </p>
          ) : (
            <div className="flex min-h-[60vh] items-center justify-center">
              <p className="text-base leading-[18px] text-content-secondary">Действий не было</p>
            </div>
          )
        ) : (
          <>
            <InfiniteQueryHead query={feedQuery} />
            {days.map((day) => (
              <section key={day.day} className="pb-4">
                {/* Плашка дня липнет к верхнему краю (макет 2157-56876 —
                  * sticky, как в мессенджерах) с отступом 24 от хедера:
                  * на мобиле шапка уезжает — safe-area + 24, на
                  * планшете/ПК — закреплённая шапка 72 + 24. */}
                <div className="sticky top-[calc(env(safe-area-inset-top)_+_24px)] z-20 mb-3 flex justify-center tablet:top-[96px]">
                  <span className="rounded-pill bg-white px-3.5 py-2 text-xs leading-[15px] text-content shadow-[0_8px_24px_rgba(0,0,0,0.12)]">
                    {day.label}
                  </span>
                </div>
                <div className="flex flex-col gap-4">
                  {day.objects.map((object_, objectIndex) => {
                    const objectOptions = options.get(object_.propertyId);
                    return (
                      <section
                        key={`${object_.propertyId}-${objectIndex}`}
                        className="flex flex-col gap-3"
                      >
                        <Link
                          href={ROUTES.property(object_.propertyId)}
                          className={HEADER_LINK}
                        >
                          <PropertyAvatar
                            photoUrl={objectOptions?.photoUrl ?? ''}
                            surface="feed"
                          />
                          <span className="min-w-0">
                            <h2 className="truncate text-xs font-medium leading-[15px] text-content">
                              {object_.propertyName}
                            </h2>
                            {objectOptions?.address && (
                              <p className="truncate text-xs leading-[15px] text-content-secondary">
                                {objectOptions.address}
                              </p>
                            )}
                          </span>
                        </Link>
                        <div className="flex flex-col gap-1.5">
                          {object_.actors.map((actor, actorIndex) => {
                            // В общей ленте шапка актёра — вход в
                            // «Действия участника» (#712): любая запись с
                            // actor_id кликабельна, владелец включительно —
                            // страница не читает участников, 404 не
                            // бывает (решение #709 о ссылке на страницу
                            // участника заменено). На странице участника
                            // шапки статичны — человек уже её предмет;
                            // обезличенные записи (actor_id null) не
                            // ссылки нигде.
                            const header =
                              !isMemberPage && actor.actorId !== null
                                ? ROUTES.historyParticipant(actor.actorId)
                                : null;
                            return (
                              <div
                                key={`${actor.key}-${actorIndex}`}
                                className="flex flex-col gap-2 rounded-m bg-surface-muted p-3"
                              >
                                <ActorHeader name={actor.name} href={header} />
                                <div className="flex flex-col gap-2">
                                  {actor.entries.map((entry) => (
                                    <HistoryRow key={entry.id} entry={entry} />
                                  ))}
                                </div>
                              </div>
                            );
                          })}
                        </div>
                      </section>
                    );
                  })}
                </div>
              </section>
            ))}
          </>
        )}
      </PageContent>

      {/* Шит «Настройки» (#711): primary по контенту (не на всю ширину,
        * макет 2184-94734), по центру; открывает шит фильтров поверх
        * ленты. */}
      <div className="fixed inset-x-0 bottom-[max(1.5rem,env(safe-area-inset-bottom))] z-40 flex justify-center">
        <Button type="button" onClick={() => setFiltersOpen(true)}>
          Настройки
        </Button>
      </div>

      {filtersOpen && (
        <HistoryFiltersSheet
          applied={filters}
          optionsQuery={filtersQuery}
          title={isMemberPage ? 'Действия участника' : undefined}
          showActorsGroup={!isMemberPage}
          onApply={(draft: HistoryFilters) => {
            applyFilters(draft);
            setFiltersOpen(false);
          }}
          onClose={() => setFiltersOpen(false)}
        />
      )}
    </>
  );
}

/** Шапка актёра в карточке: белый кружок-плейсхолдер + имя; с href —
 * ссылка на страницу участника, без — статичная шапка. */
function ActorHeader({ name, href }: { readonly name: string; readonly href: string | null }): JSX.Element {
  const content: ReactNode = (
    <>
      <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-pill bg-white">
        <BoldUser className="h-3.5 w-3.5" aria-hidden />
      </span>
      <h3 className="min-w-0 truncate text-xs font-medium leading-[15px] text-content">{name}</h3>
    </>
  );
  return href !== null ? (
    <Link href={href} className={HEADER_LINK}>
      {content}
    </Link>
  ) : (
    <div className="flex items-center gap-2">{content}</div>
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
