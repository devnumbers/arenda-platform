# Перенос макетов Figma в код: лучшие практики и варианты пайплайна (внешнее исследование)

- Дата: 2026-08-24. Исследование по запросу «пайплайн переноса макетов экранов из Figma в код, 1:1 пиксель-перфект, с Figma MCP для ИИ-агента».
- Контекст потребителя: `apps/frontend` — Next.js 16.3.1 (App Router), React 19.2.4 + React Compiler, HeroUI v3.2.1 (beta, поверх Tailwind v4 `@theme`), Tailwind v4.3.1 (CSS-first, без `tailwind.config.js`), FSD с ESLint-boundaries, vitest. `@playwright/test` 1.61 уже в devDependencies (`apps/frontend/playwright.config.ts`: chromium, `webServer: npm run dev`, `testDir: ./e2e` — пока пуста). В харнессе настроен Framelink Figma Context MCP (`get_figma_data`, `download_figma_images`) и `heroui-react` MCP (доки компонентов HeroUI v3).
- Метод: четыре параллельных web-research потока (официальные инструменты Figma для ИИ; сторонние Figma-to-code генераторы и консенсус практиков; визуальная верификация и сравнение с макетом; гигиена Figma-файлов и пайплайн дизайн-токенов). Первоисточники с датами, ссылка у каждого ключевого утверждения.

## Оглавление

1. [Резюме](#резюме)
2. [Чтение дизайна агентом: официальный Dev Mode MCP vs Framelink Context MCP](#1-чтение-дизайна-агентом)
3. [Сторонние Figma-to-code генераторы — вердикты](#2-сторонние-figma-to-code-генераторы)
4. [Визуальная верификация: диф против макета и регрессия](#3-визуальная-верификация)
5. [Гигиена Figma-файла под агента](#4-гигиена-figma-файла)
6. [Дизайн-токены → Tailwind v4 / HeroUI v3](#5-дизайн-токены)
7. [Варианты пайплайна и рекомендация](#6-варианты-пайплайна-и-рекомендация)
8. [Критерий приёмки «пиксель-перфекта»](#7-критерий-приёмки)
9. [Источники](#источники)

---

## Резюме

1. **Пиксель-перфект «за один выстрел» — миф**; это консенсус практиков 2025–2026. Независимые замеры точности генераторов — 65–80%; сами вендоры (Vercel для v0, Figma для Make) рекомендуют итеративный подход по частям. Рабочая схема — **замкнутый цикл: агент читает дизайн → реализует → измеряет расхождение → правит** ([vadim.blog, 2026-03](https://vadim.blog/pixel-perfect-playwright-figma-mcp/); [sixtythirtyten, 2026-02](https://www.sixtythirtyten.co/blog/from-figma-to-code-ai-design-to-dev-workflows-in-2026); [dev.to, 2025-12](https://dev.to/emma_schmidt_/i-tested-5-design-to-code-ai-tools-for-30-days-heres-what-actually-works-2p2d)).
2. **Главный рычаг качества — не генератор, а маппинг на существующую дизайн-систему.** Figma прямо: Code Connect — «способ №1 получать переиспользование компонентов; без него модель гадает» ([developers.figma.com](https://developers.figma.com/docs/figma-mcp-server/structure-figma-file/)). Для нашего репо это означает: экран собирается из HeroUI v3 (проверка через `heroui-react` MCP) и токенов, а не из сгенерированного сырого HTML.
3. **Половина результата — гигиена Figma-файла**: auto-layout везде (мапится во flexbox), Figma Variables вместо «пипеткой», компоненты с вариантами, смысловые имена слоёв, отдельные фреймы на брейкпоинт, текст как текст ([Figma: Structure your file for better code](https://developers.figma.com/docs/figma-mcp-server/structure-figma-file/)).
4. **Инструмент чтения**: у нас уже есть Framelink Context MCP (контекст + экспорт картинок). Официальный remote Dev Mode MCP (`https://mcp.figma.com/mcp`, OAuth) добавляет `get_design_context` (React+Tailwind-представление фрейма), `get_variable_defs`, скриншоты и Code Connect-маппинг — но лимиты Starter-тарифа ~6 вызовов в месяц, по-человечески работает с Dev/Full сидом на платном тарифе ([mcp-server-guide](https://github.com/figma/mcp-server-guide)). Практика: **включён должен быть только один из двух** — одновременный официальный + Framelink путает агента ([LogRocket, 2025-11](https://blog.logrocket.com/ux-design/design-to-code-with-figma-mcp/)).
5. **Верификация** сводится к дифу «скриншот страницы ↔ PNG-экспорт фрейма из Figma»: экспорт через уже настроенный `download_figma_images` (scale 2), скриншот через Playwright (тот же viewport, `deviceScaleFactor: 2`, скрытые скроллбары, ожидание шрифтов), сравнение pixelmatch с бюджетом ~1–2% расхождения. Буквальное 1:1 невозможно физически (антиалиасинг/хинтинг шрифтов различаются между рендерерами Figma и браузера и между ОС) — [Playwright docs](https://playwright.dev/docs/test-snapshots).
6. **Регрессионная защита после приёмки** — baseline-скриншоты в репо через `toHaveScreenshot`. Lost Pixel мёртв (репозиторий заархивирован 2026-04, команда ушлала в Figma); облачные варианты при необходимости — Argos/Chromatic.
7. **Дизайн-токены**: Figma Variables → CSS custom properties → Tailwind v4 `@theme`. HeroUI v3 специально устроена под это: тема переопределяется семантическими CSS-переменными (`--accent`, `--background`, …) через `@theme inline`-мост — экспортированные из Figma значения подставляются в кастомный theme-файл ([HeroUI v3 Theming](https://v3.heroui.com/docs/react/getting-started/theming)).
8. Code Connect (CLI-объекты `.figma.tsx`, маппинг нод на компоненты) требует сид Dev/Full на Organization/Enterprise — для нашего тарифа замена: **таблица маппинга «Figma-компонент → HeroUI v3 компонент» в репо** + правила в `apps/frontend/AGENTS.md` (аналог «design system rules» официального MCP).

---

## 1. Чтение дизайна агентом

### Официальный Figma Dev Mode MCP (remote)

- URL: `https://mcp.figma.com/mcp` (streaming HTTP), OAuth при первом подключении, настольное приложение не нужно. Установка в Claude Code: `claude mcp add --transport http figma https://mcp.figma.com/mcp`; официальный плагин `figma@claude-plugins-official` ([remote installation](https://developers.figma.com/docs/figma-mcp-server/remote-server-installation/)).
- Ключевые инструменты (август 2026, ~25 шт.): `get_design_context` (бывший `get_code`) — структурированное React+Tailwind-представление выделенного фрейма, настраивается промптом; `get_variable_defs` — переменные/токены выделения; `get_screenshot`; `get_metadata` (разреженное дерево слоёв, если ответ усечён); `get_code_connect_map` — маппинг нод на реальные компоненты кода; prompt `create_design_system_rules` — генерирует файл правил дизайн-системы для агента ([Tools and prompts](https://developers.figma.com/docs/figma-mcp-server/tools-and-prompts/)).
- Лимиты: технически доступен всем тарифам, но Starter/View/Collab — **~6 вызовов инструментов в месяц**; Dev/Full сиды на Professional/Organization/Enterprise — посекундные лимиты уровня Tier 1 REST API ([mcp-server-guide](https://github.com/figma/mcp-server-guide)). Настольный вариант сервера требует Dev/Full сид.
- Официально рекомендуемые приёмы: вставлять **ссылку на выделение** (Copy link to selection); **дробить** большие фреймы на компоненты (Card, Header, Sidebar), иначе ответы усекаются; порядок «context → metadata → screenshot → реализация»; в промпте явно указывать целевую библиотеку и путь файла («Use our Stack», «add to src/components/...») ([mcp-server-guide](https://github.com/figma/mcp-server-guide)).
- Code Connect (CLI, `figma.connect()` в `.figma.tsx`): публикует реальные сниппеты компонентов в Dev Mode и отдаёт их агенту через `get_code_connect_map` — но требует Dev/Full сид на **Organization/Enterprise** ([developers.figma.com/docs/code-connect](https://developers.figma.com/docs/code-connect/)).

### Framelink Figma Context MCP (то, что настроено у нас)

- Репозиторий [GLips/Figma-Context-MCP](https://github.com/GLips/Figma-Context-MCP) (MIT, ~15.7k звёзд), npm `figma-developer-mcp`, авторизация PAT-токеном REST API, без тарифных ограничений Figma.
- Инструменты: `get_figma_data` — упрощённый JSON дерева фрейма (абсолютное позиционирование преобразуется в семантический Flexbox/Grid, векторные фрагменты сворачиваются в иконки-экспорты, CSS чистится от дефолтов, >50% сжатия); `download_figma_images` — экспорт SVG/PNG нод с масштабом.
- Чего нет относительно официального: генерации кода (`get_design_context`), переменных/токенов, скриншотов выделения, Code Connect, design-system rules. Это чистый read-only канал контекста.
- Практика LogRocket ([2025-11](https://blog.logrocket.com/ux-design/design-to-code-with-figma-mcp/)): в сессии должен работать **один** Figma MCP — официальный или Framelink; два одновременно путают агента.

**Вывод для нас:** Framelink покрывает 80% задачи (структура + flex-семантика + экспорт PNG для верификации). Официальный remote MCP стоит добавить, если у владельца диздока есть Dev/Full сид на платном тарифе — тогда в сессиях вёрстки Framelink отключается. Если тариф не позволяет — Framelink достаточно, пробел «переменные» закрывается экспортом токенов (раздел 5).

## 2. Сторонние Figma-to-code генераторы

| Инструмент | Что генерирует | Вердикт 2025–2026 |
|---|---|---|
| Builder.io (Visual Copilot, MCP) | React/Next, Vue, …; Tailwind; **маппинг на свою библиотеку компонентов** | Лучший для команд с дизайн-системой; точность ~70–75%; бесплатно до 5 пользователей ([dev.to, 2025-12](https://dev.to/emma_schmidt_/i-tested-5-design-to-code-ai-tools-for-30-days-heres-what-actually-works-2p2d); [sixtythirtyten, 2026-02](https://www.sixtythirtyten.co/blog/from-figma-to-code-ai-design-to-dev-workflows-in-2026)) |
| Anima | React/Next, Vue, HTML; Tailwind + shadcn/MUI/AntD | Прототипирование; код «требует значительной очистки», 65–70% |
| Locofy (Lightning) | React/Next, Vue, RN, Flutter; Tailwind + MUI/Chakra | «Пиксель-перфект» — маркетинг; реальный черновик 75–80%, требует идеально организованного Figma |
| Quest AI | React + MUI и др. | Жив, но шума и агентных функций меньше |
| CodeParrot | React/Vue/… из VS Code | Домен, по сообщениям, припаркован (2026) — статус неопределён |
| Tempo / Subframe / Onlook | Визуальные IDE поверх кода | Не конвертеры «макет → код»; слой итераций |
| v0 (Vercel) | React/Next + Tailwind + shadcn/ui | Импорт Figma есть, но по отзывам фактически скриншотный; сам Vercel рекомендует итеративный подход по частям ([vercel.com/blog, 2025-01](https://vercel.com/blog/working-with-figma-and-custom-design-systems-in-v0)) |
| Figma Make | React + Tailwind (полноценное приложение) | Прототипы; продуктовой код из него не вытаскивают — перегенерируют агентом по компонентам. Мы уже использовали для `apps/landing` — там это уместно (одноразовый сайт), для `apps/frontend` — нет |

Общий вывод практиков: **генератор может дать черновик, но не финал**; точность привязана к организации Figma-файла; экономия времени 20–40%, из которых 20–30% возвращается очисткой. Для репо с готовой дизайн-системой (HeroUI v3) сторонний генератор добавляет мало: его вывод всё равно переписывается под наши компоненты и FSD-границы.

## 3. Визуальная верификация

### Диф «реализация ↔ макет»

- Экспорт эталона: Figma REST `GET /v1/images` (или наш `download_figma_images`) — PNG ноды со `scale` до 4; лимиты Tier 1: 10–20 запросов/мин на Dev/Full сид; URL живёт ~14 дней; потолок рендера ~32 Мп на ноду ([rate limits](https://developers.figma.com/docs/rest-api/rate-limits/)). Практика: экспортировать один раз на экран и кэшировать файл в репо/временно.
- Сравнение: Playwright `page.screenshot()` при фиксированном viewport (ширина = ширине фрейма), `deviceScaleFactor` = scale экспорта; выравнивание (прятать скроллбары — съедают ~15px, кропать); pixelmatch/ODiff с бюджетом расхождения. Известный трюк для текста — blur 2px перед сравнением ([playwright#7548](https://github.com/microsoft/playwright/issues/7548)). Референсная реализация цикла: [vadim.blog «Pixel-perfect: Playwright + Figma MCP»](https://vadim.blog/pixel-perfect-playwright-figma-mcp/) — «ИИ — это труд, Playwright — качество, Figma — источник истины»; измерять (computed styles, bounding boxes), а не «смотреть на глаз».
- Готовые оверлей-инструменты (ручная проверка, не CI): [Pixelay](https://hypermatic.com/pixelay/) (оверлей живого сайта на макет), UI Match; enterprise — [Applitools Figma-плагин](https://applitools.com/blog/figma-design-testing-applitools-plugin/). Проверенные отрицания: mirage.dev — студия, PerceptPixel — CDN, PixelTrue — дизайн-подписка; Chromatic×Figma — side-by-side-ревью, не пиксельный диф.
- **Lost Pixel не брать**: архивирован 2026-04-22, команда перешла в Figma ([github.com/lost-pixel](https://github.com/lost-pixel/lost-pixel)).

### Регрессия после приёмки

- Playwright `toHaveScreenshot`: встроенный pixelmatch-диф, `maxDiffPixels`/`maxDiffPixelRatio`, `mask` для динамики (время, аватары), `animations: "disabled"` по умолчанию. Базлайны платформозависимы (`chromium-darwin`) — снимать и сравнивать в одном окружении ([playwright.dev/docs/test-snapshots](https://playwright.dev/docs/test-snapshots)).
- Облако (опционально, позже): Argos (активно переманивает мигрантов Lost Pixel), Chromatic (Storybook), Percy. Для нас необязательно: self-hosted Playwright в CI покрывает задачу.

## 4. Гигиена Figma-файла

Официальные рекомендации ([Structure your Figma file for better code](https://developers.figma.com/docs/figma-mcp-server/structure-figma-file/)) + практики ([LogRocket, 2025-11](https://blog.logrocket.com/ux-design/design-to-code-with-figma-mcp/)):

- **Auto Layout на всём** — прямой маппинг во flex (`direction`/`alignment`/`padding`/`gap`); абсолютное позиционирование ломает генерацию. Проверка: ресайзить фрейм перед генерацией и смотреть, что не расползается.
- **Компоненты** для всего повторяющегося; **варианты** компонента = состояния UI в коде (hover, loading, empty, disabled).
- **Figma Variables** для цвета/спейсинга/радиуса/типографики: примитивы + семантический слой поверх (как `:root`-примитивы в Tailwind). В переменных задать **code syntax** = имя CSS-переменной (`--color-accent`) — тогда токены попадают в вывод дословно.
- **Смысловые имена слоёв** (`CardContainer`, `CTA_Button`), не `Frame1268`/`Group 5`.
- Текст — текстом (не outlines); кастомные шрифты загружены в аккаунт Figma, иначе MCP их не увидит.
- Отдельные фреймы на каждый брейкпоинт (mobile/desktop) — «пиксель-перфект» определяется на фрейм, адаптивность агент выводит семантически из пары фреймов.
- Аннотации для неочевидного поведения (hover-эффекты, скролл-зоны, что скрывается на мобильных).
- Крупные экраны дробить: ссылка на выделение компонента (Copy link to selection), не на весь фрейм-размером-с-экран.

## 5. Дизайн-токены

Два рабочих пути из Figma Variables в Tailwind v4 `@theme`:

1. **Лёгкий (без билд-пайплайна)**: плагины [Token CSS exporter](https://www.figma.com/community/plugin/1579789195204449690/token-css-exporter-dev-mode) / [Export Variables](https://www.figma.com/community/plugin/1382804904426179542/export-variables-css-scss-tailwind) → `tokens.css` (CSS custom properties по коллекциям/режимам) → в `globals.css` значения в `:root`/тёмную тему + мост `@theme inline`. Тот же JSON полезно приложить к промпту агента как машиночитаемый контекст.
2. **С дисциплиной DS**: Tokens Studio for Figma → JSON в GitHub → Style Dictionary (+ `sd-tailwindv4`-формат, экспериментальный) → `design-system.css` с `@theme` и режимами в `@layer base` ([tokens-studio/sd-tailwindv4](https://github.com/tokens-studio/sd-tailwindv4)).

**Интеграция с HeroUI v3**: тема построена так, что достаточно переопределить семантические CSS-переменные (`--accent`, `--background`, `--surface`, `--radius`, …) в кастомном theme-файле (`[data-theme="..."]`-блоки в `@layer base`, режимы = modes Figma-переменных) — утилиты и компоненты подхватят автоматически ([v3.heroui.com theming](https://v3.heroui.com/docs/react/getting-started/theming)). То есть: экспорт Figma Variables → CSS custom properties → подстановка значений под семантические имена HeroUI в нашем theme-файле. Ровно один файл-мост на всю дизайн-систему.

## 6. Варианты пайплайна и рекомендация

### Вариант A — «агент + дизайн-система + измеримый диф» (рекомендуется)

Цикл на каждый экран, всё внутри существующего стека:

1. **Подготовка (однократно)**: гигиена Figma-файла (раздел 4); экспорт токенов в Tailwind v4/HeroUI v3 (раздел 5); в репо — таблица маппинга «Figma-компонент → HeroUI v3 компонент» + правила вёрстки в `apps/frontend/AGENTS.md` (аналог design-system rules, замена недоступному Code Connect).
2. **Чтение**: агент получает ссылку на фрейм → Framelink `get_figma_data` (структура/flex-семантика) + `download_figma_images` (PNG-эталон и растровые ассеты). Если добавлен официальный MCP — `get_design_context` + `get_variable_defs`, Framelink в этих сессиях выключен.
3. **Реализация**: экран/блок собирается из HeroUI v3 (обязательная сверка API через `heroui-react` MCP — v3 beta отсутствует в тренировочных данных) и Tailwind-токенов, по правилам FSD (Server Components по умолчанию, деньги только через `formatMoneyKopecks`). `get_design_context`/примеры генераторов — черновик структуры, не источник кода.
4. **Измерение**: e2e-скрипт (Playwright, `e2e/`): viewport = фрейм, `deviceScaleFactor: 2`, скрытые скроллбары, ожидание `document.fonts.ready`, `page.screenshot()` → pixelmatch против PNG-эталона → отчёт с % расхождения и диф-картинкой.
5. **Итерация**: правки до бюджета (раздел 7), включая числовые проверки (`getComputedStyle`/`getBoundingClientRect` против значений из макета — «мерить, а не смотреть на глаз»).
6. **Состояния**: hover/loading/empty/error — по вариантам фрейма; чего нет в макете — отдельный вопрос дизайнеру, не фантазия агента.
7. **Приёмка → регрессия**: базлайн-скриншот экрана фиксируется `toHaveScreenshot` — дальнейшие правки не молча ломают вёрстку; `/code-review` по стандартам репо.

### Вариант B — только Framelink MCP + тот же цикл

Отличие от A: нет `get_design_context` (агент строит разметку сам из JSON-структуры) и нет `get_variable_defs` (токены только через файловый экспорт). Дешевле (не нужен Dev-сид), чуть больше итераций дифа. Полностью рабочий — стартуем с него, если тариф Figma не даёт официального MCP.

### Вариант C — генератор-скаффолдер (Builder.io/Anima/Locofy) + доводка агентом

Генератор выдаёт черновик по макету, агент переписывает под HeroUI v3/FSD. Оправдан при массовой миграции чужого дизайна без дизайн-системы. Для нас — лишний шаг: дизайн-система уже есть, черновик всё равно переписывается, а在外нем SaaS утекают макеты. Не рекомендуется.

### Вариант D — Figma Make

Экспорт готового React+Tailwind приложения. Уместен для одноразовых сайтов (так сделан `apps/landing`). Для продуктовых экранов — нет: код вне наших конвенций (FSD, HeroUI v3, копейки) и не поддерживается. Не рекомендуется.

## 7. Критерий приёмки

- Буквальный 1:1 недостижим физически: рендер текста в Figma ≠ рендер в браузере (антиалиасинг, хинтинг, subpixel), плюс различия ОС/железа/GPU ([playwright.dev](https://playwright.dev/docs/test-snapshots)).
- Практика индустрии: `maxDiffPixelRatio` **0.01–0.02** для текстоёмких UI, либо небольшой абсолютный `maxDiffPixels`; `threshold` (per-pixel YIQ) — дефолт 0.2, до 0.3 при шуме антиалиасинга; для текстовых зон — blur 2px ([OneUptime, 2026-01](https://oneuptime.com/blog/post/2026-01-27-playwright-visual-testing/view); [playwright#7548](https://github.com/microsoft/playwright/issues/7548)).
- Предлагаемый DoD на экран: диф против эталона ≤ 2% пикселей (без blur) при deviceScaleFactor 2 на chromium; ни одного пропущенного/лишнего блока; все значения из токенов (нет хардкода цветов/спейсингов вне токенов); компоненты — HeroUI v3, где компонент существует; состояния из макета реализованы; FSD-boundaries и линт чистые; диф-картинка приложена к тикету/PR.
- Динамические зоны (время, аватары, графики) — маскируются или фиксируются моками.

## Источники

Официальные:
- https://developers.figma.com/docs/figma-mcp-server/tools-and-prompts/
- https://developers.figma.com/docs/figma-mcp-server/remote-server-installation/
- https://developers.figma.com/docs/figma-mcp-server/structure-figma-file/
- https://help.figma.com/hc/en-us/articles/32132100833559-Guide-to-the-Figma-MCP-server
- https://developers.figma.com/docs/code-connect/
- https://developers.figma.com/docs/rest-api/rate-limits/
- https://www.figma.com/blog/design-context-everywhere-you-build/
- https://www.figma.com/blog/design-systems-ai-mcp/
- https://v3.heroui.com/docs/react/getting-started/theming (через MCP heroui-react)
- https://tailwindcss.com/docs/theme
- https://playwright.dev/docs/test-snapshots

Практики и обзоры:
- https://vadim.blog/pixel-perfect-playwright-figma-mcp/ (2026-03)
- https://www.sixtythirtyten.co/blog/from-figma-to-code-ai-design-to-dev-workflows-in-2026 (2026-02)
- https://blog.logrocket.com/ux-design/design-to-code-with-figma-mcp/ (2025-11)
- https://dev.to/emma_schmidt_/i-tested-5-design-to-code-ai-tools-for-30-days-heres-what-actually-works-2p2d (2025-12)
- https://vercel.com/blog/working-with-figma-and-custom-design-systems-in-v0 (2025-01)
- https://www.reddit.com/r/ClaudeCode/comments/1npaa7p/pixel_perfect_design_work/ (2025-09)
- https://oneuptime.com/blog/post/2026-01-27-playwright-visual-testing/view (2026-01)
- https://github.com/microsoft/playwright/issues/7548

Инструменты:
- https://github.com/GLips/Figma-Context-MCP
- https://github.com/figma/mcp-server-guide
- https://github.com/lost-pixel/lost-pixel (заархивирован 2026-04)
- https://argos-ci.com/blog/lost-pixel-alternatives
- https://github.com/tokens-studio/sd-tailwindv4
- https://hypermatic.com/pixelay/
- https://applitools.com/blog/figma-design-testing-applitools-plugin/
