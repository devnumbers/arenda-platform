# Research: канон иконок и фавиконок для PWA (2026)

> **Статус (21.09.2026):** рекомендации раздела 6 реализованы в #781. Мастера — векторные экспорты из Figma: `assets/icons/source-icon.svg` (скруглённый, узел 2358:139864) и `assets/icons/source-square.svg` (квадратный, узел 2353:52637); заливка углов `FILL_COLOR` из генератора снята — any/favicon (`public/icons/icon-*.png`, `app/icon.png`, новый `app/favicon.ico` — мульти-ICO 16/32/48) идут с прозрачными скруглёнными углами, maskable/apple-touch — непрозрачные квадраты из квадратного мастера. Позднее 21.09, решение владельца: собственный генератор снесён, набор собирается через RealFaviconGenerator (процесс и маппинг — `apps/frontend/assets/icons/README.md`); дом в иконках установки вписан в 74% холста (macOS 26 маскирует иконки Dock — полный арт режется), в табе остался полный арт. Текст ниже сохранён как research-снимок на момент исследования; формулировки «сейчас»/«текущий» описывают состояние до #781.

- **Дата:** 2026-09-21
- **Тикет-контекст:** карта «PWA — скруглённые иконки, сплэш, офлайн-экран, кнопка установки», тикет «Research: канон иконок и фавиконок по платформам (2026)»
- **Проектный контекст:** есть мастер-иконка — скруглённый квадрат (~22% радиуса, iOS-squircle-подобный) с прозрачными углами; из неё `apps/frontend/scripts/generate-icons.mjs` генерирует: manifest any 192/512, maskable 512 (углы залиты цветом), monochrome 512 (SVG-силуэт), `app/icon.png` (Next.js file-convention favicon), `app/apple-icon.png` (180). Сейчас генератор заливает углы **у всех** выходов — из-за этого в десктопном Chrome таб/ярлык выглядят квадратом.
- **Методика:** первичные источники (web.dev, developer.chrome.com, developer.apple.com, MDN, Next.js docs, трекер Chromium) + свежие гайды 2024–2026 (Evil Martians «How to Favicon», caniuse). Для источников 2019–2021 поведение перепроверено свежими подтверждениями; такие места помечены.

---

## 1. Таб браузера (favicon)

**Вывод: favicon показывается «как есть», без масок, во всех основных браузерах. Прозрачные скруглённые углы — безопасны и являются единственным способом получить скругление в табе.**

- Ни один из основных браузеров (Chrome, Safari, Firefox, Edge) не применяет к favicon форму-маску: это просто растровое/векторное изображение, композитящееся на фон строки табов. MDN прямо отделяет favicon от иконок установленных приложений: «The PWA app icon is not the same as the favicon image, which is displayed in places like the browser's address bar» ([MDN: Define your app icons](https://developer.mozilla.org/en-US/docs/Web/Progressive_web_apps/How_to/Define_app_icons)).
- Прозрачные углы композитятся на **реальный фон таба**: на светлой теме углы сольются со светлой подложкой, на тёмной — с тёмной. Никаких чёрных/белых заливок браузер не подставляет. Практическое следствие: силуэт иконки должен иметь достаточный контраст **в обеих темах** (светлый тонкий силуэт пропадёт на светлой теме). Тёмная тема/favicon обычно решается SVG-favicon с `@media (prefers-color-scheme: dark)` внутри — поддержка ниже.
- **Нужен ли favicon.ico в 2026:** формально нет — достаточно PNG через `<link rel="icon">`. Но держать один `/favicon.ico` в корне сайта всё ещё рекомендуется как fallback: часть инструментов (RSS-ридеры и пр.) запрашивает `/favicon.ico` напрямую, игнорируя `<link>`; Google Search требует квадратную crawlable-иконку для результатов поиска. «Some tools, like RSS readers, just request /favicon.ico from the server and don't bother looking elsewhere» ([Evil Martians, How to Favicon](https://evilmartians.com/chronicles/how-to-favicon-in-2021-six-files-that-fit-most-needs) — гайд актуализирован под 2026, заголовок «Three files that fit most needs» / «How to Favicon in 2026»).
- Известный нюанс Chrome: при наличии и ICO, и SVG-иконки Chrome может предпочесть ICO — лечится явным `sizes="32x32"` у ICO-ссылки; Firefox всегда предпочитает SVG (тот же источник; помечено как давнее наблюдение — при изменении набора иконок стоит перепроверить).
- **SVG-favicon:** Chrome, Edge, Firefox, Opera поддерживают давно; **Safari научился только с версии Safari 26** (2025) — Safari 3.2–18.7 SVG-favicon не рендерят и показывают PNG/ICO-fallback ([TestMu: SVG Favicon Browser Support, 2026](https://www.testmuai.com/learning-hub/svg-favicon-browser-support), [caniuse: link-icon-svg](https://caniuse.com/link-icon-svg)). Долгоживущий WebKit-баг 136059 закрыт только недавно. То есть SVG-favicon в 2026 — прогрессивное улучшение, но PNG-fallback обязателен.
- **Safari `mask-icon` (закреплённые табы): механизм мёртв, не нужен.** С Safari 12 обычный favicon используется для закреплённых табов; «Even apple.com doesn't use the mask-icon anymore» ([Evil Martians](https://evilmartians.com/chronicles/how-to-favicon-in-2021-six-files-that-fit-most-needs), [favicontools.com](https://favicontools.com)). Добавлять `<link rel="mask-icon">` в 2026 не нужно (безвредно, но бесполезно). `rel="shortcut icon"` — невалидный link-relation, использовать просто `rel="icon"`.

## 2. apple-touch-icon (iOS / «На экран „Домой“» на iOS)

**Вывод: iOS сама накладывает скруглённую маску (squircle); файл обязан быть НЕПРОЗРАЧНЫМ полным квадратом 180×180 — иначе прозрачные углы iOS закрасит чёрным.**

- Система применяет маску автоматически; дизайнеру/генератору скруглять углы самостоятельно нельзя — «предварительно скруглённые» углы будут скруглены повторно и выглядят сломанными. HIG: предоставлять квадратный арт, маску накладывает система ([Apple HIG: App icons](https://developer.apple.com/design/human-interface-guidelines/app-icons); прямая формулировка «Do not round corners yourself as Apple applies a system squircle mask» — [SplitMetrics](https://splitmetrics.com/blog/guide-to-mobile-icons)).
- Прозрачность: iOS игнорирует альфа-канал и **композитит иконку на чёрный фон** до наложения маски → прозрачные углы становятся чёрными и видны в скруглении. «iOS ignores transparency in that image and composites it onto black, so give it an opaque background» ([Favicon Tools](https://favicontools.com), [iKit](https://ikit.app)). Это подтверждал и комментарий в `generate-icons.mjs` до #781 (обоснование FILL_COLOR для apple-icon было корректным по сути; с #781 непрозрачные выходы собираются из квадратного мастера, без заливки).
- Рекомендации по контенту: полный (full-bleed) квадрат 180×180, фон заливает весь холст, логотип — с небольшим внутренним отступом (~10–20px на 180), другие платформы даунскейлят этот файл (Evil Martians; 180 нужен для iPad начиная с iOS 8).
- Chrome на iOS и «добавить на экран Домой» также потребляет `apple-touch-icon` (единственный канал иконок на домашний экран iOS — на iOS нет установки PWA в лончер с манифест-иконками).

## 3. Android launcher (установленная PWA)

**Вывод: при наличии maskable Chrome использует именно его для иконки лончера; форму маски (круг/squircle/скруглённый квадрат) выбирает вендор/лаунчер. Без maskable any-иконку кладут на белый фон. Monochrome в 2026 Chrome для PWA по-прежнему не применяет.**

- **Выбор purpose:** если в манифесте есть иконка с `purpose: "maskable"`, Chrome для отображения иконки установленного PWA использует maskable, независимо от размеров остальных иконок (формулировка из баг-репорта Chromium: «if the manifest contains a maskable image, Chrome uses the maskable image for icon display regardless of sizes of other icons» — [issues.chromium.org](https://issues.chromium.org)). Лончер применяет adaptive-icon-маску; форма зависит от производителя устройства/лаунчера — «Android displays adaptive icons in a variety of shapes across device models» ([web.dev: Maskable icon](https://web.dev/articles/maskable-icon)).
- **Если maskable нет:** обычная (any) иконка масштабируется внутрь маски на **белую подложку** — «Icons that don't use this format have white backgrounds» ([web.dev: Maskable icon](https://web.dev/articles/maskable-icon)); не-masкируемые платформы, наоборот, потребляют any.
- **Safe zone:** значимое содержимое должно умещаться в круг с радиусом **40% ширины иконки** (у MDN то же самое сформулировано как «circle which diameter is 80% of the icon's minimum dimension»); внешний край ~10% может обрезаться. Проверка: DevTools → Application → Icons → «Show only the minimum safe area for maskable icons», либо maskable.app ([web.dev](https://web.dev/articles/maskable-icon), [MDN](https://developer.mozilla.org/en-US/docs/Web/Progressive_web_apps/How_to/Define_app_icons)).
- **Где потребляется any:** install-диалог/промпт Chrome (подходит крупная PNG-иконка, обычно 512), стартовый splash screen на Android (генерируется из манифеста, требует PNG ≥512×512 — [developer.chrome.com: Lighthouse splash screen audit](https://developer.chrome.com/docs/lighthouse/pwa/installable-manifest)), платформы без маскирования (favicon-подобное использование), скриншоты/списки установленных приложений. Комбинированный `purpose: "any maskable"` **не использовать** — даёт паддинг в favicon-контекстах и заставляет браузеры выбирать не тот слой ([web.dev](https://web.dev/articles/maskable-icon), [Progressier/dev.to](https://dev.to/progressier/why-a-pwa-app-icon-shouldnt-have-a-purpose-set-to-any-maskable-4c78)).
- **monochrome (Android 13+ Material You):** механизм декларативно поддержан спекой манифеста, и иконку-силуэт держать стоит, **но Chrome на Android до сих пор не применяет monochrome для иконок установленных PWA** — открытый Chromium-тикет [40277264 «Chrome Android PWA is not using monochrome icons»](https://issues.chromium.org/issues/40277264); комментарий инженеров: «Android currently doesn't support themed icons for dynamically created shortcuts, so we can't use the monochrome icons for PWAs». У нативных приложений themed icons работают (система тонирует альфа-маску цветом обоев — [Android docs: adaptive icons](https://developer.android.com/develop/ui/compose/system/icon_design_adaptive)). Для PWA реальный themed-значок сейчас достижим только через TWA/обёртку в Play. Ещё нюанс эксплуатации: иконка установленного PWA живёт в WebAPK и обновляется по расписанию Chrome — форсировать можно через `about://webapks` → Update ([web.dev: Web app manifest updates](https://web.dev/articles/webapp-manifest-updates)).

## 4. Десктопные установленные PWA (Chrome/Edge)

**Вывод: ОС на Windows и macOS не маскирует иконку PWA — она показывается «как есть». Прозрачные скруглённые углы безопасны (никаких чёрных/белых заливок) и дают желаемое скругление. Chrome берёт иконку ярлыка из манифеста (any), поэтому критично НЕ склеивать any и maskable.**

- **Windows (таскбар, «Пуск»):** Chromium не применяет форму-маску к иконкам PWA на десктопе — маскирование это Android-механизм (adaptive icons). Иконка приходит из ярлыка как есть; прозрачные углы показывают фон таскбара/меню. Windows-специфичные размеры: MDN — «Windows can display your app icon as a 44x44 pixels image in the taskbar, or as a 150x150 pixels image in the start menu» ([MDN: Define your app icons](https://developer.mozilla.org/en-US/docs/Web/Progressive_web_apps/How_to/Define_app_icons)) — то есть Chrome даунскейлит крупную манифест-иконку под нужный размер. Кэш иконок Windows может подолгу показывать старую версию (известная жалоба, [Super User: Chrome PWA icons turn blank after pinning](https://superuser.com/questions/1850900/chrome-pwa-icons-turn-blank-after-pinning-on-windows-10-taskbar)) — при проверках это помнить.
- **macOS (Dock):** Chrome **не маскирует и не добавляет паддинг** — иконка PWA в Dock показывается буквально как есть, из-за чего maskable-квадрат выглядит «too big» среди нативных squircle-иконок. Это зафиксировано в тикете Chromium [40827667 «MacPWAs: PWA Icons are too big on macOS if 'maskable'»](https://issues.chromium.org/issues/40827667) и реальных репортах ([Navidrome #3047](https://github.com/navidrome/navidrome/issues/3047)). Следствие: прозрачный скруглённый квадрат в `purpose: "any"` — именно то, что нужно для macOS; **комбинированный `"any maskable"` или отсутствие any приведут к выбору квадратного maskable-файла** на десктопе.
- **Какой файл берёт Chrome для ярлыка:** из массива `icons` манифеста — подходящая по размеру any-иконка (крупные даунскейлятся; web.dev рекомендует 192 и 512 как обязательный минимум для инсталла). Прямого потребления favicon для ярлыка в нормальном случае нет, но встречались старые сообщения о fallback на favicon при проблемах с манифест-иконками (недоказанное, помечено как неподтверждённое).
- Итого по цели владельца: чтобы иконка выглядела скруглённой в десктопном Chrome — надо отдавать any-иконки (и favicon) с прозрачными углами мастер-иконки, а заливку углов оставить только maskable и apple-touch-icon.

## 5. Windows 11 специфика

- **Системного скругления иконок таскбара для PWA нет.** Windows 11 не авто-скругляет иконки приложений/ярлыков — иконка PWA приходит из ярлыка Chrome как есть; квадратный PNG будет квадратом, прозрачные скруглённые углы дадут скругление ([Reddit r/Windows11: square icons discussion](https://www.reddit.com/r/Windows11/comments/piuavd/), общая практика ICO/ярлыков). Это аргумент «за» прозрачные углы в any-иконках.
- **`msapplication-TileImage` / `msapplication-TileColor` / `browserconfig.xml`: в 2026 не нужны.** Это наследие IE/Windows 8-плашек; Evil Martians прямо пишет, что Windows Tile-иконки «no longer required» ([Evil Martians](https://evilmartians.com/chronicles/how-to-favicon-in-2021-six-files-that-fit-most-needs)); экосистема метатегов помечена legacy ([zhead: msapplication-TileColor](https://zhead.dev/meta/msapplication-tilecolor), [Webmasters SE: browserconfig deprecated](https://webmasters.stackexchange.com/questions/131077/in-2020-are-browserconfig-xml-and-ieconfig-xml-now-effectively-deprecated)). Современные потребности закрывают `apple-touch-icon` + манифест + обычные favicon-линки. Добавлять не нужно; если где-то остались — безопасно удалить.

## 6. Итоговый чек-лист артефактов

Матрица «где какая прозрачность нужна» (сердце решения по тикету):

| Поверхность | Файл | Форма/углы |
|---|---|---|
| Таб браузера (все ОС) | favicon PNG/SVG | **прозрачные скруглённые углы** (композит на фон таба) |
| Десктоп-ярлык Windows / Dock macOS | manifest `any` | **прозрачные скруглённые углы** (ОС не маскирует) |
| Android лончер | manifest `maskable` | **непрозрачный полный квадрат**, контент в круге 80% |
| iOS домашний экран | `apple-touch-icon` | **непрозрачный полный квадрат** (iOS сам скругляет; альфа → чёрные углы) |
| Android splash / install-диалог | manifest `any` 512 | как any (прозрачные углы допустимы; фон сплэша = `background_color`) |
| Android 13+ themed (будущее) | manifest `monochrome` | силуэт на прозрачном, сплошной цвет (сейчас Chrome для PWA не применяет) |

### Минимально достаточный набор (уже почти совпадает с текущим состоянием репо)

| # | Артефакт | Размер | purpose / подключение | Углы | Примечание |
|---|---|---|---|---|---|
| 1 | `public/icons/icon-192.png` | 192×192 | manifest, `purpose: "any"` | прозрачные скруглённые | install-минимум Chrome |
| 2 | `public/icons/icon-512.png` | 512×512 | manifest, `purpose: "any"` | прозрачные скруглённые | install-диалог, splash (источник generate-splash) |
| 3 | `public/icons/icon-maskable-512.png` | 512×512 | manifest, `purpose: "maskable"` | **непрозрачный квадрат**, логотип в круге 80% | Android лончер; генератор заливает углы — верно |
| 4 | `app/icon.png` (Next.js file-convention → `<link rel="icon">`) | 512×512 | `<link rel="icon">` | прозрачные скруглённые | **фикс из этого research: не заливать углы** — сейчас из-за заливки таб в десктопном Chrome квадратный |
| 5 | `app/apple-icon.png` (Next.js file-convention → `<link rel="apple-touch-icon">`) | 180×180 | `<link rel="apple-touch-icon">` | **непрозрачный полный квадрат** | текущее поведение генератора — верно, не менять |

Manifest уже собран правильно: раздельные `any` / `maskable` / `monochrome` записи, без комбинированного `"any maskable"` (`apps/frontend/app/manifest.ts`) — соответствие web.dev/Chromium-рекомендациям.

### Рекомендуемые дополнения

| # | Артефакт | Размер | Подключение | Углы | Зачем |
|---|---|---|---|---|---|
| 6 | `/favicon.ico` (`app/favicon.ico`) | 32×32 (или multi 16/32/48) | Next.js отдаёт в корне автоматически | прозрачные скруглённые | RSS-ридеры/краулеры запрашивают `/favicon.ico` напрямую; Google Search-иконка |
| 7 | `app/icon.svg` (или `public`+`<link>`) | векторный | `<link rel="icon" type="image/svg+xml">` | прозрачные | тёмная тема табов через `prefers-color-scheme` внутри SVG; рендерят Chrome/Edge/Firefox/Opera, Safari — с 26 |
| 8 | `public/icons/icon-monochrome-512.png` | 512×512 | manifest, `purpose: "monochrome"` | сплошной силуэт | уже есть в репо; держать как future-proof — Chrome Android пока игнорирует (Chromium 40277264) |

### Про дополнительные favicon-размеры (16/32/48)

- Обязательных размеров нет: современные браузеры сами масштабируют PNG-иконку (Evil Martians: «trust browsers to downscale»); Next.js file-convention из одного `app/icon.png` генерирует `<link>` с корректным `sizes`.
- Достаточно: один PNG 512×512 как `<link rel="icon">` + опционально один `/favicon.ico` 32×32 (позиция Evil Martians) **или** multi-size ICO 16+32+48, если хочется идеальной резкости в мелких UI Windows/старых поверхностях. Multi-size PNG-наборы (отдельные 16/32/48 файлов) в 2026 избыточны.
- ICO уместно собрать из того же мастера (прозрачные скруглённые углы) — на Windows-поверхностях прозрачность работает, заливка не нужна.

### Анти-паттерны (зафиксировать в тикете)

1. Не использовать `purpose: "any maskable"` на одном файле — десктопный Chrome/macOS возьмёт квадратный maskable-арг ([Chromium 40827667](https://issues.chromium.org/issues/40827667)).
2. Не скруглять углы apple-touch-icon вручную и не оставлять в нём альфу.
3. Не заливать углы у favicon/any — это ломает вид в табах и на десктопе (текущий баг-симптом владельца).
4. Не добавлять `rel="mask-icon"`, `msapplication-*`, `browserconfig.xml` — мёртвые механизмы.
5. Не рассчитывать, что monochrome сейчас что-то меняет на Android — держать «про запас».

## Источники

Первичные:
- web.dev — Adaptive icon support in PWAs with maskable icons: https://web.dev/articles/maskable-icon (2019; поведение переподтверждено MDN 2025 и Chromium-тикетами)
- web.dev — Web app manifest: https://web.dev/articles/add-manifest ; обновления WebAPK: https://web.dev/articles/webapp-manifest-updates
- MDN — Define your app icons (обновлено 06.2025): https://developer.mozilla.org/en-US/docs/Web/Progressive_web_apps/How_to/Define_app_icons
- MDN — manifest icons: https://developer.mozilla.org/en-US/docs/Web/Progressive_web_apps/Manifest/Reference/icons
- developer.chrome.com — Lighthouse: maskable icon: https://developer.chrome.com/docs/lighthouse/pwa/maskable-icon
- Apple HIG — App icons: https://developer.apple.com/design/human-interface-guidelines/app-icons
- Next.js — Metadata files: favicon, icon, apple-icon: https://nextjs.org/docs/app/api-reference/file-conventions/metadata/app-icons
- Chromium 40277264 — PWA не использует monochrome (открыт): https://issues.chromium.org/issues/40277264
- Chromium 40827667 — macOS PWA icons too big if maskable: https://issues.chromium.org/issues/40827667

Вторичные (свежие):
- Evil Martians — How to Favicon in 2026: https://evilmartians.com/chronicles/how-to-favicon-in-2021-six-files-that-fit-most-needs
- caniuse — SVG favicons: https://caniuse.com/link-icon-svg
- TestMu — SVG Favicon Browser Support (05.2026): https://www.testmuai.com/learning-hub/svg-favicon-browser-support
- Favicon Tools — apple-touch-icon поведение: https://favicontools.com
- Progressier/dev.to — Why a PWA app icon shouldn't have purpose "any maskable": https://dev.to/progressier/why-a-pwa-app-icon-shouldnt-have-a-purpose-set-to-any-maskable-4c78
- Android docs — Adaptive/themed icons: https://developer.android.com/develop/ui/compose/system/icon_design_adaptive
- zhead — msapplication-TileColor legacy: https://zhead.dev/meta/msapplication-tilecolor
