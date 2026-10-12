# Research: «как нативный» pull-to-refresh на iOS — механика UIRefreshControl и что из неё воспроизводимо в вебе

Research-отчёт к grilling-сессии 12.10.2026 (владелец: текущий кастомный PTR «топорный», эталон —
UIRefreshControl). Продуктовый код не менялся. Субъект — жест в standalone-PWA на iOS после
платформенного гейта (ротация ADR 0031 тем же днём: на Android системный жест, кастом — только iOS).
Источники: реверс-инжиниринг, исходники WebKit, доки Apple/Ionic/AOSP, инженерные посты; у факта —
ссылка с датой. Отчёт ресёрч-субагента (37 источников), структурировано без потери фактов.

---

## 0. TL;DR

1. **Формула нативной резинки известна**: `b = (1 − 1/((x · 0.55 / d) + 1)) · d`, где x —
   перетаск за границу, d — размерность вьюпорта. Асимптотическая, жёсткого клампа нет.
   Наша текущая линейная 0.5 с клампом ×1.8 — главный источник «топорности».
2. **Нативный спиннер при тяге не вращается**: только проявление (opacity/масштаб);
   непрерывное вращение — лишь в фазе refreshing. Наша «намотка 2°/px» — не нативный паттерн.
3. **Отпускание — пружина с вложенной скоростью** (WWDC23: масса/жёсткость/демпфирование +
   initialVelocity); быстрый отпуск улетает дальше. CSS-безье скорость не умеет в принципе.
4. **Объём схлопнулся**: в standalone-PWA на iOS документ сохраняет нативный батут по умолчанию,
   внутренние скролл-контейнеры имеют нативный momentum+bounce с iOS 13 сами. Строить «общий
   rubber-band» не нужно — нужно только не душить существующий и сделать PTR-жест как нативный.
5. **Недостижимо на iOS-вебе**: гаптика (API нет), 120Гц в standalone (WebKit капит на ~60fps),
   нулевая задержка не-пассивного touchmove.

## 1. Нативная механика UIKit

- **Rubber-band UIScrollView** — реверс-инжиниринг, де-факто стандарт:
  [gist beaucollins «Analysis of Apple's rubber band scrolling», 06.01.2013](https://gist.github.com/beaucollins/1034373),
  формула выше, c = 0.55. iOS WKWebView делегирует батут нативному UIScrollView — в исходниках
  WebKit есть `_scrollViewIsRubberBandingForRefreshControl`
  ([WKWebViewIOS.mm, main-ветка, проверено 12.10.2026](https://github.com/WebKit/WebKit/blob/main/Source/WebKit/UIProcess/API/ios/WKWebViewIOS.mm)).
  Точная формула UIKit закрыта; формула+0.55 совпадает по ощущению и воспроизводится повсеместно.
- **UIRefreshControl**: Apple документирует качественно
  ([developer.apple.com/documentation/uikit/uirefreshcontrol](https://developer.apple.com/documentation/uikit/uirefreshcontrol)):
  drag reveals the control → begins animating → value-changed. Числа (~60pt высота контрола,
  триггер ~100–130pt в зависимости от версии iOS) — фолклор, официальных цифр нет. Из кастомных
  реализаций ([AppCoda](https://www.appcoda.com/custom-pull-to-refresh)): вращение спиннера
  НЕ привязано к дистанции; флик триггерит не сам по себе, а через overshoot резинки за порог.
- **Snap-back**: официальная модель — пружина с начальной скоростью
  ([WWDC23 «Animate with springs»](https://developer.apple.com/videos/play/wwdc2023/10158/)).
  Константы UIScrollView недокументированы (фолклор: почти критическое демпфирование, лёгкий
  overshoot). Готча реверс-инжиниринга: подменённая spring-динамика обнуляет скорость — velocity
  подавать явно ([holko.pl, 07.07.2014](https://holko.pl/2014/07/07/scroll-view-inertia-bouncing-rubber-banding/)).

## 2. Возможности веба на iOS

- `touchmove` + `preventDefault()` требует `passive: false`, зарегистрированного при загрузке
  (с iOS 11.3 динамические слушатели пассивны); не-пассивный листенер добавляет JS-задержку
  на кадр ([Chrome blog «Make scrolling fast», 10.01.2017](https://developer.chrome.com/blog/scrolling-passive/)).
  Pointer Events / `touch-action` — с iOS 13 ([WebKit bug 133112](https://bugs.webkit.org/show_bug.cgi?id=133112)).
- Частота: на ProMotion-айфонах веб-контент рендерится ~60fps, в standalone-PWA 120fps отключён
  ([WebKit bug «Prefer Page Rendering Updates near 60fps»](https://bugs.webkit.org/show_bug.cgi?id=285503)).
- Гаптика: `navigator.vibrate` в iOS Safari не реализован (caniuse/swmansion); в чистом PWA
  обхода нет — только нативная обёртка (Capacitor Haptics).
- Пружина в вебе: rAF + semi-implicit Euler — стандарт
  ([Josh Comeau, 21.09.2020](https://www.joshwcomeau.com/animation/a-friendly-introduction-to-spring-physics/);
  дефолты stiffness 100 / damping 10 — [maximeheckel.com](https://blog.maximeheckel.com/posts/the-physics-behind-spring-animations/)).
  CSS `linear()`-easing — iOS 17.2+, но velocity-слеп: для velocity-aware нужен rAF.
- **Критично для объёма**: в standalone-PWA документ сохраняет rubber-band по умолчанию;
  отключают его `overscroll-behavior: none` (Safari 16+,
  [MDN](https://developer.mozilla.org/docs/Web/CSS/overscroll-behavior)) или
  `body{position:fixed;overflow:hidden}`. Внутренние `overflow:auto`-скроллеры имеют нативный
  momentum+bounce с iOS 13 без `-webkit-overflow-scrolling`
  ([SO 58684546](https://stackoverflow.com/questions/58684546/css-disable-bounce-effect-for-scroll-on-ios-13)).
  Bounce внутри убивают: `preventDefault()` в touchmove-цепочке, `touch-action:none`,
  `overscroll-behavior:none` на контейнере (iOS 16+).

## 3. Веб-реализации (цифры для калибровки)

- [apeatling/web-pull-to-refresh](https://github.com/apeatling/web-pull-to-refresh): threshold 70px, resistance 2.5, Hammer.js.
- [Discourse iOS-PWA тема](https://github.com/VaperinaDEV/pull-to-refresh-for-ios-app): THRESHOLD 100px, MAX_PULL 150px, линейный damping 0.5, спиннер — стагерированная opacity. Простое семейство, не эталон.
- [Ionic ion-refresher](https://ionicframework.com/docs/api/refresher): pullMin 60px, pullMax = pullMin+60, snapbackDuration 280ms — лучшие документированные цифры.
- [SwipeRefreshLayout AOSP](https://cs.android.com/androidx/platform/frameworks/support/+/androidx-main:swiperefreshlayout/): DRAG_RATE 0.5, триггер 64dp — это Android-константы, iOS их не использует (наша 0.5 — из этого семейства).
- Twitter Lite PTR: пост [blog.x.com, 06.04.2017](https://blog.x.com/engineering/en_us/topics/open-source/2017/how-we-built-twitter-lite) — полный reload страницы, критика ([simonhearne.com, 07.04.2017](https://simonhearne.com/2017/twitter-lite-isnt-as-good-as-the-app/)); не образец.

## 4. Решения grilling-сессии (12.10.2026, владелец)

- Эталон: UIRefreshControl; полируется все четыре фазы (тяга, спиннер, отпускание, пин).
- Физика: библиотека **motion (framer-motion)** — запрет зависимостей ADR 0031 снят ротацией.
- Трекинг: свой transform по нативной формуле (не ехать на нативном батуте документа).
- Триггер: дистанция + velocity-флик (overshoot резинки за порог).
- Спиннер: системные спицы UIActivityIndicatorView, появление opacity/масштабом, вращение
  только в refreshing.
- Объём: только PTR-жест; низ документа и внутренние контейнеры проверяются на устройстве
  (по §2 они уже нативные); если не пружинат — отдельное расследование.

## 5. Целевые параметры (замена 1:1 к текущим)

| Сейчас | Цель | Основание |
|---|---|---|
| Сопротивление константа 0.5 | Резинка `(1−1/(x·0.55/H+1))·H`, H = высота вьюпорта | §1 |
| Кламп THRESHOLD×1.8 | Без клампа (асимптота); опц. soft-cap threshold+60 | §1, §3 (Ionic pullMax) |
| Порог 70px | Оставить 70px | §3 (apeatling 70, Ionic 60, фолклор ~60pt) |
| Триггер только дистанция | + velocity: пик пружины отпускания ≥ порога → триггер | §1 |
| CSS-безье 0.3s, velocity-слепой | rAF-пружина (через motion): stiffness ≈ 170–220 /s², damping ≈ 24–28 /s (ratio ~0.85–0.95, лёгкий bounce), initialVelocity = скорость отпускания | §1, §2 |
| Спиннер 2°/px | Расцепить: тяга = translate + opacity-рамп 0→1 на первых ~50% порога; вращение только в refreshing (0.8s linear — ок) | §1 |
| Пин 60px | Оставить 60px | высота UIRefreshControl (фолклор) |
