# Моушн bottom sheet: лучшие практики 2024–2026 и варианты для шита «Ещё» (vaul → ?)

- Дата: 2026-09-22. Research — следующий шаг за [docs/research/2026-09-08-sheet-eshche-animation.md](2026-09-08-sheet-eshche-animation.md): то решение («остаёмся на vaul», тайминги 400/300/250ms) реализовано, но владелец в живом использовании оценивает анимацию как «не плавную и некрасивую». Задача этого дока — разобрать, что именно делает sheet-анимации «плавными» по канонам 2024–2026, найти причины неплавности в нашей текущей конфигурации и предложить быстрые A/B-варианты для живой итерации с владельцем.
- Объект: `apps/frontend/shared/ui/design/more-sheet.tsx` — vaul ^1.1.2, `Drawer.Root` controlled (open/onOpenChange), без snap points, контент статичен; тайминги в `apps/frontend/app/globals.css` (`.more-sheet-*`): выезд 400ms / закрытие 300ms / оверлей 250ms, кривая vaul `cubic-bezier(0.32,0.72,0,1)`, reduced-motion 150ms.
- Метод: первоисточники — блог и код Эмиля Ковальски (vaul), **исходники установленного vaul 1.1.2 из node_modules** (dist-бандл разобран построчно), исходники Base UI Drawer и shadcn/ui Drawer (GitHub, ветки master/main), токены Material 3 из репозитория material-web (v34.0.21), web.dev/MDN/motion.dev, NN/g. Ссылки на все источники — в конце; недоступное помечено.

## Резюме — что конкретно делать в нашем стеке

1. **Главная причина «неплавности» — не тайминги, а не-прерываемость.** Vaul в нашем режиме (без snap points) анимирует открытие/закрытие CSS-@keyframes по `data-state` (`slideFromBottom`/`slideToBottom`, `fadeIn`/`fadeOut`). Keyframes **нельзя плавно перезапустить** из текущей позиции: тап «Ещё» в середине закрытия → лист заново выезжает с нижнего края; тап-закрытие в середине открытия → лист скачком дорисовывается до полного, затем задвигается. Это подтверждается и правилом самого автора vaul: «CSS transition can be interrupted and smoothly transition to a new value» — про keyframes этого сказать нельзя. Лечится только переходом на transition-модель (варианты B/C).
2. **Вторая причина — «фазовость» движения из-за жёстко зашитых 500ms.** В коде vaul 1.1.2 `TRANSITIONS = { DURATION: 0.5, EASE: [0.32, 0.72, 0, 1] }`: после drag-release закрытие/возврат идут по **inline** transition 500ms — наши CSS-override 300ms при этом не работают (inline перебивает таблицу стилей). Итог: тап-закрытие 300ms, флик-закрытие 500ms, возврат после неудавшегося драга 500ms, открытие 400ms — лист закрывается с разной скоростью в зависимости от способа закрытия. Это и ощущается как «несвязность».
3. **Третья причина — «мёртвая» зона после открытия.** Vaul блокирует drag на 500ms после открытия (`shouldDrag` проверяет `openTime`), и восстанавливает `pointer-events` тела по таймеру 500ms (не по фактическому концу анимации): при наших 300ms закрытия после визуального закрытия остаётся ~200ms, когда тапы по странице могут «проглатываться», а повторное быстрое открытие — не отвечать на протаскивание.
4. **Скорость: для маленького нав-шита 400ms — много.** Каноны: Emil Kowalski — «обычно короче 300ms»; MD3 — medium2 = 300ms для entering/exiting (medium4 = 400ms — верхняя граница для больших поверхностей); референс shadcn на Base UI — 450ms только для полноэкранных шитов. Шит «Ещё» высотой ~300–350px — кандидат на 250–300ms открытия и 200–250ms закрытия (закрытие быстрее открытия — у нас уже так, это канон верный).
5. **Кривая: для входа — decelerate (резкий старт, мягкое прибытие).** Наша `cubic-bezier(0.32,0.72,0,1)` — валидный ease-out с длинным хвостом (iOS-подобный), но на 400ms хвост читается как «вязкость». Альтернативы из первоисточников: MD3 emphasized-decelerate `cubic-bezier(0.05, 0.7, 0.1, 1)`; референс shadcn/Base UI для popup — `cubic-bezier(0.22, 1, 0.36, 1)` (easeOutQuint-семейство). Для выхода — accelerate-семейство (MD3 emphasized-accelerate `cubic-bezier(0.3, 0, 0.8, 0.15)`) или та же кривая, но короче.
6. **Синхронность оверлея и листа: оверлей — быстрее листа, но в той же фазе.** При прерывании keyframes оверлей и лист перезапускаются независимо → рассинхрон. В transition-моделях (Base UI) оверлей и лист ретаргетятся одновременно и, кроме того, оверлей непрерывно следует за пальцем через CSS-переменную `--drawer-swipe-progress` (в vaul без snap points оверлей тоже следует за пальцем — inline opacity — это у нас работает, но с keyframe-фазами не совпадает по времени).
7. **Velocity-scaling — то, чего у vaul нет, а «плавность» во многом из него.** Референс Base UI: длительность выхода после свайпа `calc(var(--drawer-swipe-strength) * 400ms)`, где strength ∈ [0.1..1] — от скорости отпускания. Быстрый флик = короткий доезд, медленный отпускаемый драг = длинный. У vaul после release длительность всегда 500ms, откуда бы ни летели.
8. **Точечные правки CSS поверх vaul (вариант A) дают ~50% эффекта за 30 минут** (скорость + кривая + согласование фаз), но **не дают** прерываемость и velocity-scaling. Полный набор даёт только переход на transition-модель: минимально — переопределить keyframes на transitions через `@starting-style` (вариант B, Baseline с 08.2024), системно — переезд этого шита на Base UI Drawer (вариант C).
9. **`prefers-reduced-motion`: актуальная рекомендация — не выключать движение, а упрощать**: короткий fade + сдвиг без «длинного хвоста» (у нас 150ms — соответствует), drag-жесты не трогать (это управление, не декор; WCAG 2.3.3, Apple HIG «Make motion optional», NN/g про swipe-dismiss как барьер).

## 1. Что делает sheet-анимацию «плавной и красивой» (сводка канонов)

### 1.1 Прерываемость (interruptibility) — главное

- Emil Kowalski, «Great animations»: анимация должна позволять сменить состояние в середине полёта «while maintaining a smooth transition»; в CSS для этого **transitions, не keyframes** («A CSS transition can be interrupted and smoothly transition to a new value»).
- Практический смысл для нас: владелец тапает «Ещё» и тут же передумывает — анимация обязана плавно развернуться из текущей позиции, а не рестартовать. В vaul это сломано архитектурно (keyframes на data-state).
- Transition-ретаргетинг — базовое свойство CSS-переходов, поддерживается во всех актуальных браузерах; именно на нём построены Base UI Dialog/Drawer (`data-starting-style`/`data-ending-style`).

### 1.2 Длительности

| Источник | Рекомендация |
|---|---|
| Emil Kowalski | «обычно короче 300ms»; частые действия — ещё короче |
| Material 3 (токены material-web v34) | short4 200 / medium1 250 / **medium2 300** / medium3 350 / medium4 400ms |
| shadcn/ui Drawer (Base UI, реф. 2026) | popup 450ms (большой шит), overlay 450ms, handle 200ms, content opacity 300ms |
| vaul 1.1.2 | 500ms на всё (вверх границы) |

Для шита фиксированной навигационной высоты (~350px, открытие по кнопке) разумный коридор: **открытие 250–300ms, закрытие 200–250ms**, оверлей — на 50ms быстрее листа либо в той же фазе. Наши 400/300 — «тяжеловато», но не катастрофа; скачки 300↔500 между способами закрытия хуже, чем сама цифра.

### 1.3 Кривые

- **Enter — decelerate**: старт мгновенный (ответ на тап), прибытие мягкое. MD3: emphasized-decelerate `cubic-bezier(0.05, 0.7, 0.1, 1)`; shadcn/Base UI popup: `cubic-bezier(0.22, 1, 0.36, 1)`. Наша `cubic-bezier(0.32, 0.72, 0, 1)` — тоже ease-out, но с более пологим разгоном: на 400ms лист «разгоняется на глазах» — то самое «не плавно» на входе.
- **Exit — accelerate либо короткий ease-out**: MD3 emphasized-accelerate `cubic-bezier(0.3, 0, 0.8, 0.15)` — лист «уходит под палец» с ускорением; тормозить в конце перед уходом за экран — ошибка (замедление к тому, что исчезает).
- **Springs**: уместны для «живых» микро-взаимодействий (Emil) и как физика drag-release; в CSS воспроизводятся через `linear()` (Baseline с 2023: Chrome 113, Firefox 112, Safari 17.2) — сэмплированная пружина фиксированной длительности, без ретаргетинга. Для нав-шита с drag-физикой vaul/Base UI spring-движок не обязателен.
- MD3 v34 (первоисточник — material-web) хранит emphasized как `$easing-emphasized: $easing-standard` = `cubic-bezier(0.2, 0, 0, 1)`; спринг-токены: spatial fast — stiffness 1400, damping 0.9 (для «быстрых» пространственных переходов — ориентир, если пойдём в спринг-вариант D).

### 1.4 Непрерывность жеста и velocity

- Идеал iOS: между «палец ведёт лист», «отпустил» и «лист доезжает» нет смены фаз — доезд стартует с текущей позиции и текущей скорости.
- Рецепт Base UI: во время свайпа `data-swiping` → `transition-duration: 0` (лист = палец через CSS-переменную); на release длительность = `swipe-strength * 400ms` (strength — от скорости) + стартовая позиция = текущая (transition всегда от computed value). Отсюда «бар ChatGPT/iOS»-ощущение.
- У vaul velocity учитывается только в решении «закрыть/не закрыть» (`velocity > 0.05` порог и `VELOCITY_THRESHOLD`), но не в длительности доезда.

### 1.5 Синхронность оверлея и листа; скейл фона; radius

- Оверлей: fade **короче или равен** движению листа, начинается одновременно; при drag — непрерывно следует прогрессу (в vaul без snap points — да, inline opacity; в Base UI — через `--drawer-swipe-progress`).
- `shouldScaleBackground` (масштаб «страница = вторая карточка») — по-прежнему не включаем: для короткого шита мало эффекта, известны баги (research 09-08, §2; vaul #259). Скейл фона — опция «красивости» для полноэкранных шитов, не для нашего.
- Radius-scaling при drag: у vaul радиус 40px пропорционально уменьшается при протаскивании — работает из коробки, не трогаем. (В Base UI реф. — «bleed»-подложка вместо radius-scaling.)

### 1.6 Композитинг и производительность

- Анимировать только `transform`/`opacity` (Emil; motion.dev «Web animation performance tier list»: compositor-driven transform/opacity — S-tier; layout — D-tier). Наш лист — уже так.
- CSS-переменные во время drag — C-tier (триггерят paint), лечится `CSS.registerProperty` c `inherits: false` — ровно это делает Base UI Drawer для `--drawer-swipe-*` (ссылаясь на тот же tier list). У vaul переменная `--initial-transform` наследуемая, но статичная (не пишется в кадре) — ок.

### 1.7 Смысл и сдержанность

- Apple HIG Motion: «Gratuitous or excessive animation can distract people and may make them feel disconnected or physically uncomfortable», «Make motion optional» (страница JS-рендерится; формулировки приведены по поисковой выдаче developer.apple.com — см. Источники).
- NN/g «Bottom Sheets»: шит — транзиентный элемент поверх контекста; закрытие — оверлей-тап, Esc/Back, свайп; visible-close для доступности. Наш шит всем этому соответствует (повторный тап «Ещё» попадает в оверлей).

## 2. Современная CSS-база (2025–2026) применительно к sheet-паттерну

| Фича | Baseline | Что даёт sheet-у |
|---|---|---|
| `@starting-style` | Baseline 2024, Newly available (авг. 2024; Chrome 117+, Firefox 129+, Safari 17.5+ по MDN; точные версии — compat-таблица MDN) | Enter-переход для свежесмонтированного элемента (наш случай: Portal-контент маунтится в open-состоянии) — без keyframes |
| `transition-behavior: allow-discrete` | Baseline 2024 (там же) | Переход display/overlay при выходе из top-layer; нам не нужен, пока vaul/Radix сами держат элемент до конца exit-анимации |
| `linear()` easing | 2023 (Chrome 113, Firefox 112, Safari 17.2) | Пружины в чистом CSS; без ретаргетинга — для sheet-а вторично |
| `overscroll-behavior` | давно | Внутренних скролл-областей в шите нет — не требуется |
| `interpolate-size` / `calc-size()` | Chrome-only (2024+) | Не нужно: шит фиксированной высоты |

Вывод: «transition-only» sheet-анимация на `@starting-style` — валидный прод-паттерн на всём нашем парке (вариант B). Полностью CSS-замена vaul невозможна: drag-to-dismiss требует JS.

## 3. Ландшафт библиотек (2026)

- **vaul 1.1.2** — последняя версия и в GitHub Releases, и в npm-реестре — 1.1.2 от 14.12.2024. README: «This repo is unmaintained. I might come back to it at some point, but not in the near future. This was and always will be a hobby project…» (подтверждено на github.com/emilkowalski/vaul). Значимого форка-преемника с релизами нет (поиск; shadcn ушёл на Base UI).
- **Base UI Drawer** (`@base-ui/react/drawer`) — shadcn/ui с июля 2026 делает Base UI дефолтом («Run npx shadcn init and Base UI is the default pick»; «Base UI is stable. It's at 1.6.0 with 6M+ weekly downloads» — changelog 2026-07). Migration guide vaul→Base UI официальный (ui.shadcn.com/docs/components/base/drawer). Анимации — transition-based (`data-open/closed/starting-style/ending-style`, `data-swiping`, `data-swipe-dismiss`), drag — через зарегистрированные CSS-переменные (`--drawer-swipe-movement-y`, `--drawer-swipe-progress`, `--drawer-swipe-strength`); velocity-scaling длительности выхода; snap points; nested. Чего нет против vaul: `shouldScaleBackground`, `handleOnly`, `repositionInputs` — нам и не нужны (research 09-08). Референс shadcn (apps/v4/registry/bases/base/ui/drawer.tsx): popup `duration-450 ease-[cubic-bezier(0.22,1,0.36,1)]`, overlay `duration-450 ease-[cubic-bezier(0.32,0.72,0,1)]` + `opacity: calc(1 - var(--drawer-swipe-progress))`.
- **motion / framer-motion** — актуальная линия 13.x (13.4.0 в npm на 09.2026; `motion` и `framer-motion` публикуются синхронно). У нас в package.json заявлен `framer-motion ^12.41.0`, в исходниках не используется (проверено grep-ом) — как зависимость для варианта D «уже оплачена», но: анимации — rAF на main-thread (A-tier по motion.dev; может ронять кадры под нагрузкой — сам Эмиль приводит кейс Vercel), Radix-a11y не даёт — гибрид «Radix Dialog для a11y + motion для движения» дублирует то, что Base UI Drawer даёт одним компонентом.
- **Radix Dialog + своё** — по-прежнему «дорого и рискованно» (research 09-08).

### Сравнение вариантов носителя анимации для нашего шита

| Критерий | vaul 1.1.2 (сейчас) | vaul + CSS/`@starting-style` хак | Base UI Drawer | motion-спринг |
|---|---|---|---|---|
| Прерываемость open/close | нет (keyframes) | да (transitions) | да (transitions) | да (springs) |
| Единая скорость закрытия | нет (inline 500ms после drag) | частично (drag-release всё ещё 500ms) | да (velocity-scaling) | да |
| Оверлей следует за пальцем | да (inline opacity) | да | да (`--drawer-swipe-progress`) | да (payload velocity) |
| Velocity-scaling длительности | нет | нет | да | да (springs) |
| Drag-физика (momentum, flick) | да, зрелая | та же | да (проще дэмпинг, но со strength) | да, руками |
| A11y (focus trap, Esc, aria) | Radix | Radix | встроенное (Floating Focus Manager) | собирать самому |
| Риск/объём работ | 0 | мал (CSS + проверки Presence) | средний (один компонент + dep) | высокий |
| Поддержка | unmaintained | — | активная (shadcn-дефолт) | активная |

## 4. Причины «неплавности» конкретно нашей конфигурации → лекарства

Все пункты выверены по dist-исходникам `apps/frontend/node_modules/vaul/dist/index.mjs` (vaul 1.1.2).

1. **Перезапуск keyframes при прерывании.** `slideFromBottom` имеет `from { transform: translate3d(0, var(--initial-transform, 100%), 0) }`; `slideToBottom` — только `to`. Прерывание open→closed: старая анимация снимается, transform возвращается к базовому (0 = полностью открыт) и едет вниз — видимый скачок «дорисовало до верха». Прерывание closed→open: рестарт со 100% — лист, бывший на 70% пути, прыгает вниз и едет заново. *Лекарство:* transition-модель (B/C).
2. **Несогласованные длительности закрытия.** CSS `[data-state='closed'] { animation-duration: 300ms }` работает только для keyframe-закрытия (тап/Esc/повторный тап). Fлик-закрытие и возврат после драга ставят inline `transition: transform 0.5s cubic-bezier(0.32,0.72,0,1)` — 500ms вопреки нашему CSS. *Лекарство:* принять как дань (флик инерционен) либо B/C; `!important`-перебивание inline не делать — потеряем непрерывность драга.
3. **Мёртвая зона 500ms.** `shouldDrag` отсекает drag, если с момента открытия прошло <500ms; `pointer-events` тела восстанавливается таймером 500ms после close-start (не по факту анимации). При наших 300ms закрытия — ~200ms «глухоты» после исчезновения листа; быстрый повторный сценарий «открыл-закрыл-открыл» ощущается залипшим. *Лекарство:* C (нет таких таймеров); частично — сокращение наших длительностей ничего не меняет, это внутренние константы vaul.
4. **Первый кадр/вспышки.** Для controlled-shита со статичным контентом Portal-контент маунтится в момент открытия, keyframe стартует с 100% — вспышки «голого» листа в норме нет; известные кейсы вспышек завязаны на `shouldScaleBackground` (#259) и SSR/defaultOpen — у нас их нет.
5. **Рассинхрон фаз оверлея.** Оверлей 250ms fadeOut стартует одновременно с 300ms slideToBottom — при непрерывном закрытии всё согласовано; при прерывании (п.1) обе keyframes рестартуют независимо → на долю секунды фон и лист «разъезжаются». *Лекарство:* B/C (единый transition-контекст).
6. **Scroll-lock на iOS.** `usePositionFixed` ставит body `position: fixed` с компенсацией скролла на время открытия — классический источник «дёргания» фона на iOS при открытии/закрытии (и ряда открытых issues vaul). Проверить на живом iPhone в `/ui-walkthrough`: если фон дёргается — это vaul, а не наши тайминги; C перепроверяется отдельно (там `body { position: relative }` из миграционного гайда).

## 5. `prefers-reduced-motion` — актуальная рекомендация

- Не выключать движение полностью, а укорачивать и упрощать до fade (Emil: замена bounce на fade; у нас 150ms — соответствует); drag-жесты сохранять — это управление (WCAG 2.3.3 требует отключаемости именно декоративной анимации; NN/g отдельно предупреждает про полный отказ от swipe-dismiss как барьер).
- Если уйдём на Base UI — гейт остаётся ручным CSS (у Base UI нет встроенного reduced-motion на момент ресёрча; проверять при внедрении). В motion-варианте D — `MotionConfig reducedMotion="user"`.

## Варианты внедрения (для живой A/B-итерации с владельцем)

Механика A/B для всех вариантов одинакова: второй вариант анимации включается классом на `<html>` (например `sheet-motion-v2`) или временным флагом в коде — владелец смотрит оба на стенде (`/ui-walkthrough` в видимом браузере + живой iPhone) и выбирает. Все варианты не требуют изменения геометрии шита и TabBar.

### Вариант A — точечная правка CSS поверх vaul (полировка таймингов/кривых)

Только `globals.css`: открытие 400→280–300ms на `cubic-bezier(0.05, 0.7, 0.1, 1)` (MD3 emphasized-decelerate) либо 300ms на текущей кривой; закрытие 300→220–250ms на `cubic-bezier(0.3, 0, 0.8, 0.15)`; оверлей 250→200ms; reduced-motion 150→120ms. Заодно почистить мёртвые `transition-duration`-строки (для plain open/close их перебивают keyframes, а после drag — inline; работают только `animation-duration`).

- Плюсы: ~30 минут, ноль риска, мгновенный A/B.
- Минусы: не чинит прерываемость (п.4.1), 500ms флик (4.2), мёртвую зону (4.3).
- Ожидаемый эффект: лист станет «бодрее», но тап-в-полёте останется скачком.
- Рекомендую как **первый шаг в любом случае** — чтобы отделить в живом просмотре эффект скорости от эффекта архитектуры.

### Вариант B — vaul + transition-модель через `@starting-style` (прерываемость без новых зависимостей)

В CSS: `[data-vaul-drawer] { animation: none !important }` (гасим keyframes vaul), наш transition `transform 300ms cubic-bezier(0.05,0.7,0.1,1)`; `@starting-style { transform: translate3d(0,100%,0) }` для входа; на `[data-state='closed']` — `transform: translate3d(0,100%,0)` + transition для выхода; оверлей — аналогично на opacity. CSS-transitions ретаргетятся → прерывание из любой точки плавное. Radix Presence ждёт и transitionend, поэтому контент не размонтируется раньше времени (проверить живьём — это главный риск).

- Плюсы: ~1–2 часа, ноль новых зависимостей, чинит главную жалобу (прерываемость), фазы оверлея синхронны.
- Минусы: пост-drag inline 500ms и мёртвую зону 500ms не убирает; опора на недокументированное поведение vaul+Radix (Presence) — при будущих переездах хрупко; требует проверки на iOS Safari (Baseline Safari 17.5+ — проверить целевые устройства проекта).
- A/B: идеально — чистый CSS-переключатель.

### Вариант C — переезд MoreSheet на Base UI Drawer (целевое решение)

`@base-ui/react` (+ Drawer из него), миграция одного файла `more-sheet.tsx` по официальному гайду shadcn: `direction="bottom"`→`swipeDirection="down"`, `asChild`→`render`, оверлей = `Drawer.Backdrop`, контент = `Drawer.Popup` внутри `Drawer.Viewport`; тайминги как в shadcn-референсе (450ms popup на `cubic-bezier(0.22,1,0.36,1)`, для нашего размера — 300–350ms) или свои A/B-значения из варианта A. Из коробки: прерываемость, velocity-scaling выхода (`--drawer-swipe-strength`), оверлей по drag-прогрессу, `data-swiping: duration-0`, нет 500ms-таймеров. Глобальное требование — `body { position: relative }` (iOS Safari overlay); `SupportModal` остаётся на Radix/vaul (вложенность Dialog проверять).

- Плюсы: полный набор «плавности» из коробки; активная поддержка; официальный миграционный путь; изолированный риск (один компонент, поверх собственной Viewport-модели, проектный `Modal` не трогаем).
- Минусы: новая зависимость при канонизированном Radix/vaul-стеке (ADR 0050) — если шит останется единственным потребителем, это «второй стек ради одного шита»; поведение scroll-lock/фокуса перепроверять; ~0.5–1 день с тестами.
- A/B: рендерить Base UI-версию рядом под флагом; это же прототип на случай будущего общего переезда дизайн-слоя (shadcn-мейнстрим).

### Вариант D — motion-спринг (framer-motion уже в package.json)

`AnimatePresence` + `drag="y"` + spring (ориентир — MD3 fast spatial: stiffness 1400 / damping 0.9 в нотации MD3; в нотации motion подбирать на глаз, старт ~ stiffness 400–500, damping 40–50) поверх Radix Dialog для a11y.

- Плюсы: springs по-настоящему прерываемы и несут velocity; reduced-motion из коробки; зависимость формально уже заявлена.
- Минусы: rAF-анимации на main-thread (риск джанка под нагрузкой — tier list, кейс Vercel); a11y-обвязку собираем сами или гибридим с Radix — дублирование; против варианта C выгода только в «пружинности», которую для iOS-подобного шита канон и не требует.

### Рекомендация

Двухступенчато: **A сейчас** (полировка, чтобы в живом просмотре отделить «медленно/вязко» от «прыгает при прерывании») → **C как целевое решение**, если владелец по-прежнему не доведён прерываемостью и «фазовостью» (почти наверняка, раз жалоба на живое использование, а не на скорость). B — запасной путь, если владелец не хочет новой зависимости: он закрывает главный визуальный баг (скачки) ценой хрупкости. D — только по прямому запросу «хочу пружину».

## Подводные камни

1. **Не перебивать inline-переопределение `!important`** — сорвём `transition: none` во время драга (лист отстанет от пальца) и/или непрерывность release; все правки — только через `animation-*` (B: через выключение анимаций целиком).
2. **`onAnimationEnd` vaul — это `setTimeout(500)`**, не факт конца CSS-анимации; если когда-нибудь завяжем логику на конец анимации — считать по своим длительностям, не по коллбэку vaul.
3. **`@starting-style` и Safari**: Baseline с Safari 17.5 — на более старых iOS вход-переход не запустится (лист появится сразу). Прежде чем принимать B — проверить парк целевых iOS; деградация «без анимации входа» заметна.
4. **Radix Presence + transition**: при переходе на B убедиться, что exit-transition держит элемент смонтированным до конца (иначе лист исчезнет мгновенно на закрытии). Проверка — тап-закрытие и drag-закрытие в Chrome+Safari.
5. **Base UI Drawer требует `body { position: relative }`** глобально (миграционный гайд shadcn, iOS Safari overlay) — добавлять аккуратно, с проверкой всех fixed-слоёв приложения (TabBar, StickyBottomBar, тосты).
6. **Base UI и vaul в одном бандле**: на время A/B/C оба стека будут в графе зависимостей; после выбора варианта — удалить невыбранный (vaul при C остаётся для `Modal`-шитов <768px — зависит от решения по ADR 0050, в рамки этого шита не тащим).
7. **Reduced-motion не трогает drag** — при всех вариантах сохранять принцип «укорачиваем, не отключаем» (research 09-08 §9 остаётся в силе).
8. **Оверлей-цвет**: `bg-overlay = rgba(23,26,28,0.5)` — при переезде на Base UI следить, что `--drawer-swipe-progress`-формула даёт тот же 0.5 в покое (референс shadcn использует `min-opacity`-переменную только со snap points).

## Источники

- Emil Kowalski, «Great animations» — <300ms, ease-out vs springs, interruptibility (transitions vs keyframes), transform/opacity, reduced-motion — https://emilkowal.ski/ui/great-animations (2024)
- Emil Kowalski, «Building a drawer component» — кривая 0.5s cubic-bezier(0.32,0.72,0,1) из Ionic («to mimic iOS's Sheet»), drag-физика, `--swipe-amount` и отказ от наследуемых CSS-переменных из-за recalc — https://emilkowal.ski/ui/building-a-drawer-component (2024)
- vaul: GitHub Releases (v1.1.2, 14.12.2024 — последняя) — https://github.com/emilkowalski/vaul/releases; README с баннером «This repo is unmaintained…» — https://github.com/emilkowalski/vaul; npm latest = 1.1.2 — https://registry.npmjs.org/vaul
- **Исходники vaul 1.1.2 (первичный источник раздела 4):** `node_modules/vaul/dist/index.mjs` установленного пакета: `TRANSITIONS = { DURATION: 0.5, EASE: [0.32, 0.72, 0, 1] }`; keyframes `slideFromBottom/slideToBottom/fadeIn/fadeOut` на `data-state`; inline transition в `closeDrawer`/`resetDrawer`; блок drag 500ms в `shouldDrag` (`openTime`); restore `pointer-events` по `setTimeout(500)`; `shouldFade = … || !snapPoints` (оверлей следует за пальцем); GitHub-версия тех же исходников — https://github.com/emilkowalski/vaul/blob/master/src/index.tsx
- Material 3 motion: токены.easing/duration из официального репозитория material-web v34.0.21 (первоисточник значений; сайт m3.material.io — SPA и не читается фетчером): durations short1..extra-long4 (50–1000ms; medium2=300, medium4=400), `$easing-emphasized: $easing-standard = cubic-bezier(0.2, 0, 0, 1)`, emphasized-decelerate `cubic-bezier(0.05, 0.7, 0.1, 1)`, emphasized-accelerate `cubic-bezier(0.3, 0, 0.8, 0.15)`, spring fast spatial stiffness 1400 / damping 0.9 — https://github.com/material-components/material-web/blob/main/tokens/versions/latest/sass/_md-sys-motion.scss ; спецификация — https://m3.material.io/styles/motion/easing-and-duration/tokens-specs
- Apple HIG, Motion — принципы («Gratuitous or excessive animation…», «Make motion optional»); страница JS-рендерится, цитаты — по поисковой выдаче developer.apple.com — https://developer.apple.com/design/human-interface-guidelines/motion
- Nielsen Norman Group, «Bottom Sheets» (2023) — modal/nonmodal, dismissal, «Do Not Stack Bottom Sheets» — https://www.nngroup.com/articles/bottom-sheet/
- web.dev, «Now in Baseline: animating entry effects» (авг. 2024) — `@starting-style` + `transition-behavior: allow-discrete` в Baseline — https://web.dev/blog/entry-effects-baseline (прямой URL не открылся; статус подтверждён MDN: https://developer.mozilla.org/en-US/docs/Web/CSS/@starting-style — «Baseline 2024, Newly available», август 2024)
- MDN, `linear()` — базовая поддержка с 2023 — https://developer.mozilla.org/en-US/docs/Web/CSS/easing-function/linear() ; пружины через linear(): Josh Comeau «Springs and Bounces in Native CSS» (2025) — https://www.joshwcomeau.com/animation/springs-and-bounces-in-native-css/ ; генератор PQINA — https://pqina.nl/linear/
- motion.dev, «Web animation performance tier list» — S-tier compositor (transform/opacity), C-tier CSS-переменные + фикс через `registerProperty`/`inherits:false`, F-tier thrashing — https://motion.dev/blog/web-animation-performance-tier-list (на этот блог ссылается сам Base UI Drawer в исходниках)
- Base UI Drawer: исходники — popup/backdrop/viewport + CSS-vars (`--drawer-swipe-movement-y`, `--drawer-swipe-progress`, `--drawer-swipe-strength` = «scalar (0.1-1) used to scale the swipe release transition duration in CSS») — https://github.com/mui/base-ui/tree/master/packages/react/src/drawer ; data-атрибуты `data-starting-style`/`data-ending-style`/`data-swiping`/`data-swipe-dismiss` — packages/react/src/drawer/popup/DrawerPopupDataAttributes.ts. Внимание: сам https://base-ui.com/react/components/drawer из среды ресёрча не открылся (таймаут ×3) — API выверен по исходникам, не по сайту.
- shadcn/ui: «Base UI as the default» (июль 2026) — https://ui.shadcn.com/docs/changelog/2026-07-base-ui-default ; миграция vaul→Base UI — https://ui.shadcn.com/docs/components/base/drawer ; референс-реализация Drawer на Base UI (popup `duration-450 ease-[cubic-bezier(0.22,1,0.36,1)]`, backdrop `duration-450 ease-[cubic-bezier(0.32,0.72,0,1)]` + `calc(1 - var(--drawer-swipe-progress))`, `data-ending-style:duration-[calc(var(--drawer-swipe-strength)*400ms)]`) — https://github.com/shadcn-ui/ui/blob/main/apps/v4/registry/bases/base/ui/drawer.tsx
- motion / framer-motion: npm latest 13.4.0 (обе публикации) — https://registry.npmjs.org/motion ; репозиторий — https://github.com/motiondivision/motion
- WCAG 2.3.3 Animation from Interactions — https://www.w3.org/WAI/WCAG22/Understanding/animation-from-interactions.html
- Предыдущий research проекта (контекст и решения, не отменённые этим доком) — [2026-09-08-sheet-eshche-animation.md](2026-09-08-sheet-eshche-animation.md)
