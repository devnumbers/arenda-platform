'use client';

import { useEffect, useId, useState, type JSX, type ReactNode } from 'react';
import type { ApiError } from '@/shared/api/errors';
import type { UseQueryResult } from '@tanstack/react-query';
import {
  Add,
  BoldUser,
  Cancel,
  Check,
  CheckmarkCircle,
  Edit,
  EyeSmall,
  FullAccessSmall,
  HomeMain,
  Key,
  LockSmall,
  SmallArrowDown,
  SmallArrowUp,
  Team,
  TimeHistory,
  TrashBin,
  Undo,
  UserCircle,
  Wallet,
} from '@/shared/assets/icons';
import {
  baseActionLabel,
  HISTORY_KINDS,
  kindLabel,
  type HistoryBaseAction,
  type HistoryFilterOptions,
  type HistoryKind,
  type HistoryParticipantOption,
} from '@/entities/history';
import { PropertyAvatar } from '@/entities/property';
import { useMe } from '@/features/auth';
import {
  DEFAULT_HISTORY_FILTERS,
  historyParticipantTitle,
  historyPeriodChipLabel,
  toggleHistoryFilterGroup,
  toggleHistoryFilterOption,
  type HistoryFilters,
} from '@/features/history';
import { dateToIsoLocal, type IsoRange } from '@/shared/lib/calendar';
import {
  Button,
  CalendarRangePicker,
  Checkbox,
  ErrorCard,
  IconButton,
  StickyBottomBar,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import { HistoryFiltersSheetSkeleton } from './history-states';

/**
 * Шит «Настройки» — фильтры ленты «История действий» (#711; макеты
 * 2177-60527 — свёрнутый, 2067-163528/162950 — период и смешанные
 * состояния, 2050-158280 — раскрытые группы). Полноэкранный оверлей поверх
 * ленты (канон поверхностей: пикер-поверх экрана — состояние ленты не
 * теряется; каркас как у CalendarRangePicker: TopNav + внутренний скролл +
 * StickyBottomBar), рендерится только в открытом состоянии.
 *
 * Группы-чекбоксы со счётчиками N/M: «Основные действия», «Виды действий»,
 * «Участники» и «Объекты» (опции — GET /history/filters, #708). На
 * «Действиях участника» (#712) группа «Участники» показывает только
 * прибитого страницу человека — серым и незабираемым (disabled-чекбокс
 * канона Checkbox, макет 2184-92510, решение владельца 23.09); он не
 * часть черновика: «Сбросить»/«Применить» его не пишут. Строки —
 * подпись 14/16 с сепараторами #ebebeb (кроме последней), макет
 * 2067-163528; строка целиком кликабельна (label), цвет текста от
 * состояния чекбокса не зависит (аннотации макета 2050-158281), как и
 * область чекбокса шапки — паддинги на label, margin на кнопке съедает
 * preflight (#753). Участники: чужой — канон отображаемого имени, своя
 * строка —
 * только имя + серый суффикс «(Вы)» (по id сессии); владелец объектов
 * области (is_owner) — замок 16 перед почтой; у приглашённого без своих
 * объектов замка нет. Семантика
 * по аннотации макета («открывается сначала со свернутыми и всеми
 * выбранными категориями»): все выбрано = «все» (null) — сервер не
 * фильтрует; частичный выбор — только выбранное; ни одного — пустой
 * результат (модель history-filters). Мастер-чекбокс группы: все —
 * галочка, ни одного — пусто, частично — минус (indeterminate, макет
 * 2067-162950); тап переключает группу: «все» → «ни одного», иначе —
 * «все» (решение владельца 23.09 — из состояния «все» тап был но-опом).
 *
 * Черновик живёт, пока шит открыт: «Применить фильтры» коммитит его в
 * адрес одним push'ом («назад» возвращает без фильтров, DESIGN.md §3),
 * крестик закрывает без коммита, «Сбросить фильтры» возвращает черновик к
 * дефолту (все группы «все», без периода) — применением по-прежнему
 * остаётся «Применить».
 *
 * Период — чип над группами: без периода серый «Выбрать период ⌄», с
 * периодом синий с форматом чипа канона («1 окт — 20 дек», макет
 * 2067-162950); тап открывает канон CalendarRangePicker (пустой старт и
 * «Сбросить» — #670; чип прыжка скрыт, как у операций) поверх шита — пикер
 * правит черновик, применением остаётся «Применить фильтры».
 *
 * Опции участников/объектов едут с /history/filters: пока в пути —
 * скелетон с паритетом свёрнутого макета (§7). Если опции не приехали,
 * группы «Участники»/«Объекты» не рисуются, но шит остаётся рабочим —
 * период, действия, виды, сброс и применение правятся (находка ревью
 * #711: ошибка опций не делает шит мёртвым); ошибка — канон ErrorCard с
 * «Повторить» на месте этих групп.
 */

/** Строка группы: id значения, подпись, опциональный подзаголовок и
 * ведущая иконка/аватар (макет 2067-163528: 24-иконка действий/видов,
 * кружок 36 у участников, аватар объекта 32). isMe — суффикс «(Вы)»;
 * roleIcon — иконка роли 16 перед подзаголовком (#840, макет 2184-94261:
 * owner — замок, full_access — силуэт, viewer — глаз). */
type HistoryFilterOptionRow = {
  readonly id: string;
  readonly label: string;
  readonly subtitle?: string;
  readonly leading?: ReactNode;
  readonly leadingSize: 'icon' | 'participant' | 'object';
  /** Фото объекта для ведущего аватара (leadingSize 'object'). */
  readonly photoUrl?: string;
  readonly isMe?: boolean;
  readonly roleIcon?: HistoryParticipantOption['role'];
};

const BASE_ACTION_ICONS: Record<HistoryBaseAction, JSX.Element> = {
  added: <Add />,
  changed: <Edit />,
  completed: <Check />,
  deleted: <TrashBin />,
};

const KIND_ICONS: Record<HistoryKind, JSX.Element> = {
  property: <HomeMain />,
  rental: <Key />,
  payment: <Wallet />,
  operation: <TimeHistory />,
  contact: <UserCircle />,
  task: <CheckmarkCircle />,
  member: <Team />,
};

/** Каталог основных действий в порядке макета — тот же список идёт в
 * строки группы и в семантику «все, кроме переключённой» (null → список). */
const ACTION_IDS = ['added', 'changed', 'completed', 'deleted'] as const;

const ACTION_ROWS: ReadonlyArray<HistoryFilterOptionRow> = ACTION_IDS.map((id) => ({
  id,
  label: baseActionLabel(id),
  leading: BASE_ACTION_ICONS[id],
  leadingSize: 'icon',
}));

const KIND_ROWS: ReadonlyArray<HistoryFilterOptionRow> = HISTORY_KINDS.map((id) => ({
  id,
  label: kindLabel(id),
  leading: KIND_ICONS[id],
  leadingSize: 'icon',
}));

export type HistoryFiltersSheetProps = {
  /** Применённые фильтры ленты — стартовое состояние черновика. */
  readonly applied: HistoryFilters;
  readonly optionsQuery: UseQueryResult<HistoryFilterOptions, ApiError>;
  readonly onApply: (draft: HistoryFilters) => void;
  readonly onClose: () => void;
  /** Заголовок шита (шапка оверлея): на «Действиях участника» (#712) шит
   * поверх той же ленты — заголовок страницы, не общей. */
  readonly title?: string;
  /** Действия участника (#712, макет 2184-92510, решение владельца
   * 23.09): группа «Участники» показывает ТОЛЬКО прибитого человека —
   * серым (disabled-чекбокс канона Checkbox), тапы не проходят; он не
   * часть черновика — «Сбросить» и «Применить» его не трогают. */
  readonly pinnedParticipantId?: string;
  /** История объекта (#840, макет 2184-94176, решение владельца 24.09):
   * зеркало пина участника для группы «Объекты» — только прибитый
   * объект, серым и незабираемым; не часть черновика. */
  readonly pinnedPropertyId?: string;
};

export function HistoryFiltersSheet({
  applied,
  optionsQuery,
  onApply,
  onClose,
  title = 'История действий',
  pinnedParticipantId,
  pinnedPropertyId,
}: HistoryFiltersSheetProps): JSX.Element {
  const [draft, setDraft] = useState<HistoryFilters>(() => applied);
  // Раскрытие групп — локально, свёрнуто по умолчанию (аннотация макета
  // 2177-60527: страница открывается свёрнутой).
  const [expanded, setExpanded] = useState<Record<'actions' | 'kinds' | 'actors' | 'objects', boolean>>({
    actions: false,
    kinds: false,
    actors: false,
    objects: false,
  });
  const [periodPickerOpen, setPeriodPickerOpen] = useState(false);
  const today = dateToIsoLocal(new Date());
  // Своя строка участников — «Имя (Вы)» (макет 2067-163528): себя узнаём
  // по id сессии (канон списка участников объекта).
  const meQuery = useMe();
  const meId = meQuery.data?.id;

  const toggleExpanded = (key: 'actions' | 'kinds' | 'actors' | 'objects'): void => {
    setExpanded((state) => ({ ...state, [key]: !state[key] }));
  };

  // Esc закрывает шит; при открытом пикере периода его Esc закрывает пикер
  // (свой слушатель) — шит в этот момент не закрываем.
  useEffect(() => {
    if (periodPickerOpen) {
      return;
    }
    const onKeyDown = (event: KeyboardEvent): void => {
      if (event.key === 'Escape') {
        onClose();
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [periodPickerOpen, onClose]);

  // Строки «Участников»: в общей ленте — все опции; на «Действиях
  // участника» (#712) и на странице пары (#841) — только прибитый
  // человек (группа «1/1», макет 2184-92510). Пин не нашёлся в
  // загруженных опциях (мусорный id) — группа не рисуется вовсе:
  // показывать всех участников серыми значило бы врать о прибитости
  // (зеркало guard'а «Объектов» #840).
  const pinnedParticipant =
    pinnedParticipantId !== undefined
      ? (optionsQuery.data?.participants ?? []).find(
          (participant) => participant.id === pinnedParticipantId,
        )
      : undefined;
  const participantsSource =
    pinnedParticipantId !== undefined && pinnedParticipant === undefined
      ? []
      : pinnedParticipant !== undefined
        ? [pinnedParticipant]
        : (optionsQuery.data?.participants ?? []);

  const participantRows: ReadonlyArray<HistoryFilterOptionRow> = participantsSource.map(
    (participant) => ({
      id: participant.id,
      label: historyParticipantTitle(participant, participant.id === meId),
      subtitle: participant.email,
      leadingSize: 'participant',
      isMe: participant.id === meId,
      roleIcon: participant.isOwner ? 'owner' : participant.role,
    }),
  );

  const objectRows: ReadonlyArray<HistoryFilterOptionRow> = (
    optionsQuery.data?.objects ?? []
  ).map((object_) => ({
    id: object_.id,
    label: object_.name,
    subtitle: object_.address,
    leadingSize: 'object',
    photoUrl: object_.photoUrl,
  }));

  // Строки «Объектов»: в общей ленте — все опции; на «Истории объекта»
  // (#840, макет 2184-94176) — только прибитый объект (группа «1/1»,
  // зеркало «Участников» #712). Пин не нашёлся в загруженных опциях
  // (мусорный id) — группа не рисуется вовсе: показывать все объекты
  // серыми значило бы врать о прибитости.
  const pinnedObjectRow =
    pinnedPropertyId !== undefined
      ? objectRows.find((row) => row.id === pinnedPropertyId)
      : undefined;
  const objectsSource =
    pinnedPropertyId !== undefined && pinnedObjectRow === undefined
      ? []
      : pinnedObjectRow !== undefined
        ? [pinnedObjectRow]
        : objectRows;

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Фильтры истории"
      className="fixed inset-0 z-50 flex flex-col bg-surface font-sans"
    >
      <TopNav
        leading={<IconButton icon={<Cancel />} label="Закрыть фильтры" onClick={onClose} />}
      >
        <TopNavTitle title={title} />
      </TopNav>

      <div className="min-h-0 flex-1 overflow-y-auto">
        <div className="mx-auto w-full max-w-[560px] px-6 pb-[136px] pt-6 tablet:mt-[72px]">
          {optionsQuery.isPending ? (
            <HistoryFiltersSheetSkeleton />
          ) : (
            <div className="flex flex-col gap-4">
              {/* Чип периода: серый «Выбрать период» / синий с диапазоном
                * (макеты 2177-60527 / 2067-162950). */}
              {draft.period !== null ? (
                <Button
                  type="button"
                  size="small"
                  className="self-start gap-1.5"
                  onClick={() => setPeriodPickerOpen(true)}
                >
                  {historyPeriodChipLabel(draft.period)}
                  <SmallArrowDown className="h-6 w-6" aria-hidden />
                </Button>
              ) : (
                <Button
                  type="button"
                  variant="secondary"
                  size="small"
                  className="self-start gap-1.5"
                  onClick={() => setPeriodPickerOpen(true)}
                >
                  Выбрать период
                  <SmallArrowDown className="h-6 w-6" aria-hidden />
                </Button>
              )}

              <FilterGroupCard
                title="Основные действия"
                rows={ACTION_ROWS}
                selected={draft.actions}
                expanded={expanded.actions}
                onToggleExpanded={() => toggleExpanded('actions')}
                onToggleAll={(actions) => setDraft((state) => ({ ...state, actions: actions as HistoryBaseAction[] }))}
                onToggleOption={(id) =>
                  setDraft((state) => ({
                    ...state,
                    actions: toggleHistoryFilterOption(
                      ACTION_IDS,
                      state.actions,
                      id as HistoryBaseAction,
                    ),
                  }))
                }
              />
              <FilterGroupCard
                title="Виды действий"
                rows={KIND_ROWS}
                selected={draft.kinds}
                expanded={expanded.kinds}
                onToggleExpanded={() => toggleExpanded('kinds')}
                onToggleAll={(kinds) => setDraft((state) => ({ ...state, kinds: kinds as HistoryKind[] }))}
                onToggleOption={(id) =>
                  setDraft((state) => ({
                    ...state,
                    kinds: toggleHistoryFilterOption(HISTORY_KINDS, state.kinds, id as HistoryKind),
                  }))
                }
              />
              {optionsQuery.isError ? (
                /* Опции участников/объектов не приехали: их группы не
                  * рисуются, остальные фильтры и применение работают
                  * (правка ревью #711 — черновик правится и без опций). */
                <ErrorCard
                  title="Не удалось загрузить фильтры"
                  onRetry={() => void optionsQuery.refetch()}
                />
              ) : (
                <>
                  {(pinnedParticipantId === undefined || participantRows.length > 0) && (
                    <FilterGroupCard
                      title="Участники"
                      rows={participantRows}
                      selected={draft.actorIds}
                      disabled={pinnedParticipantId !== undefined}
                      expanded={expanded.actors}
                      onToggleExpanded={() => toggleExpanded('actors')}
                      onToggleAll={(actorIds) => setDraft((state) => ({ ...state, actorIds }))}
                      onToggleOption={(id) =>
                        setDraft((state) => ({
                          ...state,
                          actorIds: toggleHistoryFilterOption(
                            participantRows.map((row) => row.id),
                            state.actorIds,
                            id,
                          ),
                        }))
                      }
                    />
                  )}
                  {(pinnedPropertyId === undefined || objectsSource.length > 0) && (
                    <FilterGroupCard
                      title="Объекты"
                      rows={objectsSource}
                      selected={draft.propertyIds}
                      disabled={pinnedPropertyId !== undefined}
                      expanded={expanded.objects}
                      onToggleExpanded={() => toggleExpanded('objects')}
                      onToggleAll={(propertyIds) => setDraft((state) => ({ ...state, propertyIds }))}
                      onToggleOption={(id) =>
                        setDraft((state) => ({
                          ...state,
                          propertyIds: toggleHistoryFilterOption(
                            objectsSource.map((row) => row.id),
                            state.propertyIds,
                            id,
                          ),
                        }))
                      }
                    />
                  )}
                </>
              )}

              {/* Сброс возвращает черновик к дефолту (все группы «все», без
                * периода); коммит — «Применить фильтры». */}
              <Button type="button" variant="white" className="w-full text-base" onClick={() => setDraft(DEFAULT_HISTORY_FILTERS)}>
                <Undo className="h-6 w-6" aria-hidden />
                Сбросить фильтры
              </Button>
            </div>
          )}
        </div>
      </div>

      {/* Применение видно всегда, когда шит редактируем (§7: панель
        * скрывается вместо погашенной кнопки; черновик правится и при
        * ошибке опций — находка ревью #711). */}
      {!optionsQuery.isPending && (
        <StickyBottomBar>
          <Button type="button" className="w-full" onClick={() => onApply(draft)}>
            Применить фильтры
          </Button>
        </StickyBottomBar>
      )}

      {periodPickerOpen && (
        <CalendarRangePicker
          today={today}
          value={draft.period}
          monthJump={false}
          onClose={() => setPeriodPickerOpen(false)}
          onConfirm={(range: IsoRange) => {
            setDraft((state) => ({ ...state, period: range }));
            setPeriodPickerOpen(false);
          }}
          onReset={() => {
            setDraft((state) => ({ ...state, period: null }));
            setPeriodPickerOpen(false);
          }}
        />
      )}
    </div>
  );
}

/** Карточка группы (макет 2177-60527): серый блок radius 24, шапка —
 * мастер-чекбокс 24 + кнопка раскрытия с заголовком 16/18, счётчиком N/M и
 * шевроном; в раскрытом — строки опций по 56. disabled (#712 — прибитый
 * участник): чекбоксы серые, тапы по ним не проходят; раскрытие работает. */
function FilterGroupCard({
  title,
  rows,
  selected,
  expanded,
  disabled = false,
  onToggleExpanded,
  onToggleAll,
  onToggleOption,
}: {
  readonly title: string;
  readonly rows: ReadonlyArray<HistoryFilterOptionRow>;
  readonly selected: ReadonlyArray<string> | null;
  readonly expanded: boolean;
  readonly disabled?: boolean;
  readonly onToggleExpanded: () => void;
  readonly onToggleAll: (next: ReadonlyArray<string> | null) => void;
  readonly onToggleOption: (id: string) => void;
}): JSX.Element {
  const count = selected === null ? rows.length : selected.length;
  const masterId = useId();
  return (
    <section className="rounded-3xl bg-surface-muted py-1">
      <div className="flex min-h-14 items-center">
        {/* Область чекбокса — сама кликабельная зона (аннотация макета
          * 2050-158281: «нажимается по этой области, при свернутом виде
          * тоже»): паддинги на label, не margin на кнопке — иначе
          * preflight `button{margin:0}` их съедает (#753). */}
        <label htmlFor={masterId} className="flex cursor-pointer items-center py-4 pl-5 pr-2">
          <Checkbox
            id={masterId}
            className="h-6 w-6"
            checked={selected === null ? true : selected.length > 0 ? 'indeterminate' : false}
            disabled={disabled}
            onCheckedChange={() => onToggleAll(toggleHistoryFilterGroup(selected))}
            aria-label={`Выбрать все: ${title}`}
          />
        </label>
        <button
          type="button"
          className="flex min-w-0 flex-1 cursor-pointer items-center py-4 pl-2 pr-5 text-left outline-none focus-visible:ring-4 focus-visible:ring-primary"
          onClick={onToggleExpanded}
          aria-expanded={expanded}
        >
          <span className="min-w-0 flex-1 truncate text-base font-medium leading-[18px] text-content">
            {title}
          </span>
          <span className="shrink-0 text-[13px] font-medium leading-[15px] text-content-tertiary">
            {count}/{rows.length}
          </span>
          {expanded ? (
            <SmallArrowUp className="ml-1 h-6 w-6 shrink-0" aria-hidden />
          ) : (
            <SmallArrowDown className="ml-1 h-6 w-6 shrink-0" aria-hidden />
          )}
        </button>
      </div>
      {expanded && (
        <div className="pb-1">
          {rows.map((row, index) => (
            <FilterOptionRow
              key={row.id}
              row={row}
              last={index === rows.length - 1}
              checked={selected === null || selected.includes(row.id)}
              disabled={disabled}
              onToggle={() => onToggleOption(row.id)}
            />
          ))}
        </div>
      )}
    </section>
  );
}

/** Строка опции (макет 2067-163528): чекбокс 24 на 44 от края, ведущая
 * иконка/аватар, подпись 14/16 + подзаголовок 13/15; между строками —
 * сепаратор #ebebeb на контейнере текста (кроме последней строки).
 * Вся строка — label: клик в любое место переключает опцию; цвет текста
 * от состояния чекбокса не зависит (макет 2050-158281: невыбранная
 * остаётся чёрной). У себя — суффикс «(Вы)» серым; перед подзаголовком —
 * иконка роли 16 (#840, макет 2184-94261). Disabled (#712): чекбокс
 * серый, текст приглушён, тапы не проходят. */
function FilterOptionRow({
  row,
  last,
  checked,
  disabled = false,
  onToggle,
}: {
  readonly row: HistoryFilterOptionRow;
  readonly last: boolean;
  readonly checked: boolean;
  readonly disabled?: boolean;
  readonly onToggle: () => void;
}): JSX.Element {
  const checkboxId = useId();
  return (
    <label
      htmlFor={checkboxId}
      className={`flex min-h-14 items-center pl-11 pr-5 ${disabled ? 'cursor-default' : 'cursor-pointer'}`}
    >
      <Checkbox
        id={checkboxId}
        className="h-6 w-6"
        checked={checked}
        disabled={disabled}
        onCheckedChange={onToggle}
        aria-label={row.isMe === true ? `${row.label} (Вы)` : row.label}
      />
      {row.leadingSize === 'icon' && (
        <span className="ml-4 flex h-6 w-6 shrink-0 items-center justify-center text-content">
          {row.leading}
        </span>
      )}
      {row.leadingSize === 'participant' && (
        <span className="ml-4 flex h-9 w-9 shrink-0 items-center justify-center rounded-pill bg-white">
          <BoldUser className="h-4 w-4" aria-hidden />
        </span>
      )}
      {row.leadingSize === 'object' && (
        <span className="ml-4 shrink-0">
          <PropertyAvatar photoUrl={row.photoUrl} surface="filter" />
        </span>
      )}
      <span
        className={`ml-3 flex min-w-0 flex-1 flex-col justify-center gap-1 self-stretch ${last ? '' : 'border-b border-[#ebebeb]'}`}
      >
        <span
          className={`truncate text-sm font-medium leading-4 ${disabled ? 'text-content-tertiary' : 'text-content'}`}
        >
          {row.label}
          {row.isMe === true && <span className="text-content-tertiary"> (Вы)</span>}
        </span>
        {row.subtitle !== undefined && (
          <span className="flex items-center gap-1">
            {row.roleIcon !== undefined && <RoleIcon role={row.roleIcon} />}
            <span className="truncate text-[13px] leading-[15px] text-content-tertiary">
              {row.subtitle}
            </span>
          </span>
        )}
      </span>
    </label>
  );
}

/** Иконка роли 16 перед подзаголовком строки участника (#840, макет
 * 2184-94261): owner — замок (канон #711), full_access — силуэт,
 * viewer — глаз (канон «Просмотра», #698). Роль — смысловая информация
 * строки, поэтому иконка несёт accessible name, а не aria-hidden. */
function RoleIcon({ role }: { readonly role: HistoryParticipantOption['role'] }): JSX.Element {
  const className = 'h-4 w-4 shrink-0 text-content-tertiary';
  if (role === 'owner') {
    return <LockSmall className={className} role="img" aria-label="Роль: владелец" />;
  }
  if (role === 'viewer') {
    return <EyeSmall className={className} role="img" aria-label="Роль: просмотр" />;
  }
  return <FullAccessSmall className={className} role="img" aria-label="Роль: полный доступ" />;
}
