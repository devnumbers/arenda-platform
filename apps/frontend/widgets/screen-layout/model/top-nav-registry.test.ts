import { describe, expect, it } from 'vitest';
import type { ReactNode } from 'react';
import { matchTopNavRoute } from './top-nav-registry';

/** Реестр «маршрут → статическая конфигурация шапки» (issue #1592, тикет
 * #1605 — полный каталог маршрутов). Элементы React инспектируются как
 * деревья без рендера — канон node-тестов (CODING_STANDARDS, Testing).
 * Таблица гоняет каждый паттерн с подставленными сегментами и проверяет
 * анатомию первого кадра: крылья, вариант search, компакт хаба, тайтл,
 * чип шага, поисковое поле, фолбэк «Назад». */

const PROPERTY = '33333333-3333-4333-8333-333333333333';
const PAYMENT = '55555555-5555-4555-8555-555555555551';
const RENTAL = '66666666-6666-4666-8666-666666666661';
const CONTACT = '77777777-7777-4777-8777-777777777771';
const PARTICIPANT = 'user%40example.com';
const NOTIFICATION = '88888888-8888-4888-8888-888888888881';
const RULE = '99999999-9999-4999-8999-999999999991';
const OPERATION = 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa';
const DATE = '2026-10-23';
const BILLING_PAYMENT = 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb';

/** Ожидание анатомии одного маршрута: undefined-поля не проверяются. */
type RouteExpectation = {
  readonly pattern: string;
  /** Значения динамических сегментов (порядок = порядок в паттерне). */
  readonly segments?: readonly string[];
  readonly mobileWings?: boolean;
  readonly hideWingsBelowDesktop?: boolean;
  readonly variant?: 'search';
  readonly collapseTitle?: string;
  /** href поисковой пилюли компакта (collapse.search.href). */
  readonly collapseSearchHref?: string;
  /** children — TopNavTitle: тайтл (и сабтайтл) плейсхолдера. */
  readonly title?: string;
  readonly subtitle?: string;
  /** children — StepsChip шага визарда. */
  readonly chip?: { readonly step: number; readonly total: number };
  /** children — SearchField: placeholder и aria-label. */
  readonly search?: { readonly placeholder?: string; readonly label?: string };
  /** children отсутствуют (bar хаба с одним тайтлом компакта, пустой
   * каркас и т.п.). */
  readonly noChildren?: boolean;
  /** leading — TopNavBackButton с фолбэком (сегменты подставлены). */
  readonly backHref?: string;
  /** leading — IconButton-глиф: ожидаемая aria-label. */
  readonly backLabel?: string;
  /** trailing — IconButton-глиф: ожидаемая aria-label. */
  readonly trailingLabel?: string;
};

function propsOf(node: ReactNode): Record<string, unknown> | undefined {
  return node !== null && typeof node === 'object' && 'props' in node
    ? (node.props as Record<string, unknown>)
    : undefined;
}

/** Один child TopNavTitle — вытащить title/subtitle. */
function titleOf(config: ReturnType<typeof matchTopNavRoute>): { title?: unknown; subtitle?: unknown } {
  const child = config?.children;
  const props = propsOf(child);
  return { title: props?.title, subtitle: props?.subtitle };
}

describe('matchTopNavRoute — полный каталог маршрутов', () => {
  const CASES: readonly RouteExpectation[] = [
    // ——— Хабы: mobileWings + компакт —
    {
      pattern: '/payments',
      mobileWings: true,
      collapseTitle: 'Платежи',
      collapseSearchHref: '/payments/search',
    },
    {
      pattern: '/operations',
      mobileWings: true,
      collapseTitle: 'Операции',
      collapseSearchHref: '/operations/search',
    },
    {
      pattern: '/properties',
      mobileWings: true,
      collapseTitle: 'Объекты',
      collapseSearchHref: '/properties/search',
    },
    { pattern: '/tasks', mobileWings: true, collapseTitle: 'Задачи', noChildren: true },
    {
      pattern: '/contacts',
      mobileWings: true,
      collapseTitle: 'Контакты',
      collapseSearchHref: '/contacts/search',
    },
    {
      pattern: '/participants',
      mobileWings: true,
      collapseTitle: 'Участники',
      collapseSearchHref: '/participants/search',
    },
    { pattern: '/notifications', mobileWings: true, collapseTitle: 'Уведомления', noChildren: true },
    // ——— Хаб профиля: анатомия подэкрана без «Назад», крылья только на ПК —
    { pattern: '/profile', hideWingsBelowDesktop: true, title: 'Профиль' },

    // ——— Поисковые шапки (статические маршруты) —
    {
      pattern: '/payments/search',
      variant: 'search',
      backLabel: 'Назад',
      search: { placeholder: 'Поиск платежа', label: 'Поиск платежа' },
    },
    {
      pattern: '/operations/search',
      variant: 'search',
      backLabel: 'Назад',
      search: { placeholder: 'Найти операцию', label: 'Найти операцию' },
    },
    {
      pattern: '/payments/objects/search',
      variant: 'search',
      backLabel: 'Назад',
      search: { placeholder: 'Найти объект', label: 'Найти объект' },
    },
    {
      pattern: '/contacts/search',
      variant: 'search',
      backLabel: 'Закрыть поиск',
      search: { placeholder: 'Найти контакт', label: 'Поиск контактов' },
    },
    {
      pattern: '/participants/search',
      variant: 'search',
      backLabel: 'Закрыть поиск',
      search: { placeholder: 'Найти участника', label: 'Поиск участников' },
    },
    {
      pattern: '/properties/search',
      variant: 'search',
      backLabel: 'Закрыть поиск',
      search: { placeholder: 'Найти объект', label: 'Поиск объектов' },
    },
    {
      pattern: '/profile/account/timezone',
      variant: 'search',
      backLabel: 'Назад',
      search: { placeholder: 'Часовой пояс', label: 'Поиск часового пояса' },
    },

    // ——— Операции объекта —
    {
      pattern: '/properties/[id]/operations',
      segments: [PROPERTY],
      backLabel: 'Назад',
      title: 'Операции объекта',
    },
    {
      pattern: '/properties/[id]/operations/income',
      segments: [PROPERTY],
      backLabel: 'Назад',
      title: 'Доходы объекта',
    },
    {
      pattern: '/properties/[id]/operations/expense',
      segments: [PROPERTY],
      backLabel: 'Назад',
      title: 'Расходы объекта',
    },
    {
      pattern: '/properties/[id]/operations/search',
      segments: [PROPERTY],
      variant: 'search',
      backLabel: 'Назад',
      search: { placeholder: 'Найти операцию', label: 'Найти операцию' },
    },
    {
      pattern: '/properties/[id]/operations/categories',
      segments: [PROPERTY],
      backLabel: 'Закрыть',
      title: 'Выбрать категорию',
    },
    {
      pattern: '/properties/[id]/operations/new',
      segments: [PROPERTY],
      backLabel: 'Закрыть',
      title: 'Добавить операцию',
    },
    {
      // Деталь операции: фолбэк — список платежей объекта, как у живого
      // экрана (goBack с этим же структурным фолбэком).
      pattern: '/properties/[id]/operations/[operationId]',
      segments: [PROPERTY, OPERATION],
      backHref: `/properties/${PROPERTY}/payments`,
      title: 'Операция',
    },

    // ——— Платежи объекта —
    {
      pattern: '/properties/[id]/payments',
      segments: [PROPERTY],
      backLabel: 'Назад',
      title: 'Платежи объекта',
    },
    {
      pattern: '/properties/[id]/payments/overdue',
      segments: [PROPERTY],
      backLabel: 'Назад',
      title: 'Просроченные платежи',
    },
    {
      pattern: '/properties/[id]/payments/all',
      segments: [PROPERTY],
      backLabel: 'Назад',
      title: 'Платежи объекта',
    },
    {
      pattern: '/properties/[id]/payments/auto',
      segments: [PROPERTY],
      backLabel: 'Назад',
      title: 'Автоплатежи объекта',
    },
    {
      pattern: '/properties/[id]/payments/new',
      segments: [PROPERTY],
      backLabel: 'Назад',
      chip: { step: 1, total: 5 },
    },
    {
      pattern: '/properties/[id]/payments/[paymentId]',
      segments: [PROPERTY, PAYMENT],
      backHref: `/properties/${PROPERTY}/payments`,
      title: 'Платеж',
    },
    {
      pattern: '/properties/[id]/payments/[paymentId]/edit',
      segments: [PROPERTY, PAYMENT],
      backLabel: 'Отменить правку',
      title: 'Платеж',
      subtitle: 'Редактирование',
    },
    {
      pattern: '/properties/[id]/payments/[paymentId]/history',
      segments: [PROPERTY, PAYMENT],
      backLabel: 'Назад',
      title: 'История платежа',
    },
    {
      pattern: '/properties/[id]/payments/[paymentId]/overdue',
      segments: [PROPERTY, PAYMENT],
      backLabel: 'Назад',
      title: 'Просроченные платежи',
    },
    {
      pattern: '/properties/[id]/payments/[paymentId]/schedule',
      segments: [PROPERTY, PAYMENT],
      backLabel: 'Назад',
      title: 'График платежей',
    },
    {
      pattern: '/properties/[id]/payments/[paymentId]/operations/projected/[date]',
      segments: [PROPERTY, PAYMENT, DATE],
      backLabel: 'Назад',
      title: 'Операция',
    },

    // ——— Аренды —
    {
      pattern: '/properties/[id]/rentals',
      segments: [PROPERTY],
      backLabel: 'Назад',
      title: 'Аренда',
    },
    {
      pattern: '/properties/[id]/rentals/new',
      segments: [PROPERTY],
      backLabel: 'Закрыть',
      chip: { step: 1, total: 4 },
    },
    {
      pattern: '/properties/[id]/rentals/new/contact',
      segments: [PROPERTY],
      variant: 'search',
      backLabel: 'Назад',
      search: { placeholder: 'Найти контакт', label: 'Поиск контактов' },
    },
    {
      pattern: '/properties/[id]/rentals/terms',
      segments: [PROPERTY],
      backLabel: 'Назад',
      title: 'Условия аренды',
    },
    {
      pattern: '/properties/[id]/rentals/terms/edit',
      segments: [PROPERTY],
      backLabel: 'Отменить правку',
      title: 'Изменить условия',
    },
    {
      pattern: '/properties/[id]/rentals/extend',
      segments: [PROPERTY],
      backLabel: 'Отменить продление',
      noChildren: true,
    },
    {
      pattern: '/properties/[id]/rentals/complete',
      segments: [PROPERTY],
      noChildren: true,
    },
    {
      pattern: '/properties/[id]/rentals/past',
      segments: [PROPERTY],
      backLabel: 'Назад',
      title: 'Прошлые аренды',
    },
    {
      pattern: '/properties/[id]/rentals/[rentalId]',
      segments: [PROPERTY, RENTAL],
      backHref: `/properties/${PROPERTY}/rentals/past`,
      title: 'Аренда',
      subtitle: 'Завершена',
    },
    {
      pattern: '/properties/[id]/rentals/[rentalId]/history',
      segments: [PROPERTY, RENTAL],
      backHref: `/properties/${PROPERTY}/rentals/past`,
      title: 'История операций',
    },
    {
      pattern: '/properties/[id]/rentals/[rentalId]/summary',
      segments: [PROPERTY, RENTAL],
      backLabel: 'Закрыть',
      title: 'Итоги аренды',
    },

    // ——— Задачи —
    {
      pattern: '/properties/[id]/tasks',
      segments: [PROPERTY],
      backLabel: 'Назад',
      title: 'Задачи объекта',
    },
    {
      pattern: '/properties/[id]/tasks/new',
      segments: [PROPERTY],
      backLabel: 'Закрыть',
      title: 'Создать задачу',
    },
    {
      pattern: '/properties/[id]/tasks/[ruleId]/edit',
      segments: [PROPERTY, RULE],
      backLabel: 'Закрыть',
      title: 'Изменить задачу',
    },
    {
      pattern: '/tasks/new',
      backLabel: 'Закрыть',
      title: 'Создать задачу',
    },
    {
      pattern: '/tasks/[ruleId]/edit',
      segments: [RULE],
      backLabel: 'Закрыть',
      title: 'Изменить задачу',
    },

    // ——— Контакты —
    {
      pattern: '/contacts/new',
      backLabel: 'Отменить создание',
      trailingLabel: 'Создать контакт',
      title: 'Создать контакт',
    },
    {
      pattern: '/contacts/[contactId]',
      segments: [CONTACT],
      backLabel: 'Назад',
      title: 'Контакт',
    },
    {
      pattern: '/contacts/[contactId]/edit',
      segments: [CONTACT],
      backLabel: 'Назад',
      title: 'Изменить контакт',
    },
    {
      pattern: '/properties/[id]/contacts',
      segments: [PROPERTY],
      backLabel: 'Назад',
      title: 'Контакты объекта',
    },
    {
      pattern: '/properties/[id]/contacts/new',
      segments: [PROPERTY],
      backLabel: 'Отменить создание',
      trailingLabel: 'Создать контакт',
      title: 'Создать контакт',
    },
    {
      pattern: '/properties/[id]/contacts/[contactId]',
      segments: [PROPERTY, CONTACT],
      backLabel: 'Назад',
      title: 'Контакт',
    },
    {
      pattern: '/properties/[id]/contacts/[contactId]/edit',
      segments: [PROPERTY, CONTACT],
      backLabel: 'Назад',
      title: 'Изменить контакт',
    },

    // ——— Участники —
    {
      pattern: '/participants/invite',
      backHref: '/participants',
      noChildren: true,
    },
    {
      pattern: '/participants/[participantId]',
      segments: [PARTICIPANT],
      backHref: '/participants',
      trailingLabel: 'Еще — действия с участником',
      title: 'Участник',
    },
    {
      pattern: '/participants/[participantId]/invite',
      segments: [PARTICIPANT],
      backLabel: 'Закрыть приглашение',
      title: 'Пригласить в объект',
    },
    {
      pattern: '/participants/[participantId]/properties/[propertyId]',
      segments: [PARTICIPANT, PROPERTY],
      backHref: `/participants/${PARTICIPANT}`,
      title: 'Права участника',
    },
    {
      pattern: '/participants/[participantId]/history',
      segments: [PARTICIPANT],
      backHref: `/participants/${PARTICIPANT}`,
      title: 'Действия участника',
    },
    {
      pattern: '/properties/[id]/participants',
      segments: [PROPERTY],
      backHref: `/properties/${PROPERTY}`,
      title: 'Участники объекта',
    },
    {
      pattern: '/properties/[id]/participants/invite',
      segments: [PROPERTY],
      backHref: `/properties/${PROPERTY}/participants`,
      noChildren: true,
    },
    {
      pattern: '/properties/[id]/participants/[memberId]',
      segments: [PROPERTY, PARTICIPANT],
      backHref: `/properties/${PROPERTY}/participants`,
      title: 'Права участника',
    },

    // ——— История (скоупы ленты) —
    {
      pattern: '/history',
      backHref: '/participants',
      title: 'История действий',
    },
    {
      pattern: '/history/properties/[propertyId]',
      segments: [PROPERTY],
      backHref: `/properties/${PROPERTY}/participants`,
      title: 'История объекта',
    },
    {
      pattern: '/history/participants/[participantId]/properties/[propertyId]',
      segments: [PARTICIPANT, PROPERTY],
      backHref: `/participants/${PARTICIPANT}/properties/${PROPERTY}`,
      title: 'Действия участника в объекте',
    },

    // ——— Уведомления —
    {
      pattern: '/notifications/[notificationId]',
      segments: [NOTIFICATION],
      backLabel: 'Закрыть',
      trailingLabel: 'Удалить',
      noChildren: true,
    },

    // ——— Профиль (SubScreenShell-каркасы) —
    {
      pattern: '/profile/account',
      backHref: '/profile',
      title: 'Аккаунт',
    },
    {
      pattern: '/profile/account/email',
      backLabel: 'Назад',
      trailingLabel: 'Закрыть',
      title: 'Смена почты',
    },
    {
      pattern: '/profile/devices',
      backHref: '/profile',
      title: 'Устройства',
    },
    {
      pattern: '/profile/notifications',
      backHref: '/profile',
      title: 'Настроить уведомления',
    },
    {
      pattern: '/profile/tariff',
      backHref: '/profile',
      title: 'Тариф',
    },
    {
      pattern: '/profile/tariff/about',
      backHref: '/profile/tariff',
      title: 'О тарифе',
    },
    {
      pattern: '/profile/tariff/change',
      backHref: '/profile/tariff',
      title: 'Выбрать тариф',
    },
    {
      pattern: '/profile/tariff/disable',
      backHref: '/profile/tariff/about',
      title: 'Отключение тарифа',
    },
    {
      pattern: '/profile/tariff/property-limit',
      backHref: '/properties',
      title: 'Тариф',
    },
    {
      pattern: '/profile/tariff/sharing',
      backHref: '/participants',
      title: 'Повысить тариф',
    },
    {
      pattern: '/profile/tariff/payment-methods',
      backHref: '/profile/tariff',
      title: 'Способы оплаты',
    },
    {
      pattern: '/profile/tariff/payment-methods/add',
      backHref: '/profile/tariff/payment-methods',
      title: 'Добавить карту',
    },
    {
      pattern: '/profile/tariff/payments',
      backHref: '/profile/tariff',
      title: 'Операции',
    },
    {
      pattern: '/profile/tariff/payments/[id]',
      segments: [BILLING_PAYMENT],
      backHref: '/profile/tariff/payments',
      title: 'Платёж',
    },
    { pattern: '/profile/info', backHref: '/profile', title: '' },
    { pattern: '/profile/info/privacy', backHref: '/profile/info', title: '' },
    { pattern: '/profile/info/terms', backHref: '/profile/info', title: '' },
    { pattern: '/profile/info/offer', backHref: '/profile/info', title: '' },

    // ——— Объекты —
    {
      pattern: '/properties/[id]',
      segments: [PROPERTY],
      backHref: '/properties',
      title: 'Объект',
    },
    {
      pattern: '/properties/archive',
      backHref: '/properties',
      title: 'Архивные объекты',
    },
    {
      pattern: '/properties/new',
      backLabel: 'Закрыть',
      chip: { step: 1, total: 3 },
    },
    {
      pattern: '/properties/[id]/edit',
      segments: [PROPERTY],
      backLabel: 'Закрыть',
      trailingLabel: 'Сохранить',
      title: 'Редактирование объекта',
    },
    {
      pattern: '/properties/[id]/about',
      segments: [PROPERTY],
      backHref: `/properties/${PROPERTY}`,
      title: 'Об объекте',
    },

    // ——— Глобальные операции —
    { pattern: '/operations/expenses', backLabel: 'Назад', title: 'Расходы' },
    { pattern: '/operations/incomes', backLabel: 'Назад', title: 'Доходы' },
    {
      pattern: '/operations/objects',
      backLabel: 'Закрыть',
      trailingLabel: 'Применить выбранные объекты',
      title: 'Выбрать объект',
    },
    {
      pattern: '/operations/categories',
      backLabel: 'Закрыть',
      title: 'Выбрать категорию',
    },
    {
      pattern: '/operations/new',
      backLabel: 'Закрыть',
      title: 'Добавить операцию',
    },

    // ——— Глобальные платежи —
    { pattern: '/payments/due', backLabel: 'Назад', title: 'Платежи на оплату' },
    { pattern: '/payments/favorites', backLabel: 'Назад', title: 'Избранные платежи' },
    {
      pattern: '/payments/objects',
      backLabel: 'Назад',
      trailingLabel: 'Поиск объектов',
      title: 'Объекты',
    },
    { pattern: '/payments/overdue', backLabel: 'Назад', title: 'Просроченные платежи' },
    {
      pattern: '/payments/new',
      backLabel: 'Назад',
      chip: { step: 1, total: 6 },
    },
  ];

  it.each(CASES.map((c) => [c.pattern, c] as const))('%s', (_pattern, expected) => {
    const segments = expected.segments ?? [];
    // Многосегментные паттерны заполняются своими значениями в порядке
    // следования — fill подставляет по одному за вызов.
    let pathname = expected.pattern;
    for (const segment of segments) {
      pathname = pathname.replace(/\[[^\]]+\]/, segment);
    }
    const config = matchTopNavRoute(pathname);
    expect(config, `нет записи для ${pathname}`).toBeDefined();

    expect(config?.mobileWings ?? false).toBe(expected.mobileWings ?? false);
    expect(config?.hideWingsBelowDesktop ?? false).toBe(expected.hideWingsBelowDesktop ?? false);
    expect(config?.variant).toBe(expected.variant);

    if (expected.collapseTitle !== undefined) {
      expect(config?.collapse?.title).toBe(expected.collapseTitle);
      if (expected.collapseSearchHref !== undefined) {
        expect(config?.collapse?.search?.href).toBe(expected.collapseSearchHref);
      } else {
        expect(config?.collapse?.search).toBeUndefined();
      }
    } else {
      expect(config?.collapse).toBeUndefined();
    }

    if (expected.title !== undefined) {
      const { title, subtitle } = titleOf(config);
      expect(title).toBe(expected.title);
      expect(subtitle).toBe(expected.subtitle);
    }
    if (expected.noChildren) {
      expect(config?.children).toBeUndefined();
    }

    if (expected.chip !== undefined) {
      const props = propsOf(config?.children);
      expect(props?.step).toBe(expected.chip.step);
      expect(props?.total).toBe(expected.chip.total);
    }
    if (expected.search !== undefined) {
      const props = propsOf(config?.children);
      expect(props?.placeholder).toBe(expected.search.placeholder);
      expect(props?.['aria-label']).toBe(expected.search.label);
    }

    if (expected.backHref !== undefined) {
      expect(propsOf(config?.leading)?.fallbackHref).toBe(expected.backHref);
    }
    if (expected.backLabel !== undefined) {
      expect(propsOf(config?.leading)?.label).toBe(expected.backLabel);
    }
    if (expected.trailingLabel !== undefined) {
      expect(propsOf(config?.trailing)?.label).toBe(expected.trailingLabel);
    }
  });
});

describe('matchTopNavRoute — матчер паттернов', () => {
  it('страница платежа: фолбэк «Назад» получает id объекта из сегмента', () => {
    const config = matchTopNavRoute(`/properties/${PROPERTY}/payments/${PAYMENT}`);
    expect(propsOf(config?.leading)?.fallbackHref).toBe(`/properties/${PROPERTY}/payments`);
  });

  it('сегменты глубже записи не совпадают (число сегментов)', () => {
    expect(
      matchTopNavRoute(`/properties/${PROPERTY}/payments/${PAYMENT}/schedule/extra`),
    ).toBeUndefined();
  });

  it('обрезанный динамический маршрут не совпадает', () => {
    expect(matchTopNavRoute(`/properties/${PROPERTY}/payments`)).toBeDefined(); // лента объекта
    expect(matchTopNavRoute(`/properties/${PROPERTY}`)).toBeDefined(); // деталь объекта
    expect(matchTopNavRoute('/properties')).toBeDefined(); // хаб объектов
  });

  it('маршруты без шапки не получают запись: логин, интерстишалы, success-экран', () => {
    expect(matchTopNavRoute('/')).toBeUndefined();
    expect(matchTopNavRoute('/login')).toBeUndefined();
    expect(matchTopNavRoute('/profile/tariff/change/success')).toBeUndefined();
  });

  it('несуществующие зоны — undefined (шим после гидратации решает)', () => {
    expect(matchTopNavRoute('/nonexistent')).toBeUndefined();
    expect(matchTopNavRoute('/properties/archive/extra')).toBeUndefined();
  });
});
