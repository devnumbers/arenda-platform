# Research: нативный pull-to-refresh в PWA — можно ли получить системный жест на каждой платформе

Research-отчёт по запросу владельца (12.10.2026): в standalone-PWA кабинета есть кастомный
pull-to-refresh, но он «выглядит не как нативный для каждой системы»; эталон — PWA T-Banka,
где перезагрузка нативная. Продуктовый код не менялся. Метод: канонические статьи
(developer.chrome.com, firt.dev), багтрекеры и свежие SO-треды 2024–2026; у каждого факта —
источник из списка в конце. Плюс снят манифест T-Banka с его сервера (см. §3).

---

## 0. TL;DR

1. **На Android — да, настоящий нативный PTR в установленной PWA есть, и он включён по умолчанию.**
   Chrome (WebAPK, `display: standalone`) показывает системный reload-жест — тот самый круговой
   спиннер браузера. Проблема разработчиков обычно обратная: как его **отключить**
   (`overscroll-behavior-y: contain/none` на теле документа).
2. **На iOS — нет, и не включается.** WebKit физически отключает нативный PTR для
   `display: standalone/fullscreen` home-screen приложений; CSS/мета-тегами не возвращается.
   Единственный путь на iOS — кастомный жест (что у нас уже реализовано по канону UIRefreshControl).
3. **Наш кастомный компонент на Android сам глушит нативный жест**: `preventDefault()` на
   `touchmove` (non-passive) + класс `overscroll-behavior-y: contain` на `<html>` во время тяги.
   Поэтому в standalone на Android пользователь видит нашу «подделку», а не системный спиннер.
   Утверждение ADR 0031 «в standalone нативный Android-PTR и так не показывается» —
   не подтверждается источниками.
4. **T-Bank делает ровно то, что просят**: их PWA — честный `display: "standalone"` (манифест
   снят 12.10.2026), нативный жест не подавляется → пользователь получает системную перезагрузку.
5. **Цена нативности на Android**: системный PTR делает **полную перезагрузку документа**,
   а не мягкий `invalidateQueries()`, как сейчас. Перехватить его нельзя (нет API). Это смена
   контракта ADR 0031 — решение за владельцем (варианты в §4).

---

## 1. Платформенная матрица нативного PTR

| Контекст | Нативный PTR? | Чем управляется |
|---|---|---|
| Android, Chrome-вкладка | да | `overscroll-behavior-y: contain/none` отключает |
| Android, установленная PWA (WebAPK, standalone) | **да, по умолчанию** | отключается только `overscroll-behavior-y` на теле; флага/настройки у пользователя нет |
| iOS, Safari-вкладка | да (браузерный жест) | — |
| iOS, home-screen standalone PWA | **нет, вкл. невозможно** | — |
| Десктоп-PWA | жеста нет (мышь) | — |

- Свойство `overscroll-behavior-y: contain` на `body` отключает PTR, сохраняя glow-эффекты;
  `none` глушит и их. Работает и в standalone — это единственный надёжный способ, т.к. в
  установленной PWA у пользователя нет ни тулбара, ни флагов (developer.chrome.com, «Take control
  of your scroll», ноя 2017 — каноническая статья; firt.dev «PWAs Power Tips», раздел «Control
  Reload Gesture»: «*Using 'contain', the reload gesture will be disabled…*» — совет даётся именно
  владельцам PWA, т.е. жест в PWA присутствует).
- Множественные треды «как отключить pull-down-refresh в моей установленной PWA» (Brave-комьюнити
  2023, OutSystems 2021, Vaadin-форум 2026) — обратная сторона того же факта: в standalone
  жест есть и мешает разработчикам.
- iOS standalone: свежий SO-тред (09.2026) «Why does pull-to-refresh work in iOS Safari but not
  in a standalone PWA» — ответ: WebKit отключает жест в standalone, `overscroll-behavior-y`
  его не вернёт; плюс SO #75972895 (уже цитируется в ADR 0031) и кейс Discourse (12.2024),
  который сделал кастомный PTR именно потому, что в iOS-PWA жеста нет. Кастомный PTR на iOS —
  общепринятая практика.
- API «перехвата» системного PTR не существует: жест отрисовывает и исполняет браузер,
  результат — обычная перезагрузка документа.

## 2. Почему у нас сейчас «не нативно» (диагноз по коду)

- `PullToRefresh` (apps/frontend/shared/ui/pull-to-refresh) активен **во всех standalone**
  (`useStandalone` = matchMedia `display-mode: standalone`) — на Android тоже.
- Во время тяги компонент вызывает `event.preventDefault()` на `touchmove` (passive: false) и
  вешает на `<html>` класс `ptr-suppress-overscroll` (`overscroll-behavior-y: contain`).
  Оба механизма — штатные способы **отключения** системного PTR на Android. То есть на Android
  мы сознательно подменяем системный жест своим iOS-стилизованным.
- Противоречия в доках: ADR 0031 одновременно пишет «на Android нативный PTR работает» и
  «в standalone он и так не показывается» (второе неверно); DESIGN.md §14 («Мобильный UX»)
  заявляет «нативный pull-to-refresh на прокручиваемых страницах работает — overscroll-запретов
  на теле нет» — верно для браузерного режима, но в standalone жест подавляется компонентом.
- Вне standalone компонент возвращает `null` — в браузерных вкладках пользователь уже получает
  нативное поведение платформы (на iOS Safari это браузерный PTR, на Android — тоже).

## 3. Что делает T-Bank

Манифест `https://www.tbank.ru/manifest.json` (снят 12.10.2026): `"name": "Tinkoff"`,
`"display": "standalone"`, `start_url: "/?utm_source=pwaStandaloneApp"` — настоящая
устанавливаемая PWA (в Chrome ставится как WebAPK). Никакого веб-API «своего системного»
PTR не существует, а наблюдаемый владельцем UX совпадает с системным спиннером Chrome —
значит, T-Bank просто **не подавляет нативный жест** (в отличие от нас). Допущение: их CSS
напрямую не проверялся (сайт отдаёт сертификат Минцифры, недоступный из внешних сетей),
но иной механизм у веб-платформы отсутствует.

## 4. Варианты

**A. Нативный везде, где он есть (рекомендуется).**
Android standalone: не подавлять системный жест (гейтить кастомный компонент только на
iOS/WebKit) → системный спиннер и перезагрузка, как у T-Banka. iOS standalone: оставить
кастомный UIRefreshControl-жест (уже реализован). Вкладки/десктоп: как сейчас.
- Цена: PTR на Android становится **hard reload** (полная перезагрузка документа) вместо
  мягкого refresh данных; меняется контракт ADR 0031 («единый UX на всех платформах» и
  «PTR = только данные») — ADR ротировать. Reload быстрый: shell отдаётся из SW-кеша
  (ADR 0032), новый код подтягивается тем же SW-lifecycle.
- Реализация маленькая: платформенный гейт в активации компонента (+ снятие suppress-механики
  для Android), ротация ADR 0031 / DESIGN.md / docstring, тесты.

**B. Единый кастомный, доведённый до неотличимости.** На Android стилизовать под Material
`SwipeRefreshLayout`, на iOS — уже так. Много настройки «на глаз», остаётся подделкой,
запрос «нативный» не решает; ADR 0031 уже отверг SwipeRefreshLayout-стилистику.

**C. Оставить как есть.** Текущее состояние — то, чем владелец недоволен.

## 5. Если берём вариант A — план изменений

1. `PullToRefresh`: активировать слушатели/рендер только при standalone **на iOS/WebKit**
   (платформенный детект рядом с `useStandalone`; feature-detect «есть ли системный PTR»
   не существует — ADR 0031 это уже отмечал).
2. На Android ничего не подавлять: `preventDefault`/`ptr-suppress-overscroll` остаются
   только в iOS-ветке; глобальных `overscroll-behavior` на теле по-прежнему нет.
3. Док-ротации тем же изменением: ADR 0031 (гибрид вместо единого UX; на Android PTR = hard
   reload, данные+код — как повезло, SW-lifecycle делает reload дешёвым), DESIGN.md §14,
   docstring компонента.
4. Тесты: юнит на гейт активации; e2e PTR-сценарий переучить под платформенный гейт.

## Источники

- [Take control of your scroll — customizing pull-to-refresh effects and overflow, developer.chrome.com, 11.2017](https://developer.chrome.com/blog/overscroll-behavior) — канон: `overscroll-behavior-y: contain/none` отключает PTR; паттерн кастомного PTR.
- [PWAs Power Tips, firt.dev (Maximiliano Firtman), 2024–2026](https://firt.dev/pwa-design-tips) — «Control Reload Gesture»: в PWA reload-жест управляется `overscroll-behavior-y`.
- [Why does pull-to-refresh work in iOS Safari but not in a standalone PWA, Stack Overflow (агент-зеркало), 09.2026](https://agents.stackoverflow.com/questions/75972895) — WebKit отключает PTR в standalone; CSS не возвращает.
- [Pull to Refresh for iOS App — Theme component, meta.discourse.org, 12.2024](https://meta.discourse.org/t/pull-to-refresh-for-ios-app-theme-component/325711) — iOS-PWA без PTR → кастомный компонент как практика.
- [PWA iOS Limitations and Safari Support, magicbell.com, 2026](https://www.magicbell.com/blog/pwa-ios-limitations) — обзор ограничений iOS standalone.
- [Disable pull-down-refresh, community.brave.app, 2023](https://community.brave.app/t/disable-pull-down-refresh/451587) и [OutSystems, 2021](https://www.outsystems.com/forums/discussion/69014/disable-screen-refresh-when-pull-down/) — треды «отключите PTR в standalone-приложении» (жест присутствует по умолчанию).
- Манифест T-Banka: `curl -k https://www.tbank.ru/manifest.json`, снят 12.10.2026 с машины владельца.
- Локальный код: `apps/frontend/shared/ui/pull-to-refresh/PullToRefresh.tsx`, `docs/adr/0031-pwa-pull-to-refresh.md`, `apps/frontend/DESIGN.md` §14.
