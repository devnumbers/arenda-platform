'use client';

import { useEffect, useLayoutEffect, useRef, useState, type JSX, type ReactNode } from 'react';
import Link from 'next/link';
import { ArrowLeft, BoldUser, Search } from '@/shared/assets/icons';
import { type HistoryFilterOptions, type HistoryObjectOption } from '@/entities/history';
import type { HistoryActorGroup } from '@/features/history';
import { useMe } from '@/features/auth';
import { PropertyAvatar } from '@/entities/property';
import {
  groupHistoryByDay,
  historyFeedScope,
  isDefaultHistoryFilters,
  pinnedHistoryFilters,
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
import {
  HistoryFeedSkeleton,
  HistoryMemberFeedSkeleton,
  HistoryPropertyFeedSkeleton,
} from './history-states';
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
 * бывает; обезличенные записи, actor_id null, не ссылки). Свой актор
 * (actor_id = id сессии, useMe) подписан серым суффиксом «(Вы)» — канон
 * шита фильтров, семантика «как в Telegram» (решение владельца 24.09):
 * каждый зритель видит метку у себя; на кликабельность метка не влияет.
 * Роль на экране
 * не показывается (как в макете) — словарь роли живёт в entity
 * (ADR 0061). Вход — кебаб «Ваших участников» (#843, решение владельца
 * 24.09), фолбэк «Назад» — туда же.
 *
 * Пока лента не заполняет вьюпорт, она прижата к нижнему краю — как
 * свежее сообщение в мессенджере (решение владельца 24.09): при живых
 * записях PageContent несёт flex-1 + justify-end — узел контента
 * ScreenLayout уже flex-1 от min-h-screen оболочки, поэтому лента тянется
 * ровно до нижнего края на всех ярусах (высота шапки и safe-area
 * учитываются сами, без констант). Пустые состояния — не сообщения:
 * «Действий не было» и «Ничего не найдено» живут как раньше.
 *
 * Поиск по истории (#710) — иконка в шапке ленты (макет 2157-56876),
 * шапка меняется на поисковую поверх того же экрана (прецедент «Ваших
 * участников» #697; серверный поиск — канон #601: дебаунс 300 мс, трим,
 * пустой ввод после трима запроса не порождает — под полем остаётся
 * сама лента, макет 2089-165788). Ввод ищет серверно по q (всегда-OR
 * предикат «как в Telegram» — ресерч #839, тикет #842); смена запроса
 * держит прежнюю выдачу до ответа
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
 * группы адреса действуют поверх (historyFeedScope с пином actorId).
 * Группировка «день → объект → актёр → строки» переиспользуется как
 * есть — в рамках
 * одного человека объекты внутри дня остаются секциями (макет 2184-94731:
 * шапки объектов и серые карточки). Поиск и «Настройки» переиспользуются;
 * группа «Участники» шита показывает прибитого человека — серым и
 * незабираемым (макет 2184-92510, решение владельца 23.09), в черновик и
 * адрес он не пишется: pinnedHistoryFilters не читает URL-actors, а
 * «Применить» его вычищает. Вход — тап по актёру в общей ленте (все
 * записи с actor_id — включая владельца: страница не читает участников,
 * 404 не бывает; решение #709 о ссылке на страницу участника заменено:
 * она доступна из списка «Ваши участники») и кебаб страницы участника
 * (#698). Шапки актёров на самой странице статичны — человек уже её
 * предмет. Фолбэк «Назад» — общая лента.
 *
 * ## История объекта (#840; макет 2184-95600, шит 2184-94176)
 *
 * Та же лента, прибитая к одному объекту: маршрут
 * /history/properties/[propertyId] несёт uuid объекта (property_ids =
 * один, ADR 0061 §7: тот же GET /history), группы адреса действуют
 * поверх (historyFeedScope с пином propertyId). Объект — предмет
 * страницы: шапка-
 * карточка (аватар, название, адрес — те же «History Title» ленты)
 * стоит ОДНА над всей лентой (макет 2184-95600: карточки актёров идут
 * без объектных секций), статична и рисуется из опций /history/filters —
 * канала шапок ленты (докстринг useHistoryFilters); опций нет
 * (pending/ошибка/мусорный id) — шапки нет, лента живёт (канон шита
 * #711). Группировка «день → объект → актёр» вырождается: внутри дня
 * сразу карточки актёров. Актёры кликабельны, как в общей ленте — вход
 * в «Действия участника». Шит: группа «Объекты» показывает прибитый
 * объект — серым и незабираемым (макет 2184-94176, решение владельца
 * 24.09, зеркало «Участников» #712); pinnedHistoryFilters не читает
 * URL-objects, «Применить» его вычищает. Вход — кебаб «Участников
 * объекта» (макет 1980-139712), фолбэк «Назад» — туда же.
 *
 * ## Действия участника в объекте (#841; макет 2177-59620 — вход)
 *
 * Та же лента, прибитая к паре человек+объект: маршрут
 * /history/participants/[participantId]/properties/[propertyId] несёт
 * uuid юзера (actor_id журнала) и uuid объекта — actor_ids и
 * property_ids по одному (ADR 0061 §7: тот же GET /history; сервер
 * AND'ит обе группы — бэк #708 без изменений), группы адреса действуют
 * поверх пары (historyFeedScope с парой пинов). Оба предмета — предмет
 * страницы: шапка-карточка объекта стоит над всей лентой (зеркало
 * #840), шапки актёров статичны (зеркало #712 — человек прибит),
 * группировка внутри дней вырождена — сразу карточки актёра без
 * объектных секций. Шит «Настройки» — группы «Участники» и «Объекты»
 * ОБЕ прибиты серым («1/1», незабираемые); pinnedHistoryFilters
 * не читает URL-actors/URL-objects, «Применить» их вычищает. Вход —
 * строка «Действия участника в объекте» на «Правах участника»
 * (макет 2177-59620; только у зарегистрированных — у pending действий
 * не бывает, канон кебаба #712). Фолбэк «Назад» — «Права участника»
 * (источник входа).
 */
export type HistoryFeedScreenProps = {
  /** Действия участника (#712): uuid юзера, прибивающий ленту
   * (actor_ids = один); вместе с propertyId — «Действия участника в
   * объекте» (#841, двойной пин). Undefined — общая лента. */
  readonly participantId?: string;
  /** История объекта (#840): uuid объекта, прибивающий ленту
   * (property_ids = один); вместе с participantId — двойной пин #841.
   * Undefined — общая лента. */
  readonly propertyId?: string;
};

export function HistoryFeedScreen({
  participantId,
  propertyId,
}: HistoryFeedScreenProps = {}): JSX.Element {
  // Личность экрана: страница участника прибивает человека, страница
  // объекта — объект, страница пары (#841) — обоих; заголовок, фолбэк
  // «Назад» и прибитые группы шита следуют за предметом; прибитые
  // группы в URL не живут.
  // Личность экрана одним вычислением: заголовок живёт в TopNavTitle и
  // в шите, прибитые группы и скоуп — в пинах страницы ниже.
  const isMemberPage = participantId !== undefined;
  const isPropertyPage = propertyId !== undefined;
  const isMemberPropertyPage = isMemberPage && isPropertyPage;
  const pageTitle = isMemberPropertyPage
    ? 'Действия участника в объекте'
    : isMemberPage
      ? 'Действия участника'
      : isPropertyPage
        ? 'История объекта'
        : 'История действий';
  const [searchMode, setSearchMode] = useState(false);
  const [search, setSearch] = useState('');
  const [filtersOpen, setFiltersOpen] = useState(false);
  const searchInputRef = useRef<HTMLInputElement | null>(null);

  // Фильтры (#711) живут в адресе ленты; чтение и запись — через канон
  // useUrlParams (DESIGN.md §3), применение — push («назад» возвращает
  // без фильтров). Прибитые группы адреса не читаются: на странице
  // участника — «Участники» (человек прибит путём), на странице объекта —
  // «Объекты», на странице пары (#841) — обе: и в шит, и в скоуп идёт
  // нормализованная модель (pinnedHistoryFilters по пинам выше),
  // «Применить» заодно вычищает чужой параметр.
  const { filters: urlFilters, applyFilters } = useHistoryFiltersState();
  // Пины страницы (HistoryFilterPins): прибитые человек/объект/пара
  // считаются один раз и для нормализатора фильтров (ниже), и для скоупа.
  const pins = isMemberPropertyPage
    ? { actorId: participantId, propertyId }
    : isMemberPage
      ? { actorId: participantId }
      : isPropertyPage
        ? { propertyId }
        : undefined;
  const filters = pinnedHistoryFilters(urlFilters, pins);

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

  // Скоуп ленты = фильтры адреса (#711) + живой поиск (#710) + пины
  // страницы (historyFeedScope: прибитые человек/объект/пара #712/#840/
  // #841 подставляются в скоуп, неприбитые группы адреса действуют
  // поверх). Группа фильтров, выбранная «в ноль», означает пустой
  // результат — скоупа нет (null), запрос не делается.
  const scope = historyFeedScope(filters, pins);
  const feedQuery = useHistoryFeed(
    scope === null ? {} : searchMode && searching ? { ...scope, q } : scope,
    { enabled: scope !== null },
  );
  const filtersQuery = useHistoryFilters();
  // Свой актор (решение владельца 24.09): узнаём по id сессии — канон
  // шита фильтров (#710); серый суффикс «(Вы)» у своей шапки.
  const meQuery = useMe();
  const meId = meQuery.data?.id;
  // Экран с плавающей нижней кнопкой — TabBar глушится (канон
  // StickyBottomBar, без белого шита: макет оставляет контент видимым).
  useTabBarSuppression();

  const entries = feedQuery.data ?? [];
  // Якорь низа — строго по факту рендера ленты: keepPreviousData
  // удерживает прежние entries под «в ноль»-фильтрами и ошибкой, а эти
  // ветки рисуют пустые состояния — без якоря (решение владельца 24.09).
  const feedRendered = scope !== null && feedQuery.isSuccess && entries.length > 0;
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
            <TopNavBackButton
              fallbackHref={
                isMemberPropertyPage
                  ? ROUTES.participantRights(participantId, propertyId)
                  : isMemberPage
                    ? ROUTES.history
                    : isPropertyPage
                      ? ROUTES.propertyParticipants(propertyId)
                      : ROUTES.participants
              }
            />
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
          <TopNavTitle title={pageTitle} />
        </TopNav>
      )}

      {/* Якорь низа (решение владельца 24.09): при живых записях лента
        * прижата к нижнему краю вьюпорта, пока не заполняет его, — как
        * свежее сообщение в мессенджере. Механизм — flex-1 от узла
        * контента ScreenLayout (уже flex-1 от min-h-screen), точный низ
        * на всех ярусах без констант; пустые состояния (ниже) — не
        * сообщения, без якоря. */}
      <PageContent className={feedRendered ? 'flex-1 justify-end px-4' : 'px-4'}>
        {scope === null ? (
          /* Фильтры выбраны «в ноль» — результат пуст по семантике
            * (запроса нет, #711). */
          <p className="pt-16 text-center text-base leading-[18px] text-content-secondary">
            Ничего не найдено
          </p>
        ) : feedQuery.isPending ? (
          /* Скелетон — паритет композиции страницы (решение владельца
            * 24.09): прибитым страницам — их короткий скелетон, общий
            * двухгрупповый остаётся только общей ленте (§7). */
          isPropertyPage ? (
            <HistoryPropertyFeedSkeleton />
          ) : isMemberPage ? (
            <HistoryMemberFeedSkeleton />
          ) : (
            <HistoryFeedSkeleton />
          )
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
            {isPropertyPage &&
              /* Шапка-карточка прибитого объекта — одна над всей лентой
                * (#840, макет 2184-95600), статична: объект — предмет
                * страницы. Данные — из опций /history/filters, канала
                * шапок ленты; опций нет (pending/ошибка/мусорный id) —
                * шапки нет, лента живёт (канон шита #711). */
              (() => {
                const pinned = (filtersQuery.data?.objects ?? []).find(
                  (object_) => object_.id === propertyId,
                );
                return pinned === undefined ? null : <PinnedPropertyHeader object_={pinned} />;
              })()}
            {days.map((day) => (
              <section key={day.day} className="pb-6">
                {/* Плашка дня липнет к верхнему краю (макет 2157-56876 —
                  * sticky, как в мессенджерах) с отступом 24 от хедера:
                  * на мобиле шапка уезжает — safe-area + 24, на
                  * планшете/ПК — закреплённая шапка 72 + 24. Зазоры
                  * плашка↔блоки — по 24 (решение владельца 23.09). */}
                <div className="sticky top-[calc(env(safe-area-inset-top)_+_24px)] z-20 mb-6 flex justify-center tablet:top-[96px]">
                  <span className="rounded-pill bg-white px-3.5 py-2 text-xs leading-[15px] text-content shadow-[0_8px_24px_rgba(0,0,0,0.12)]">
                    {day.label}
                  </span>
                </div>
                <div className="flex flex-col gap-4">
                  {day.objects.map((object_, objectIndex) => {
                    const objectOptions = options.get(object_.propertyId);
                    return isPropertyPage ? (
                      /* История объекта (#840) и «Действия участника в
                        * объекте» (#841): объект прибит — предмет
                        * страницы, шапка стоит над всей лентой; внутри
                        * дня сразу карточки актёров (макет 2184-95600).
                        * Шапки актёров кликабельны только на странице
                        * объекта — на странице пары человек тоже её
                        * предмет (зеркало #712), шапки статичны. */
                      <ActorCards
                        key={`${object_.propertyId}-${objectIndex}`}
                        actors={object_.actors}
                        isMemberPage={isMemberPage}
                        meId={meId}
                      />
                    ) : (
                      <section
                        key={`${object_.propertyId}-${objectIndex}`}
                        className="flex flex-col gap-3"
                      >
                        <Link
                          href={ROUTES.property(object_.propertyId)}
                          className={HEADER_LINK}
                        >
                          <PropertyHeaderContent
                            name={object_.propertyName}
                            photoUrl={objectOptions?.photoUrl ?? ''}
                            address={objectOptions?.address}
                          />
                        </Link>
                        <ActorCards actors={object_.actors} isMemberPage={isMemberPage} meId={meId} />
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
          title={isMemberPage || isPropertyPage ? pageTitle : undefined}
          pinnedParticipantId={participantId}
          pinnedPropertyId={propertyId}
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

/** Карточки актёров одной объектной группы (общие для всех скоупов):
 * isSelf — актёр = читатель (решение владельца 24.09, по id сессии),
 * headerHref — вход в «Действия участника» (#712: в общей ленте и на
 * «Истории объекта» #840 любая запись с actor_id кликабельна, владелец
 * включительно; на прибитых страницах шапки статичны — человек уже их
 * предмет; обезличенные записи, actor_id null, не ссылки нигде). */
function ActorCards({
  actors,
  isMemberPage,
  meId,
}: {
  readonly actors: readonly HistoryActorGroup[];
  readonly isMemberPage: boolean;
  readonly meId?: string;
}): JSX.Element {
  return (
    <div className="flex flex-col gap-1.5">
      {actors.map((actor, actorIndex) => (
        <ActorCard
          key={`${actor.key}-${actorIndex}`}
          actor={actor}
          isSelf={meId !== undefined && actor.actorId === meId}
          headerHref={
            !isMemberPage && actor.actorId !== null
              ? ROUTES.historyParticipant(actor.actorId)
              : null
          }
        />
      ))}
    </div>
  );
}

/** Серая карточка актёра (день → актёр → строки): шапка + строки записей.
 * isSelf — серый суффикс «(Вы)» в шапке, кликабельность не трогает. */
function ActorCard({
  actor,
  isSelf,
  headerHref,
}: {
  readonly actor: HistoryActorGroup;
  readonly isSelf: boolean;
  readonly headerHref: string | null;
}): JSX.Element {
  return (
    <div className="flex flex-col gap-2 rounded-m bg-surface-muted p-3">
      <ActorHeader name={actor.name} isSelf={isSelf} href={headerHref} />
      <div className="flex flex-col gap-2">
        {actor.entries.map((entry) => (
          <HistoryRow key={entry.id} entry={entry} />
        ))}
      </div>
    </div>
  );
}

/** Шапка-карточка прибитого объекта (#840, макет 2184-95600): одна над
 * всей лентой, статична — объект предмет страницы; зазор до ленты — 12. */
function PinnedPropertyHeader({
  object_,
}: {
  readonly object_: HistoryObjectOption;
}): JSX.Element {
  return (
    <div className="mb-3 flex min-w-0 items-center gap-2">
      <PropertyHeaderContent name={object_.name} photoUrl={object_.photoUrl} address={object_.address} />
    </div>
  );
}

/** Заголовок объектной группы ленты: аватар 24 (feed) + название + адрес.
 * Один контент для кликабельной шапки секции и статичной карточки пина. */
function PropertyHeaderContent({
  name,
  photoUrl,
  address,
}: {
  readonly name: string;
  readonly photoUrl: string;
  readonly address?: string;
}): JSX.Element {
  return (
    <>
      <PropertyAvatar photoUrl={photoUrl} surface="feed" />
      <span className="min-w-0">
        <h2 className="truncate text-xs font-medium leading-[15px] text-content">{name}</h2>
        {address && (
          <p className="truncate text-xs leading-[15px] text-content-secondary">{address}</p>
        )}
      </span>
    </>
  );
}

/** Шапка актёра в карточке: белый кружок-плейсхолдер + имя; с href —
 * ссылка на страницу участника, без — статичная шапка. Свой актёр
 * подписан серым суффиксом «(Вы)» — канон шита фильтров (#710). */
function ActorHeader({
  name,
  isSelf,
  href,
}: {
  readonly name: string;
  readonly isSelf: boolean;
  readonly href: string | null;
}): JSX.Element {
  const content: ReactNode = (
    <>
      <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-pill bg-white">
        <BoldUser className="h-3.5 w-3.5" aria-hidden />
      </span>
      <h3 className="min-w-0 truncate text-xs font-medium leading-[15px] text-content">
        {name}
        {isSelf && <span className="text-content-tertiary"> (Вы)</span>}
      </h3>
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
