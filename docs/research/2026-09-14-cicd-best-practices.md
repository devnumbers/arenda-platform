# CI/CD и dependency-автоматизация: best practices 2025–2026 для solo-монорепо

- **Дата:** 2026-09-14
- **Статус:** research. Это сырьё для решения владельца, не рецепт и не тикет. Закоммичен 15.09 по слову владельца (до этого висел незакоммиченным — норма для `docs/research/`).
- **Метод:** обзор публичных источников 2024–2026 (GitHub docs/changelog/discussions, доки Dependabot/Renovate/Semgrep/Docker, гайды и практический опыт команд) + сверка с фактическим состоянием `.github/` этого репо.
- **TL;DR:** наш supply-chain-контур (govulncheck, trivy, semgrep nightly, gitleaks, cosign, SBOM, SHA-пины) уже на уровне best practice — его не трогаем. Резервы: (а) скорость и биллинг CI (concurrency, path-filtering, консолидация микро-джоб, ярусность e2e/docker), (б) dependency-контур, который сейчас не работает совсем — alerts выключены, 30 красных PR, 0 слитых.

---

## 1. Краткий контекст репо

### CI (`ci.yml`, ~20 джоб, все блокирующие, полный прогон ~25 минут)

- Backend: lint, test `-race` (в контейнере golang — тулчейн прода), vet, govulncheck.
- Миграции: up-down-up на postgres-сервисе, migrations-lint (squawk + доменные правила).
- Гейты качества: nolint-gate (0 `nolint` в Go), ts-suppressions-gate (0 подавлений в TS/JS).
- Интеграция: backend-integration (testcontainers, `-race`, Docker-out-of-Docker).
- Freshness-гейты: openapi, sqlc, versions, frontend API-клиент, property-attributes, payment-categories.
- Frontend: build+lint+typecheck, tests (Vitest), e2e (Playwright с подъёмом postgres+backend+прод-сборки, артефакты — отчёт и скриншоты).
- Admin: lint+typecheck+build, tests (Vitest). Tools: contract-тесты тулов. Knip: advisory (`continue-on-error`).
- Supply chain: gitleaks (docker CLI, образ запинен), npm-audit — только при изменении lockfile (diff-гейт), docker-матрица ×4 (buildx + hadolint + trivy HIGH/CRITICAL, `ignore-unfixed`, cache `type=gha`).

### Факты, проверенные чтением файлов

- Все actions и даже docker-образ gitleaks запинены на полные SHA; `permissions: contents: read` на верхнем уровне ci.yml.
- `setup-go` и `setup-node` везде с кэшем; docker-сборка уже с `--cache-from/--cache-to type=gha`.
- **Блока `concurrency` нет** — допуши в один PR не отменяют устаревшие прогоны.
- **Path-filtering нет** (кроме самодельного diff-гейта у npm-audit) — PR с правкой только лендинга платит за backend-integration и e2e.
- `workflow_call` у ci.yml — CI переиспользуется деплой-пайплайном (reusable workflow уже в строю).

### Деплой и зависимости

- `security.yml`: semgrep (p/ci, p/xss) + trivy-fs — nightly строго; на PR non-blocking.
- `_deploy.yml` (reusable): push в dev/main → CI → build 4 образов (trivy + SBOM CycloneDX + cosign keyless sign) → collect digest'ов → cosign verify → рендер env из секрета со сверкой ключей → SSH на VPS: pg_dump бэкап (+ off-site S3) → миграции → `up -d --wait` → внешние smoke → авто-откат при падении → точечная чистка образов. Ручной откат — workflow_dispatch с 4 digest-inputs.
- `dependabot.yml`: 9 записей (gomod, npm×3, docker×4, github-actions), weekly monday, cooldown 7 дней, группы minor/patch (npm), target-branch dev, assignee Ivan-ee.
- Фактическое состояние: Dependabot alerts **выключены**; 30 открытых version-update PR, **0 слитых за всю историю**, все красные; origin/dev красный и на 239 коммитов позади локального dev; origin/main не обновлялся с 2026-08-11; branch protection недоступна (приватное репо на GitHub Free); workflow local-first — мержи в dev локальные, пуши редкие и осознанные.

---

## 2. Вопрос 1. Структура CI для small/solo-монорепо (Go+Node)

### Норма времени CI на PR в 2025–2026

- Золотой стандарт «обратной связи» — **меньше 10 минут**. Правило десяти минут Мартина Фаулера до сих пор воспроизводится как норма в свежих гайдах: [GitLab — CI best practices](https://about.gitlab.com/topics/ci-cd/continuous-integration-best-practices/), [Hokstad Consulting](https://hokstadconsulting.com/blog/ci-cd-feedback-loops-best-practices), [Wonderment Apps — Top 10 CI/CD best practices 2025](https://www.wondermentapps.com/blog/ci-cd-pipeline-best-practices/).
- Аргументация: долгие прогоны ломают фокус (context switching), поощряют «пуш и забыл», напрямую ухудшают DORA-метрики (change lead time, deployment frequency). Для соло-владельца «время CI» = «время от фичи до stage».
- Реалистичная трактовка для соло: полный прогон до ~15 минут приемлем, если **быстрый сигнал** (lint/unit/typecheck) приходит в первые минуты, а тяжёлое идёт параллельно или отдельным ярусом. Наши ~25 минут — выше нормы; хвост держат e2e и docker-матрица.

### Паттерны упрощения без ослабления проверок

1. **Path-filtering джоб** — индустриальный стандарт монорепо. Два уровня:
   - `on: pull_request: paths:` на уровне workflow — грубо: гейтит весь workflow, ломается на общем коде, конфликтует с required checks;
   - **job-level** через `dorny/paths-filter`: джоба-детектор изменений + `if:` на остальных — рекомендуемый подход ([dorny/paths-filter](https://github.com/dorny/paths-filter), [community #177835](https://github.com/orgs/community/discussions/177835)).
   - Классическая проблема «required check висит в pending, если джоба скиплена» ([#44490](https://github.com/orgs/community/discussions/44490), [#26251](https://github.com/orgs/community/discussions/26251)) — **нас не касается**: branch protection на GitHub Free для приватного репо недоступна, required checks нет. У нас path-filtering заметно проще, чем в типовых гайдах: скип чека ничего не блокирует.
   - Ловушка монорепо — общий код: фронтовые джобы зависят от бекенд-спеки (генерация API-клиента), значит их фильтр обязан включать пути спеки и workflow-файлов ([codewithkarani](https://www.codewithkarani.com/blog/monorepo-github-actions-path-filters-shared-code)).
2. **Консолидация микро-джоб.** Отдельная джоба на 30-секундный make-таргет — это оплата startup раннера + checkout (~30–60 сек списанных минут) и ещё одна строка в Checks UI. Норма — группировать быстрые независимые проверки в одну «мета»-джобу с несколькими шагами, либо встраивать шагами в джобу с готовым toolchain: freshness-гейты openapi/sqlc несут Go-setup, который уже есть в backend-джобе; frontend-api-freshness несёт npm-setup, который уже есть во frontend-джобе.
3. **Ярусность (tiered CI)** — самый громкий паттерн 2025: быстрый контур на каждый PR (lint, unit, typecheck, freshness) против полного контура на merge/nightly (e2e, сканеры, тяжёлые сборки): [Ranger — E2E in CI/CD](https://www.ranger.net/post/ultimate-guide-to-e2e-testing-in-ci-cd), [GitLab](https://about.gitlab.com/topics/ci-cd/continuous-integration-best-practices/). Применительно к нам естественный расклад: PR — lint/unit/typecheck/freshness/gitleaks; merge/nightly — e2e целиком, docker+trivy, semgrep/trivy-fs (последние уже так живут).
4. **E2E не на каждый PR.** Практикующий консенсус: на PR — смоук/критический путь или ничего; полный сьют — nightly: *«We run them nightly. Our E2E tests take about 45 minutes, and we don't want this clogging PRs»* ([r/ExperiencedDevs](https://www.reddit.com/r/ExperiencedDevs/comments/1pza4pr/at_what_point_do_you_run_e2e_tests/)), [Ranger](https://www.ranger.net/post/ultimate-guide-to-e2e-testing-in-ci-cd). Компенсации: смоук перед деплоем (роль частично уже играют внешние smoke в `_deploy.yml`) или nightly с уведомлением.
5. **Reusable workflows** для повторяющейся логики (build+scan+sign, deploy) — у нас уже есть (`_deploy.yml`, `workflow_call` у ci.yml): соответствие лучшим практикам, ничего делать не надо.
6. **Чему следовать не стоит:** affected-only через Turborepo/Nx — стандарт для JS-workspace монорепо, к нашей структуре (4 независимых приложения + Go-модуль) напрямую неприменим; наш аналог — path-filtering.

---

## 3. Вопрос 2. Dependency-автоматизация: Dependabot vs Renovate в 2026

### Сравнение инструментов

| Критерий | Dependabot | Renovate |
|---|---|---|
| Настройка | YAML в репо, почти zero-config | Конфиг в репо (config-as-code) + presets; ~15–30 мин на освоение |
| Бесплатность приватных репо | Да: alerts и version updates бесплатны на всех планах | Да: hosted GitHub App бесплатен для приватных репо; плюс бесплатный self-hosted CE |
| Группировка | `groups` по типам обновлений | Мощнее: presets, монорепо-группы, `lockFileMaintenance`, отдельные расписания |
| Cooldown/«выдержка» релиза | `cooldown.default-days` | `minimumReleaseAge` (бывш. `stabilityDays`) |
| Automerge | Только через Actions-костыль | Встроенный `automerge` + `automergeSchedule` |
| Ветка-цель | `target-branch` — только для version updates; **security updates всегда в default branch** | `baseBranch` настраивается для всего, включая vulnerability-PR |
| Lockfile без смены версий | Нет | `lockFileMaintenance` |

Источники: [Rafter — Renovate vs Dependabot (2026)](https://rafter.so/blog/renovate-vs-dependabot), [Konvu](https://konvu.com/compare/dependabot-vs-renovate), [AppSecSanta](https://appsecsanta.com/sca-tools/dependabot-vs-renovate), [официальная таблица Renovate](https://docs.renovatebot.com/bot-comparison/), [Mend Renovate hosted — free tier](https://docs.renovatebot.com/mend-hosted/overview/), [Renovate App](https://github.com/apps/renovate).

- Вердикт сравнений 2026: **Dependabot — zero-config простота, Renovate — контроль и «чистая очередь PR»** на нетривиальных репо; на монорепо с несколькими экосистемами Renovate выигрывает по функциям, Dependabot — по цене входа.
- Частный нюанс для приватного финтех-смежного репо: hosted Renovate **клонирует код на инфраструктуру Mend** — это аргумент при выборе; нейтрализуется self-hosted CE ([обсуждение](https://github.com/renovatebot/renovate/discussions/29327), [Mend Renovate Community](https://www.mend.io/mend-renovate-community/)).

### Механика Dependabot security alerts + security updates

- **Dependabot alerts** (сигналы об уязвимых зависимостях) бесплатны для приватных репо на всех планах: [community #171374](https://github.com/orgs/community/discussions/171374), [docs — About Dependabot alerts](https://docs.github.com/code-security/dependabot/dependabot-alerts/about-dependabot-alerts).
- У нас они **выключены** — сейчас уязвимости видит только govulncheck (Go, в PR) и npm-audit (npm, только в PR с изменёнными lockfiles). Для npm/gomod/docker-баз вне PR это слепая зона: advisory, вышедший во вторник, не будет виден до следующего PR с lockfiles.
- **Security updates** (авто-PR с фиксом) целятся **только в default branch**; `target-branch` из `dependabot.yml` для них игнорируется: [dependabot-core #2767](https://github.com/dependabot/dependabot-core/issues/2767), [Stack Overflow](https://stackoverflow.com/questions/72084723/change-target-branch-of-open-dependabot-pr), [GitHub docs](https://docs.github.com/code-security/dependabot/dependabot-security-updates/configuring-dependabot-security-updates).
- У нас default branch — main (прод), а рабочий поток идёт через dev. Как это живут команды:
  1. **сделать dev default branch** — security-PR придут в dev ([совет сообщества](https://www.reddit.com/r/github/comments/1tfjzp2/i_need_some_advice_on_managing_dependabot_branch/)); цена: default branch влияет на базы новых PR, триггеры Actions, раскрытие содержимого по умолчанию;
  2. **оставить main default и принять семантику** «security-PR в main = исключение из local-first»: их мерж и пуш — явное слово владельца (для CVE это, возможно, корректная семантика: фикс уязвимости и есть причина пушнуть);
  3. **перейти на Renovate**, где `baseBranch` применим и к vulnerability-PR — тогда весь поток снова идёт в dev.

### Стратегии, чтобы dependency-PR реально сливались

- Проблема «30 открытых красных PR, 0 слитых» — **процессная, не инструментальная**: любой бот без ритуала слияния генерирует очередь мусора. Краснеет всё вместе: чеки PR считаются от базы origin/dev, а origin/dev красный и на 239 коммитов позади — даже «хороший» dependency-PR не может позеленеть, пока база красная и устаревшая.
- **Еженедельное «окно обновлений»**: зависимости разбираются в фиксированный слот (например, понедельник утром — наш schedule уже такой), остальную неделю PR просто ждут: [Ben Hanzl — Renovate schedule](https://www.benhanzl.com/til/renovate-configuration/) — updates до 6am monday, уязвимости в любое время, lockfile maintenance отдельным днём.
- **Авто-мерж зелёных minor/patch**: Renovate умеет нативно (`automerge` + `automergeSchedule`); для Dependabot — workflow с `gh pr checks --watch` + `gh pr merge` — работает **без** branch protection, в отличие от нативного auto-merge, которому нужны required checks: [SO #72685861](https://stackoverflow.com/questions/72685861/auto-merge-dependabot-pr-after-all-checks-have-passed), [GitHub docs — Automating Dependabot with Actions](https://docs.github.com/en/code-security/tutorials/secure-your-dependencies/automate-dependabot-with-actions), [dev.to — Let Dependabot merge its own PRs](https://dev.to/nickytonline/let-dependabot-merge-its-own-prs-27pc).
- **Группы minor/patch** — меньше PR, меньше конфликтов (у нас есть для npm; у gomod/docker групп нет).
- **Cooldown** против pull-the-package-атак — у нас уже есть (7 дней на всех записях).
- **«Деплой-трейн» для зависимостей**: обновления едут в stage автоматически со следующим пушем; риск обновления гасится бэкапом и автооткатом, которые уже есть в `_deploy.yml`.
- **Жизнь с редкими пушами**: главный практический вывод — origin/dev не должен протухать. Достаточно ритуала раз в неделю: слить локальный dev на сервер (или запушить по явному слову владельца), тогда базы PR свежие, статусы честные, а накопившиеся dependency-PR разбираются в окно одним заходом. Без этого весь PR-механизм деградирует, что мы и наблюдаем.

### Если бы переезжали на Renovate: что изменилось бы конкретно у нас

- Один `renovate.json` заменил бы 9 записей `dependabot.yml`: пресеты для gomod/npm/docker/github-actions с общей конфигурацией (schedule, cooldown/`minimumReleaseAge`, группы, лимиты) вместо дублирования этих полей в каждой записи.
- `baseBranch: dev` распространился бы и на vulnerability-PR — решает конфликт «security-updates всегда в default branch» без смены default branch.
- Встроенный `automerge` для зелёных minor/patch + `automergeSchedule` (окно) — без написания собственного Actions-workflow с `gh pr checks --watch`.
- `lockFileMaintenance` — периодическое обновление lockfiles без смены версий; у Dependabot аналога нет.
- Цена входа: новый конфиг-язык (config-as-code, presets), и решение по приватности — hosted app (клонирует код на инфраструктуру Mend) против self-hosted CE (свой раннер, бесплатная лицензия, больше эксплуатации).
- Отказ от миграции тоже рационален: Dependabot с cooldown, группами и окном обновлений покрывает большую часть потребностей соло; решает не инструмент, а ритуал слияния.

---

## 4. Вопрос 3. Supply-chain security пропорционально solo SaaS

### Состав стека: что стандарт де-факто, что избыточно

- Консенсус для малых команд: SBOM + SCA-сканер (Trivy) + reachability-скан для Go (govulncheck) + secrets (gitleaks) + SAST (semgrep) + подписание артефактов (cosign); «предпочитай несколько инструментов, автоматизирующих гейты в CI, а не платформы»: [Oligo — Supply Chain Security 2025](https://www.oligo.security/academy/ultimate-guide-to-software-supply-chain-security-in-2025), [Veracode](https://www.veracode.com/blog/top-software-supply-chain-security-best-practices/), [OpenSSF](https://openssf.org/blog/2025/02/06/securing-public-sector-supply-chains-is-a-team-sport/).
- Наш набор — **верхняя граница нормы для соло**; избыточность возможна только в каденциях (где что гоняем), не в составе. Разбор по инструментам:
  - **govulncheck** — стандарт для Go, и не «ещё один сканер»: call-graph-анализ показывает только реально достижимые уязвимости, резко снижая шум ([Google Security Blog](https://security.googleblog.com/2023/04/supply-chain-security-for-go-part-1.html)). Для нас: держать на PR как есть.
  - **Trivy** (image+fs) — де-факто универсальный сканер малых команд; настройки `ignore-unfixed` + HIGH/CRITICAL — грамотная фильтрация шума. Вопрос только в каденции image-скана на PR (см. раздел 6).
  - **semgrep** — рекомендуемая каденция: **diff-aware на PR + полный скан по расписанию** (nightly/weekly), узкие правила на PR и широкие ночью: [Semgrep — managed scans](https://docs.semgrep.dev/deployment/managed-scanning/overview), [appsec.guide — Semgrep CI](https://appsec.guide/docs/static-analysis/semgrep/continuous-integration/). Наша схема (nightly строго, PR non-blocking) точно соответствует шаблону.
  - **gitleaks** — стандарт для secrets; блокирующий на PR — норма.
  - **cosign keyless + SBOM CycloneDX** — выше медианы для соло (медиана соло вообще ничего не подписывает), но это ровно то, что OpenSSF/SLSA продвигают для «финансовых» сервисов; у нас уже автоматизировано в деплое, стоимость владения ~нулевая. Не избыточно для record-keeping платформы с чужими финансовыми записями.
- **OIDC**: актуален для облачных провайдеров (AWS/GCP/Azure) вместо долгоживущих ключей ([GitHub docs — Secure use](https://docs.github.com/en/actions/reference/security/secure-use), [hardening guide 2026](https://www.buildmvpfast.com/blog/github-actions-supply-chain-security-hardening-guide-2026)). У нас деплой по SSH-ключу — OIDC неприменим напрямую; это осознанный трейд-офф, а не дыра. Косвенно OIDC уже используется: cosign keyless подписывает через Federação OIDC GitHub.
- **Digest-pinning действий** — GitHub официально: пин на полный SHA — «единственный способ использовать action как immutable release»; с августа 2025 есть org-политика, требующая SHA-пиннинг ([GitHub docs](https://docs.github.com/en/actions/reference/security/secure-use), [changelog 2025-08-15](https://github.blog/changelog/2025-08-15-github-actions-policy-now-supports-blocking-and-sha-pinning-actions/)). У нас запинено всё — целевое состояние достигнуто.

### Каденции: наш стек против рекомендуемой

| Контроль | Рекомендация | У нас | Оценка |
|---|---|---|---|
| govulncheck | на PR или weekly | на PR, блокирующий | соответствует |
| trivy image | на PR при изменении образов / на деплое | на PR (блок.) + на деплое | соответствует; на PR можно path-gate |
| trivy fs / semgrep | nightly полный, на PR diff/non-blocking | nightly строго, PR non-blocking | соответствует |
| gitleaks | на PR | на PR, блокирующий | соответствует |
| cosign + SBOM | на release/деплой | на каждый деплой | соответствует |
| npm audit | при изменении lockfile или weekly | на PR при изменении lockfile + pre-push | выше медианы (защита от advisory-дрейфа) |
| Зависимостные alerts | постоянно включены | **выключены** | единственная дыра контура |

Вывод: состав и каденции у нас на уровне (местами выше) рекомендаций для соло. Резерв не в выбрасывании контролей, а в их переносе с критического пути PR.

---

## 5. Вопрос 4. Деплой на один VPS через docker-compose

- **Compose на проде одного VPS — принятая норма** для соло и малых команд; Kubernetes советуют лишь когда масштаб требует ([Bunnyshell](https://bunnyshell.com/blog/is-docker-compose-production-ready/), [Ploetzli](https://blog.ploetzli.ch/2024/docker-deployment-best-practices/)). Переезд на K8s для одного VPS — анти-паттерн: оверхед без выгоды.
- **Триггеры деплоя.** Спектр: deploy-on-push (continuous deployment) → тег-релизы → ручной dispatch. Консенсус источников:
  - **stage: авто на каждый merge/push** — сходятся все ([Reddit r/devops](https://www.reddit.com/r/devops/comments/cypbhe/branches_merges_tags_deployments_environments/), [GitLab](https://about.gitlab.com/blog/from-code-to-production-a-guide-to-continuous-deployment-with-gitlab/));
  - **prod: либо тоже авто** (если доверяют CI и автооткату), либо тег/dispatch как «осознанный акт релиза» с чистой точкой отката ([dev.to — tag-based deployment](https://dev.to/srinivasamcjf/tag-based-deployment-in-jenkins-cicd-using-github-a-practical-guide-4em));
  - **release train** — батчить готовые изменения и shipping по расписанию — золотая середина для соло с редкими пушами ([Medium — Release Train vs Continuous Deployment](https://hector-reyesaleman.medium.com/release-trains-vs-continuous-deployment-13015e7f89ff), [glossary](https://glossary.deployment.to/release-train/)).
- **Разница строгости stage vs prod — да, практикуется повсеместно**: stage автодеплой, prod с гейтом. Стандартный гейт (environment required reviewers) на приватном репо Free недоступен ([GitHub docs — environments](https://docs.github.com/actions/deployment/targeting-different-environments/using-environments-for-deployment), [community #114537](https://github.com/orgs/community/discussions/114537)); функциональный эквивалент для соло — `workflow_dispatch` на prod, либо семантика «push в main = осознанный акт».
- **Наша цепочка сильнее медианы** и содержит всё, что гайды требуют от прод-деплоя ручным путём: digest-only образы + cosign verify перед выкладкой, env из секрета со сверкой ключей, бэкап до миграций (+ off-site S3), `up -d --wait` (health-gate), внешние smoke, авто-откат, ручной rollback с 4 digest-inputs.
- Единственный спорный пункт по строгим гайдам: prod-выкладка как сайд-эффект пуша в main. При local-first workflow владельца «редкий пуш = явное слово» это фактически уже ручной гейт; формализация (dispatch/тег) дала бы только читаемую историю релизов — косметика, не безопасность.
- Упрощать без потери безопасности здесь нечего: деплой-контур одновременно самый критичный (финтех-смежный прод) и уже самый проработанный.

---

## 6. Вопрос 5. Скорость CI: оптимизации GitHub Actions с оценками эффекта

1. **`concurrency` + `cancel-in-progress`** — отмена устаревших прогонов того же PR: официальный механизм ([workflow syntax](https://docs.github.com/actions/reference/workflow-syntax-for-github-actions), [docs — concurrency](https://docs.github.com/en/actions/writing-workflows/choosing-what-your-workflow-does/control-the-concurrency-of-workflows-and-jobs)); вариант «отменять только PR-прогоны, не деплои» — [discussion #69704](https://github.com/orgs/community/discussions/69704). У нас блока нет. Эффект: каждый допуш в PR вместо двух полных прогонов делает один; экономия минут пропорциональна частоте допушей (типично 10–30% PR-минут), плюс меньше очередь и быстрее реальный сигнал.
2. **Кэши.** Go (`setup-go` с GOMODCACHE/GOCACHE) и npm (`setup-node cache`) есть — это и есть «single biggest CI win for Go» ([setup-go](https://github.com/actions/setup-go), [runs-on — Go CI](https://runs-on.com/docs/guides/languages/go/)). Docker layer cache `type=gha` включён; кейс HyperDX: 8 мин → 4 мин в среднем, ~1 мин на тёплом кэше ([HyperDX](https://hyperdx.io/blog/docker-buildx-cache-with-github-actions), [Docker docs — gha backend](https://docs.docker.com/build/cache/backends/gha/)). Осторожно: лимит кэша 10 ГБ на репо, крупные образы вымывают чужой кэш — альтернатива `type=registry` в GHCR ([quanttype](https://quanttype.net/p/weeknote-17-caching-docker-builds/)).
3. **Path-filtering** — крупнейший резерв нашего ci.yml. Оценки: affected-only запуски дают «до 12×» (vendor-оценка [WarpBuild](https://warpbuild.com/blog/github-actions-monorepo-guide)); консервативно: PR с изменениями одного приложения перестают платить за 3/4 сетки, docs/tools-only PR почти бесплатны. Побочный эффект — экономия биллинг-минут (см. п. 6).
4. **Вынос e2e и docker-сканов из PR-пути** — второй по величине резерв. frontend-e2e (postgres+backend+прод-сборка фронта) и docker-матрица (4 образа, max-parallel 2, + trivy) — самые длинные джобы; их перенос в nightly/advisory/деплой-контур типично срезает 30–50% wall-time PR. Важно: build+trivy 4 образов **уже дублируется** в deploy-пайплайне перед каждой выкладкой — дубль на PR частично избыточен; e2e-хвост частично страхуют внешние smoke на деплое.
5. **Консолидация микро-джоб** (раздел 2, п. 2) — эффект в основном на биллинг-минуты (−5–10 на PR) и читаемость Checks UI, меньше на wall-time (критический путь — backend/e2e/docker).
6. **Биллинг-контекст (важно для Free-плана):** GitHub Free для приватных репо включает **2000 Actions-минут/месяц** ([GitHub — Actions limits](https://docs.github.com/en/actions/reference/limits), [billing](https://docs.github.com/billing/managing-billing-for-github-actions/about-billing-for-github-actions)); счётчик — **сумма минут всех джоб**, не wall-time. 20+ джоб × ~25-минутный прогон — порядка 100+ списанных минут на PR, плюс nightly security и build×4 на каждый деплой. Грубый ориентир: даже 10–15 полных PR-прогонов в месяц заметно едят квоту (точные цифры — в GitHub usage). Сокращение CI — одновременно скорость и деньги.
7. **Affected-only через Turborepo/Nx** — к нашей структуре неприменим напрямую (см. раздел 2); наш аналог — path-filtering из п. 3.

---

## 7. Что у нас уже соответствует best practices (не трогать)

- **SHA-пиннинг всех actions** и даже docker-образа gitleaks; `permissions: contents: read` — целевое состояние по GitHub-гайдам и OpenSSF.
- **Reusable workflows** (`_deploy.yml`, `workflow_call` у ci.yml).
- **Cooldown 7 дней** на всех записях Dependabot (защита от pull-the-package; даже semgrep-правило его требует) + **группы minor/patch** для npm.
- **npm-audit с diff-гейтом на lockfiles** — защита от «advisory-дрейфа», красящего чужие PR; выше медианы.
- **knip как advisory** (`continue-on-error`) — правильный шаблон «сигнал, не гейт».
- **govulncheck** (reachability, низкий шум), **gitleaks**, **semgrep nightly строго / PR non-blocking** — ровно рекомендуемые каденции.
- **Кэши**: setup-go/setup-node везде, docker `type=gha`.
- **Freshness-гейты** (openapi/sqlc/versions/api-client/attributes/categories) — редкая и сильная практика для generated-кода.
- **Миграции up-down-up** и migrations-lint (squawk + доменные правила) на PR.
- **Deploy-цепочка**: digest-pinning + cosign verify + бэкап (+off-site) + `up -d --wait` + smoke + авто-откат + ручной rollback — сильнее типового «SSH + docker compose up».
- **security.yml**: nightly строго, на PR мягко — образцовая каденция.
- Вывод: security-постулаты не пересматриваем; все кандидаты ниже — про скорость, биллинг и оживление dependency-контура.

---

## 8. Применимые паттерны для arenda-platform (кандидаты на решение)

Это кандидаты, не решения — каждый требует явного слова владельца. Сортировка: быстрые победы вверху.

| # | Кандидат | Эффект | Риск | Сложность |
|---|---|---|---|---|
| 1 | Включить **Dependabot alerts** | Закрывает единственную дыру security-контура (см. раздел 4) | Низкий | 5 минут |
| 2 | **`concurrency` + `cancel-in-progress`** в ci.yml | 10–30% PR-минут на допушах, быстрее сигнал | Низкий | ~5 строк |
| 3 | **Консолидация микро-джоб** | −6–8 джоб, −5–10 биллинг-минут/PR, читаемый Checks | Низкий | Средняя |
| 4 | **Path-filtering джоб** | Крупнейший резерв wall-time и минут | Низкий–средний | Средняя |
| 5 | **e2e: ярусность** (смоук/advisory на PR, полный nightly или перед деплоем) | −5–10 мин wall на PR | Средний | Средняя |
| 6 | **Docker-матрица: path-gate или advisory на PR** | −5–15 мин wall на типичном PR | Средний | Средняя |
| 7 | **Окно обновлений + авто-мерж зелёных minor/patch** | Оживляет dependency-контур | Средний | Средняя |
| 8 | **Решение по default branch / security-updates** | Определяет механику прихода security-фиксов | Средний | Решение |
| 9 | **Green origin/dev**: еженедельный синк | Честные статусы всех PR | Низкий | Процессная |
| 10 | **knip → nightly** | −1 джоба с PR | Низкий | Низкая |

Пояснения к кандидатам:

1. **Dependabot alerts** — включаются в Settings → Code security; бесплатны для приватных репо. Без них npm/docker-уязвимости вне PR невидимы. Шум настраивается, ложных блокировок не даёт (это только уведомления).
2. **concurrency** — группа `ci-${{ github.ref }}`, `cancel-in-progress` только для `pull_request` (выражением), чтобы не ронять деплой-прогоны.
3. **Консолидация**: nolint-gate + ts-suppressions + versions-freshness + migrations-lint → одна «gates»-джоба; openapi/sqlc freshness — шагами в backend-джобу (Go уже установлен); frontend-api-freshness — шагом во frontend-джобу. Состав проверок не меняется, меняется только упаковка.
4. **Path-filtering**: admin/admin-test — только `apps/admin/**`; landing — `apps/landing/**`; docker — Dockerfiles/compose/lockfiles. Обязательно: общие пути (бекенд-спека — в фильтры фронтовых джоб; изменения `.github/workflows/**` — «бежать всегда»).
5. **e2e**: минимальный вариант — перевести в advisory на PR (как knip) + полный nightly с уведомлением; продвинутый — смоук-подмножество на PR. Артефакты (скриншоты для сверки с Figma) сохраняются в обоих вариантах.
6. **Docker**: минимальный вариант — path-gate (собирать только при изменении образов/compose/lockfiles); продвинутый — перенести целиком в деплой-контур, где build+trivy уже есть. Компромисс: hadolint + compose-config оставить на PR (секунды), уйти тяжёлый build.
7. **Dependency-окно**: раз в неделю (понедельник уже настроен) разбирать очередь: ревью → зелёные minor/patch мержить (вручную или workflow-автоматом через `gh pr checks --watch`), major — руками в отдельном порядке. Automerge только minor/patch, никогда major, всегда с текущим cooldown.
8. **Default branch**: три варианта в разделе 3. Рекомендация из источников — whichever выбран, security-фиксы должны иметь предсказуемый маршрут; сейчас их нет вовсе.
9. **Green dev**: без свежего origin/dev не работают ни статусы PR, ни security-updates, ни «деплой-трейн». Минимальный ритуал — синк раз в неделю перед dependency-окном.
10. **knip** — advisory и на PR почти не даёт сигнала (дед-код редко меняется от PR к PR); nightly сохраняет функцию.

**Анти-кандидаты (что делать не надо):** отказываться от cosign/SBOM/trivy в деплое; снимать SHA-пины; убирать cooldown/группы; отключать govulncheck/gitleaks; переезжать на Kubernetes или вторую среду; делать prod-деплой «строже» через недоступные на Free механизмы.

**Связки:** #1+#7+#8+#9 лечат dependency-контур (единственный блок, который сейчас не работает совсем); #2–#6+#10 — скорость и биллинг CI. Ни один кандидат не ослабляет supply-chain-контур, который уже на уровне best practice.

### Ограничения исследования

- Оценки эффектов (проценты, «минуты») — из публичных кейсов и гайдов, не из замеров нашего репо: per-job таймингов наших прогонов я не собирал. Перед любым кандидатом из раздела 8 стоит снять фактические длительности джоб (GitHub usage report или логи прогонов) — это уточнит приоритеты #4–#6.
- Часть источников — vendor-блоги (WarpBuild, Blacksmith, HyperDX, runs-on): их цифры оптимистичны по определению; в отчёте они помечены или взяты с консервативной поправкой.
- Состояние GitHub-планов и функций (branch protection, environments, минимальные квоты) зафиксировано на 2026-09-14 и может меняться — перед реализацией сверяться с актуальными docs.
- Вопрос «Renovate или Dependabot» в отчёте намеренно оставлен открытым: оба варианта рабочие, выбор — за владельцем.

---

## 9. Источники

### CI-структура и скорость
- https://about.gitlab.com/topics/ci-cd/continuous-integration-best-practices/ — правило десяти минут Фаулера, ярусность
- https://hokstadconsulting.com/blog/ci-cd-feedback-loops-best-practices — feedback loop < 10 мин
- https://www.wondermentapps.com/blog/ci-cd-pipeline-best-practices/ — top-10 практик 2025
- https://github.com/dorny/paths-filter — job-level path filtering
- https://github.com/orgs/community/discussions/177835 — skip jobs в монорепо
- https://github.com/orgs/community/discussions/44490 — bucket/summary-job для required checks (нас не касается, но объясняет, почему в гайдах сложнее)
- https://github.com/orgs/community/discussions/26251 — required checks и path-фильтры
- https://www.codewithkarani.com/blog/monorepo-github-actions-path-filters-shared-code — ловушка общего кода
- https://warpbuild.com/blog/github-actions-monorepo-guide — affected-only, vendor-оценки 12×
- https://www.ranger.net/post/ultimate-guide-to-e2e-testing-in-ci-cd — e2e: smoke на PR, полный ночью
- https://www.reddit.com/r/ExperiencedDevs/comments/1pza4pr/at_what_point_do_you_run_e2e_tests/ — e2e nightly из практики
- https://docs.github.com/actions/reference/workflow-syntax-for-github-actions — concurrency/cancel-in-progress
- https://docs.github.com/en/actions/writing-workflows/choosing-what-your-workflow-does/control-the-concurrency-of-workflows-and-jobs — concurrency
- https://github.com/orgs/community/discussions/69704 — отмена только для PR
- https://github.com/actions/setup-go и https://runs-on.com/docs/guides/languages/go/ — кэши Go CI
- https://hyperdx.io/blog/docker-buildx-cache-with-github-actions — docker cache: 8→4 мин, ~1 мин тёплый
- https://docs.docker.com/build/cache/backends/gha/ — gha cache backend
- https://quanttype.net/p/weeknote-17-caching-docker-builds/ — лимит кэша 10 ГБ, registry cache
- https://docs.github.com/en/actions/reference/limits и https://docs.github.com/billing/managing-billing-for-github-actions/about-billing-for-github-actions — квота 2000 мин/мес Free, биллинг по джобам

### Dependabot / Renovate
- https://rafter.so/blog/renovate-vs-dependabot — сравнение 2026
- https://konvu.com/compare/dependabot-vs-renovate — zero config vs full control
- https://appsecsanta.com/sca-tools/dependabot-vs-renovate — сравнение 2026
- https://docs.renovatebot.com/bot-comparison/ — официальная матрица
- https://docs.renovatebot.com/key-concepts/automerge/ — automerge, lockFileMaintenance
- https://docs.renovatebot.com/configuration-options/ — automergeSchedule и пр.
- https://docs.renovatebot.com/mend-hosted/overview/ — бесплатный tier для приватных репо
- https://github.com/renovatebot/renovate/discussions/29327 — hosted app клонирует код (приватность)
- https://www.mend.io/mend-renovate-community/ — self-hosted CE бесплатно
- https://github.com/apps/renovate — Renovate GitHub App
- https://github.com/dependabot/dependabot-core/issues/2767 — security updates всегда в default branch
- https://stackoverflow.com/questions/72084723/change-target-branch-of-open-dependabot-pr — то же
- https://docs.github.com/code-security/dependabot/dependabot-security-updates/configuring-dependabot-security-updates — официальная докa
- https://docs.github.com/code-security/dependabot/dependabot-alerts/about-dependabot-alerts — про alerts
- https://github.com/orgs/community/discussions/171374 — alerts бесплатны для приватных репо
- https://stackoverflow.com/questions/72685861/auto-merge-dependabot-pr-after-all-checks-have-passed — automerge без branch protection
- https://docs.github.com/en/code-security/tutorials/secure-your-dependencies/automate-dependabot-with-actions — Automating Dependabot with Actions
- https://dev.to/nickytonline/let-dependabot-merge-its-own-prs-27pc — паттерн approve+auto-merge
- https://www.benhanzl.com/til/renovate-configuration/ — расписание «окон обновлений»
- https://www.reddit.com/r/github/comments/1tfjzp2/i_need_some_advice_on_managing_dependabot_branch/ — dev как default branch

### Supply-chain security
- https://security.googleblog.com/2023/04/supply-chain-security-for-go-part-1.html — govulncheck, reachability
- https://www.oligo.security/academy/ultimate-guide-to-software-supply-chain-security-in-2025 — гайд 2025
- https://www.veracode.com/blog/top-software-supply-chain-security-best-practices/ — essentials
- https://openssf.org/blog/2025/02/06/securing-public-sector-supply-chains-is-a-team-sport/ — OpenSSF/SLSA/cosign
- https://docs.semgrep.dev/deployment/managed-scanning/overview — semgrep: diff на PR, полный weekly
- https://appsec.guide/docs/static-analysis/semgrep/continuous-integration/ — semgrep CI-каденции
- https://docs.github.com/en/actions/reference/security/secure-use — SHA-пиннинг как immutable release, OIDC
- https://github.blog/changelog/2025-08-15-github-actions-policy-now-supports-blocking-and-sha-pinning-actions/ — org-политика SHA-пиннинга (авг 2025)
- https://www.buildmvpfast.com/blog/github-actions-supply-chain-security-hardening-guide-2026 — hardening checklist

### Деплой
- https://bunnyshell.com/blog/is-docker-compose-production-ready/ — compose для одного VPS норма
- https://blog.ploetzli.ch/2024/docker-deployment-best-practices/ — override-файлы окружений
- https://about.gitlab.com/blog/from-code-to-production-a-guide-to-continuous-deployment-with-gitlab/ — гибрид: stage авто, prod по тегу
- https://dev.to/srinivasamcjf/tag-based-deployment-in-jenkins-cicd-using-github-a-practical-guide-4em — тег как осознанный релиз
- https://hector-reyesaleman.medium.com/release-trains-vs-continuous-deployment-13015e7f89ff — release train vs CD
- https://glossary.deployment.to/release-train/ — release train
- https://www.atlassian.com/continuous-delivery/principles/continuous-integration-vs-delivery-vs-deployment — CD vs continuous delivery
- https://www.reddit.com/r/devops/comments/cypbhe/branches_merges_tags_deployments_environments/ — практики триггеров

### Ограничения GitHub Free (приватные репо)
- https://github.com/orgs/community/discussions/174400 — branch protection не бесплатна для приватных репо
- https://github.com/orgs/community/discussions/72725 — матрица планов для branch protection
- https://docs.github.com/actions/deployment/targeting-different-environments/using-environments-for-deployment — environment protection rules
- https://github.com/orgs/community/discussions/114537 — required reviewers требуют платный план на приватных репо
