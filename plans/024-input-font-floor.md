# 024 — Гарантия 16px: флоор на токене + поиск с coarse-указателем

- **Status**: TODO
- **Commit**: 6ea8bc4ed
- **Severity**: MEDIUM
- **Category**: mobile-native §4 / Never Ship («page zooms into input»; флоор 16px вместо запрета зума); карта #1416 / тикет #1421
- **Estimated scope**: 3 файла (globals.css, tokens.css, search-field.tsx), один базовый блок + один класс

## Problem

Токен `--font-size-input: 1rem` объявлен (`shared/styles/tokens.css:59`,
комментарий там же обещает, что именно он держит гарантию ≥16px), но не
потребляется нигде — единственная ссылка на него это комментарий в
`app/layout.tsx:52`. Гарантия фактически держится на per-component
`text-base`: `text-field.tsx:138,166`, `amount-field.tsx` (свои классы).
Базовый слой тем временем держит легаси-нормализацию `font-size: 100%`
(`globals.css:1038-1046`) — флоора нет: нативный контрол без явных классов
наследует родителя и может уехать ниже 16px.

Проветрено по месту и исключение из «уже в каноне: инпуты ≥16px»:
`search-field.tsx:40` держит нативный инпут на `text-sm` (14px) — iOS Safari
зумит страницу при фокусе и не отъезжает на блюре. Поиск живёт на мобильных
поверхностях (поиск адреса, поиски списков) — реальный зум-кейс. Канон §4
прямо: дизайн-кегль меньше 16 поднимается только там, где важно —
`@media (pointer: coarse) { input { font-size: 16px; } }`.

## Target

1. Флоор в базовом слое — сразу после легаси-нормализации контролов
   (внутрь существующего `@layer base`, начатого на `globals.css:1027`):

```css
/* apps/frontend/app/globals.css — target (после блока button,input… из
   легаси-сброса) */
/* Флоор 16px нативных контролов (канон mobile-native §4; тикет #1421):
   iOS зумит страницу при фокусе в инпуте <16px и не отъезжает на блюре.
   Источник значения — --font-size-input (tokens.css); поля с явными
   утилитами его переопределяют по слою, флоор ловит только контролы без
   явного кегля. */
    input,
    select,
    textarea {
        font-size: var(--font-size-input);
    }
```

2. Поиск: на coarse-указателях инпут поднимается до флоора, дизайн-кегль
   14/16 остаётся мышам и тачпадам:

```tsx
// apps/frontend/shared/ui/design/search-field.tsx:40 — target
<input
    type="search"
    className="min-w-0 flex-1 border-none bg-transparent text-sm leading-4 text-content outline-none placeholder:text-content-tertiary [@media(pointer:coarse)]:text-base [@media(pointer:coarse)]:leading-5 [&::-webkit-search-cancel-button]:hidden"
```

3. Комментарий токена (`tokens.css:56-60`) дополнить вторым предложением:

```css
/* Form controls. Must be ≥16px: iOS Safari auto-zooms the page on focus
   when an <input>/<textarea> has font-size < 16px. See layout.tsx —
   maximumScale is intentionally omitted for WCAG 1.4.4, so the font-size
   guard is what prevents the zoom. Consumed by the base-layer floor in
   globals.css (component utilities may override it per-layer). */
```

## Repo conventions to follow

- Значение — только из токена; литерал `16px`/`1rem` в флооре не писать.
- Флоор — в `@layer base` после легаси-блока: та же специфичность, позже по
  файлу — выигрывает у `font-size: 100%` нормализации, проигрывает утилитам
  (слой utilities сильнее base — так уже устроен файл, комментарий над
  легаси-блоком).
- `button` во флоор не входит: кнопки не зумят страницу, кегли кнопок —
  дизайн-решение.

## Steps

1. `globals.css`: добавить флоор внутрь `@layer base` после легаси-блока
   контролов (Target п.1).
2. `search-field.tsx:40`: добавить два coarse-варианта (Target п.2).
3. `tokens.css`: дополнить комментарий токена (Target п.3).

## Boundaries

- Do NOT трогать `text-field.tsx` и `amount-field.tsx` — их `text-base` уже
  равен токену; дублировать токен в классах компонентов не нужно.
- Do NOT поднимать дизайн-кегль поиска для fine-указателей — 14/16 остаётся
  десктопу (Figma 706:12561).
- Do NOT запрещать зум (`maximum-scale`) — WCAG 1.4.4, комментарий
  `layout.tsx:50-53`.
- Do NOT переносить легаси-блок нормализации — только добавить после.
- `globals.css` правят ещё планы 012–014 (аудит анимаций) — не исполнять
  параллельно с ними; секция добавляется в конец существующего `@layer base`,
  не в чужие блоки.
- Если легаси-блок контролов не на `:1038-1046` (дрейф) — СТОП и отчёт.

## Verification

- **Mechanical**: `npx tsc --noEmit`, `npm run lint`, `make frontend-test` —
  зелёные; в собранном CSS присутствует правило `input,select,textarea{font-size:var(--font-size-input)}`
  (grep по `.next`-сборке или stylelint-выводу); поиск на десктоп-вьюпорте
  остаётся 14px, при `pointer: coarse` — 16px (devtools-эмуляция media).
- **Feel check** (iPhone, Safari): фокус в поиске (адрес объекта, поиск
  списков) — страница НЕ зумит; поля форм — не зумит (и раньше не зумили).
- **Done when**: флоор стоит на токене; поиск на таче 16px; unit/e2e без
  красных.
