# Pending-состояния кнопок и защита от двойного сабмита: внешние практики и приложение к нашему стеку

- Research-тикет: «Ресерч: лучшие решения в интернете — pending-состояния кнопок и защита от двойного сабмита» ([#1114](https://github.com/devnumbers/arenda-platform/issues/1114)), карта [#1112](https://github.com/devnumbers/arenda-platform/issues/1112).
- Дата: 2026-10-05. Все утверждения — из первоисточников (официальная документация React 19, TanStack Query v5, MDN/WAI-ARIA, GOV.UK Design System, Radix Themes, shadcn/ui, Stripe, RFC 9110, datatracker IETF), ссылка у каждого утверждения; мнения отдельных авторов (блог TkDodo) помечены как мнение. Факты ресерча #1113 (живые пробы) берутся как установленный контекст и не перепроверялись.
- Стек потребителя: React 19.2.4, Next.js App Router, TanStack Query `^5.51.0` (useMutation), hand-rolled формы поверх useState — form-библиотек нет, это зафиксированная позиция (`apps/frontend/package.json` — ни react-hook-form, ни react-aria-forms; `apps/frontend/CODING_STANDARDS.md`, раздел Forms: «No form library and no schema validator — this is deliberate, not a gap»), shadcn-канон по ADR 0050 (`docs/adr/0050-frontend-design-base-shadcn.md`).
- Размер задачи по нашим файлам (для оценок «цены внедрения» ниже): корни `apps/frontend/shared/ui/design/button.tsx:103` и `round-action-button.tsx:55`; 35 поверхностей с `loading={…isPending}` (45 `loading=` всего, без тестов); 14 feature-хуков с `useMutation` (`features/*/api/hooks.ts`); 30 вызовов `mutateAsync` в `widgets/`/`app/`; транспорт — один тонкий файл `apps/frontend/shared/api/client.ts`.
- Решение владельца по направлению (корневой фикс `disabled ?? loading` → `disabled || loading`) ресерчем не отменяется — ниже канон, подводные камни и механика пре-рендер гарда.

## Оглавление

1. [Резюме](#резюме)
2. [(1) Корневой дизейбл: канон и подводные камни](#1-корневой-дизейбл-канон-и-подводные-камни)
3. [Окно до перерисовки: почему disabled не помогает](#окно-до-перерисовки-почему-disabled-не-помогает)
4. [(2) Сравнение механизмов пре-рендер гарда](#2-сравнение-механизмов-пре-рендер-гарда)
5. [Рекомендация для тикетов исправлений](#рекомендация-для-тикетов-исправлений)
6. [Что не удалось подтвердить по первоисточникам](#что-не-удалось-подтвердить-по-первоисточникам)
7. [Источники](#источники)

---

## Резюме

1. **«loading влечёт disabled» — канон индустрии, а не наша самодеятельность.** Radix Themes: `loading` — «The button will be disabled while loading» ([docs](https://www.radix-ui.com/themes/docs/components/button)); React-доки в канонических примерах учат `disabled={pending}` ([useFormStatus](https://react.dev/reference/react-dom/hooks/useFormStatus)); react-hook-form: `isSubmitting` — «true if the form is currently being submitted» ([docs](https://react-hook-form.com/docs/useform/formstate)); TkDodo: гасить сабмит-кнопку pending-состоянием мутации ([React Query and Forms](https://tkdodo.eu/blog/react-query-and-forms)). При этом в upstream shadcn/ui у `Button` пропа `loading` нет вообще — это локальное расширение репозитория, и его семантику мы определяем сами ([исходник button.tsx](https://github.com/shadcn-ui/ui/blob/main/apps/v4/registry/new-york-v4/ui/button.tsx)). Канон ADR 0050 («loading — кнопка дизейблится и приглушается») совпадает с индустрией; баг — только в операторе: `disabled ?? loading` даёт явному `disabled={false}`-выражению право побить loading, `disabled || loading` — инвариант «в полёте глохнет всегда», как у Radix Themes.
2. **Подводный камень корневого дизейбла ровно один, и он не про наш вред:** нативный `disabled` подавляет функциональность и выводит элемент из tab-порядка ([MDN, сравнение с aria-disabled](https://developer.mozilla.org/en-US/docs/Web/Accessibility/ARIA/Attributes/aria-disabled)), а GOV.UK прямо просит избегать задизейбленных кнопок («Disabled buttons have poor contrast and can confuse some users, so avoid them if possible» — [Button](https://design-system.service.gov.uk/components/button/)). Для короткого submit-полёта это принятая в индустрии цена; для «долго неактивной» кнопки инструментарий другой — `aria-disabled` + ручное подавление. Indастрия отделяет submit-pending от background-pending раздельными пропсами (`loading` и `disabled` — независимы в Radix Themes); фоновые рефетчи (isFetching запроса) в `loading` кнопки не подставляются. Наши retry-кнопки («Повторить») этому канону уже соответствуют: `loading` только на рефетч, после ошибки кнопка жива ([PropertiesErrorState.tsx](/Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend/widgets/properties/ui/PropertiesErrorState.tsx)) — корневой фикс их не ломает.
3. **Окно до перерисовки — не баг кнопки, а механика React:** «React waits until *all* code in the event handlers has run before processing your state updates»; «After the event handler completes, React will trigger a re-render» ([Queueing state updates](https://react.dev/learn/queueing-a-series-of-state-updates)). Два клика, чьи хендлеры отработали в одном JS-таске, видят один и тот же DOM — любой `disabled` беспомощен. Ни React, ни TanStack Query машинной защиты от этого не дают: TanStack «Per default, all mutations run in parallel — even if you invoke `.mutate()` of the same mutation multiple times» ([Mutations guide](https://tanstack.com/query/latest/docs/framework/react/guides/mutations)), а React form actions дубликаты не отбрасывают, а ставят в очередь («React queues and executes multiple calls to `dispatchAction` sequentially» — [useActionState](https://react.dev/reference/react/useActionState)).
4. **Каноническая последняя линия — серверная идемпотентность** (Stripe Idempotency-Key: сохранение результата первой попытки, сравнение параметров, 24 часа — [docs](https://docs.stripe.com/api/idempotent_requests)); общесетевого стандарта нет: IETF draft `draft-ietf-httpapi-idempotency-key-header` — expired (v07, 2025-10-15) ([datatracker](https://datatracker.ietf.org/doc/draft-ietf-httpapi-idempotency-key-header/)), RFC 9110 лишь определяет идемпотентность методов и запрещает автоматический ретрай неидемпотентных без средств убедиться («A client SHOULD NOT automatically retry a request with a non-idempotent method unless…» — [§9.2.2](https://www.rfc-editor.org/rfc/rfc9110.txt)).
5. **Рекомендация (детали в разделе 5):** (а) корневой фикс `disabled || loading` в обоих компонентах; (б) единая обёртка feature-мутаций с синхронным гвардом «одна полётная мутация» в `features/*/api/hooks.ts` — одна точка вместо ручных `startSendOnce` на 35 поверхностей; (в) серверные идемпотентные ключи на creations — отдельным тикетом как страховка. Не делать: дедуп POST на уровне apiClient (против высказан мейнтейнером TanStack; наш мультиинвайт — живой контрпример) и миграцию на form actions ради защиты (её там нет).

---

## (1) Корневой дизейбл: канон и подводные камни

### 1.1. Канон: pending гасит кнопку

- **Radix Themes** (база нашего shadcn-канона): проп `loading` — «display a loading spinner in place of button content… The button will be disabled while loading»; `loading` и `disabled` — независимые пропсы, `loading` влечёт disabled ([docs](https://www.radix-ui.com/themes/docs/components/button)). Это точная модель «disabled ∪ loading»: активен, пока не глохнет по любой из причин.
- **React 19**: канонический пример `useFormStatus` — `<button type="submit" disabled={pending}>` ([react.dev](https://react.dev/reference/react-dom/hooks/useFormStatus)); то же в анонсе React 19 ([blog](https://react.dev/blog/2024/12/05/react-19)). Обратите внимание: у DOM-хука поле называется `pending` (не `isPending`; `isPending` — у `useActionState`: «The `isPending` flag that tells you if any dispatched Actions for this Hook are pending» — [docs](https://react.dev/reference/react/useActionState)).
- **TanStack Query v5**: «`isPending` or `status === 'pending'` — The mutation is currently running» ([Mutations guide](https://tanstack.com/query/latest/docs/framework/react/guides/mutations)); TkDodo (мнение): «you can use the isLoading prop returned from useMutation» → `disabled` на время мутации ([React Query and Forms](https://tkdodo.eu/blog/react-query-and-forms)).
- **Form-библиотеки**: react-hook-form `formState.isSubmitting` — «true if the form is currently being submitted. false otherwise» ([docs](https://react-hook-form.com/docs/useform/formstate)) — та же модель: один флаг полёта, кнопка глохнет от него.
- **shadcn/ui upstream**: у `Button` пропа `loading` нет вовсе; `disabled` пробрасывается как есть, стили `disabled:pointer-events-none disabled:opacity-50` ([button.tsx, new-york-v4](https://github.com/shadcn-ui/ui/blob/main/apps/v4/registry/new-york-v4/ui/button.tsx)). Значит, наш `loading` — локальная семантика, и ADR 0050 («loading — кнопка дизейблится и приглушается», docblock `button.tsx`) — единственный источник истины по ней. Баг `disabled ?? loading` — нарушение собственного канона, а не расхождение с внешним.

### 1.2. Подводные камни «loading всегда диспейблит» и как их отделяют

- **a11y/focus.** Нативный `disabled` «suppress[es] all functionality along with disallowing the element's value from participating in form submission»; в отличие от него `aria-disabled="true"` «only semantically exposes these elements as being disabled» и «does not change the focusability of such elements» — остаётся в tab-порядке, но функциональность нужно глушить вручную (`pointer-events: none` не блокирует клавиатурную активацию) ([MDN aria-disabled](https://developer.mozilla.org/en-US/docs/Web/Accessibility/ARIA/Attributes/aria-disabled)). GOV.UK Design System просит избегать задизейбленных кнопок в принципе: «Disabled buttons have poor contrast and can confuse some users, so avoid them if possible. Only use disabled buttons if research shows it makes the user interface easier to understand» ([Button](https://design-system.service.gov.uk/components/button/)). Отсюда разделение практик: **короткий submit-полёт — нативный `disabled`** (цена: кратковременный уход фокуса — прямо это поведение в react.dev/MDN отдельно не документировано, см. раздел 6); **длительно неактивная кнопка с причиной — `aria-disabled` + сохранение фокусируемости** (пользователь должен уметь её найти и прочитать). Для нашего полёта мутации канон — `disabled`, при этом `aria-busy={loading}` у нас уже выставлен в обоих компонентах (`button.tsx:101`, `round-action-button.tsx:54`) — по канону `aria-busy` это «an element is currently being modified», сигнал AT подождать с анонсами ([MDN aria-busy](https://developer.mozilla.org/en-US/docs/Web/Accessibility/ARIA/Attributes/aria-busy)).
- **Кнопки «Повторить».** Правило: глушить только в полёте; после ошибки кнопка обязана ожить (иначе пользователь заперт). Наши error-state-кнопки уже так устроены: `loading` приходит от рефетча и падает в `false` вместе с концом попытки ([PropertiesErrorState.tsx](/Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend/widgets/properties/ui/PropertiesErrorState.tsx), [PropertyDetailError.tsx](/Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend/widgets/property-detail/ui/PropertyDetailError.tsx)). Корневой фикс ничего в них не меняет: у них нет явного `disabled`, а «loading → погашена → ожила» — ровно целевое поведение. Отдельный grep по `loading={…isFetching|isLoading|refetch}` в наших виджетах/фичах других кандидатов на «застревание» не нашёл — риск регрессии от `||` локализован auth-степами и формами, где явный `disabled={валидность}` сегодня побивает loading (`PhoneStep.tsx:69`, `EmailStep.tsx:69`) — это и есть дыра #1113.
- **Optimistic UI и тумблеры.** React-доки для optimistic-сценариев дают `useOptimistic` («show a temporary value while an Action is in progress», откат при ошибке — [docs](https://react.dev/reference/react/useOptimistic)) — и их собственные примеры при этом всё равно гасят контрол на время экшена (`disabled={isPending}`, `disabled={item.deleting}` с приглушением строки). TkDodo (мнение) про «double or even triple click» по лагающим тумблерам: мгновенный фидбек или disabled+loading, а не повторные отправки ([Mastering Mutations](https://tkdodo.eu/blog/mastering-mutations-in-react-query)). Расхождение внутри индустрии только в том, *чем* гасить тумблер (optimistic-переключение вместо паузы), но не в том, что повторный фаер одной и той же записи надо исключать. Наш вред (#1113: только creations без дедупа) лежит ровно в «дизейблить submit»-категории.
- **Submit-pending vs background-pending.** Разделение делается источником флага и/или раздельными пропсами: у Radix Themes `loading` и `disabled` — независимые пропсы ([docs](https://www.radix-ui.com/themes/docs/components/button)); в react-query v5 семантически разные флаги: `isPending` мутации (полёт записи) vs `isFetching`/`isRefetching` запроса (фоновое перечитывание) — второй в loading-кнопку подставлять нельзя, фоновый рефетч не должен гасить кнопки (у нас `staleTime: 30_000` и `refetchOnWindowFocus: false` — фоновых перечитываний мало, `CODING_STANDARDS.md`, react-query conventions). Итог: инвариант «loading гасит» безопасен при одном условии — в `loading` попадает только `isPending` мутаций, что в нашем коде и происходит (35 поверхностей, см. сводку выше).

---

## Окно до перерисовки: почему disabled не помогает

Механика того самого окна, официально:

- «React waits until *all* code in the event handlers has run before processing your state updates»; «After the event handler completes, React will trigger a re-render. During the re-render, React will process the queue»; «React processes state updates after event handlers have finished running. This is called batching» ([react.dev, Queueing a series of state updates](https://react.dev/learn/queueing-a-series-of-state-updates)).
- При этом «React does not batch across *multiple* intentional events like clicks — each click is handled separately» (там же). Для дискретных пользовательских кликов это даёт честное окно только между тасками: клик → хендлер → (после таска) ререндер → кнопка погашена. Но два `click()`, вызванных одним JS-таском (программно, из одного хендлера/скрипта), — это один таск: оба хендлера выполнятся до единого ререндера, второй увидит «живую» кнопку. Что и зафиксировано живыми пробами #1113. Отсюда вывод для дизайна защиты: **UI-атрибуты (`disabled`) по построению не закрывают это окно — нужен синхронный флаг до/вне рендера** (реф/лок в слое мутации), либо серверная идемпотентность.

---

## (2) Сравнение механизмов пре-рендер гарда

### 2.0. Что официально гарантировано (и что нет)

- **TanStack Query v5 не дедуплицирует мутации — намеренно.** «Per default, all mutations run in parallel — even if you invoke `.mutate()` of the same mutation multiple times» ([Mutations guide](https://tanstack.com/query/latest/docs/framework/react/guides/mutations)). Мейнтейнер TkDodo (мнение, [GH Discussion #4131](https://github.com/TanStack/query/discussions/4131)): «there is no easy way to detect if mutations are 'the same'. variables can change etc, and sometimes, it is okay to fire the same mutation twice» (пример: две одинаковые задачи «сходить за покупками» — валидны); дедуп он допускает только точечно, на слое персистенции офлайн-очереди. Для контраста, дедуп *запросов* — заявленная ценность библиотеки: «Deduping multiple requests for the same data into a single request» ([Overview](https://tanstack.com/query/latest/docs/framework/react/overview)).
- **`scope.id` — серийность, не дедуп.** Официально: «All mutations with the same `scope.id` will run in serial» ([Mutations guide](https://tanstack.com/query/latest/docs/framework/react/guides/mutations); RFC-предложение — [GH #7126](https://github.com/TanStack/query/discussions/7126)). Вторая мутация того же scope не отбрасывается — она уйдёт *после* первой. От двойного сабмита creations это не спасает (получим два последовательных POST и второй 201), поэтому как защита непригодна; полезна как гарантия порядка (у нас таких кейсов сейчас нет).
- **React form actions двойной сабмит не блокируют.** Официальные доки не заявляют автоматической защиты: для pending-состояния учат ставить `disabled={pending}` вручную ([useFormStatus](https://react.dev/reference/react-dom/hooks/useFormStatus), [<form>](https://react.dev/reference/react-dom/components/form), [React 19 blog](https://react.dev/blog/2024/12/05/react-19)). Более того, `useActionState` дубликаты **ставит в очередь**: «React queues and executes multiple calls to `dispatchAction` sequentially. Each call to `reducerAction` receives the result of the previous call»; «If `dispatchAction` is called multiple times, React queues and executes them in order…»; при ошибке «React cancels all queued actions»; отмена очереди через AbortController возможна, но «Aborting an Action isn't always safe» ([useActionState](https://react.dev/reference/react/useActionState)). Итог: миграция на form actions не убирает двойной POST — второй уйдёт следом за первым.
- **`mutate` vs `mutateAsync`** (сопутствующее, влияет на форму обёртки): TkDodo (мнение): `mutate` = «literally implemented with: *mutateAsync().catch(noop)*», а с `mutateAsync` «you also have to catch errors manually, or you might get an unhandled promise rejection»; `mutateAsync` оправдан, когда нужен Promise (цепочки/параллельные ожидания) ([Mastering Mutations](https://tkdodo.eu/blog/mastering-mutations-in-react-query)). У нас 30 `mutateAsync` в виджетах — легитимно там, где нужен результат (наш визард ждёт `payment` для success-экрана и оборачивает в try/catch, [payment-create-wizard-flow.tsx](/Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend/widgets/payments/ui/payment-create-wizard/payment-create-wizard-flow.tsx)); обёртка должна работать с обоими спеллингами. Ещё два официальных факта о per-call колбэках: «they will be fired up only *once* and only if the component is still mounted» ([guide](https://tanstack.com/query/latest/docs/framework/react/guides/mutations)); «per-call callbacks fire only for the latest call you've made, and only while the component is still mounted» ([reference](https://tanstack.com/query/latest/docs/framework/react/reference/useMutation)) — т.е. `onSuccess: close` в `mutate(draft, {…})` при погашении дубля не сработает лишний раз, если дубль отброшен до вызова `mutate`.

### 2.1. Вариант A: общая обёртка мутации (одна точка на все поверхности)

**Суть.** Общий хук поверх `useMutation` (например `useGuardedMutation` в `shared/` — «hooks not tied to a feature» там разрешены), который: держит **синхронный** лок «одна полётная мутация» (ref, ставится до вызова `mutationFn`, снимается в `onSettled`) и повторный вызов отбрасывает (drop, first wins); наружу отдаёт обычный `UseMutationResult` с честным `isPending` для кнопки. Подключается в 14 feature-хуков (`features/*/api/hooks.ts` — канон размещения уже централизует все мутации, `CODING_STANDARDS.md`).

- **Канонических реализаций в экосистеме нет**: TanStack готового гварда не предоставляет (см. 2.0; `useMutationState` — «a hook that gives you access to all mutations in the `MutationCache`» — даёт *наблюдение*, [reference](https://tanstack.com/query/latest/docs/framework/react/reference/useMutationState), но подписка асинхронна относительно рендера и окно одного таска не закрывает). Это будет наш локальный канон — тот же `startSendOnce` из #1099, поднятый из «ручного труда на каждой поверхности» в одну реализацию.
- **Цена внедрения у нас**: одна новая реализация + правка 14 хуков (механическая: обернуть `useMutation`); вызывные места и кнопки не меняются. Ниже, чем 35 ручных рефов, и не забывается в новых поверхностях.
- **Риски**: drop-семантика должна применяться только там, где повтор в полёте не имеет смысла — для всех наших поверхностей это так (одна форма = одна полётная запись); если когда-нибудь появится легитимный «второй такой же POST из той же формы» (мультиинвайт из одного экрана), обёртку нужно уметь выключать per-hook (флаг). Синхронность лока — принципиальна: `isPending`-проверка через реактовский стейт тот же таск не проходит (см. раздел «Окно»).
- **Кто рекомендует похожее**: TkDodo (мнение) — гасить кнопку pending-ом ([React Query and Forms](https://tkdodo.eu/blog/react-query-and-forms)); обёртка не заменяет это, а добавляет машинный слой под ним.

### 2.2. Вариант B: дедуп одинаковых POST на уровне apiClient

**Суть.** В `shared/api/client.ts` (один тонкий fetch-обёрточ, [client.ts](/Users/smirnowwwivan/Nambers/arenda-planform/apps/frontend/shared/api/client.ts)) — Map in-flight промисов по ключу `method+path+body`.

- **Официальной рекомендации так делать нет.** В доках TanStack Query дедуп заявлен только для запросов по ключу ([Overview](https://tanstack.com/query/latest/docs/framework/react/overview)); мутации намеренно не дедуплицируются, и мейнтейнер формулирует против слепого дедупа: «sometimes, it is okay to fire the same mutation twice» ([GH #4131](https://github.com/TanStack/query/discussions/4131)). Для fetch-слоя (стандарт `fetch`, MDN) аналогичного механизма не существует.
- **Цена внедрения у нас** — самая низкая (один файл), **риск — концептуальный и неприемлемый**: ключ `method+path+body` схлопнет и легитимные параллельные одинаковые мутации. Живой пример из нашего кода — мультиинвайт (#1113): два одинаковых POST инвайта с равными телами — ожидаемое поведение, а не дубль. Плюс дедуп на клиенте не ловит повторный сабмит *после* завершения первого (F5, повторный клик через секунду) и не покрывает ретраи сети — т.е. не решает задачу полностью даже ценой риска.
- **Вывод**: не рекомендовать как общий механизм; если когда-нибудь понадобится точечный дедуп — он правильнее живёт в конкретном feature-хуке, где известна семантика конкретного эндпоинта.

### 2.3. Вариант C: серверные идемпотентные ключи (последняя линия)

**Суть.** Клиент генерирует ключ на каждую операцию создания; бекенд сохраняет результат первой попытки и на повтор возвращает его же.

- **Канон — Stripe**: «use an idempotency key… if a connection error occurs, you can safely repeat the request without risk of creating a second object»; «Stripe's idempotency works by saving the resulting status code and body of the first request made for any given idempotency key, regardless of whether it succeeds or fails. Subsequent requests with the same key return the same result, including `500` errors»; ключи — V4 UUID, хранятся ≥24 часов, повтор с тем же ключом и *другими* параметрами — ошибка; «All `POST` requests accept idempotency keys» ([Idempotent requests](https://docs.stripe.com/api/idempotent_requests)).
- **Статус стандарта**: RFC 9110 определяет идемпотентность методов и прямо предостерегает от авто-ретрая неидемпотентных: «A client SHOULD NOT automatically retry a request with a non-idempotent method unless it has some means to know that the request semantics are actually idempotent… or some means to detect that the original request was never applied» ([§9.2.2](https://www.rfc-editor.org/rfc/rfc9110.txt)). HTTP-заголовок `Idempotency-Key` стандарта не имеет: IETF draft `draft-ietf-httpapi-idempotency-key-header` — **Expired Internet-Draft** (v07 от 2025-10-15; абстракт: заголовок «can be used to make non-idempotent HTTP methods such as POST or PATCH fault-tolerant») ([datatracker](https://datatracker.ietf.org/doc/draft-ietf-httpapi-idempotency-key-header/)); варианта с суффиксом `-status`, названного в тикете, в datatracker нет («No documents match your query»). Т.е. делаем по де-факто конвенции (Stripe), а не по стандарту.
- **Цена внедрения у нас**: бекенд — middleware + хранение «ключ → результат» (TTL) на creations (наш перечень вреда #1113: `POST /properties`, `…/payments`, `…/operations`, `…/tasks/rules`); клиент — генерация ключа в feature-хуках на creations. Зато это единственный механизм, ловящий *все* источники дубля: двойной клик в окне до перерисовки, повторный клик после, F5, ретраи сети, гонки инстансов. Именно поэтому «последняя линия».
- **Риски**: TTL против длинных офлайн-окон (у Stripe 24 часа — де-факто прикидка); согласование «тот же ключ = тот же результат» с нашими шумовыми тостами (409/404 на set-семантике — отдельная тема #1113, сюда не входит).

### 2.4. Вариант D: React 19 form actions (`<form action>` + `useFormStatus`/`useActionState`)

- Защиты от двойного сабмита **не дают** (см. 2.0: ручной `disabled={pending}`; очередь дубликатов, а не отбрасывание — [useActionState](https://react.dev/reference/react/useActionState)). Побочный бонус очереди — второй POST хотя бы не параллельный; для creations это всё равно дубль.
- **Цена внедрения у нас — высокая**: почти все наши submit-поверхности — кнопки с `onClick` вне `<form>` (hand-rolled формы по канону `CODING_STANDARDS.md`: «Button-submit forms: controlled `useState` fields + … derived `canSubmit`»), `useFormStatus` работает только из компонента внутри `<form>` («must be called from a component that is rendered inside a `<form>`» — [docs](https://react.dev/reference/react-dom/hooks/useFormStatus)); плюс `<form>`-экшн сбрасывает неконтролируемые поля («After the `action` function succeeds, all uncontrolled field elements in the form are reset» — [<form>](https://react.dev/reference/react-dom/components/form)) — несовместимо с нашими draft-store-визардами без отдельной работы.
- **Вывод**: миграция ради этого гварда не оправдана — не рекомендовать.

---

## Рекомендация для тикетов исправлений

1. **Корневой фикс `disabled || loading`** в `shared/ui/design/button.tsx:103` и `round-action-button.tsx:55`. Обоснование: инвариант «в полёте глохнет всегда» — канон Radix Themes (loading влечёт disabled) и совпадает с нашим же ADR 0050; `??` нарушает его при каждом явном `disabled={…}`-выражении (у нас таких большинство). Регрессий не ожидается: retry-поверхности не задеты (loading только на рефетч), «мягких» `loading`-кнопок в дереве нет (grep см. в сводке). Это фикс дыры (1) на всех 35 поверхностях одной правкой двух файлов.
2. **Единая обёртка feature-мутаций с синхронным локом** (вариант A) — машинный гвард дыры (2) в одной точке: `useGuardedMutation` в `shared/` + подключение в 14 feature-хуков; drop «повтора в полёте», лок снимается в `onSettled`, флаг-выключатель per-hook на случай будущих легитимных параллельных одинаковых мутаций (мультиинвайт). Это канонизация `startSendOnce` из #1099 без ручного труда на поверхностях; кнопки с `disabled || loading` остаются видимым UX-слоем (причина погашения), лок — машинным (окно до перерисовки).
3. **Серверные идемпотентные ключи на creations** (вариант C) — отдельным тикетом по бекенду, как последняя линия: покрывает то, что не ловит клиент (повторы после окна, ретраи, перезагрузки). Паттерн — Stripe; стандарта нет (draft expired) — фиксируем это в ADR/доках бекенда, чтобы не сослались на несуществующий RFC.
4. **Не делать**: дедуп POST в apiClient (вариант B) — против позиции мейнтейнера TanStack и наш мультиинвайт; миграцию на form actions (вариант D) — защиты там нет, цена при hand-rolled формах высокая.

---

## Что не удалось подтвердить по первоисточникам

Честный список границ ресерча (без выдумок):

- **Автоматического запрета двойного сабмита в React form actions нет нигде** — это не «не нашли», а подтверждённое отсутствие: доки учат гасить кнопку вручную и описывают очередь экшенов (раздел 2.0, 2.4).
- **Официальной рекомендации дедупить POST в fetch-слое не существует**; позиция мейнтейнера TanStack — мнение из GH-дискуссии, но оно единственное авторитетное высказывание по теме, которое удалось найти.
- **Потеря фокуса задизейбленной кнопки**: прямого нормативного текста в react.dev/MDN именно про «фокус уходит на body» найти не удалось (MDN-страницы атрибута `disabled` и экспертные a11y-материалы были недоступны на момент ресерча — timeouts). Подтверждаемое смежное: HTML `disabled` «suppress[es] all functionality», а `aria-disabled` «does not change the focusability» ([MDN](https://developer.mozilla.org/en-US/docs/Web/Accessibility/ARIA/Attributes/aria-disabled)) — т.е. фокусируемость при `disabled` снимается; следствие для UX (пользователь с клавиатуры теряет контекст) — общеизвестная a11y-критика (в духе GOV.UK «can confuse some users»), но без прямой цитаты утверждать сильнее не будем.
- **Стандарта на `Idempotency-Key` нет** — draft expired; название `draft-ietf-httpapi-idempotency-key-header-status` из тикета в datatracker не резолвится (проверено поиском: «No documents match your query»).
- Next.js-доки (guides/forms) на момент ресерча стабильно недоступны по сети — Next-специфику не цитировал; всё, что относится к form actions, взято из react.dev (Server Actions в App Router — это React form actions).

---

## Источники

**React (официальные доки, 19.x):**
- useFormStatus — <https://react.dev/reference/react-dom/hooks/useFormStatus> (поле `pending`; канонический `disabled={pending}`; требование рендериться внутри `<form>`)
- useActionState — <https://react.dev/reference/react/useActionState> (`isPending`; очередь `dispatchAction`; отмена очереди при ошибке; «Aborting an Action isn't always safe»)
- `<form>` (action) — <https://react.dev/reference/react-dom/components/form> (action в Transition; сброс неконтролируемых полей после успеха)
- Queueing a series of state updates (батчинг) — <https://react.dev/learn/queueing-a-series-of-state-updates>
- useOptimistic — <https://react.dev/reference/react/useOptimistic>
- React 19 announcement (Actions) — <https://react.dev/blog/2024/12/05/react-19>

**TanStack Query v5 (официальные доки + позиция мейнтейнера):**
- Mutations guide — <https://tanstack.com/query/latest/docs/framework/react/guides/mutations> (`isPending`; «all mutations run in parallel»; `scope.id` серийность; `retry: false` по умолчанию)
- useMutation reference — <https://tanstack.com/query/latest/docs/framework/react/reference/useMutation>
- useMutationState reference — <https://tanstack.com/query/latest/docs/framework/react/reference/useMutationState>
- Overview (дедуп запросов) — <https://tanstack.com/query/latest/docs/framework/react/overview>
- GH Discussion #4131 «Deduplicate identical mutations» (TkDodo) — <https://github.com/TanStack/query/discussions/4131>
- GH Discussion #7126 «RFC: Scoped Mutations» — <https://github.com/TanStack/query/discussions/7126>

**Блог TkDodo (мнение мейнтейнера, не канон):**
- React Query and Forms — <https://tkdodo.eu/blog/react-query-and-forms>
- Mastering Mutations in React Query — <https://tkdodo.eu/blog/mastering-mutations-in-react-query>

**Доступность (первоисточники спецификаций/доков):**
- MDN, aria-disabled — <https://developer.mozilla.org/en-US/docs/Web/Accessibility/ARIA/Attributes/aria-disabled>
- MDN, aria-busy — <https://developer.mozilla.org/en-US/docs/Web/Accessibility/ARIA/Attributes/aria-busy>
- GOV.UK Design System, Button — <https://design-system.service.gov.uk/components/button/>

**Дизайн-системы:**
- Radix Themes, Button (`loading` влечёт disabled) — <https://www.radix-ui.com/themes/docs/components/button>
- shadcn/ui Button (upstream, нет `loading`) — <https://github.com/shadcn-ui/ui/blob/main/apps/v4/registry/new-york-v4/ui/button.tsx>

**Идемпотентность:**
- Stripe, Idempotent requests — <https://docs.stripe.com/api/idempotent_requests>
- RFC 9110, §9.2.2 Idempotent Methods — <https://www.rfc-editor.org/rfc/rfc9110.txt>
- IETF draft-ietf-httpapi-idempotency-key-header (Expired, v07 2025-10-15) — <https://datatracker.ietf.org/doc/draft-ietf-httpapi-idempotency-key-header/>

**React Hook Form:**
- formState.isSubmitting — <https://react-hook-form.com/docs/useform/formstate>

**Наш код/доки (оценка цены внедрения):**
- `apps/frontend/shared/ui/design/button.tsx` (строка 103: `disabled ?? loading`), `apps/frontend/shared/ui/design/round-action-button.tsx` (строка 55)
- `apps/frontend/widgets/tasks/ui/task-create-screen.tsx` (строка 272), `apps/frontend/widgets/payments/ui/payment-create-wizard/payment-create-wizard-flow.tsx` (строки 289–294), `apps/frontend/features/auth/ui/phone-step/PhoneStep.tsx` (строка 69), `apps/frontend/widgets/properties/ui/PropertiesErrorState.tsx`
- `apps/frontend/shared/api/client.ts`; `apps/frontend/CODING_STANDARDS.md` (Forms, react-query conventions); `docs/adr/0050-frontend-design-base-shadcn.md`; `apps/frontend/package.json`
