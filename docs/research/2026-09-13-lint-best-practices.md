# Research: лучшие практики линтинга по вендорам (сентябрь 2026) + stylelint и tools/*.mjs кандидаты

- Тикет: #640 (карта #636 «Усиление линтеров»)
- Дата: 2026-09-13
- Вопрос: чего контуру не хватает **против практик самих вендоров и индустриального консенсуса** (не против абстракции): (а) eslint/typescript-eslint/golangci/React/knip/squawk/trivy/govulncheck — новое с августовского раунда research #321/#328; oxlint/Biome как доп-слой; (б) stylelint для `apps/frontend`; (в) линтинг `tools/*.mjs`.
- Метод: первоисточники (eslint.org, typescript-eslint.io, golangci-lint.run, react.dev, knip.dev, GitHub-ченджлоги squawk/trivy, oxc.rs, biomejs.dev, stylelint.io) + advisory-прогоны на фактическом коде через временные конфиги в `/tmp` (stylelint 17.15.0 и eslint 9.39.5 через `npx -p`; `package.json` не тронут, зависимости не добавлены, временные конфиги удалены). Пины и версии сверены с Makefile и `apps/*/node_modules`.

## 0. База (факты на 2026-09-13)

Пины: golangci-lint v2.12.2, govulncheck v1.7.0, trivy-образ `0.74.0` по дайджесту, knip 6.32.2, squawk v2.62.0, Node 24, Go 1.26.5 (`apps/backend/go.mod`). ESLint: `^9.39.5` в обоих приложениях (фронт — через `eslint-config-next` 16.3.1, админка — напрямую + `typescript-eslint` ^8.67.0 + `eslint-plugin-react-hooks` ^7.1.1). Конвенция карты: advisory-прогон → счётчик до нуля → флип в error; подавления запрещены.

Состояние относительно вендорских «максимумов»: бар #323/#330/#378 уже **выше** вендорских recommended по всем осям, где это было измерено в августе (type-checked-семейства, compiler-powered react-hooks, курируемый strict golangci). Этот док ищет дельту, появившуюся **после** августа, и непокрытые поверхности.

---

## (а) Вендорские практики против нашего контура

### а.1 ESLint 9 → 10 — главный кандидат (сигнал: высокий; усилия: средние)

- **ESLint v9.x — EOL с 2026-08-06** (заявлено в релизном посте v10). Мы сидим на `^9`. Первоисточник: [eslint v10.0.0 released](https://eslint.org/blog/2026/02/eslint-v10.0.0-released/).
- Актуальная линейка: v10.9.0 (2026-08) → 10.9.1/10.10.0 в npm. [eslint v10.9.0 released](https://eslint.org/blog/2026/08/eslint-v10.9.0-released/).
- Что v10 даёт **бесплатно** нашему контуру:
  - **Трекинг JSX-ссылок**: исчезают классы ложных срабатываний `no-unused-vars` и ложных отрицаний `no-undef` на JSX-элементах — то самое правило, на котором стоят деньги/безопасность-селекторы рядом с JSX.
  - **`eslint:recommended` расширен** новыми правилами, признанными вендором важными; `no-shadow-restricted-names` теперь ловит `globalThis`.
  - Конфиг-лукап **file-based** (от каталога каждого файла) — у нас и так per-app `eslint.config.mjs`, breaking-изменение прозрачное; `.eslintrc`-наследие удалено целиком (мы полностью на flat config с #330).
- Совместимость звеньев (проверено по npm peer-deps и релизам):
  - `eslint-config-next` 16.3.1: peer `eslint >=9.0.0` — v10 проходит (проверено `npm view eslint-config-next@16.3.1 peerDependencies`).
  - `eslint-plugin-react-hooks` 7.1.1: поддержка ESLint v10 добавлена в 7.1.0 (facebook/react releases; у нас уже 7.1.1).
  - `typescript-eslint` 8.70.0: peer `^8.57.0 || ^9.0.0 || ^10.0.0` (проверено `npm view typescript-eslint@8.70.0 peerDependencies`).
- Требование Node `^20.19 || ^22.13 || >=24` — у нас Node 24 (`Makefile NODE_VERSION`), проходит.
- Дельта пресета `eslint-config-next`: 16.3.1 → 16.3.5 (последняя) — патч-линия next 16, пресеты обновляются вместе; кандидат на попутный бамп (оси (в) карты).

**Внедрение по конвенции карты**: бамп `eslint` в обоих приложениях, advisory-прогон, флип. Отдельно проверить `e2e/**`-блок фронта (правила `react-hooks/*` в нём не меняются) и витые селекторы `no-restricted-syntax` (синтаксис селекторов не менялся).

### а.2 typescript-eslint — пресеты не двигались, звено свежее (сигнал: низкий)

- Актуальная версия 8.70.0 (2026-09-07): новый rule `no-generated-empty-object-type` (неприменим — у нас гейтится импорт генератов на другом уровне), опция `flagUnions` у `no-misused-promises` (у нас это правило error во фронте — опция кандидат на advisory-прогон, сигнал ожидаемо нулевой), фиксы `no-unnecessary-condition`/`no-deprecated`. Пресеты `recommendedTypeChecked`/`strictTypeChecked` **не менялись** в 8.68–8.70 (GitHub Releases typescript-eslint) — августовский вердикт #328 по пресетам не устарел.
- Админка `^8.67.0` — caret уже резолвит 8.70 при обновлении lockfile; фронт получает typescript-eslint транзитивно из `eslint-config-next`.
- Vendor-практика «linting with type information» через `projectService` у нас уже канонична в обоих приложениях ([typescript-eslint v8](https://typescript-eslint.io/blog/announcing-typescript-eslint-v8/)).

### а.3 React: eslint-plugin-react-hooks v7 — мы уже на вендорском максимуме, осталась одна «latest»-хвостушка (сигнал: низко-средний; усилия: низкие)

- Официальная референс-страница плагина теперь живёт в react.dev: [eslint-plugin-react-hooks reference](https://react.dev/reference/eslint-plugin-react-hooks) — вендор считает весь compiler-powered набор обязательной частью контура. Автономный `eslint-plugin-react-compiler` деприкейтен и влит в hooks-плагин v6+.
- Факт по нашим конфигам (дамп установленных пресетов 7.1.1):
  - Фронт: `eslint-config-next` 16.3.1 разварачивает `reactHooks.configs.recommended.rules` = 16 правил, из них 14 compiler-powered в error (`static-components`, `use-memo`, `preserve-manual-memoization`, `immutability`, `globals`, `refs`, `set-state-in-effect`, `error-boundaries`, `purity`, `set-state-in-render`, `config`, `gating` + классика). Совпадает с базовой линией #328 — активный, не «документированный».
  - Админка: `reactHooks.configs.flat.recommended` — тот же compiler-powered набор в error.
- Дельты против вендора:
  1. **`react-hooks/void-use-memo`** — единственное правило из `recommended-latest`/`flat.recommended-latest`, которого нет в используемых нами пресетах (17-е правило vendor-«latest»). Кандидат: advisory-прогон → 0 → флип. Усилия — один блок в конфиге.
  2. **`incompatible-library` и `unsupported-syntax`** стоят в `warn` в самих вендорских пресетах (осознанно, т.к. зависят от стека). По конвенции карты это кандидаты на advisory-прогон → флип в error (сигнал ожидаемо ~0, но флип закрывает вопрос).
- `babel-plugin-react-compiler` 1.0.0 уже в deps фронта — компилятор принят на уровне сборки, `gating`-правило уже error: вендорская связка «compiler + его линтер» у нас полная.
- Известный вендорский перф-кейс (react-hooks = 42–56% времени линта в больших проектах, facebook/react#35395) в v7.1.0 смягчён пропуском не-React файлов — для нас неактуально, но ещё один довод не резать plugin.

### а.4 golangci-lint v2.12.2 → v2.13.2 (сигнал: средний; усилия: низкие)

Вендор не публикует «recommended configuration» — только каталог линтеров и дефолт `standard` ([docs/linters](https://golangci-lint.run/docs/linters/)); наш курируемый strict шире вендорского дефолта, поэтому ось усиления здесь — **новые опции уже включённых линтеров** в v2.13.0 (2026-08-19; [ченджлог](https://golangci-lint.run/docs/product/changelog/)):

- `fatcontext` — новые `check-loops`, `check-function-literals` (у нас линтер включён без опций = старое поведение; кандидаты на advisory-прогон).
- `goconst` — `ignore-map-keys`, `exclude-types`; `dupword` — `skip-raw-strings`.
- `iface` — новый анализатор `unusedmethod` (у нас iface в error с #336).
- `modernize` — +7 анализаторов: `atomictypes`, `embedlit`, `errorsastype`, `importcomment`, `reflecttypeassert`, `slicesclip`, `slicesbackward` (у нас modernize с двумя disable — расширяется само при бампе).
- `gofumpt` — новые extra-подтоглы `extra.group-params`, `extra.clothe-returns`, `extra.balance-calls` (у нас `extra-rules: true` — проверить, что подтоглы не меняют форматирование; форматирующие, не репортящие).
- Поведенческие: `recvcheck` 0.3.0 получил новые дефолтные исключения (наш прогон может «похудеть» — перепроверить флип), `exhaustruct` деприкейтнут в пользу `exhaustruct_v5` (мы его не включаем — не касается), поддержка go1.27 впрок (go.mod 1.26.5).
- Линии v2.13.1/2.13.2 — фиксы (staticcheck 0.8.1, iface 1.5.1, cache entropy).

**Рекомендация**: бамп пина `GOLANGCI_LINT_VERSION` до v2.13.2 + advisory-прогон двух новых опций fatcontext; дальше по конвенции. Этап-2 REMEDIATION-маркеры в `.golangci.yml` не трогаются.

### а.5 knip / squawk / trivy / govulncheck — дельты малы (сигнал: низкий)

- **knip** 6.32.2 → 6.35.1; major-переход на v6 (oxc-parser backend, «до 60% быстрее», [knip.dev/blog/knip-v6](https://knip.dev/blog/knip-v6)) мы уже прошли — пин 6.32.2 внутри v6-линии. Минорный бамп, сигнала в ченджлогах нет.
- **squawk** v2.62.0 → v2.65.0: **ни одного нового lint-правила** после нашего пина (проверено по [CHANGELOG](https://github.com/sbdchd/squawk/blob/master/CHANGELOG.md): v2.63–v2.65 — парсер-валидации, fmt, IDE; правки поведения `ban-duplicate-column-assignments` и `prefer-robust-stmts` касаются уже существующих). Бамп можно отложить до появления правил — сигнал нулевой.
- **trivy**: наш пин `0.74.0` по дайджесту — **актуальная последняя** (релиз 2026-08-14, GitHub Releases aquasecurity/trivy). Важно из 2026: в феврале–марте случились два supply-chain-инцидента вокруг trivy-экосистемы — компрометация GitHub Actions-окружения Aqua и **вредоносный релиз `trivy-action` v0.69.4** ([Aqua](https://www.aquasec.com/blog/trivy-supply-chain-attack-what-you-need-to-know/), [StepSecurity](https://www.stepsecurity.io/blog/trivy-compromised-a-second-time---malicious-v0-69-4-release), [обсуждение #10425](https://github.com/aquasecurity/trivy/discussions/10425)). Наш способ вызова — `docker run` пин-образа **по дайджесту**, не `trivy-action` — подход уже правильный; вывод: держать дайджест-пины и не заводить action-обёртки.
- **govulncheck** v1.7.0 → v1.8.0 (последняя в `golang.org/x/vuln`, проверено `go list -m -versions`). Рутинный бамп пина.

### а.6 oxlint / Biome как ДОПОЛНИТЕЛЬНЫЙ слой (верdict: не сейчас)

- **oxlint** (1.82.0): 2026-07-22 вышла стабильная type-aware-версия tsgolint v7 ([oxc.rs blog](https://oxc.rs/blog/2026-07-22-type-aware-linting-stable)) — зрелость достигнута, официальный месседж «быстрый слой, дополняющий ESLint», JS-plugins в alpha с марта ([oxc.rs](https://oxc.rs/blog/2026-03-11-oxlint-js-plugins-alpha.html)).
- **Biome** (2.5.13): type-aware без tsc с v2 «Biotype» ([biomejs.dev/blog/biome-v2](https://biomejs.dev/blog/biome-v2/)), линтит и CSS.
- Против внедрения в наш контур:
  1. **Сигнал нулевой по построению**: наши eslint-конфиги уже гоняют вендорские строгие наборы в error; oxlint дал бы дубли тех же находок с меньшей точностью типов (иное ядро) — второй источник истины без второго класса багов.
  2. **Конфликт с zero-suppression-гейтом**: `ts-suppressions` (#399) считает подавления в TS/JS; oxlint использует совместимые с eslint директивы — либо он молча читает запрещённые комментарии (их не существует), либо потребуется отдельная история «oxlint-подавления вне гейта», что ломает принцип «исключение = явное правило с обоснованием».
  3. Боль не доказана: pre-commit на staged-файлах ≈7.5 s (tooling.md), полный lint не является блокером цикла. Выигрыш — только скорость, цена — вторая конфиг-поверхность и вторая матрица версий.
- Вывод: **замена движка вне карты, доп-слой возможен, но сегодня не оправдан**; вернуться триггерно — если время линта станет болью или oxlint допишет JS-plugins до stable.

---

## (б) CSS-линтинг: stylelint (сигнал: средне-низкий; усилия: низкие)

Актуальное состояние вендора: stylelint **17.15.0** (2026-09-04, новое правило `selector-no-unmatchable` — [ченджлог](https://stylelint.io/changelog/)); шеринговый конфиг `stylelint-config-recommended` **18.0.0** (ESM, требует stylelint 17+ и Node ≥ 20.19 — у нас Node 24, проходит).

Perimeter `apps/frontend`: **43 исходных CSS-файла, 4515 строк** (исключая `.next`): `app/globals.css` (841), токены `shared/styles/tokens.css` (104) + 41 `*.module.css` по shared/ui, widgets, features, entities. Дизайн-слой — Tailwind v4 (`@theme`, ADR 0050 в комментариях globals.css) поверх токенов, `DESIGN.md` описывает систему.

**Advisory-прогон** (stylelint 17.15.0 + stylelint-config-recommended 18.0.0 через `npx -p`, временный конфиг в `/tmp`, только core-правила, без плагинов):

```
files linted: 43 | files with findings: 4 | total: 16 (все error)
no-descending-specificity       6   (globals.css 3, TextField 3)
no-duplicate-selectors          5   (globals.css — повторные :root/html/body/img-блоки)
at-rule-no-unknown              2   (globals.css @theme — FP ванильного прогона, Tailwind v4)
font-family-no-duplicate-names  2   (globals.css preflight-идиома `monospace, monospace`)
selector-pseudo-class-no-unknown 1  (PullToRefresh :global — FP, CSS modules)
```

Интерпретация: **реальный сигнал ≈ 11 из 16**, почти целиком `globals.css` (повторные `:root`/`html`/`body`-блоки легаси-палитры рядом с новым дизайн-слоем — организация файла, не баги). Модульные CSS (41 файл) почти стерильны: 4 замечания, из них 2 — артефакты ванильного конфига (нужны `ignoreAtRules: ["theme"]` и CSS-modules-осведомлённость — это настройка одного конфига, у stylelint для этого есть штатные механизмы).

Оценка целесообразности: вендор считает stylelint стандартным контуром для любого CSS-проекта; у нас поверхность мала и чиста (16 замечаний на 4.5k строк — лучший показатель среди всех прогонов этого research), но дубли селекторов в globals.css — ровно тот класс, который руками не ловится. Кандидат: stylelint + config-recommended с точечной настройкой (@theme, CSS modules), **advisory** в стиле knip → счётчик до нуля → решение о флипе. Усиление «токены дизайн-системы гейтовать» (`declaration-property-value-allowed-list` на палитру из tokens.css) — отдельное решение поверх базового, это уже кастомизация, а не vendor-recommended.

---

## (в) tools/*.mjs: инвентарь и advisory-прогон (сигнал: низкий)

Инвентарь (по факту, `find tools -name "*.mjs" -not -path "*/node_modules/*"`):

- **32 файла, 5254 строк**; крупнейшие: `property-attributes/gen/emit-go.mjs` (566), `gen/emit-frontend-ts.mjs` (372), `migration-lint/domain-rules.mjs` (357), `suppression-gate/suppression-gate.mjs` (312).
- Node-пакеты со своим `package.json` и тестами: `hooks`, `migration-lint`, `nolint-gate`, `suppression-gate`, `dev-env` (все vitest) — гейт `make tools-test` (Makefile `TOOLS_TEST_DIRS`), плюс `payment-categories`, `property-attributes` (генераторы, тесты в комплекте, knip их покрывает). Без `package.json`: `bruno`, `e2e`, `healthcheck`, `screenshots` (не Node-пакеты).
- knip-обзор: 6 tools-пакетов в `KNIP_DIRS` (advisory).

Vendor-практика для node-скриптов: eslint flat config с `languageOptions.globals` из пакета `globals` (node-набор) поверх `@eslint/js` recommended — ровно «минимальный пресет без фреймворковых плагинов», как рекомендуют доки ESLint по настройке flat config ([eslint.org docs](https://eslint.org/docs/latest/use/configure/)).

**Advisory-прогон** (eslint 9.39.5 из node_modules админки + `@eslint/js` recommended + `globals.node/es2025`, временный конфиг в `/tmp`):

```
files linted: 32 | findings: 3 (все no-unused-vars)
tools/payment-categories/generate.mjs:23        'readFileSync' is defined but never used
tools/property-attributes/generate.mjs:27       'readFileSync' is defined but never used
tools/property-attributes/gen/emit-go.mjs:25    'aliasOf' is defined but never used
```

Оценка: 3 находки на 5.2k строк — контур гигиеничен; контрактные тесты (vitest) уже ловят runtime-класс ошибок, который обычно и мотивирует линтить скрипты. Постоянный гейт дал бы дешёвый, но слабый сигнал при ненулевой стоимости (конфиг-поверхность + обновления eslint ещё для одного периметра). **Рекомендация**: (1) разово вычистить 3 неиспользуемых импорта (5 минут, без гейта); (2) постоянный eslint по `tools/` — только advisory-слаботочный вариант в духе knip, если владелец хочет гигиену генераторов под наблюдением; блокирующий флип не оправдан измеренным сигналом.

---

## Сводка кандидатов (сигнал / усилия)

| # | Кандидат | Сигнал | Усилия | Примечание |
|---|----------|--------|--------|------------|
| 1 | ESLint 9 → 10 (оба приложения) + eslint-config-next 16.3.5 | **высокий** (v9 EOL; JSX-трекинг; расширенный recommended) | средние | peer-цепочка совместима; по конвенции advisory→флип |
| 2 | golangci-lint v2.13.2 + advisory новых опций (fatcontext, iface/unusedmethod, modernize+7) | средний | низкие | пин в Makefile; поведение recvcheck перепроверить |
| 3 | react-hooks: `void-use-memo` + флип warn→error (`incompatible-library`, `unsupported-syntax`) | низко-средний | низкие | единственная дельта от vendor-latest |
| 4 | stylelint 17 + config-recommended 18 (advisory) для `apps/frontend` | средне-низкий | низкие | 16 замечаний; настройка @theme + CSS modules |
| 5 | govulncheck v1.8.0, knip 6.35.1 | низкий | низкие | рутина пинов |
| 6 | squawk v2.65.0 | нулевой сейчас | низкие | новых правил после пина нет — отложить |
| 7 | разовая чистка 3 unused-vars в tools/*.mjs | низкий | минимальные | без нового гейта |
| 8 | oxlint/Biome как доп-слой | отрицательный сейчас | средние | вернуться триггерно (скорость линта / JS-plugins stable) |

## Источники

- ESLint: [v10.0.0 released](https://eslint.org/blog/2026/02/eslint-v10.0.0-released/), [v10.9.0 released](https://eslint.org/blog/2026/08/eslint-v10.9.0-released/), [docs: flat config](https://eslint.org/docs/latest/use/configure/)
- typescript-eslint: [Releases 8.68–8.70](https://github.com/typescript-eslint/typescript-eslint/releases), [v8 announcement (projectService)](https://typescript-eslint.io/blog/announcing-typescript-eslint-v8/)
- React: [eslint-plugin-react-hooks reference](https://react.dev/reference/eslint-plugin-react-hooks), [npm eslint-plugin-react-hooks](https://www.npmjs.com/package/eslint-plugin-react-hooks), [perf #35395](https://github.com/react/react/issues/35395)
- golangci-lint: [changelog](https://golangci-lint.run/docs/product/changelog/), [docs/linters](https://golangci-lint.run/docs/linters/), [migration guide](https://golangci-lint.run/docs/product/migration-guide/)
- knip: [knip v6](https://knip.dev/blog/knip-v6), [releases](https://github.com/webpro-nl/knip/releases)
- squawk: [CHANGELOG](https://github.com/sbdchd/squawk/blob/master/CHANGELOG.md)
- trivy: [releases](https://github.com/aquasecurity/trivy/releases), [Aqua incident blog](https://www.aquasec.com/blog/trivy-supply-chain-attack-what-you-need-to-know/), [StepSecurity on trivy-action v0.69.4](https://www.stepsecurity.io/blog/trivy-compromised-a-second-time---malicious-v0-69-4-release), [incident discussion #10425](https://github.com/aquasecurity/trivy/discussions/10425)
- govulncheck: [pkg.go.dev/golang.org/x/vuln](https://pkg.go.dev/golang.org/x/vuln)
- oxlint: [type-aware stable (tsgolint v7)](https://oxc.rs/blog/2026-07-22-type-aware-linting-stable), [JS plugins alpha](https://oxc.rs/blog/2026-03-11-oxlint-js-plugins-alpha.html)
- Biome: [v2 Biotype](https://biomejs.dev/blog/biome-v2/), [changelog](https://biomejs.dev/internals/changelog/)
- stylelint: [changelog 17.15.0](https://stylelint.io/changelog/), [stylelint-config-recommended 18.0.0](https://github.com/stylelint/stylelint-config-recommended)
- Внутреннее: docs/agents/tooling.md; docs/research/2026-08-18-golangci-lint-catalog.md; docs/research/2026-08-18-eslint-ts-strictness.md; Makefile (пины); apps/backend/.golangci.yml; apps/frontend/eslint.config.mjs; apps/admin/eslint.config.mjs
