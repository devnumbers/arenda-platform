# ADR 0031: PWA pull-to-refresh — кастомный жест в standalone

В standalone-PWA кабинета нет UI-способа обновить данные: на iOS WebKit физически отключает нативный pull-to-refresh для `display: standalone/fullscreen`, а на Android нативный PTR работает, но UX расходится с iOS. Решено реализовать **собственный PTR-жест на `touch`-событиях**, активный **только в standalone** (`isStandaloneMode()`), с единым поведением на всех платформах. По срабатывании жест делает **мягкий refresh данных** (`queryClient.invalidateQueries()`), а **не** `window.location.reload()` — потому что ответственность за подтягивание нового деплоя берёт на себя [ADR 0032](./0032-pwa-service-worker-silent-update-on-navigation.md) (silent SW-update-lifecycle), и дублировать её в PTR нет смысла. Нативный Android-PTR не подавляем глобально (в standalone он и так не показывается).

**Разделение ответственностей (после ADR 0032):**

| Механизм | Что обновляет | Триггер |
|---|---|---|
| `ServiceWorkerUpdater` (ADR 0032) | **Код приложения** (новый JS/CSS/SW) | смена роута / `visibilitychange → hidden` → `controllerchange` → reload |
| `PullToRefresh` (этот ADR) | **Данные на экране** (react-query cache) | жест pull-down в standalone |

## Considered Options

1. **Кастомный PTR-жест в standalone (выбрано).** Touch-обработка на `window`, активация только при `scrollTop <= 0`. Поведение идентично на iOS/Android/desktop-тач.
2. *Нативный PTR везде.* Невозможно: Apple отключает нативный PTR в standalone намертво, CSS/мета-тегами не включается ([SO #75972895](https://stackoverflow.com/questions/75972895/ios-pwa-how-to-re-enable-pull-to-refresh)).
3. *Гибрид — нативный на Android + кастомный на iOS.* Два разных UX; feature-detect ненадёжен; отказались ради единообразия.
4. *In-app кнопка «обновить» + SW-update-баннер вместо жеста.* Не даёт интуитивного триггера, которого лишён iOS-пользователь в standalone.
5. *Готовая библиотека (`react-simple-pull-to-refresh` и т.п.).* Отвергнута: чужой UX сложно подогнать под дизайн-систему, + новая зависимость ради управляемого объёма кода.

### Действие по срабатывании: soft-refresh vs hard-reload

Изначально (до ADR 0032) было решено `window.location.reload()`. После реализации [ADR 0032](./0032-pwa-service-worker-silent-update-on-navigation.md) это устарело: новый деплой приезжает сам через silent SW-update-lifecycle. PTR переориентирован на **обновление данных** (react-query: `staleTime: 30s`, `refetchOnWindowFocus: false`).

- **Мягкий refresh (выбрано):** `queryClient.invalidateQueries()` без аргументов → помечает все запросы stale и рефетчит активные. Состояние UI сохраняется.
- *Hard-reload.* Теряет UI-состояние, избыточен после ADR 0032.
- *Гибрид.* Связывает два механизма, ломает разделение ответственностей.

### Нативный feel: эталон и механика

Эталон визуала/ощущения — **iOS `UIRefreshControl`** ([Apple docs](https://developer.apple.com/documentation/UIKit/UIRefreshControl)). Предыдущая итерация (fixed-overlay-индикатор поверх контента) воспринималась как «приклеенная крутилка», а не нативный жест: главное отличие нативного PTR — **двигается сам контент**, а не индикатор поверх него. Переписано под нативную механику:

| Аспект | iOS `UIRefreshControl` (эталон) | Реализация |
|---|---|---|
| Что двигается при тяге | весь scroll-content | `transform: translateY()` на `.content` (контент кабинета); `Sidebar`/`BottomNav` (fixed) стоят |
| Спиннер во время тяги | масштабируется и накапливает угол поворота пропорционально pull | `scale = clamp(pullY/THRESHOLD, 0, 1)`, `rotate = pullY × 2deg` |
| После порога | непрерывное вращение | `@keyframes spin` 0.8s linear |
| Возврат ниже порога | spring/rubber-band | `cubic-bezier(0.34, 1.56, 0.64, 1)` (back-out ~8% overshoot) |
| Цвет/размер спиннера | тонкий серый, ~24px | `var(--color-text-muted)`, 24px |

Android `SwipeRefreshLayout` (Material, цветной круглый индикатор над контентом) рассмотрен и отвергнут: в приложении с синим брендом чистый iOS-серый спиннер в потоке контента нейтральнее и узнаваем. Реальная spring-физика (velocity-зависимая) через `framer-motion`/`requestAnimationFrame` отвергнута: CSS back-out на 90% неотличим для пользователя в коротком PTR-жесте, и не открывает orphan-зависимость `framer-motion` и не добавляет spring-math.

## Consequences

- **Где:** `apps/frontend/shared/ui/pull-to-refresh/` (компонент `PullToRefresh` + `*.module.css` + barrel export), монтируется в `widgets/cabinet-layout/ui/CabinetLayout.tsx` рядом с `ServiceWorkerRegister`/`ServiceWorkerUpdater`.
- **Активация:** только при `isStandaloneMode() === true`. В обычном браузере компонент возвращает `null`, пользователь получает нативное поведение платформы.
- **Доступ к контенту:** `PullToRefresh` принимает `contentRef: RefObject<HTMLDivElement>` через пропс. `CabinetLayout` создаёт ref, вешает на `<div className={styles.content} ref={contentRef}>` и передаёт `<PullToRefresh contentRef={contentRef} />`. PTR ставит `transform: translateY()` на этот узел во время жеста.
- **Механика жеста:** `touchstart`/`touchmove`/`touchend` на `window`; старт только при `window.scrollY <= 0`; сопротивление 0.5; порог armed 70px (визуальных); clamp визуального pull на `THRESHOLD × 1.8`. Слушатели биндятся один раз на lifecycle; `phase` держится в ref, синхронизируется со state через `useEffect`, чтобы не пересоздавать слушатели mid-gesture.
- **Движение контента:** во время тяги — `contentRef.style.transform = translateY(pullYpx)` (без transition, следует за пальцем). При возврате/фиксации — `transition: transform 0.3s cubic-bezier(0.34, 1.56, 0.64, 1)`. При armed-release контент фиксируется на `60px`, по завершении refresh уезжает на `0`.
- **Progressive-спиннер:** позиция `fixed` по центру, 8px ниже верха `.content`; `scale = clamp(pullY/THRESHOLD, 0, 1)` + `rotate = pullY × 2deg` во время тяги; при armed-release → `scale: 1` + непрерывный `@keyframes spin`. Цвет `var(--color-text-muted)`, диаметр 24px, толщина 2.5px, без текста/эмодзи.
- **Действие по срабатывании:** `queryClient.invalidateQueries()` (без аргументов, fire-and-forget).
- **Индикатор завершения:** спиннер висит, пока `useIsFetching() > 0`, но не дольше верхней границы **3 сек** (защита от зависшего бэкенда/4 retry с backoff); затем контент и спиннер уезжают наверх.
- **Обработка ошибок:** отдельной error-семантики от PTR нет. Упавшие запросы попадают в стандартный error-flow своих виджетов (`FinanceErrorState onRetry`, и т.д.). Старые данные остаются в кеше как stale.
- **Координация с `ServiceWorkerUpdater` (ADR 0032):** не координируется. Механизмы ортогональны: PTR обновляет данные, SW-updater — код. Если в момент жеста активировался waiting SW и триггерит reload — он выигрывает гонку естественным образом, пользователь не замечает разницы.
- **Конфликт со скроллом:** скролл документовый (`window`), вложенных контейнеров нет. PTR читает `window.scrollY` — тот же контейнер, с которым работает `ScrollToTop`. Координация не требуется.
- **Подавление нативного overscroll:** глобально `overscroll-behavior` не меняем. На время активного жеста (палец на экране, `scrollY <= 0`) `touchmove` вызывает `preventDefault()`. CSS-класс `overscroll-behavior-y: contain` оставлен как запасной вариант.
- **Доступность:** PTR — supplementary mobile-only жест; клавиатурного/скринридер-дубликата нет. `aria-live` не добавляем до явного требования accessibility-аудита.
