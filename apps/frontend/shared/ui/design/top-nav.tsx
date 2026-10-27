'use client';

import { useId, useLayoutEffect, useEffect, useRef, type JSX } from 'react';
import { useHostsPersistentTopNav } from './persistent-top-nav-host';
import {
  isTopNavHydrationSettled,
  settleTopNavHydration,
  topNavPatchStore,
} from './top-nav-registration';
import { TopNavView, type TopNavProps } from './top-nav-view';

export type { TopNavCollapse, TopNavProps } from './top-nav-view';
export { TopNavTitle, type TopNavTitleProps } from './top-nav-view';

/** Единый хедер новых экранов — шим персистентной шапки (решение
 * владельца 23.10.2026, issue #1592, анти-дёргание аватара). Третья
 * итерация гашения перемонтажа шапки: photo-session-cache гасил цикл
 * перезагрузки фото (22.10), PersistentNavWings убрали перемонтаж крыла
 * на ПК (22.10), теперь весь DOM-узел шапки один — он живёт в
 * ScreenLayout и переживает клиентские навигации.
 *
 * Три режима по контексту:
 *  - под ScreenLayout (PersistentTopNavHostProvider): DOM не рендерим —
 *    пропы регистрируются в topNavPatchStore, их рендерит единственный
 *    PersistentTopNav над контентом; узел шапки не перемонтируется,
 *    лого и аватар переживают навигацию, контент слотов меняется в
 *    layout-фазе коммита (до покраски — без кадра со старым тайтлом
 *    поверх новой страницы);
 *  - overlay-поверхности: рендер на месте (свой TopNavView) — шапка
 *    поверхности живёт внутри неё и глобальную не заменяет (канон
 *    #865);
 *  - вне ScreenLayout (ui-kit, логин, юнит-среда): локальный TopNavView
 *    — поведение прежнее, дерево элемент-тестов не меняется.
 *
 * JSX-сигнатура и пропы — прежние (TopNavProps): 147+ точек вызова не
 * переписываются, смысл «экран декларирует шапку» сохраняется. Рендер
 * слотов и анатомия — TopNavView (top-nav-view.tsx); докстринги анатомии
 * там же. */
export function TopNav(props: TopNavProps): JSX.Element | null {
  const hosted = useHostsPersistentTopNav();
  useRegisterTopNavProps(props, hosted && props.overlay !== true);
  if (props.overlay === true || !hosted) {
    return <TopNavView {...props} />;
  }
  return null;
}

/** Слот текущего экземпляра шима: useId стабилен между рендерами
 * экземпляра и уникален между экземплярами; условный рендер истории
 * (searchMode ? <TopNav…> : <TopNav…>) монтирует один экземпляр за раз —
 * слоты не спорят.
 *
 * Две фазы регистрации (issue #1605, react/react#37014): коммит
 * гидратации и клиентская навигация. React 19.2 ставит $RC-раскрытие
 * стриминг-границы в rAF-очередь (~300 мс); синхронный ре-рендер,
 * приземлившийся в это окно (стор-синк из layout-эффекта шима),
 * материализует дубль контента границы на ~300 мс — strict-mode e2e
 * ловит его на тексте. Поэтому первая регистрация экземпляра — в
 * пассивной фазе (после коммита гидратации; SSR-кадр окна держит
 * реестр маршрутов), а клиентские навигации регистрируются в
 * layout-фазе, как прежде: новая страница не выкрасится под старым
 * тайтлом (анти-stale-свойство #1592). */
function useRegisterTopNavProps(props: TopNavProps, enabled: boolean): void {
  const slotId = useId();
  // Свежие пропы для пассивного эффекта без перезапуска: пропы-узлы
  // пересоздаются каждым рендером экрана, а маунт-эффект нужен один раз.
  // Запись — в layout-эффекте (легальное для refs место): он идёт на
  // каждом рендере до пассивных эффектов того же коммита.
  const propsRef = useRef(props);
  // Layout-фаза коммита: регистрация нового экрана и чистка слотов
  // старого ложатся между DOM-коммитом и покраской — браузер не
  // выкрасит кадр, где новая страница живёт под старым тайтлом шапки.
  // Без списка зависимостей: пропы-узлы пересоздаются каждым рендером
  // экрана, пропуск записи между ними стоил бы рассинхрона; лишние
  // записи без уведомлений невозможны (стор уведомляет всегда — см.
  // topNavPatchStore.set), цену съедает дешёвый рендер хоста.
  useLayoutEffect(() => {
    propsRef.current = props;
    // Гидратационный коммит не осел — первая регистрация уйдёт в
    // пассивную фазу (ниже); гонять стор-синк в окне гидратации нельзя
    // (react/react#37014 — дубль контента стриминг-границы).
    if (!enabled || !isTopNavHydrationSettled()) return;
    topNavPatchStore.set(slotId, props);
    return () => {
      topNavPatchStore.delete(slotId);
    };
  });
  // Пассивная фаза: раз на маунт экземпляра. В гидратации это первая
  // запись после коммита (окно гонки закрыто); в клиентской навигации —
  // дубль layout-фазной записи с теми же пропами (стор уведомляет
  // всегда — безвредно). Первым добравшийся до пассивной фазы шим
  // осаждает гидратацию — с этого момента навигационные экземпляры
  // регистрируются layout-фазой.
  useEffect(() => {
    if (!enabled) return;
    topNavPatchStore.set(slotId, propsRef.current);
    settleTopNavHydration();
    return () => {
      topNavPatchStore.delete(slotId);
    };
  }, [enabled, slotId]);
}
