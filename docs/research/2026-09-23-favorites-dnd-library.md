# Библиотека плавного перетаскивания строк избранного: dnd-kit vs framer-motion Reorder vs pragmatic-dnd vs самописный pointer-код

- Дата: 2026-09-23. Research-тикет [#812](https://github.com/devnumbers/arenda-platform/issues/812) карты [#811](https://github.com/devnumbers/arenda-platform/issues/811) «Режим правки избранного: drag-экран + выделение».
- Объект: `apps/frontend/widgets/payments/ui/payments-favorites-screen.tsx` — `FavoritesEditList`, самописный DnD на pointer-событиях (перестановка «прыжком», строка не следует за указателем, без автоскролла, клавиатуры и aria-анонса); строка — `PaymentRowButton` (слоты leading/trailing, grip `Move` в trailing); черновик порядка — `draftOrder` в useState + `moveFavorite` (`apps/frontend/widgets/payments/lib/favorites-edit-model.ts`), сохранение полным PUT `/payments/favorites/order`.
- Метод: первоисточники — **исходники установленного framer-motion 12.41.0 из node_modules** (`dist/es/components/Reorder/*` — Group/Item/auto-scroll разобраны), npm-реестр (peer-deps, даты релизов), bundlejs (замеры бандлов), официальные доки motion.dev / dndkit.com / atlassian.design, GitHub issues клавиатурных драм. Ссылки — в конце.

## Резюме — рекомендация

**`framer-motion` Reorder (уже в deps, ^12.41.0) + собственный клавиатурный слой (~40–60 строк).**

1. **Reorder 12.41 закрывает из коробки 6 из 8 критериев** — больше, чем принято считать: с версии 12.24.x в OSS-пакете есть встроенный автоскролл (`docs: «Auto-scroll lists»`; в наших node_modules — `Reorder/utils/auto-scroll.mjs`, порог 50px, скорость до 25px/кадр, документ-скролл и scrollable-предки, тач и мышь — драйвер один, pointer-жест). Хэндл-only — официальный паттерн из доков (`dragListener={false}` + `useDragControls` + `dragControls`); 1:1 — Item тащится трансформом за пальцем (`dragSnapToOrigin` + `layout`); соседи пружят FLIP'ом (layout-анимации — то, ради чего motion и берут); лифт — `whileDrag={{ scale, boxShadow }}`; controlled `values`/`onReorder` ложится 1:1 на наш `draftOrder`/`moveFavorite`.
2. **Клавиатурного порядка в Reorder нет** — это единственное «белое пятно» против критериев. Дописывается руками без всякого API: ручка уже кнопка — Space/Enter «берёт», ArrowUp/ArrowDown зовёт `moveFavorite`, анонсы — в `aria-live`-узел. Мало того, что это просто: библиотечный клавиатурный сенсор нам всё равно пришлось бы пристраивать на наш `PaymentRowButton` — здесь контролируемость и даёт короткий путь.
3. **dnd-kit (`@dnd-kit/core` + `sortable`) — сильнейший «из коробки» (хэндл, DragOverlay 1:1, FLIP, автоскролл, KeyboardSensor + встроенные live-анонсы), но проект заморожен**: последний стабильный релиз 6.3.1/10.0.0 — 05.12.2024, ни одного релиза за ~21 месяц; issue [#1830 (04.11.2025)](https://github.com/clauderic/dnd-kit/issues?q=) «Active Maintenance Status and Suitability for Production Use?»; усилия автора ушли в переписку `@dnd-kit/react` (beta 0.1.x, 2025) — roadmap-обсуждение январь 2026 подтверждает: 6.x — тупиковая ветка. React 19 у него «формально совместим» (peer `>=16.8.0`, ставится без ERESOLVE), но ни один релиз React 19-эры не проверен мейнтейнером — берем риск без апдейтов на хвосте.
4. **pragmatic-dnd — живой (3.1.0 от 29.08.2026, релизы каждые пару недель), но философия «мы даём события, UX строй сам»**: ни 1:1-плашки, ни FLIP, ни перестановки из коробки — официальный sortable-пример показывает индикатор вставки, а не следующую за пальцем строку. Автоскролл и клавиатура — отдельными пакетами (`-auto-scroll`, `-react-accessibility`). Для одного экрана это максимум ручной работы из всех библиотечных вариантов.
5. **Бандл: +0 kB против +~20–22 kB.** framer-motion уже в deps и в бандле (шит, тултипы) — Reorder добавляет килобайты, а не библиотеку. dnd-kit по замеру bundlejs — 21.5 kB gzip за core+sortable (с дублированием utilities). Плюс zero нового peer/совместимого риска: framer-motion 12.41.0 заявляет `react ^18 || ^19` — наш React 19.2.4 прямо в матрице.

Пейеджек для тикета экрана 1: `Reorder.Group values={draftOrder} onReorder={setDraftOrder}` + `Reorder.Item` на каждый `PaymentRowButton` (обёртка-`li`), `dragListener={false}`, ручка `controls.start(e)` на `onPointerDown`; `whileDrag` — scale ~1.02 + тень; клавиатура и анонсы — свой слой на ручке.

## 1. Критерии #812 по кандидатам — сводная таблица

| Критерий | framer-motion Reorder 12.41 | dnd-kit core 6.3.1 + sortable 10.0.0 | pragmatic-dnd 3.1.0 | Самописный pointer (апгрейд) |
|---|---|---|---|---|
| Drag строго за хэндл | из коробки: `dragListener={false}` + `dragControls` (офиц. доки) | из коробки: listeners только на ref хэндла | из коробки: `draggable()` вешается на что скажешь | руками (уже сделано) |
| Плашка следует 1:1 | из коробки: Item тащится x/y-трансформом за указателем, `dragSnapToOrigin` | из коробки: `<DragOverlay>` (плашка в оверлее, 1:1) | **нет** — HTML5 DnD даёт браузерный drag-призрак, 1:1-плашку рендеришь и позиционируешь сам | руками |
| Лифт-эффект (scale/тень) | из коробки: `whileDrag` | руками (CSS на drag-активных классах) | руками | руками |
| FLIP/пружинное расступание соседей | из коробки: layout-анимации (`layout` включён у Item) | из коробки: transform-transition соседей | **нет** — руками | руками (сложно делать хорошо) |
| Автоскролл у краёв (тач+мышь) | **из коробки с 12.24.x**: `Reorder/utils/auto-scroll.mjs` (проверено в наших node_modules 12.41.0) | из коробки: встроенный автоскролл, настраиваемый | отдельным пакетом `-auto-scroll` | руками |
| Сенсоры: long-press строки не дёргает drag | из коробки: жест живёт только на хэнде (`dragListener={false}`) | из коробки: PointerSensor с `activationConstraint`, listeners на хэнде | из коробки: старт драга только на зарегистрированном элементе | руками (уже развязано) |
| Клавиатура (Space/стрелки) + aria-анонс | **нет — дописывать** (~40–60 строк на ручке + `aria-live`) | из коробки: KeyboardSensor + `sortableKeyboardCoordinates` + встроенные live-анонсы | отдельным пакетом `-react-accessibility` + руками логику перестановки | руками (весь слой) |
| React 19.2.4 + Next 16 client | peer `^18 \|\| ^19` (12.41.0, npm); уже работает в приложении | peer `>=16.8.0` — ставится; **релизов в React 19-эру ноль**, проверка только комьюнити | peer нет (framework-agnostic core); React 19 не заявлен, но не упирается | никаких |
| Бандл (gzip, добавка) | ~0 (уже в deps) | ~20–22 kB (bundlejs: core 18.9 + sortable 13.2, с дублем utilities) | ядро <10 kB (заявление Atlassian) + 2 доп. пакета | 0 |
| Живость проекта | motion 12.x активная линия; 13.4.x уже вышел (09.2026) | **заморожен с 05.12.2024**; преемник `@dnd-kit/react` в beta | активный: 3.1.0 от 29.08.2026 | n/a |
| Модель controlled-порядка в useState | из коробки: `values`/`onReorder` | из коробки: `items` + `onDragEnd` (id-массив) | руками: порядок — только твой стейт | уже есть (`moveFavorite`) |

## 2. framer-motion Reorder — что подтверждено первоисточником

### 2.1 Что нашлось в наших node_modules (12.41.0)

- `dist/es/components/Reorder/Item.mjs`: Item рендерится как `motion.li`-семейство c `drag: axis`, **`dragSnapToOrigin: true`**, `layout: true` (по умолчанию) и `zIndex` 1 во время смещения — ровно поведение «плашка за пальцем, соседи переезжают layout-анимацией». `onDrag` Item'а вызывает **`autoScrollIfNeeded(groupRef.current, pointer, axis, velocity)`**.
- `dist/es/components/Reorder/utils/auto-scroll.mjs`: автоскролл ищет ближайший scrollable-предок (`overflow: auto|scroll`) или документ; порог края **50px**, скорость до **25px/кадр** с квадратичным ускорением к краю; старт фичи **скоростно-вентилируемый** (начинает, только когда вектор указателя идёт к краю), потолок скролла зафиксирован на момент входа в зону («prevents infinite scroll»). Работает и для тача, и для мыши — драйвер pointer-данных, без разделения.
- `Group.mjs`: **controlled-модель** — `values` + `onReorder(newValues)`; Group сам мапит локальную перестановку двух элементов на полный массив (сохраняя незарегистрированные элементы — важная оговорка для будущей виртуализации). Нигде в каталоге Reorder нет ни `keydown`, ни `Keyboard` — **клавиатуры нет** (grep по исходникам пуст; на странице доков Reorder клавиатура тоже не упомянута).
- Banner `"use client"` в обоих модулях — корректная работа в Next App Router под RSC-границей.

### 2.2 Хэндл-only — официальный паттерн

Доки motion.dev (React → Reorder → «Drag triggers»): `useDragControls` + на `Reorder.Item` `dragListener={false}` и `dragControls={controls}`, на ручке `onPointerDown={(e) => controls.start(e)}`. Это развязывает жест drag от всех прочих жестов строки: future long-press-выделение живёт на самой строке и физически не может стартовать Reorder-драг, потому что Reorder слушает только хэндл. (Ровно это сегодня сломано у нас «в обратную сторону»: наш pointer-код вешает всё на ручку — конфликтов нет, но нет и плавности.)

### 2.3 Известные грабли (риски)

- **Автоскролл молодой и чиненый**: фича появилась в 12.24.x (changelog framer-motion: «Support for auto-scrolling when a Reorder.Item reaches the edges of its parent scrollable container»); «Reorder: Fixed viewport autoscroll» — фикс эпохи 12.41 (и Motion+ patch 2.7.2 от 29.01.2026). Есть свежий баг-репорт (янв. 2026) о Reorder внутри scrollable-контейнера с воркараундом «padding → дочернему элементу». Часть фиксов мотыля через платные Motion+ патчи — в OSS 12.41 автоскролл есть и рабочий (проверено исходниками), но при живой приёмке его поведение (в т.ч. скоростной вентиль «только при движении к краю») надо смотреть на стенде — это ровно «Доработки по живой приёмке» карты #811.
- **Версионная линия**: установлена ^12.41.0; у motion уже вышла 13.x (13.4.x, сент. 2026, peer `^18 || ^19`). В 12.x остаёмся осознанно (range `^12` не подпустит 13); апгрейд до 13 — отдельное решение вне этого тикета.
- **Порог перестановки** — velocity-based (`check-reorder.mjs` учитывает скорость и смещение, а не только середину соседа): ощущение «пружин» будет отличаться от dnd-kit'овского «прошёл середину — поменялись». На приёмке это плюс (живее), но настраивается слабо.

### 2.4 Клавиатурный слой, который придётся дописать

Оценка ~40–60 строк в `FavoritesEditList`:

```tsx
// ручка: настоящий фокусируемый <button> (сегодня — aria-hidden + tabIndex={-1},
// «клавиатурный порядок тикетом не заведён» — по карте #811 теперь заведён)
const [grabbedId, setGrabbedId] = useState<string | null>(null);
const [announce, setAnnounce] = useState("");

onKeyDown={(e) => {
  if (e.key === " " || e.key === "Enter") { e.preventDefault(); toggleGrab(payment.id); }
  if (grabbed && (e.key === "ArrowUp" || e.key === "ArrowDown")) {
    e.preventDefault();
    onMove(index, index ± 1);           // наш moveFavorite
    setAnnounce(`${payment.title}, позиция ${index ± 1} из ${draftOrder.length}`);
  }
  if (e.key === "Escape") setGrabbedId(null);
}}
// <p className="sr-only" aria-live="polite">{announce}</p>
```

Альтернатива без «grab-режима» — Alt+ArrowUp/Down (паттерн разбирается в GitHub Blog «Exploring the challenges in creating an accessible sortable», 07.2024). Обе схемы — обычный controlled-стейт, FLIP-пружины соседей при клавиатурном `onMove` сработают те же (layout-анимации).

## 3. dnd-kit — почему не рекомендация, несмотря на лучший «out of the box»

- **Заморозка подтверждена реестром**: `@dnd-kit/core` — последний релиз 6.3.1 **05.12.2024**, поле `modified` реестра — та же дата; `@dnd-kit/sortable` 10.0.0 — тоже декабрь 2024. Ни одного пре-релиза в 2025–2026.
- **Живость**: issue «Active Maintenance Status and Suitability for Production Use?» (04.11.2025); авторская активность ушла в `@dnd-kit/react` (beta 0.1.x с апреля по август 2025, новый DragDropManager+плагины); публичное roadmap-уточнение «@dnd-kit/react vs @dnd-kit/core» — январь 2026. Вывод: 6.x поддержки не ждёт.
- **React 19**: peer-deps `react >=16.8.0` — ставится чисто (ERESOLVE не будет, `--legacy-peer-deps` не нужен), известного канонического рантайм-бага под React 19 поиском не находится. Но это «не сломалось», а не «поддерживается»: ни один релиз не выходил в эпоху React 19/Next 16. При регрессии чинить придётся форком.
- **Что было бы «из коробки»** (честно в плюс): KeyboardSensor + `sortableKeyboardCoordinates`, встроенные скринридер-анонсы (live region в core), `<DragOverlay>` (настоящая «плавающая плашка» поверх списка — физически точнее Reorder'а), настраиваемый автоскролл (без скоростного вентиля), activationConstraint на сенсорах.
- **Бандл**: bundlejs — core 18.9 kB + sortable 13.2 kB gzip (замер по отдельности дублирует общий `@dnd-kit/utilities`; совместный замер трёх пакетов — 21.5 kB gzip). Для одного экрана +20 kB против +0 — довод не решающий, но против при прочих равных.

## 4. pragmatic-drag-and-drop — живой, но «строй UX сам»

- **Живость образцовая**: 3.1.0 — **29.08.2026**, до этого 3.0.0 (14.08.2026) и 2.0.2 (05.08.2026); фреймворк-агностичное ядро, peer-deps на React нет вообще (deps: `raf-schd`, `bind-event-listener`, `@babel/runtime`).
- **Философия**: безопасная обёртка над нативным HTML5 DnD (мышь) + pointer-события (тач). Библиотека **ничего не двигает и не анимирует** — ни 1:1-плашки, ни FLIP-перестановки: официальный пример sortable рендерит состояние «подниму-индикатор-вставки», а строка следует за пальцем браузерным drag-призраком (полупрозрачный снимок), управлять которым нельзя. Критерий «плашка следует 1:1 с лифт-эффектом» = собственный мониторинг позиции + собственный absolute-оверлей + собственные анимации. По объёму ручной работы это ближе к «самописному апгрейду», чем к библиотеке.
- Автоскролл — отдельный пакет `@atlaskit/pragmatic-drag-and-drop-auto-scroll`; клавиатурная доступность — отдельный пакет `@atlaskit/pragmatic-drag-and-drop-react-accessibility` (+ собственная логика перестановки). Ядро — <10 kB gzip (заявление Atlassian Design), плюс оба пакета сверху.
- **Когда это был бы выбор**: много списков с кастомными жестами, кросс-контейнерный drag, общий язык жестов на весь продукт. Для одного экрана — оверинжиниринг ручной работы.

## 5. Самописный pointer-код (апгрейд текущего) — против

Сегодня в `FavoritesEditList` есть: pointer-capture на ручке, вставка «по середине ряда под курсором», мгновенная перестановка без анимаций. Апгрейд до целевого поведения потребует руками: 1:1-трансформ плашки (absolute-клон или transform оригинала), FLIP-анимации соседей (самому — проще взять layout из motion), автоскролл с порогами/ускорением/потолками (самая граблеёмкая часть — тач-инерция и iOS overscroll), фокус-менеджмент клавиатуры, live-region. Это ровно тот список, который framer-motion уже закрыл проверенным кодом; единственный плюс — 0 зависимости, но зависимость уже в deps.

## 6. Совместимость с моделью экрана

`Reorder.Group values={draftOrder} onReorder={(next) => setDraftOrder(next)}` — Reorder требует именно контролируемый массив значений (invariant «Reorder.Group must be provided a values prop»), наш `draftOrder` уже им является; `onReorder` — это тот же `setDraftOrder`, что сегодня идёт в `moveFavorite`. `hasFavoritesEdits`/`remainingFavoriteIds` и полный PUT не меняются вовсе. В `Reorder.Item` оборачивается строка — `PaymentRowButton` остаётся без правок (Item — обёртка-`li` вокруг, слоты ведут себя как сейчас); пометка «строка в правке не открывает платёж» сохраняется, т.к. тап-логика ведущей строки не меняется.

Существенная правка строки, которую тянет клавиатурный порядок: ручка-`Move` сегодня `aria-hidden` + `tabIndex={-1}` (комментарий в коде: «клавиатурный порядок тикетом не заведён»). Становится фокусируемой кнопкой с `aria-label` («Переместить, …»), grab-стейт и анонсы — в списке.

## 7. Вскрывшиеся вопросы (карта разложит позже)

1. **Автоскролл на приёмке**: скоростной вентиль Reorder (скролл стартует только при движении к краю) и «padding → дочернему элементу» воркараунд для scrollable-контейнеров — проверить на стенде; если не устроит — точечный апгрейд до собственного autoscroll поверх Reorder (на `useMotionValueEvent`), не смена библиотеки.
2. **Схема клавиатуры**: Space-grab + стрелки vs Alt+стрелки — выбрать при реализации (влияние на `PaymentRowButton`-хендл и текст анонсов; на десктопе Live-регион одинаков).
3. **`prefers-reduced-motion`**: отключать ли layout-пружины и лифт при reduced motion (motion даёт `useReducedMotion` из того же пакета) — решить на экране 1.
4. **framer-motion 12 → 13**: 13.4.x уже вышел; при следующем касании фронтенд-зависимостей оценить апгрейд (Reorder API между 12/13 не сверялся в этом ресёрче).

## Источники

- Исходники **framer-motion 12.41.0** (локальные node_modules apps/frontend): `dist/es/components/Reorder/Group.mjs`, `Item.mjs`, `utils/auto-scroll.mjs`, `utils/check-reorder.mjs`, `dist/index.d.ts` (ReorderGroupProps/ReorderItemProps, useDragControls).
- npm-реестр: [@dnd-kit/core](https://www.npmjs.com/package/@dnd-kit/core) (6.3.1, peer `react >=16.8.0`, релиз 05.12.2024, `time.modified` = 05.12.2024), [@dnd-kit/sortable](https://www.npmjs.com/package/@dnd-kit/sortable) 10.0.0, [framer-motion](https://www.npmjs.com/package/framer-motion) (12.41.0 peer `react ^18 || ^19`; latest 13.4.1), [@atlaskit/pragmatic-drag-and-drop](https://www.npmjs.com/package/@atlaskit/pragmatic-drag-and-drop) (3.1.0 от 29.08.2026).
- Доки [motion.dev — Reorder](https://motion.dev/docs/react-reorder): «Drag triggers» (useDragControls + `dragListener={false}`), «Auto-scroll lists» ([changelog](https://motion.dev/changelog): автоскролл добавлен в 12.24.x, «Reorder: Fixed viewport autoscroll», Motion+ patch 2.7.2 от 29.01.2026).
- GitHub clauderic/dnd-kit: issue [#1830](https://github.com/clauderic/dnd-kit/issues/1830) «Active Maintenance Status and Suitability for Production Use?» (04.11.2025); roadmap-обсуждение «@dnd-kit/react vs @dnd-kit/core» (янв. 2026); changelog нового [dndkit.com](https://dndkit.com) (beta `@dnd-kit/react` 0.1.x, апр.–авг. 2025).
- [Atlassian Design — Pragmatic drag and drop](https://atlassian.design/components/pragmatic-drag-and-drop/about) (ядро «performance focused», <10 kB); пакеты [@atlaskit/pragmatic-drag-and-drop-auto-scroll](https://www.npmjs.com/package/@atlaskit/pragmatic-drag-and-drop-auto-scroll) и [@atlaskit/pragmatic-drag-and-drop-react-accessibility](https://www.npmjs.com/package/@atlaskit/pragmatic-drag-and-drop-react-accessibility).
- GitHub Blog — [«Exploring the challenges in creating an accessible sortable»](https://github.blog) (07.2024) — клавиатурные схемы sortable (Space/Enter-grab, Escape, Enter-commit).
- Замеры бандлов: [bundlejs.com](https://bundlejs.com) (`@dnd-kit/core@6.3.1` 18.9 kB gzip; `+ sortable@10.0.0` и pragmatic-core — 21.5 kB gzip суммарно; по отдельности с дублированием utilities).
