# Research: «apple-style» drag горизонтальных секций хаба (мышь на десктопе)

- Дата: 2026-10-12. Research-тикет [#1345](https://github.com/devnumbers/arenda-platform/issues/1345) (label `wayfinder:research`), тикет реализации — «Анимация секций хаба» [#1349](https://github.com/devnumbers/arenda-platform/issues/1349).
- Объект: ленты секций хаба «Платежи» — `apps/frontend/widgets/payments/ui/payments-global-screen.tsx`, строки 380 / 441 / 514 (`flex snap-x snap-mandatory gap-2 overflow-x-auto`, скрытый скроллбар, карточки `PaymentCardButton` 168.5×132). Мышью лента не перетаскивается — только колесо/тачпад; владелец хочет поведение «как у Apple».
- Метод: первоисточники — **исходники установленного framer-motion 12.41 из node_modules** (генератор `inertia` в motion-dom, drag-контролы, constraints — разобраны, тот же путь, что в [docs/research/2026-09-23-favorites-dnd-library.md](../docs/research/2026-09-23-favorites-dnd-library.md)), документация Apple/Unity (константы UIScrollView), gist с реверс-инжинирингом резинки, доки Embla v9 (эталонная web-реализация), Smashing Magazine/bram.us (bleed-паттерн), внутренние каноны (ADR 0031, DESIGN.md §1/§8/§14, CODING_STANDARDS «Column width rule»). Ссылки — в конце.

---

## 0. TL;DR

1. **Физику и так можно не писать заново**: генератор `inertia` framer-motion (уже в deps, 12.41.0) — структурно та же модель, что UIScrollView: экспоненциальное затухание `target − amplitude·e^(−t/timeConstant)` + хук `modifyTarget` = веб-аналог `targetContentOffset(forProposedContentOffset:withScrollingVelocity:)`. Резинка UIScrollView `(1 − 1/(x·c/d + 1))·d`, `c = 0.55` — **уже в проекте** (`pull-math.ts`), но на нативном скроллере десктопа она не нужна: края батутят сами (macOS) или не батутят ни у кого (Windows) — резинка не отличима от натива.
2. **Рекомендация — вариант А: pointer-drag прокси поверх нативного скролла**, только `pointerType: mouse|pen`. Во время жеста `scroll-snap-type: none` (spec-«off switch»), `scrollLeft` следует за мышью, velocity копит трекер из `pull-math.ts` (окно 100мс); на отпускании — цель = snap-точка, ближайшая к «идеальной точке остановки» `scrollLeft + v·timeConstant`-эквивалент, доезд через `animate()` framer-motion, затем snap вернуть. Тач-устройства не трогаются вовсе.
3. **Bleed ≤560 решается штатно**: обёртка хаба несёт собственный `px-6`, значит скроллерам секций можно внутренний `max-tablet:-mx-6 + px-6 + scroll-padding-inline: 24px` — это ровно тот случай, который «Column width rule» CODING_STANDARDS прямо называет легитимным («inner -mx cancelling a container's own padding stays legitimate»); запрет касается только компенсаций против page wrapper'а.

## 1. Вопрос

Горизонтальные snap-ленты хаба («Избранные», «Просроченные», «Платежи объектов») на десктопе прокручиваются только колесом; grab-and-drag мышью, как в iOS/macOS (UIScrollView / трекпад), отсутствует. Нужно: (а) поведение мыши «как у Apple» — drag за карточку с инерцией и точной остановкой на карточке; (б) тач остаётся нативным; (в) `snap-mandatory`, reduced-motion и вертикальный скролл страницы не ломаются; (г) на мобайл-ярусе (≤560) карточки секций уходят в край вьюпорта (bleed) при общей колонке max-w 560.

## 2. Как у Apple (физика UIScrollView)

### 2.1 Инерция (deceleration)

- **Константы**: `UIScrollView.DecelerationRate` — `normal = 0.998`, `fast = 0.99` ([Apple docs — decelerationRate](https://developer.apple.com/documentation/uikit/uiscrollview/decelerationrate)). Семантику коэффициента crisp-формулировкой даёт документация обёртки в Unity: «speed reduction per second: 0.5 halves the speed each second, 0 stops immediately» ([Unity — ScrollView.scrollDecelerationRate](https://docs.unity3d.com/ScriptReference/UIElements.ScrollView.scrollDecelerationRate.html)). Модель — экспоненциальное затухание скорости `v(t) = v₀·r^t`; интеграл до остановки даёт проекцию дистанции (для веба точный масштаб времени UIScrollView не нужен — веб-порты калибруют «константу времени» по ощущению, см. §4).
- **Целевая точка остановки — главный приём Apple**: декелерация в UIKit не «проигрывается покадрово до естественного упора», а **вычисляется заранее** как target и доезжает к нему. Делегат `scrollViewWillEndDragging(withVelocity:targetContentOffset:)` получает проектируемую точку остановки указателем и может её поправить — канонический способ строить «paging по карточкам» ([Apple docs — scrollViewWillEndDragging](https://developer.apple.com/documentation/uikit/uiscrollviewdelegate/scrollviewwillenddragging(withvelocity:targetcontentoffset:))). Скорость отпускания влияет на то, *какая* цель выбрана (флик уезжает на 1+ экран), а не на «сколько экспонента докрутит».
- **Paging vs free scroll**: `pagingEnabled` — остановка кратна странице (жёстко); кастомный snap через delegate — свободный drag + доезд к ближайшей к проекции карточке. Точный веб-эквивалент второй схемы документирован у Embla v9: `dragFree: 'snap'` — «dragging behaves exactly like true, but the carousel settles on the snap point closest to its natural destination» ([Embla — options](https://www.embla-carousel.com/docs/api/options)); свободный режим (`dragFree: true`) хабу не нужен — карточки должны останавливаться ровно.

### 2.2 Rubber-band у краёв

- Классическая реверс-инжиниринговая формула (iOS 6+, de-facto стандарт, `c = 0.55`): `b = (1 − 1/((x·c/d) + 1)) · d`, где x — тяга за границу, d — размерность вьюпорта; асимптотическая, сопротивление растёт с убывающей отдачей ([gist «Analysis of Apple's rubber band scrolling»](https://gist.github.com/originell/6961057); в репо уже верифицирована и перенесена в [research/ios-ptr-native-feel.md](ios-ptr-native-feel.md) и `pull-math.ts`). Современные варианты отличаются только подстройкой `c` (жёстче/мягче кривая) — структурно формула та же; WWDC23 «Animate with springs» задает отпускание пружиной с вложенной скоростью.
- **Для этого тикета резинка не нужна**: вариант А (нативный скроллер) не даёт тянуть контент за край в принципе — на macOS браузеры сами батутят нативный `overflow`-скролл, на Windows/Chrome Linux overscroll-подпрыгивания нет ни у кого. Формула пригодилась бы только варианту Б (transform-физика), где края рисуешь сам.

### 2.3 Web-физика, уже стоящая в проекте (первоисточник — node_modules 12.41.0)

- `motion-dom/dist/es/animation/generators/inertia.mjs`: `ideal = origin + power·velocity` (дефолты **power 0.8, timeConstant 325мс**), `modifyTarget(ideal)` — официальный хук подгонки цели (тот самый аналог `targetContentOffset`), движение — `target − amplitude·e^(−t/timeConstant)`, выход за `min/max` подхватывается пружиной (`bounceStiffness 500 / bounceDamping 10`).
- Drag-release в `VisualElementDragControls.startAnimation` собирает ровно этот генератор: `type: "inertia"`, `velocity` от указателя (при `dragMomentum`), `timeConstant: 750`, `bounceStiffness/bounceDamping` от `dragElastic`, всё перекрывается пропом `dragTransition` (`modifyTarget` проходит насквозь).
- Нюанс: `dragElastic` framer-motion — **линейный** `mixNumber(min, point, elastic)` (`gestures/drag/utils/constraints.mjs`), не асимптотическая резинка UIScrollView. Для варианта Б резинку пришлось бы подставлять свою (из `pull-math.ts`) через `dragTransition`/drag-колбэки — ещё один довод не брать вариант Б.
- velocity-aware доезд: у motion есть `animate()`/`useMotionValue` — та же точка входа, что у PTR (ADR 0031: «пружине отдаётся скорость смещения контента»).

## 3. Факты из проекта

- **Версии** (`apps/frontend/package.json`): framer-motion `^12.41.0`, React 19.2.4, Next 16.4.0, Tailwind `^4.3.1`. Брейкпоинт-токены: `--breakpoint-tablet: 561px`, `--breakpoint-desktop: 1024px` (`app/globals.css`); префикс `max-tablet:` сгенерируется тем же механизмом v4, что уже работающий `max-desktop:` (`shared/ui/design/page-content.tsx`).
- **Ленты хаба**: три экземпляра одинаковой разметки (строки 380/441/514) — `flex snap-x snap-mandatory gap-2 overflow-x-auto [scrollbar-width:none]`, дети `PaymentCardButton` c `snap-start`, лимит 4 карточки + замыкающая «Все …». Никакого drag-кода на лентах нет (`grep scrollLeft` по фронтенду пуст).
- **Пре́цедент физики в чистом модуле** — `apps/frontend/shared/ui/pull-to-refresh/pull-math.ts` (ADR 0031, ротация 12.10.2026): `rubberBandOffset` (та формула 0.55), `springPeak` (замкнутая форма пружины), `createVelocityTracker` (скользящее окно 100мс) + 16 юнит-тестов. Для drag-прокси **прямо переиспользуется** `createVelocityTracker`; резинка и пружина остаются в PTR.
- **Reorder/DnD**: `Reorder.Group` + `useDragControls` + `useReducedMotion` уже в бою (`payments-favorites-screen.tsx`); решение по выбору библиотеки DnD — [docs/research/2026-09-23-favorites-dnd-library.md](../docs/research/2026-09-23-favorites-dnd-library.md). Это вертикальный DnD порядка, к горизонтальному скроллу отношения не имеет — но подтверждает: физические жесты дома строим на motion, без новых зависимостей.
- **Каноны feel** (DESIGN.md §8): единая кривая `cubic-bezier(0.32, 0.72, 0, 1)` (`--dl-ease`), 350ms движение/трансформы; `prefers-reduced-motion` не отключает движение, а укорачивает до ~150ms, **drag-жесты — «управление, не декор»** и не гасятся; hover — внутри `@media (hover: hover)`. Свой хук `useReducedMotion` (`shared/lib/hooks/useReducedMotion.ts`). §14 «Стабильность»: скрытие внутренних скроллбаров держится на `scrollbar-width: none` — текущие ленты уже соответствуют.
- **Bleed**: правило «Column width rule» CODING_STANDARDS §Screen shell (решение 26.09, аудит #870): «-mx» против page wrapper'а запрещён (600px-баг хабов платежей), но «inner -mx cancelling a container's **own** padding … stays legitimate». Обёртка контента хаба сама несёт `px-6` (строка 192 payments-global-screen) → гасить можно её паддинг, не паддинг кабинета. Пре́цедент full-bleed-строки внутри паддинг-обёртки — `payment-edit-screen.tsx` (комментарий «ряд email-уведомлений — full-bleed (px-6 внутри канонного ряда)»).
- **Тач сегодня уже нативный**: внутренние `overflow:auto`-скроллеры iOS ≥13 имеют momentum+bounce сами (установлено в [research/ios-ptr-native-feel.md](ios-ptr-native-feel.md) §2) — ломать нечем, пока drag-прокси гейтится по `pointerType`.

## 4. Сравнение вариантов реализации

| Критерий | **А. Pointer-drag прокси поверх нативного скролла** (scrollLeft + инерция к snap-точке) | Б. Кастомная физика на transform (framer-motion drag/inertia) | В. Готовая карусельная либа (Embla/Swiper) |
|---|---|---|---|
| Тач остаётся нативным | да — прокси гейтится `pointerType: mouse/pen`, тач не заходит в код | нет — drag/hook перехватывает все указатели, нативный momentum/bounce теряется, резинку рисуешь сам | частично — у либ свой тач-движок |
| Snap-точки | да: на время жеста `scroll-snap-type: none` (spec off-switch, CSSWG), доезд в snap руками, потом snap вернуть — на таче `snap-mandatory` живёт как сейчас | руками: `modifyTarget` по массиву offsetLeft-точек | из коробки, но подменяет механику целиком |
| Физика «как Apple» | до ~95%: drag 1:1 + velocity + доезд к цели через `inertia`/`animate` motion; не хватает только edge-резинки (на десктопе её и не видно) | 100% управляемая, включая резинку из `pull-math.ts` | высокая, но чужие константы (Embla duration/threshold), подгонять сложно |
| Объём кода | хук ~100–150 строк + чистый модуль математики с юнит-тестами (по образцу pull-math) | хук + физика + edge-cases (клик vs drag, vertical-гвард, resize, RTL) — заметно больше | ~0 кода, но +бандл и чужая модель DOM |
| Зависимости | 0 новых (motion уже в deps) | 0 новых | **новая зависимость** — против правил проекта (решения #812, ADR 0031) |
| Доступность/семантика | не меняется: лента — нативный скроллер, кнопки-карточки кликабельны, клавиатура/скринридер как сейчас | меняется: скроллер перестаёт быть скроллером (нужны tabindex/role/клавиатурная прокрутка руками) | меняется на структуру либы |
| Главные риски | click-vs-drag порог (Embla: `dragThreshold` 10px, «only affects when clicks are fired», mouse-only); конфликт с вертикальным скроллом (гвард \|dx\|>\|dy\|); восстановление snap после доезда | вертикальный скролл страницы на таче, resize/RTL, потеря нативности на всех платформах | жесть бандла, конфликт с snap-каноном, чужой DOM |
| RTL (не актуально сейчас, но) | `scrollLeft` с отрицательными значениями в RTL — известная грабля, учесть в модуле | transform — инвариантен | у либ решено |

## 5. Рекомендация (архитектура)

**Вариант А. Три части, по прецеденту ADR 0031 («математика — чистый модуль, жест — хук, движение — motion»):**

1. **Чистый модуль физики** (например, `shared/ui/horizontal-section/drag-scroll-math.ts`, рядом с потребителем или в `shared/lib/`): 
   - `projectStopPoint(scrollLeft, velocityPxPerS, timeConstant)` — идеальная точка остановки экспоненциальной модели (`scrollLeft + v·timeConstant·power/1000`-эквивалент; константы калибруются, стартовая гипотеза — дефолты motion power 0.8 / timeConstant ~325–750мс);
   - `pickSnapTarget(ideal, snapPoints, velocityPxPerS)` — ближайшая к идеалу snap-точка, с флик-правилом: при |velocity| выше порога разрешить шаг на одну точку дальше по направлению (аналог targetContentOffset);
   - `isHorizontalDrag(dx, dy, thresholdPx)` — гвард старта (`|dx| > |dy|` и `|dx| > threshold`, threshold ~8px; ориентир Embla `dragThreshold: 10px`, mouse-only);
   - юнит-тесты — обязательно (канон pull-math.ts).
2. **Хук `useMouseDragScroll(ref)`** — pointer-обработчики: только `pointerType === "mouse" | "pen"`; `pointerdown` → запомнить позицию (drag отложенный до порога — чтобы клик по карточке жил); после `isHorizontalDrag` → `setPointerCapture`, класс-модификатор со `scroll-snap-type: none` и `scroll-behavior: auto`, `scrollLeft = start − dx`, `createVelocityTracker()` (переиспользовать из pull-math.ts) на каждый `pointermove`; `pointerup` → цель через модуль, доезд `animate()` framer-motion (onUpdate пишет `scrollLeft`), по завершении — вернуть snap-класс. После реального drag один раз погасить `click` (capture-фаза, `stopPropagation`, снять после первого) — иначе отпускание на карточке откроет платёж.
3. **Разметка лент** — без структурных правок: к трём `div`-лентам добавляются класс модификатора bleed + вызов хука (или один маленький компонент-обёртка `HorizontalSection` — решить на реализации; три копии одной разметки сами просятся в общий компонент, но это отдельное решение).

**Как жить со snap**: `scroll-snap-type: none` во время жеста — штатный «off switch» (CSSWG scroll-snap); восстанавливать строго после остановки доезда, иначе браузер перещёлкнет к ближайшей точке рывком. На таче код не выполняется вовсе — `snap-mandatory` не затрагивается.

**prefers-reduced-motion** (DESIGN.md §8: жест — управление, доезд — движение): сам drag не трогается; инерционный доезд на reduced — заменяется мгновенной установкой целевой точки (`scrollTo` без smooth / прямой `scrollLeft`), т.е. «укорачиваем до нуля», не отключая жест.

**Константы для калибровки** (стартовые, уточняются на живой приёмке): threshold 8px; timeConstant ~325–500мс; флик-порог velocity ~500–800 px/s; длительность доезда — производная от дистанции (velocity-aware доезд через inertia, не фиксированные 350ms `--dl-ease` — дальний флик на 350ms будет дёрганым; кривую `--dl-ease` использовать не для доезда, а для коротких микродвижений).

**Резинка за краями**: не реализовывать — на нативном скроллере macOS батутит браузер, на Windows его нет ни у кого; имитация на scrollLeft потребовала бы трансформ-костылей поверх краёв без выгоды.

## 6. Bleed-паттерн ≤560

Ярус мобайл 320–560 (DESIGN.md §1): колонка `max-w-column` равна вьюпорту, обёртка хаба несёт `px-6` (24px). Канонический рецепт (Smashing Magazine, bram.us — «bleed-карусель внутри падднутого контейнера»):

```
ленте секции, только на ≤560:
  margin-inline: -24px        /* гасим собственный px-6 обёртки — легитимно по Column width rule */
  padding-inline: 24px        /* карточки стоят на визуальной линии 24 */
  scroll-padding-inline: 24px /* snap-точки остаются на линии 24, первый/последний снап не липнет к краю */
```

- В Tailwind: `max-tablet:-mx-6 max-tablet:px-6 max-tablet:[scroll-padding-inline:24px]` (вариант `max-tablet:` сгенерируется из токена `--breakpoint-tablet: 561px`; механизм уже доказан `max-desktop:` в page-content.tsx).
- Заголовки секций (`h2` в обёртке с `px-6`) не трогаются — bleed только у лент. Вертикальные секции страницы (пилюля поиска, плейсхолдеры, скелетоны) остаются на `px-6` обёртки.
- Во время bleed-скролла карточка уезжает под край вьюпорта — так и задумано; `scroll-padding-inline` гарантирует, что в покое и после снапа карточка снова стоит на линии 24.
- Альтернатива «без негативов» — перенести `px-6` с обёртки на каждый ряд (по типу payment-edit-screen) и у лент вместо этого внутренний `px-6`: чище семантически, но трогает все строки хаба и каждый новый ряд хаба обязан помнить про паддинг; рекомендую точечный `-mx-6`, раз он явно легитимен правилом.

## 7. Открытые вопросы для тикета реализации #1349

1. **Флик-семантика**: свободный доезд к ближайшей к проекции точке (Embla `dragFree:'snap'`, универсально) vs «один экран за флик» (iOS paged feel) — решить с владельцем; влияет на `pickSnapTarget`.
2. **Кривая/длительность доезда**: velocity-aware `inertia` (рекомендация) vs фиксированные токены `--dl-ease` — нужно живое сравнение на стенде; фиксировать канон в DESIGN.md §8.
3. **Компонент или хук**: три ленты дублируют одну разметку — выделить ли общий `HorizontalSection` (затронет скелетон `PaymentsGlobalSectionSkeleton`) или ограничиться хуком + классами.
4. **Клик-vs-drag порог и подавление клика**: подтвердить на приёмке, что drag ≤ порога не «съедает» клик по карточке и что e2e (playwright mouse drag) надёжно различает оба сценария; пункт «кликабельность блоков — вся площадь» (#1242) не должен пострадать.
5. **RTL**: сейчас приложение LTR; в модуль математики заложить нормализацию `scrollLeft` (отрицательные значения в RTL), чтобы не всплыть при будущей локализации.
6. **Витрина**: если получится переиспользуемый хук/компонент — попадание на `/ui-kit` в том же коммите (канон DESIGN.md «Фиксация решений»).
7. **Тачпад-жесты на десктопе**: горизонтальный scroll колесом/трекпадом нативный и не трогается; убедиться, что pointer-drag и колесо не ссорятся во время анимации доезда (новый `pointerdown` останавливает `animate()` — гасить бегущий доезд на новый жест, как в PTR).

## Источники

- Установленный **framer-motion 12.41.0 / motion-dom** (node_modules apps/frontend): `motion-dom/dist/es/animation/generators/inertia.mjs` (power 0.8, timeConstant 325, modifyTarget, exp-decay, bounce-пружина), `framer-motion/dist/es/gestures/drag/VisualElementDragControls.mjs` (type inertia, timeConstant 750, bounceStiffness/bounceDamping от dragElastic), `gestures/drag/utils/constraints.mjs` (dragElastic = линейный mix, не резинка).
- [Apple — UIScrollView.DecelerationRate](https://developer.apple.com/documentation/uikit/uiscrollview/decelerationrate) (normal 0.998 / fast 0.99); [Apple — scrollViewWillEndDragging(withVelocity:targetContentOffset:)](https://developer.apple.com/documentation/uikit/uiscrollviewdelegate/scrollviewwillenddragging(withvelocity:targetcontentoffset:)) (проектируемая точка остановки, изменение цели делегатом).
- [Unity — ScrollView.scrollDecelerationRate](https://docs.unity3d.com/ScriptReference/UIElements.ScrollView.scrollDecelerationRate.html) (семантика коэффициента: «0.5 halves the speed each second»).
- [Gist «Analysis of Apple's rubber band scrolling»](https://gist.github.com/originell/6961057) (формула `(1 − 1/((x·c/d)+1))·d`, c = 0.55, iOS 6+); дублируется в репо: [research/ios-ptr-native-feel.md](ios-ptr-native-feel.md) (§1, там же WKWebViewIOS.mm WebKit — bounce делегируется UIScrollView), [ADR 0031](../docs/adr/0031-pwa-pull-to-refresh.md).
- [WWDC23 «Animate with springs»](https://developer.apple.com/videos/play/wwdc2023/10158/) (пружина с вложенной скоростью — модель отпускания).
- [Embla Carousel v9 — options](https://www.embla-carousel.com/docs/api/options) (`dragFree: 'snap'` — «settles on the snap point closest to its natural destination»; `dragThreshold` 10px, mouse-only, клик-vs-drag).
- [motion.dev — Drag](https://motion.dev/docs/react-drag) (`dragMomentum`, `dragTransition` bounceStiffness/bounceDamping, `dragElastic` 0..1).
- CSSWG draft (scroll-snap): `scroll-snap-type: none` — штатный off-switch; [MDN — scroll-snap-stop](https://developer.mozilla.org/en-US/docs/Web/CSS/scroll-snap-stop), [MDN — scroll-padding](https://developer.mozilla.org/en-US/docs/Web/CSS/scroll-padding).
- Bleed-паттерн: Smashing Magazine — «CSS Scroll Snapping Aligned With Global Page Layout» (Brecht De Ruyte, 12.2023), [bram.us — full-bleed scroll-snap carousel](https://www.bram.us/2021/05/20/css-full-bleed-scroll-snapping-carousel-with-centered-slides/), [web.dev — Well-controlled scrolling with CSS Scroll Snap](https://web.dev/articles/css-scroll-snap).
- Проект: `apps/frontend/shared/ui/pull-to-refresh/pull-math.ts` (+ тесты), `apps/frontend/widgets/payments/ui/payments-global-screen.tsx`, `apps/frontend/widgets/payments/ui/payments-favorites-screen.tsx`, `apps/frontend/DESIGN.md` (§1, §8, §14), `apps/frontend/CODING_STANDARDS.md` («Column width rule»), [docs/research/2026-09-23-favorites-dnd-library.md](../docs/research/2026-09-23-favorites-dnd-library.md), [docs/research/2026-09-22-bottom-sheet-motion-best-practices.md](../docs/research/2026-09-22-bottom-sheet-motion-best-practices.md), `apps/frontend/shared/lib/hooks/useReducedMotion.ts`.
