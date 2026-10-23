'use client';

import { useId, useLayoutEffect, type JSX } from 'react';
import { useHostsPersistentTopNav } from './persistent-top-nav-host';
import { topNavPatchStore } from './top-nav-registration';
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
 * слоты не спорят. */
function useRegisterTopNavProps(props: TopNavProps, enabled: boolean): void {
  const slotId = useId();
  // Layout-фаза коммита: регистрация нового экрана и чистка слотов
  // старого ложатся между DOM-коммитом и покраской — браузер не
  // выкрасит кадр, где новая страница живёт под старым тайтлом шапки.
  // Без списка зависимостей: пропы-узлы пересоздаются каждым рендером
  // экрана, пропуск записи между ними стоил бы рассинхрона; лишние
  // записи без уведомлений невозможны (стор уведомляет всегда — см.
  // topNavPatchStore.set), цену съедает дешёвый рендер хоста.
  useLayoutEffect(() => {
    if (!enabled) return;
    topNavPatchStore.set(slotId, props);
    return () => {
      topNavPatchStore.delete(slotId);
    };
  });
}
