import type { TopNavProps } from './top-nav-view';

/** Регистрационный стор динамической части шапки (решение владельца
 * 23.10.2026, issue #1592 — персистентная шапка): экраны через шим TopNav
 * кладут свои пропы во внешний store, PersistentTopNav в ScreenLayout
 * читает снапшот через useSyncExternalStore. Направление данных —
 * «снизу вверх» (layout ← экраны), поэтому контекст React не годится —
 * только внешний store. Снапшот иммутабелен (замена Map, не мутация):
 * useSyncExternalStore различает кадры по ссылке.
 *
 * SSR: getServerSnapshot отдаёт стабильную пустую карту — первый кадр
 * рендерит реестр маршрутов (widgets/screen-layout/model), динамика
 * доезжает после гидратации; ни один кадр не падает в пустоту между
 * «регистрация удалена» и «регистрация записана» — в одном коммите React
 * сперва чистит слоты размонтированных экранов, затем пишет новые, и оба
 * шага лежат внутри layout-фазы (до покраски). */

/** Стабильная пустая карта — одна на модуль: useSyncExternalStore
 * сравнивает снапшоты по ссылке, новая Map на каждый вызов ломала бы
 * гидратацию бесконечными перерисовками. */
const EMPTY_PATCHES: ReadonlyMap<string, TopNavProps> = new Map();

export type TopNavPatchStore = {
  /** Записать/заменить слот экрана. Уведомляет подписчиков всегда:
   * пропы-узлы создаются заново каждый рендер экрана, глубокое сравнение
   * неоправданно; лишние перерисовки хоста глушит React Compiler
   * (стабильные JSX-идентичности) и дешёвый рендер самого хоста. */
  readonly set: (slotId: string, props: TopNavProps) => void;
  /** Убрать слот размонтированного экрана; отсутствие слота — безмолвный
   * no-op без уведомления (StrictMode дважды зовёт cleanup). */
  readonly delete: (slotId: string) => void;
  readonly subscribe: (listener: () => void) => () => void;
  /** Текущий снапшот — иммутабельная карта слот → пропы экрана. */
  readonly getSnapshot: () => ReadonlyMap<string, TopNavProps>;
  /** Серверный снапшот для useSyncExternalStore — стабильная пустая. */
  readonly getServerSnapshot: () => ReadonlyMap<string, TopNavProps>;
};

function createTopNavPatchStore(): TopNavPatchStore {
  let patches: ReadonlyMap<string, TopNavProps> = EMPTY_PATCHES;
  const listeners = new Set<() => void>();

  const notify = (): void => {
    for (const listener of listeners) listener();
  };

  return {
    set(slotId, props) {
      const next = new Map(patches);
      next.set(slotId, props);
      patches = next;
      notify();
    },
    delete(slotId) {
      if (!patches.has(slotId)) return;
      const next = new Map(patches);
      next.delete(slotId);
      patches = next;
      notify();
    },
    subscribe(listener) {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },
    getSnapshot() {
      return patches;
    },
    getServerSnapshot() {
      return EMPTY_PATCHES;
    },
  };
}

/** Синглтон на модуль: шимы TopNav всех экранов и хост ScreenLayout
 * обязаны видеть один store. */
export const topNavPatchStore: TopNavPatchStore = createTopNavPatchStore();

/** Результат сборки EffectiveTopNav: пропы для рендера и признак «шапка
 * на этом маршруте вообще есть» — маршрут без записи реестра и без
 * регистраций рендерит пустоту (сегодня такой экран не рисовал header);
 * SSR первого кадра опирается только на реестр. */
export type MergedTopNav = {
  readonly props: TopNavProps;
  readonly present: boolean;
};

/** Собрать пропы персистентной шапки: реестр маршрута — база (статика и
 * анатомия, SSR первого кадра), регистрации экранов — надстройка (шим
 * перебивает реестр целиком по каждому полю, shallow-merge верхнего
 * уровня). */
export function mergeTopNavConfig(
  base: TopNavProps | undefined,
  patches: ReadonlyMap<string, TopNavProps>,
): MergedTopNav {
  let merged: TopNavProps = base ?? {};
  for (const patch of patches.values()) {
    merged = { ...merged, ...patch };
  }
  return { props: merged, present: base !== undefined || patches.size > 0 };
}
