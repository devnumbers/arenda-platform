import { ROUTES } from '@/shared/config/routes';
import { TopNavBackButton, TopNavTitle, type TopNavProps } from '@/shared/ui/design';

/** Реестр «маршрут → статическая конфигурация шапки» (решение владельца
 * 23.10.2026, issue #1592 — гибрид реестр+шим): база для первого кадра.
 * Шапка живёт в ScreenLayout и не может взять пропы экрана на SSR —
 * экранные регистрации доезжают после гидратации. Реестр отдаёт статику
 * и анатомию по pathname: вариант `search`, крылья, компакт-конфиг хаба,
 * статичные тайтлы-плейсхолдеры (канон loading-хрома — «Платеж», §14
 * «шапка не прыгает»), ведущая «Назад» с фолбэком. Динамика (тайтлы из
 * данных, поисковый ввод, действия-колбэки) — только через шим, реестр
 * её не знает. После гидратации шим перебивает реестр (shallow-merge,
 * patch поверх базы — mergeTopNavConfig).
 *
 * Прототип #1592 несёт три маршрута приёмки (хаб «Платежи», страница
 * платежа, поиск операций); заполнение всех ~120 маршрутов — работа
 * тикета реализации, до того немаршрутизированные экраны отрисовывают
 * шапку после гидратации (клиентские переходы — мгновенно, SSR-первый
 * кадр без анатомии). */

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

const ENTRIES: readonly TopNavRouteEntry[] = [
  {
    // Хаб «Платежи» (#578): крылья и на мобайле, компакт — лупа поиска
    // платежей (динамический «+» приносит экран своим шимом).
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
    // Страница платежа (#465): «Назад» к списку платежей объекта и
    // тайтл-плейсхолдер — канон loading-хрома; настоящий тайтл
    // («Автоплатёж»/«Платеж») приносит экран.
    pattern: '/properties/[propertyId]/payments/[paymentId]',
    build: (params) => ({
      leading: (
        <TopNavBackButton
          fallbackHref={ROUTES.propertyPayments(params.propertyId ?? '')}
        />
      ),
      children: <TopNavTitle title="Платеж" />,
    }),
  },
  {
    // Поиск операций (#476): поисковая шапка; поле с вводом — шим экрана.
    pattern: ROUTES.operationsSearch,
    build: () => ({
      variant: 'search',
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
