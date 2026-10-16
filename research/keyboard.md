# Клавиатура на мобильном экране суммы — автоподъём и кнопка над клавиатурой

Research-тикет [#1148](https://github.com/devnumbers/arenda-platform/issues/1148) (part of #1147), 2026-10-06.

Контекст продукта: `apps/frontend` (Next.js 16.3.7 App Router, React 19). Сейчас:

- `shared/ui/design/amount-field.tsx` — скрытый `input type="text" inputMode="decimal"` поверх невидимого span-измерителя; ширина input равна ширине набранного текста (узкая зона тапа).
- `shared/ui/design/sticky-bottom-bar.tsx` — `fixed inset-x-0 bottom-0 z-40`, нижний паддинг `pb-[max(1.5rem,env(safe-area-inset-bottom))]`; клавиатура перекрывает кнопку.
- `app/layout.tsx` уже экспортирует `viewport: Viewport` (`width: device-width`, `initialScale: 1`, `viewportFit: cover`) — поле `interactiveWidget` ещё не задано; тип `Viewport.interactiveWidget` в установленном Next 16.3.7 есть (`next/dist/lib/metadata/types/extra-types.d.ts`).
- Визард платежа/операции — одностраничный клиентский: шаги переключаются `setStep` в `useState` внутри одного route (`widgets/payments/ui/payment-create-wizard/payment-create-wizard-flow.tsx`), то есть появление шага суммы — прямой результат клика «Продолжить», а не навигация по роуту. Это ключевой факт для вердикта A.

---

## A. Автоподъём клавиатуры при появлении шага

### ВЕРДИКТ-ГЕЙТ

> **Автоподъём на iOS: ВОЗМОЖЕН С ОГОВОРКАМИ.**
>
> iOS Safari (WebKit) намеренно не поднимает программную клавиатуру: программный `focus()` показывает клавиатуру **только если фокус происходит в ответ на жест пользователя** (та же задача event loop, что и обработчик клика). Это заявленная политика Apple, а не баг: «we do not want programmatic focus to bring up the keyboard when you do not have a hardware keyboard attached» вне жеста — [WebKit bug 195884, комментарий Apple/Daniel Bates, 19.03.2019](https://bugs.webkit.org/show_bug.cgi?id=195884).
>
> Следствия для #1151:
> 1. **Работает** — фокус синхронно внутри жеста «Продолжить»: шаг суммы в проекте монтируется из `setStep` в обработчике клика (дискретное событие), React коммитит апдейт до возврата управления браузеру; `autoFocus` на монтируемом input либо `ref.focus()`/`flushSync` в обработчике остаётся в задаче жеста → клавиатура поднимается.
> 2. **Не работает** — фокус после `await`/`setTimeout`/восстановления черновика/навигации по роуту: задача жеста завершена, WebKit клавиатуру не покажет. Формально гарантии нет и от `autofocus`-на-монте вне жеста; HTML-спека вообще не регулирует клавиатуру — решение отдаётся user agent ([WHATWG HTML, autofocus](https://html.spec.whatwg.org/multipage/interaction.html#the-autofocus-attribute); [MDN autofocus](https://developer.mozilla.org/en-US/docs/Web/HTML/Reference/Global_attributes/autofocus) — «can also cause dynamic keyboards to display on some touch devices», без гарантии).
> 3. **«Обязательный» автоподъём на iOS недостижим** — платформа исключает подъём без жеста; способа «показать клавиатуру без жеста» в Safari нет (VirtualKeyboard API WebKit не поддерживает, см. B).

Отсюда: тикет #1151 (автоподъём при переходе «Продолжить» → шаг суммы) **стартовать можно** — путь внутри жеста есть; но автоподъём при возврате на шаг назад / восстановлении черновика / deep-link на шаг суммы на iOS сделать нельзя — проектировать эти ветки без ожидания клавиатуры.

### Почему так — по источникам

- **Формально клавиатура вне стандарта.** Focusing steps и `autofocus` в [WHATWG HTML](https://html.spec.whatwg.org/multipage/interaction.html#the-autofocus-attribute) ничего не говорят о виртуальной клавиатуре: показывать её при фокусе или нет — политика браузера. WebKit и Blink её ограничивают.
- **Apple — намеренно.** [WebKit bug 195884](https://bugs.webkit.org/show_bug.cgi?id=195884): фокус без жеста — клавиатура не показывается (аргументация: клавиатура занимает экран, вызывает зум/скролл — «annoying and a distraction»); с подключённой аппаратной клавиатурой программный фокус работает (фикс bug 190017). В iOS 16 поведение уточнили только для нативных embedding-клиентов WKWebView (`evaluateJavaScript`), и то за linked-on-or-after проверкой — на поведение Safari это не влияет.
- **React 19 / flushSync.** Реакт батчит апдейты; [flushSync](https://react.dev/reference/react-dom/flushSync) — документированный способ гарантировать, что «by the time the next line of code runs, React has already updated the DOM», и «✅ Correct: flushSync in event handlers is safe». Практический вывод: фокус, выполненный в обработчике клика (напрямую `ref.focus()` или из синхронного коммита, вызванного `setStep` — дискретные события React коммитит до возврата браузеру), остаётся в задаче жеста. Для надёжности против будущих оптимизаций рендера можно явно обернуть `setStep` в `flushSync` — это документированно-безопасное место.
- **Android Chrome — та же политика.** `focus()` переносит фокус, но клавиатура показывается только при наличии user activation (жест); из таймеров/асинхронных колбэков — нет. Для явного управления у Chromium есть `navigator.virtualKeyboard.show()` (тоже требует активного элемента; Safari/Firefox не поддерживают — [MDN VirtualKeyboard.show](https://developer.mozilla.org/en-US/docs/Web/API/VirtualKeyboard/show)).
- **iOS 26 / Safari 26.** В обзорах релиза — [WebKit Features in Safari 26.0](https://webkit.org/blog/17333/webkit-features-in-safari-26-0/) и [News from WWDC25](https://webkit.org/blog/16993/news-from-wwdc25-web-technology-coming-this-fall-in-safari-26-beta/) — изменений поведения клавиатуры/viewport при фокусе нет; политика (клавиатура только из жеста) сохраняется.

### Рекомендуемый паттерн для #1151 (фрагмент)

```tsx
// payment-create-wizard-flow.tsx — переход на шаг суммы по «Продолжить»
import { flushSync } from 'react-dom';

const goNext = (): void => {
  flushSync(() => { setStep(nextStep); }); // синхронный коммит внутри жеста
  // шаг смонтирован; фокус ставит autoFocus на input шага суммы,
  // либо здесь: amountInputRef.current?.focus();
};
```

`AmountStep`/`WizardAmountField` на мобайле рендерит `AmountField` с `desktop:hidden` (второй ярус — отдельный input ПК-бокса): на мобиле input не в `display:none`, autoFocus применится к видимому полю — конфликтов ярусов нет. ПК не трогаем (≥1024 — автоподъём не нужен и не делается).

---

## B. Кнопка действия над клавиатурой (fixed bottom bar не должен перекрываться)

Платформенное поведение при открытой клавиатуре ([Chrome developers, «Prepare for viewport resize behavior changes coming to Chrome on Android»](https://developer.chrome.com/blog/viewport-resize-behavior)):

- **iOS Safari — «resizes visual only»**: клавиатура уменьшает **только visual viewport**; layout viewport не меняется, `position: fixed` элементы остаются на месте и «may be hidden behind the keyboard». Это и есть текущий баг проекта.
- **Android Chrome до 108** уменьшал layout viewport (fixed-панель сама поднималась); **с Chrome 108** дефолт сменили на «resizes visual only» — чтобы совпасть с iOS Safari.
- Firefox Android исторически уменьшал оба.

### Подход 1: viewport meta `interactive-widget=resizes-content`

- Что делает: клавиатура уменьшает **и layout viewport** → `fixed bottom-0` панель поднимается над клавиатурой сама, без JS.
- Поддержка: **Chrome/Chromium Android 108+**, Firefox Android 133+; **Safari (iOS/macOS) — не поддерживает, мету игнорирует**; десктопным Chrome она не нужна. Подтверждение: [MDN meta viewport → interactive-widget](https://developer.mozilla.org/en-US/docs/Web/HTML/Reference/Elements/meta/name/viewport) и BCD (`chrome_android: 108`, `firefox_android: 133`, `safari/safari_ios: false`). Специфицировано в черновике [CSS Viewport §interactive-widget](https://drafts.csswg.org/css-viewport/#interactive-widget-section). Chrome iOS игнорирует её по построению (движок WebKit — [Chrome blog](https://developer.chrome.com/blog/viewport-resize-behavior)).
- В Next.js выставляется в `app/layout.tsx` (поле документировано в [generateViewport reference](https://nextjs.org/docs/app/api-reference/functions/generate-viewport); `viewport`/`generateViewport` — с v14.0.0, в Next 16.3.7 тип на месте).

```ts
// apps/frontend/app/layout.tsx (дополнить существующий export)
export const viewport: Viewport = {
    width: 'device-width',
    initialScale: 1,
    // … themeColor / viewportFit / colorScheme без изменений …
    // Клавиатура уменьшает layout viewport: fixed-панель (StickyBottomBar)
    // поднимается над клавиатурой на Android Chrome 108+ / Firefox Android
    // 133+. iOS Safari мету игнорирует — там работает VisualViewport-хук.
    interactiveWidget: 'resizes-content',
};
```

- Побочный эффект (документирован в Chrome blog): при открытой клавиатуре на Android меняется ICB → значения `vh`/`vw` пересчитываются. В проекте размеры панели не завязаны на `vh` — риска нет; закладывать логику на `dvh` под клавиатуру нельзя ни на одной платформе.

### Подход 2: VirtualKeyboard API (`overlaysContent`, `env(keyboard-inset-*)`)

- Возможности: `navigator.virtualKeyboard.overlaysContent = true`, `boundingRect` + `geometrychange`, CSS `env(keyboard-inset-top/right/bottom/left/width/height)`, `virtualkeyboardpolicy="manual"` + `show()/hide()` — [MDN VirtualKeyboard API](https://developer.mozilla.org/en-US/docs/Web/API/VirtualKeyboard_API), [MDN VirtualKeyboard.show](https://developer.mozilla.org/en-US/docs/Web/API/VirtualKeyboard/show).
- Поддержка: **только Chromium (Chrome 94+, desktop и Android)**; Safari/WebKit — нет ([WebKit bug 230225](https://bugs.webkit.org/show_bug.cgi?id=230225)), Firefox — нет ([bug 1730568](https://bugzil.la/1730568)). У WebKit официальной позиции нет — issue в статусе «Needs position» с ярлыком concerns: device independence ([WebKit standards-positions #16](https://github.com/WebKit/standards-positions/issues/16)).
- Вывод: кроссплатформенным решением быть не может; как прогрессирующее улучшение на Android избыточен при рабочем `interactive-widget`.

### Подход 3 (iOS): VisualViewport API — трансляция fixed-панели в координаты visual viewport

- `window.visualViewport` (Safari/iOS 13+, Chrome 61+, Firefox 91+ — [MDN VisualViewport API](https://developer.mozilla.org/en-US/docs/Web/API/VisualViewport), BCD): клавиатура уменьшает `visualViewport.height`; canonical-паттерн из доков — перепозиционировать панель по `resize`/`scroll` (пример MDN: `footer.style.top = ${vv.offsetTop + vv.height - 50}px`). Панель с `position: fixed` привязана к layout viewport; смещение `translateY` на разницу «низ visual viewport − низ layout viewport» ставит её ровно над клавиатурой и корректно при пинч-зуме (значения уже в CSS-пикселях с учётом scale).
- В WebKit дальнейших изменений в Safari 26 по этой части не заявлено (см. ссылки на релизные обзоры выше).

#### Рекомендуемый код для проекта

Хук (новый файл, без пересечений с существующими либами — аналогов в `shared/lib` нет):

```ts
// apps/frontend/shared/lib/keyboard/use-visual-keyboard-inset.ts
'use client';

import { useEffect, useState } from 'react';

/**
 * Высота зазора под клавиатурой в CSS-пикселях: насколько низ visual
 * viewport (то, что пользователь видит) выше низа layout viewport, к
 * которому приколочена fixed-панель. 0 — клавиатура закрыта. iOS Safari
 * клавиатурой уменьшает только visual viewport (Chrome blog, «viewport
 * resize behavior», группа «resizes visual only»), поэтому инсет > 0
 * возникает именно там; на Android с interactive-widget=resizes-content
 * layout уже ужат и инсет остаётся 0 — хук безвреден.
 */
export function useVisualKeyboardInset(): number {
    const [inset, setInset] = useState(0);

    useEffect(() => {
        const vv = window.visualViewport;
        if (vv === undefined || vv === null) {
            return undefined;
        }
        let raf = 0;
        const update = (): void => {
            raf = 0;
            const layoutHeight = document.documentElement.clientHeight;
            const visualBottom = vv.offsetTop + vv.height;
            setInset(Math.max(0, Math.round(layoutHeight - visualBottom)));
        };
        // resize и scroll обязаны идти парой: при открытой клавиатуре
        // visual viewport ещё и панамируется (MDN VisualViewport;
        // Chrome «visual-viewport-api» — батчить через rAF).
        const schedule = (): void => {
            if (raf === 0) {
                raf = requestAnimationFrame(update);
            }
        };
        vv.addEventListener('resize', schedule, { passive: true });
        vv.addEventListener('scroll', schedule, { passive: true });
        update();
        return () => {
            if (raf !== 0) {
                cancelAnimationFrame(raf);
            }
            vv.removeEventListener('resize', schedule);
            vv.removeEventListener('scroll', schedule);
        };
    }, []);

    return inset;
}
```

Точка применения — `StickyBottomBar` (сдвиг трансформом, не `bottom`: без re-layout, композиторный слой):

```tsx
// apps/frontend/shared/ui/design/sticky-bottom-bar.tsx
export function StickyBottomBar({ children, dragHandle = false, className }: StickyBottomBarProps): JSX.Element {
    useTabBarSuppression();
    const keyboardInset = useVisualKeyboardInset();

    return (
        <div
            style={{ transform: `translateY(-${keyboardInset}px)` }}
            className={cn('fixed inset-x-0 bottom-0 z-40 rounded-t-sheet bg-surface font-sans',
                'desktop:mx-auto desktop:max-w-column', className)}
        >
            {/* … без изменений … */}
        </div>
    );
}
```

`useVisualKeyboardInset` включать только на <1024 (ПК-хром не перекрывается клавиатурой): на desktop-ярусе трансформ всегда 0, но лучше не создавать слушатели — завязать на существующий матчинг яруса проекта.

Подводные камни паттерна (все — из официальных описаний API):

1. **Слушать и `resize`, и `scroll`.** При открытой клавиатуре visual viewport панамируется — клавиатурный `scroll` приходит без `resize`; при обычном скролле страницы `visualViewport.scroll` не срабатывает, поэтому отдельный window-scroll-листенер для этой задачи не нужен (MDN; [Chrome «visual-viewport-api»](https://developer.chrome.com/blog/visual-viewport-api)).
2. **Батчить через rAF.** События приходят пачками и во время анимации клавиатуры — без rAF возможны дрожание и лишние рендеры (Chrome, та же статья).
3. **Высоту layout брать из `document.documentElement.clientHeight`, не из `window.innerHeight`.** MDN описывает `innerHeight` как высоту layout viewport, но на мобильных WebKit исторически наблюдались расхождения с visual viewport при зуме/клавиатуре; `clientHeight` — стабильно layout viewport (usage note в MDN innerHeight).
4. **Пинч-зум и ориентация.** `vv.height`/`offsetTop` — CSS-пиксели с учётом `scale`, формула самокорректируется при зуме; на поворот приходит `resize` — пересчёт и там.
5. **Не строить на `dvh`.** Динамические viewport-единицы следят за UA-интерфейсом (адресная строка), а не клавиатурой; на iOS клавиатура `dvh` не меняет вовсе. Отличие от Android `resizes-content` (там ICB и `vh` меняются) — ещё один довод держать логику в хуке, а не в CSS-единицах.
6. **safe-area остаётся.** Трансформ сдвигает панель целиком; `env(safe-area-inset-bottom)` в паддинге сохраняется — при открытой клавиатуре home indicator под ней, это ожидаемо.

### Итоговая рекомендация (кроссплатформенно)

Двухслойное решение:

1. **`interactiveWidget: 'resizes-content'`** в `app/layout.tsx` — закрывает Android Chrome 108+ / Firefox Android 133+ декларативно, без JS (и возвращает довоённое Chrome-поведение, которое совпадает с ожиданием «кнопка над клавиатурой»).
2. **VisualViewport-хук + `translateY` в `StickyBottomBar`** — закрывает iOS Safari (мету игнорирует); на Android с включённой метой инсет = 0, хук бездействует.

VirtualKeyboard API не используем: Chromium-only, WebKit не поддерживает и не занял позицию. Плюсы/минусы по браузерам:

| Браузер | Слой 1: `resizes-content` | Слой 2: visualViewport-хук | Результат |
|---|---|---|---|
| Safari iOS / iOS WebKit-WebView | игнорирует (BCD: `safari_ios: false`) | работает (Safari 13+) | кнопка над клавиатурой |
| Chrome Android 108+ | работает | инсет 0, бездействует | кнопка над клавиатурой |
| Firefox Android 133+ | работает | инсет 0, бездействует | кнопка над клавиатурой |
| Chrome desktop / ПК вообще | не требуется | слушатели не вешать на ПК | без изменений (ПК не трогаем) |

---

## C. Зона тапа большого дисплея суммы

Текущий `AmountField` даёт зону тапа шириной с набранный текст (span-измеритель) — у пустого поля это один глиф «0». Best practice:

1. **Весь блок дисплея кликабелен.** Обернуть дисплей в `<label>` (нативная активация лейбла исполняет фокусировку контрола — поведение платформенное, [WHATWG HTML §the-label-element](https://html.spec.whatwg.org/multipage/forms.html#the-label-element): «should match the platform's label behavior»), либо повесить на блок обработчик клика с синхронным `inputRef.current.focus()`. **Да, фокус из жеста пользователя поднимает клавиатуру гарантированно на iOS** — это ровно тот случай, который Apple оставила рабочим ([WebKit bug 195884](https://bugs.webkit.org/show_bug.cgi?id=195884): клавиатура показывается, «when it occurs in response to a user gesture»). JS-вариант с `focus()` в обработчике — самый документированно-надёжный (не зависит от платформенных нюансов активации лейбла); `<label>` — семантичнее и без JS, бонусом бесплатная доступность.
2. **Размер цели.** WCAG 2.2, 2.5.8 Target Size (Minimum) (AA): цель ≥ **24×24 CSS-пикселей** ([Understanding SC 2.5.8](https://www.w3.org/WAI/WCAG22/Understanding/target-size-minimum.html)); Apple HIG: минимальная тапаемая область **44×44 pt** ([Human Interface Guidelines, Buttons](https://developer.apple.com/design/human-interface-guidelines/buttons)). Дисплей суммы (шрифт 44/48, крупный центрированный блок) после расширения на весь блок перекрывает оба норматива с запасом — фактически это крупнейшая тап-цель экрана; заодно решается проблема тапа по пустому полю «0 ₽».
3. При JS-варианте сохранить доступность: input остаётся единственным фокусируемым элементом (`aria-label="Сумма"` уже есть), блок-обёртке не давать `tabIndex` — фокус по клику уходит в настоящий input, скринридер и клавиатура работают как раньше.

---

## Таблица поддержки по браузерам

| Механизм | Chrome | Chrome Android | Safari / iOS Safari | Firefox | Firefox Android | Источник |
|---|---|---|---|---|---|---|
| `interactive-widget=resizes-content` | нет (не нужен) | **108+** | нет (игнорирует мету) | нет (не нужен) | **133+** | MDN + BCD; CSS Viewport draft; Chrome blog |
| VirtualKeyboard API (`overlaysContent`, `keyboard-inset-*`) | **94+** | 94+ | **нет** (WebKit bug 230225) | нет (bug 1730568) | нет | MDN + BCD; WebKit standards-positions #16 |
| VisualViewport API | **61+** | 61+ | **13+ / iOS 13+** | 91+ | 68+ | MDN + BCD |
| Клавиатура по программному `focus()` вне жеста | нет | нет | **нет (политика Apple)** | нет | нет | WebKit bug 195884; Chrome virtualkeyboard docs |
| Клавиатура по `focus()` внутри жеста пользователя | да | да | **да** | да | да | WebKit bug 195884 |

## Рекомендации по тикетам

- **#1151 (экран суммы)** — стартовать можно: автоподъём клавиатуры при «Продолжить» реализуем (фокус в задаче жеста, паттерн из раздела A); ветки без жеста (назад, черновик) проектировать без автоподъёма на iOS.
- **Кнопка над клавиатурой** — внедрить двухслойное решение из B: `interactiveWidget: 'resizes-content'` в `app/layout.tsx` + `useVisualKeyboardInset` + `translateY` в `StickyBottomBar` (слушатели только на <1024).
- **Зона тапа** — в `AmountField` сделать весь дисплейный блок целью тапа (label или click+focus), см. C.

## Источники (официальные)

- WHATWG HTML: [The autofocus attribute](https://html.spec.whatwg.org/multipage/interaction.html#the-autofocus-attribute), [The label element](https://html.spec.whatwg.org/multipage/forms.html#the-label-element)
- WebKit: [Bug 195884 — Autofocus on text input does not show keyboard](https://bugs.webkit.org/show_bug.cgi?id=195884) (политика Apple о клавиатуре и жесте), [Bug 230225 — VirtualKeyboard API](https://bugs.webkit.org/show_bug.cgi?id=230225), [standards-positions #16 — VirtualKeyboard API](https://github.com/WebKit/standards-positions/issues/16), [WebKit Features in Safari 26.0](https://webkit.org/blog/17333/webkit-features-in-safari-26-0/), [News from WWDC25: Safari 26 beta](https://webkit.org/blog/16993/news-from-wwdc25-web-technology-coming-this-fall-in-safari-26-beta/)
- Chrome Developers: [Prepare for viewport resize behavior changes coming to Chrome on Android](https://developer.chrome.com/blog/viewport-resize-behavior) (`interactive-widget`, классификация платформ), [The VisualViewport API](https://developer.chrome.com/blog/visual-viewport-api) (паттерн событий, rAF-батчинг)
- MDN: [VirtualKeyboard API](https://developer.mozilla.org/en-US/docs/Web/API/VirtualKeyboard_API), [VirtualKeyboard.show()](https://developer.mozilla.org/en-US/docs/Web/API/VirtualKeyboard/show), [VisualViewport API](https://developer.mozilla.org/en-US/docs/Web/API/VisualViewport_API), [Window.visualViewport](https://developer.mozilla.org/en-US/docs/Web/API/Window/visualViewport), [meta viewport → interactive-widget](https://developer.mozilla.org/en-US/docs/Web/HTML/Reference/Elements/meta/name/viewport), [autofocus](https://developer.mozilla.org/en-US/docs/Web/HTML/Reference/Global_attributes/autofocus), [Window.innerHeight](https://developer.mozilla.org/en-US/docs/Web/API/Window/innerHeight); BCD: `html/elements/meta/name/viewport/interactive-widget.json`, `api/VirtualKeyboard.json`, `api/VisualViewport.json`
- CSS Viewport (W3C draft): [§ interactive-widget](https://drafts.csswg.org/css-viewport/#interactive-widget-section)
- React: [flushSync](https://react.dev/reference/react-dom/flushSync)
- Next.js: [generateViewport / viewport export](https://nextjs.org/docs/app/api-reference/functions/generate-viewport) (поле `interactiveWidget`; с v14.0.0)
- Нормативы доступности/платформы: [WCAG 2.2 SC 2.5.8 Target Size (Minimum)](https://www.w3.org/WAI/WCAG22/Understanding/target-size-minimum.html), [Apple HIG — Buttons](https://developer.apple.com/design/human-interface-guidelines/buttons)
