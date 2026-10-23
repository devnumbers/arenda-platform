import { ROUTES } from '@/shared/config/routes';
import { ArrowLeft, Cancel, Check, Search, TrashBin } from '@/shared/assets/icons';
import {
  IconButton,
  SearchField,
  StepsChip,
  TopNavBackButton,
  TopNavTitle,
  type TopNavProps,
} from '@/shared/ui/design';
import {
  WIZARD_TOTAL_STEPS as PAYMENT_WIZARD_STEPS,
  wizardTotalSteps,
} from '@/features/payments';
import { WIZARD_TOTAL_STEPS as RENTAL_WIZARD_STEPS } from '@/features/rentals';
import { PROPERTY_CREATE_TOTAL_STEPS } from '@/features/properties';
import {
  historyScreenBackHref,
  historyScreenTitle,
} from '@/features/history';

/** Реестр «маршрут → статическая конфигурация шапки» (решение владельца
 * 23.10.2026, issue #1592 — гибрид реестр+шим; полный каталог — тикет
 * #1605): база для первого кадра. Шапка живёт в ScreenLayout и не может
 * взять пропы экрана на SSR — экранные регистрации доезжают после
 * гидратации. Реестр отдаёт статику и анатомию по pathname: вариант
 * `search`, крылья, компакт-конфиг хабов, статичные тайтлы, ведущая
 * «Назад». Динамика (тайтлы из данных, поисковый ввод, действия-колбэки,
 * право-гейтнутые кнопки) — только через шим, реестр её не знает; после
 * гидратации шим перебивает реестр целиком по каждому полю
 * (shallow-merge, mergeTopNavConfig).
 *
 * Источник каждой записи — статический хром самого экрана в фазе
 * загрузки (канон §7): глифы кнопок без onClick, тайтл-плейсхолдер,
 * неконтролируемое поле поиска. Экраны-ветвления (визарды, состояния
 * ошибки) дают реестру свой loading-хром — сценарий по умолчанию; шим
 * приносит фактическое состояние. Записи нет у маршрутов без шапки
 * вообще: success-экран смены тарифа — полноэкранная поверхность без
 * TopNav; /login живёт вне ScreenLayout; overlay-поверхности рендерят
 * шапку локально (Q7) и реестру не подчиняются. */

/** Сегмент динамического параметра pathname → значение параметра
 * страницы: Next декодирует params страницы, usePathname держит сырой
 * сегмент. Выравниваем для идентификаторов, которые маршрутизаторы
 * кодируют сами (участники — email в pending-состоянии, ROUTES.participant
 * делает encodeURIComponent): без декодирования фолбэк уехал бы на
 * дважды закодированный адрес. Для uuid-сегментов декодирование —
 * no-op; битая escape-последовательность отдаётся как есть. */
function decodeSegment(value: string | undefined): string | undefined {
  if (value === undefined) return undefined;
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

/** Паттерн маршрута Next.js: статические сегменты сходятся точно,
 * `[param]` — любой непустой сегмент. Число сегментов обязательно
 * совпадает. */
function matchRoutePattern(pattern: string, pathname: string): Record<string, string> | undefined {
  const patternSegments = pattern.split('/');
  const pathSegments = pathname.split('/');
  if (patternSegments.length !== pathSegments.length) return undefined;
  const params: Record<string, string> = {};
  for (const [index, segment] of patternSegments.entries()) {
    const pathSegment = pathSegments[index];
    if (pathSegment === undefined) return undefined;
    if (segment.startsWith('[') && segment.endsWith(']')) {
      if (pathSegment === '') return undefined;
      params[segment.slice(1, -1)] = pathSegment;
      continue;
    }
    if (segment !== pathSegment) return undefined;
  }
  return params;
}

/** Запись реестра: паттерн + сборщик статических пропов. Параметры
 * приходят только от совпавшего паттерна — сегмент `[x]` по построению
 * непустой. */
type TopNavRouteEntry = {
  readonly pattern: string;
  readonly build: (params: Readonly<Record<string, string>>) => TopNavProps;
};

/** Глиф-кнопка ведущего слота первого кадра: без onClick — переход
 * приносит экран своим шимом после гидратации (канон loading-хрома §7).
 * Общие для десятков записей элементы — константы модуля: React-элементы
 * иммутабельны и переиспользуемы между записями и рендерами. */
const BACK_GLYPH: TopNavProps['leading'] = <IconButton icon={<ArrowLeft />} label="Назад" />;
const CLOSE_GLYPH: TopNavProps['leading'] = <IconButton icon={<Cancel />} label="Закрыть" />;

const CANCEL_EDIT_GLYPH: TopNavProps['leading'] = (
  <IconButton icon={<Cancel />} label="Отменить правку" />
);
const CANCEL_CREATE_GLYPH: TopNavProps['leading'] = (
  <IconButton icon={<Cancel />} label="Отменить создание" />
);
const CLOSE_SEARCH_GLYPH: TopNavProps['leading'] = (
  <IconButton icon={<ArrowLeft />} label="Закрыть поиск" />
);

const ENTRIES: readonly TopNavRouteEntry[] = [
  // ——— Хабы: крылья и на мобайле, компакт с пилюлей поиска (динамический
  // trailing «+»/кебаб приносит экран своим шимом — право-гейтнутый). —
  {
    // Хаб «Платежи» (#578): лупа поиска платежей.
    pattern: ROUTES.payments,
    build: () => ({
      mobileWings: true,
      collapse: {
        title: 'Платежи',
        search: { href: ROUTES.paymentsSearch, label: 'Найти платеж' },
      },
    }),
  },
  {
    // Лента «Операции» (#541): лупа поиска операций.
    pattern: ROUTES.operations,
    build: () => ({
      mobileWings: true,
      collapse: {
        title: 'Операции',
        search: { href: ROUTES.operationsSearch, label: 'Найти операцию' },
      },
    }),
  },
  {
    // Хаб «Объекты» (#563): пилюля поиска объектов (#586).
    pattern: ROUTES.properties,
    build: () => ({
      mobileWings: true,
      collapse: {
        title: 'Объекты',
        search: { href: ROUTES.propertySearch, label: 'Найти объект' },
      },
    }),
  },
  {
    // Лента «Задачи» (#523): хаб без поисковой пилюли.
    pattern: ROUTES.tasks,
    build: () => ({ mobileWings: true, collapse: { title: 'Задачи' } }),
  },
  {
    // Книга контактов (#508): лупа поиска контактов (#1238).
    pattern: ROUTES.contacts,
    build: () => ({
      mobileWings: true,
      collapse: {
        title: 'Контакты',
        search: { href: ROUTES.contactSearch, label: 'Найти контакт' },
      },
    }),
  },
  {
    // Раздел «Участники» (#1313): лупа поиска участников.
    pattern: ROUTES.participants,
    build: () => ({
      mobileWings: true,
      collapse: {
        title: 'Участники',
        search: { href: ROUTES.participantsSearch, label: 'Найти участника' },
      },
    }),
  },
  {
    // Центр уведомлений (#744): хаб без поисковой пилюли.
    pattern: ROUTES.notifications,
    build: () => ({ mobileWings: true, collapse: { title: 'Уведомления' } }),
  },

  // ——— Хаб «Профиль»: анатомия подэкрана без «Назад», крылья только
  // на ПК (решение владельца 02.10) —
  {
    pattern: ROUTES.profile,
    build: () => ({
      hideWingsBelowDesktop: true,
      children: <TopNavTitle title="Профиль" />,
    }),
  },

  // ——— Поисковые шапки статических маршрутов: вариант search + глиф
  // «Назад» + неконтролируемое поле (ввод приносит экран) —
  {
    pattern: ROUTES.paymentsSearch,
    build: () => ({
      variant: 'search',
      leading: BACK_GLYPH,
      children: <SearchField placeholder="Поиск платежа" aria-label="Поиск платежа" />,
    }),
  },
  {
    pattern: ROUTES.operationsSearch,
    build: () => ({
      variant: 'search',
      leading: BACK_GLYPH,
      children: <SearchField placeholder="Найти операцию" aria-label="Найти операцию" />,
    }),
  },
  {
    pattern: ROUTES.paymentsObjectsSearch,
    build: () => ({
      variant: 'search',
      leading: BACK_GLYPH,
      children: <SearchField placeholder="Найти объект" aria-label="Найти объект" />,
    }),
  },
  {
    pattern: ROUTES.contactSearch,
    build: () => ({
      variant: 'search',
      leading: CLOSE_SEARCH_GLYPH,
      children: <SearchField placeholder="Найти контакт" aria-label="Поиск контактов" />,
    }),
  },
  {
    pattern: ROUTES.participantsSearch,
    build: () => ({
      variant: 'search',
      leading: CLOSE_SEARCH_GLYPH,
      children: <SearchField placeholder="Найти участника" aria-label="Поиск участников" />,
    }),
  },
  {
    pattern: ROUTES.propertySearch,
    build: () => ({
      variant: 'search',
      leading: CLOSE_SEARCH_GLYPH,
      children: <SearchField placeholder="Найти объект" aria-label="Поиск объектов" />,
    }),
  },
  {
    // Пикер часового пояса (#594): поисковая шапка аккаунта.
    pattern: ROUTES.profileAccountTimezone,
    build: () => ({
      variant: 'search',
      leading: BACK_GLYPH,
      children: <SearchField placeholder="Часовой пояс" aria-label="Поиск часового пояса" />,
    }),
  },

  // ——— Операции объекта (#474) —
  {
    pattern: '/properties/[id]/operations',
    build: () => ({ leading: BACK_GLYPH, children: <TopNavTitle title="Операции объекта" /> }),
  },
  {
    pattern: '/properties/[id]/operations/income',
    build: () => ({ leading: BACK_GLYPH, children: <TopNavTitle title="Доходы объекта" /> }),
  },
  {
    pattern: '/properties/[id]/operations/expense',
    build: () => ({ leading: BACK_GLYPH, children: <TopNavTitle title="Расходы объекта" /> }),
  },
  {
    pattern: '/properties/[id]/operations/search',
    build: () => ({
      variant: 'search',
      leading: BACK_GLYPH,
      children: <SearchField placeholder="Найти операцию" aria-label="Найти операцию" />,
    }),
  },
  {
    pattern: '/properties/[id]/operations/categories',
    build: () => ({ leading: CLOSE_GLYPH, children: <TopNavTitle title="Выбрать категорию" /> }),
  },
  {
    pattern: '/properties/[id]/operations/new',
    build: () => ({ leading: CLOSE_GLYPH, children: <TopNavTitle title="Добавить операцию" /> }),
  },
  {
    // Деталь операции: фолбэк «Назад» — список платежей объекта (как у
    // живого экрана); тайтл-плейсхолдер — канон loading-хрома. Динамический
    // [operationId] — после статических подпутей операций: матчер идёт по
    // порядку записей (в Next.js статики выигрывают у динамик — здесь то
    // же правило порядком).
    pattern: '/properties/[id]/operations/[operationId]',
    build: (params) => ({
      leading: (
        <TopNavBackButton fallbackHref={ROUTES.propertyPayments(params.id ?? '')} />
      ),
      children: <TopNavTitle title="Операция" />,
    }),
  },

  // ——— Платежи объекта (#463) и его секции (#466) —
  {
    pattern: '/properties/[id]/payments',
    build: () => ({ leading: BACK_GLYPH, children: <TopNavTitle title="Платежи объекта" /> }),
  },
  {
    pattern: '/properties/[id]/payments/overdue',
    build: () => ({ leading: BACK_GLYPH, children: <TopNavTitle title="Просроченные платежи" /> }),
  },
  {
    // Секции-каталоги (#1043): «Все правила» и «Автоплатежи» делят хром,
    // тайтл — из CATALOG экрана.
    pattern: '/properties/[id]/payments/all',
    build: () => ({ leading: BACK_GLYPH, children: <TopNavTitle title="Платежи объекта" /> }),
  },
  {
    pattern: '/properties/[id]/payments/auto',
    build: () => ({ leading: BACK_GLYPH, children: <TopNavTitle title="Автоплатежи объекта" /> }),
  },
  {
    // Визард создания платежа (объектный вход): шаг 1 из 5 — канон
    // loading-хрома визарда.
    pattern: '/properties/[id]/payments/new',
    build: () => ({
      leading: BACK_GLYPH,
      children: <StepsChip step={1} total={PAYMENT_WIZARD_STEPS} size="m" />,
    }),
  },
  {
    // Страница платежа (#465): «Назад» к списку платежей объекта и
    // тайтл-плейсхолдер; настоящий тайтл («Автоплатёж»/«Платеж»)
    // приносит экран.
    pattern: '/properties/[id]/payments/[paymentId]',
    build: (params) => ({
      leading: (
        <TopNavBackButton fallbackHref={ROUTES.propertyPayments(params.id ?? '')} />
      ),
      children: <TopNavTitle title="Платеж" />,
    }),
  },
  {
    // Правка платежа (#467): хром правки — сценарий по умолчанию фазы
    // загрузки (роль неизвестна); фактическое состояние приносит шим.
    pattern: '/properties/[id]/payments/[paymentId]/edit',
    build: () => ({
      leading: CANCEL_EDIT_GLYPH,
      children: <TopNavTitle title="Платеж" subtitle="Редактирование" />,
    }),
  },
  {
    pattern: '/properties/[id]/payments/[paymentId]/history',
    build: () => ({ leading: BACK_GLYPH, children: <TopNavTitle title="История платежа" /> }),
  },
  {
    pattern: '/properties/[id]/payments/[paymentId]/overdue',
    build: () => ({ leading: BACK_GLYPH, children: <TopNavTitle title="Просроченные платежи" /> }),
  },
  {
    pattern: '/properties/[id]/payments/[paymentId]/schedule',
    build: () => ({ leading: BACK_GLYPH, children: <TopNavTitle title="График платежей" /> }),
  },
  {
    // Проекция будущего вхождения: дата — из данных, плейсхолдер — канон
    // loading-хрома.
    pattern: '/properties/[id]/payments/[paymentId]/operations/projected/[date]',
    build: () => ({ leading: BACK_GLYPH, children: <TopNavTitle title="Операция" /> }),
  },

  // ——— Аренды (#530–#535) —
  {
    pattern: '/properties/[id]/rentals',
    build: () => ({ leading: BACK_GLYPH, children: <TopNavTitle title="Аренда" /> }),
  },
  {
    // Визард создания аренды: шаг 1 из 4.
    pattern: '/properties/[id]/rentals/new',
    build: () => ({
      leading: CLOSE_GLYPH,
      children: <StepsChip step={1} total={RENTAL_WIZARD_STEPS} size="m" />,
    }),
  },
  {
    // Шаг «Контакт арендатора» (#807): поисковая шапка мастера.
    pattern: '/properties/[id]/rentals/new/contact',
    build: () => ({
      variant: 'search',
      leading: BACK_GLYPH,
      children: <SearchField placeholder="Найти контакт" aria-label="Поиск контактов" />,
    }),
  },
  {
    pattern: '/properties/[id]/rentals/terms',
    build: () => ({ leading: BACK_GLYPH, children: <TopNavTitle title="Условия аренды" /> }),
  },
  {
    pattern: '/properties/[id]/rentals/terms/edit',
    build: () => ({ leading: CANCEL_EDIT_GLYPH, children: <TopNavTitle title="Изменить условия" /> }),
  },
  {
    // Продление (#533): бар с отменой, тайтла нет.
    pattern: '/properties/[id]/rentals/extend',
    build: () => ({
      leading: <IconButton icon={<Cancel />} label="Отменить продление" />,
    }),
  },
  {
    // Завершение (#534): каркас несёт пустой бар — шаги мастера приносит
    // шим (компонент держит `<TopNav />` во всех ветках-гардах).
    pattern: '/properties/[id]/rentals/complete',
    build: () => ({}),
  },
  {
    pattern: '/properties/[id]/rentals/past',
    build: () => ({ leading: BACK_GLYPH, children: <TopNavTitle title="Прошлые аренды" /> }),
  },
  {
    // Завершённая детализация (#535): тайтл-плейсхолдер с сабтайтлом.
    pattern: '/properties/[id]/rentals/[rentalId]',
    build: (params) => ({
      leading: (
        <TopNavBackButton fallbackHref={ROUTES.propertyRentalPast(params.id ?? '')} />
      ),
      children: <TopNavTitle title="Аренда" subtitle="Завершена" />,
    }),
  },
  {
    pattern: '/properties/[id]/rentals/[rentalId]/history',
    build: (params) => ({
      leading: (
        <TopNavBackButton fallbackHref={ROUTES.propertyRentalPast(params.id ?? '')} />
      ),
      children: <TopNavTitle title="История операций" />,
    }),
  },
  {
    pattern: '/properties/[id]/rentals/[rentalId]/summary',
    build: () => ({ leading: CLOSE_GLYPH, children: <TopNavTitle title="Итоги аренды" /> }),
  },

  // ——— Задачи (#499, #500, #502, лента #523) —
  {
    pattern: '/properties/[id]/tasks',
    build: () => ({ leading: BACK_GLYPH, children: <TopNavTitle title="Задачи объекта" /> }),
  },
  {
    pattern: '/properties/[id]/tasks/new',
    build: () => ({ leading: CLOSE_GLYPH, children: <TopNavTitle title="Создать задачу" /> }),
  },
  {
    pattern: '/properties/[id]/tasks/[ruleId]/edit',
    build: () => ({ leading: CLOSE_GLYPH, children: <TopNavTitle title="Изменить задачу" /> }),
  },
  {
    pattern: ROUTES.taskCreate,
    build: () => ({ leading: CLOSE_GLYPH, children: <TopNavTitle title="Создать задачу" /> }),
  },
  {
    pattern: '/tasks/[ruleId]/edit',
    build: () => ({ leading: CLOSE_GLYPH, children: <TopNavTitle title="Изменить задачу" /> }),
  },

  // ——— Контакты (#508–#510, плоская книга) —
  {
    pattern: ROUTES.contactNew,
    build: () => ({
      leading: CANCEL_CREATE_GLYPH,
      trailing: <IconButton icon={<Check />} label="Создать контакт" />,
      children: <TopNavTitle title="Создать контакт" />,
    }),
  },
  {
    pattern: '/contacts/[contactId]',
    build: () => ({ leading: BACK_GLYPH, children: <TopNavTitle title="Контакт" /> }),
  },
  {
    pattern: '/contacts/[contactId]/edit',
    build: () => ({ leading: BACK_GLYPH, children: <TopNavTitle title="Изменить контакт" /> }),
  },
  {
    pattern: '/properties/[id]/contacts',
    build: () => ({ leading: BACK_GLYPH, children: <TopNavTitle title="Контакты объекта" /> }),
  },
  {
    pattern: '/properties/[id]/contacts/new',
    build: () => ({
      leading: CANCEL_CREATE_GLYPH,
      trailing: <IconButton icon={<Check />} label="Создать контакт" />,
      children: <TopNavTitle title="Создать контакт" />,
    }),
  },
  {
    pattern: '/properties/[id]/contacts/[contactId]',
    build: () => ({ leading: BACK_GLYPH, children: <TopNavTitle title="Контакт" /> }),
  },
  {
    pattern: '/properties/[id]/contacts/[contactId]/edit',
    build: () => ({ leading: BACK_GLYPH, children: <TopNavTitle title="Изменить контакт" /> }),
  },

  // ——— Участники (#698, #699, #700; D1 #1313) —
  {
    pattern: ROUTES.participantsInvite,
    build: () => ({ leading: <TopNavBackButton fallbackHref={ROUTES.participants} /> }),
  },
  // Страница участника (/participants/[participantId]) — БЕЗ записи
  // реестра (шим приносит шапку после гидратации, паритет с
  // до-реестровым днём). Причина: SSR-хедер на этом маршруте обостряет
  // гонку стриминга и гидратации Next.js — контент ServerPrefetchBoundary
  // (#887) приходит вторым флушем в шаблоне div#S:0[hidden], и при
  // гидратации поверх уже заменённого $RC-фолбэка React на ~200 мс
  // вставляет контент повторно (дубль hero-блока страницы; e2e
  // participants-page.spec.ts:34 ловит strict-mode на тексте). На остальных
  // #887-страницах с SSR-шапкой (/profile, /properties/[id]) гонка
  // зондируется чисто; здесь $RC стоит в самом хвосте HTML — окно
  // максимальное. Разбирается отдельно от реестра (стриминг Next.js,
  // владелец).
  {
    pattern: '/participants/[participantId]/invite',
    build: () => ({
      leading: <IconButton icon={<Cancel />} label="Закрыть приглашение" />,
      children: <TopNavTitle title="Пригласить в объект" />,
    }),
  },
  {
    // «Права участника» (#698): идентификатор участника декодируется —
    // ROUTES.participant кодирует сам (email в pending-состоянии).
    pattern: '/participants/[participantId]/properties/[propertyId]',
    build: (params) => ({
      leading: (
        <TopNavBackButton
          fallbackHref={ROUTES.participant(decodeSegment(params.participantId) ?? '')}
        />
      ),
      children: <TopNavTitle title="Права участника" />,
    }),
  },
  {
    // «Действия участника» (#1582): скоуп одной страницы.
    pattern: '/participants/[participantId]/history',
    build: (params) => {
      const participantId = decodeSegment(params.participantId);
      return {
        leading: (
          <TopNavBackButton fallbackHref={historyScreenBackHref(participantId)} />
        ),
        children: <TopNavTitle title={historyScreenTitle(participantId)} />,
      };
    },
  },
  {
    pattern: '/properties/[id]/participants',
    build: (params) => ({
      leading: <TopNavBackButton fallbackHref={ROUTES.property(params.id ?? '')} />,
      children: <TopNavTitle title="Участники объекта" />,
    }),
  },
  {
    pattern: '/properties/[id]/participants/invite',
    build: (params) => ({
      leading: (
        <TopNavBackButton fallbackHref={ROUTES.propertyParticipants(params.id ?? '')} />
      ),
    }),
  },
  {
    // «Права участника» в member-режиме (#1362): назад к участникам объекта.
    pattern: '/properties/[id]/participants/[memberId]',
    build: (params) => ({
      leading: (
        <TopNavBackButton fallbackHref={ROUTES.propertyParticipants(params.id ?? '')} />
      ),
      children: <TopNavTitle title="Права участника" />,
    }),
  },

  // ——— История (#704, #840, #841): личность ленты — чистая функция
  // features/history/lib/screen-chrome, одна точка с живым экраном —
  {
    pattern: ROUTES.history,
    build: () => ({
      leading: <TopNavBackButton fallbackHref={historyScreenBackHref()} />,
      children: <TopNavTitle title={historyScreenTitle()} />,
    }),
  },
  {
    pattern: '/history/properties/[propertyId]',
    build: (params) => ({
      leading: (
        <TopNavBackButton
          fallbackHref={historyScreenBackHref(undefined, params.propertyId)}
        />
      ),
      children: <TopNavTitle title={historyScreenTitle(undefined, params.propertyId)} />,
    }),
  },
  {
    pattern: '/history/participants/[participantId]/properties/[propertyId]',
    build: (params) => {
      const participantId = decodeSegment(params.participantId);
      return {
        leading: (
          <TopNavBackButton
            fallbackHref={historyScreenBackHref(participantId, params.propertyId)}
          />
        ),
        children: <TopNavTitle title={historyScreenTitle(participantId, params.propertyId)} />,
      };
    },
  },

  // ——— Уведомления (#745): деталь с корзиной; тайтла у экрана нет —
  {
    pattern: '/notifications/[notificationId]',
    build: () => ({
      leading: CLOSE_GLYPH,
      trailing: <IconButton icon={<TrashBin />} label="Удалить" />,
    }),
  },

  // ——— Профиль: SubScreenShell-каркасы страниц (title/fallback —
  // пропы страниц, реестр их повторяет) —
  {
    pattern: ROUTES.profileAccount,
    build: () => ({
      leading: <TopNavBackButton fallbackHref={ROUTES.profile} />,
      children: <TopNavTitle title="Аккаунт" />,
    }),
  },
  {
    // Флоу смены почты (#722): «Назад» на шаг и «Закрыть» — оба глифа.
    pattern: ROUTES.profileChangeEmail,
    build: () => ({
      leading: BACK_GLYPH,
      trailing: <IconButton icon={<Cancel />} label="Закрыть" />,
      children: <TopNavTitle title="Смена почты" />,
    }),
  },
  {
    pattern: ROUTES.profileDevices,
    build: () => ({
      leading: <TopNavBackButton fallbackHref={ROUTES.profile} />,
      children: <TopNavTitle title="Устройства" />,
    }),
  },
  {
    pattern: ROUTES.profileNotifications,
    build: () => ({
      leading: <TopNavBackButton fallbackHref={ROUTES.profile} />,
      children: <TopNavTitle title="Настроить уведомления" />,
    }),
  },
  {
    pattern: ROUTES.profileTariff,
    build: () => ({
      leading: <TopNavBackButton fallbackHref={ROUTES.profile} />,
      children: <TopNavTitle title="Тариф" />,
    }),
  },
  {
    pattern: ROUTES.profileTariffAbout,
    build: () => ({
      leading: <TopNavBackButton fallbackHref={ROUTES.profileTariff} />,
      children: <TopNavTitle title="О тарифе" />,
    }),
  },
  {
    pattern: ROUTES.profileTariffChange,
    build: () => ({
      leading: <TopNavBackButton fallbackHref={ROUTES.profileTariff} />,
      children: <TopNavTitle title="Выбрать тариф" />,
    }),
  },
  {
    pattern: ROUTES.profileTariffDisable,
    build: () => ({
      leading: <TopNavBackButton fallbackHref={ROUTES.profileTariffAbout} />,
      children: <TopNavTitle title="Отключение тарифа" />,
    }),
  },
  {
    // Интерстишал лимита (#1469): назад в хаб объектов.
    pattern: ROUTES.profileTariffPropertyLimit,
    build: () => ({
      leading: <TopNavBackButton fallbackHref={ROUTES.properties} />,
      children: <TopNavTitle title="Тариф" />,
    }),
  },
  {
    // Интерстишал шаринга (#1470): назад к участникам.
    pattern: ROUTES.profileTariffSharing,
    build: () => ({
      leading: <TopNavBackButton fallbackHref={ROUTES.participants} />,
      children: <TopNavTitle title="Повысить тариф" />,
    }),
  },
  {
    pattern: ROUTES.profilePaymentMethods,
    build: () => ({
      leading: <TopNavBackButton fallbackHref={ROUTES.profileTariff} />,
      children: <TopNavTitle title="Способы оплаты" />,
    }),
  },
  {
    pattern: ROUTES.profilePaymentMethodsAdd,
    build: () => ({
      leading: <TopNavBackButton fallbackHref={ROUTES.profilePaymentMethods} />,
      children: <TopNavTitle title="Добавить карту" />,
    }),
  },
  {
    pattern: ROUTES.profilePayments,
    build: () => ({
      leading: <TopNavBackButton fallbackHref={ROUTES.profileTariff} />,
      children: <TopNavTitle title="Операции" />,
    }),
  },
  {
    // Деталь платёжной операции подписки (#624): тайтл — дата-время из
    // данных, плейсхолдер — канон loading-хрома.
    pattern: '/profile/tariff/payments/[id]',
    build: () => ({
      leading: <TopNavBackButton fallbackHref={ROUTES.profilePayments} />,
      children: <TopNavTitle title="Платёж" />,
    }),
  },
  {
    // Юрстраницы профиля: тайтл шапки пуст — документ несёт свой H1
    // (каркасы страниц с title="").
    pattern: ROUTES.profileInfo,
    build: () => ({
      leading: <TopNavBackButton fallbackHref={ROUTES.profile} />,
      children: <TopNavTitle title="" />,
    }),
  },
  {
    pattern: ROUTES.profilePrivacy,
    build: () => ({
      leading: <TopNavBackButton fallbackHref={ROUTES.profileInfo} />,
      children: <TopNavTitle title="" />,
    }),
  },
  {
    pattern: ROUTES.profileTerms,
    build: () => ({
      leading: <TopNavBackButton fallbackHref={ROUTES.profileInfo} />,
      children: <TopNavTitle title="" />,
    }),
  },
  {
    pattern: ROUTES.profileOffer,
    build: () => ({
      leading: <TopNavBackButton fallbackHref={ROUTES.profileInfo} />,
      children: <TopNavTitle title="" />,
    }),
  },

  // ——— Объекты —
  {
    // Архив (#563): сабтайтл-счётчик — из данных, приносит шим.
    pattern: ROUTES.propertyArchive,
    build: () => ({
      leading: <TopNavBackButton fallbackHref={ROUTES.properties} />,
      children: <TopNavTitle title="Архивные объекты" />,
    }),
  },
  {
    // Визард создания объекта: шаг 1 из 3; «Закрыть» — хром фазы загрузки
    // самого экрана (его шаг 1 несёт крестик в левом слоте).
    pattern: ROUTES.propertyNew,
    build: () => ({
      leading: CLOSE_GLYPH,
      children: <StepsChip step={1} total={PROPERTY_CREATE_TOTAL_STEPS} size="m" />,
    }),
  },
  {
    // Деталь объекта — после статических двухсегментных /properties/archive
    // и /properties/new: матчер идёт по порядку записей, динамический [id]
    // не должен глотать статические сегменты (в Next.js статики выигрывают
    // у динамик — здесь то же правило порядком).
    pattern: '/properties/[id]',
    build: () => ({
      leading: <TopNavBackButton fallbackHref={ROUTES.properties} />,
      children: <TopNavTitle title="Объект" />,
    }),
  },
  {
    // Правка объекта: хром правки — сценарий по умолчанию загрузки.
    pattern: '/properties/[id]/edit',
    build: () => ({
      leading: CLOSE_GLYPH,
      trailing: <IconButton icon={<Check />} label="Сохранить" />,
      children: <TopNavTitle title="Редактирование объекта" />,
    }),
  },
  {
    // «Об объекте» (#588): trailing-карандаш право-гейтнутый — шим.
    pattern: '/properties/[id]/about',
    build: (params) => ({
      leading: <TopNavBackButton fallbackHref={ROUTES.property(params.id ?? '')} />,
      children: <TopNavTitle title="Об объекте" />,
    }),
  },

  // ——— Глобальные операции (#541–#548, #570) —
  {
    pattern: ROUTES.operationsExpenses,
    build: () => ({ leading: BACK_GLYPH, children: <TopNavTitle title="Расходы" /> }),
  },
  {
    pattern: ROUTES.operationsIncomes,
    build: () => ({ leading: BACK_GLYPH, children: <TopNavTitle title="Доходы" /> }),
  },
  {
    pattern: ROUTES.operationsObjects,
    build: () => ({
      leading: CLOSE_GLYPH,
      trailing: <IconButton icon={<Check />} label="Применить выбранные объекты" />,
      children: <TopNavTitle title="Выбрать объект" />,
    }),
  },
  {
    pattern: ROUTES.operationsCategories,
    build: () => ({ leading: CLOSE_GLYPH, children: <TopNavTitle title="Выбрать категорию" /> }),
  },
  {
    pattern: ROUTES.operationsNew,
    build: () => ({ leading: CLOSE_GLYPH, children: <TopNavTitle title="Добавить операцию" /> }),
  },

  // ——— Глобальные платежи (#573, #578–#582, #1347; визард — решение
  // владельца 23.10) —
  {
    pattern: ROUTES.paymentsDue,
    build: () => ({ leading: BACK_GLYPH, children: <TopNavTitle title="Платежи на оплату" /> }),
  },
  {
    pattern: ROUTES.paymentsFavorites,
    build: () => ({ leading: BACK_GLYPH, children: <TopNavTitle title="Избранные платежи" /> }),
  },
  {
    pattern: ROUTES.paymentsObjects,
    build: () => ({
      leading: BACK_GLYPH,
      trailing: <IconButton icon={<Search />} label="Поиск объектов" />,
      children: <TopNavTitle title="Объекты" />,
    }),
  },
  {
    pattern: ROUTES.paymentsOverdue,
    build: () => ({ leading: BACK_GLYPH, children: <TopNavTitle title="Просроченные платежи" /> }),
  },
  {
    // Визард платежа, глобальный вход: раскладка global — 6 шагов.
    pattern: '/payments/new',
    build: () => ({
      leading: BACK_GLYPH,
      children: <StepsChip step={1} total={wizardTotalSteps('global')} size="m" />,
    }),
  },
];

/** Статическая конфигурация шапки для pathname или undefined — маршрут
 * без записи (шим после гидратации решит, есть ли шапка вообще). */
export function matchTopNavRoute(pathname: string): TopNavProps | undefined {
  for (const entry of ENTRIES) {
    const params = matchRoutePattern(entry.pattern, pathname);
    if (params !== undefined) return entry.build(params);
  }
  return undefined;
}
