# 022 — Клавиатура: `enterKeyHint` и автозаполнение OTP-кода

- **Status**: TODO
- **Commit**: 6ea8bc4ed
- **Severity**: MEDIUM
- **Category**: mobile-native §4 (раскладка клавиатуры: `enterkeyhint`, автозаполнение кода из SMS); карта #1416 / тикет #1421
- **Estimated scope**: 7 файлов, ~15 атрибутов на существующих элементах

## Problem

`enterKeyHint` не использован во всём приложении (grep по `apps/frontend` —
ноль), а кодовые поля не помечены `autoComplete="one-time-code"` — iOS не
предлагает автозаполнение кода из SMS/почты, клавиша Return везде безымянная
«Ввод». Канон §4: «set the keyboard: … `enterkeyhint="send"` / `"search"` /
`"done"` so the return key says what it does», кодам — автозаполнение.

Механика уже готова: `TextField` прокидывает rest-пропсы в нативный инпут
(`text-field.tsx:93`, спреды `:178`/`:216`/`:226`), `SearchField` — тоже
(`search-field.tsx:25,44`), то есть атрибуты ставятся на местах вызова, без
правок компонентов. Проветренные места:

**Шаренные компоненты (внутренний дефолт, до спреда — переопределяемо):**

1. `shared/ui/design/search-field.tsx:38-40` — единственное поисковое поле
   дизайн-слоя: `enterKeyHint="search"` (iOS рисует клавишу «Поиск»).
2. `shared/ui/design/amount-field.tsx:154-160` — поле суммы (рядом уже стоит
   `autoComplete="off"` `:158`): `enterKeyHint="done"`.

**Кодовые поля (6-значный код подтверждения, все — `TextField` с
`inputMode="numeric"`):**

3. `features/auth/ui/code-step/CodeStep.tsx:74-86` — код входа (аудит:
   `:80`) — `autoComplete="one-time-code"` + `enterKeyHint="done"` +
   `autoCapitalize="none" autoCorrect="off"` (канон §4: кодам — без
   автозаглавных и автокоррекции).
4. `widgets/profile/ui/PhoneChangeScreen.tsx:211-222` — код подтверждения
   смены телефона (SMS) — тот же набор.
5. `widgets/profile/ui/EmailChangeScreen.tsx:302-313` — код для текущей
   почты — тот же набор.
6. `widgets/profile/ui/EmailChangeScreen.tsx:358-369` — код для новой почты —
   тот же набор.

**Формы, где Enter реально сабмитит (есть `<form onSubmit>` и
`type="submit"`):**

7. `features/auth/ui/phone-step/PhoneStep.tsx:67,73,96` — телефон входа —
   `enterKeyHint="go"` (шаг ведёт к следующему экрану).
8. `features/auth/ui/email-step/EmailStep.tsx:66,72,93` — почта входа —
   `enterKeyHint="go"`.
9. `widgets/profile/ui/PhoneChangeScreen.tsx:238-243` — новый телефон
   (форма `#phone-change-phone`, `:234`) — `enterKeyHint="done"`.
10. `widgets/profile/ui/EmailChangeScreen.tsx:331-336` — новая почта
    (форма `:327`) — `enterKeyHint="done"`.

## Target

```tsx
// apps/frontend/features/auth/ui/code-step/CodeStep.tsx:74 — target
<TextField
    ref={inputRef}
    variant="titleIn"
    title="Код"
    name="code"
    type="text"
    inputMode="numeric"
    autoComplete="one-time-code"
    autoCapitalize="none"
    autoCorrect="off"
    enterKeyHint="done"
    value={code}
    onChange={handleChange}
    onClear={onClear}
    error={error}
/>
```

```tsx
// apps/frontend/shared/ui/design/search-field.tsx:38 — target (до {...props})
<input
    type="search"
    enterKeyHint="search"
    className="…"
    placeholder={placeholder}
    disabled={disabled}
    value={value}
    {...props}
/>
```

Остальные места — те же атрибуты по таблице из Problem, значением из колонки
выше; комментарии в коде не нужны (атрибут самодокументирующ, канон записан
здесь и в скилле).

## Repo conventions to follow

- Честность подсказки: `enterKeyHint` ставится только там, где клавиша
  действительно делает действие. `contact-form.tsx` и оба экрана-инвайта
  (`participants-invite-screen.tsx:137`, `property-participants-invite-screen.tsx:86`)
  НЕ размечаются — у их `<form>`/дивов нет сабмита клавишей (сохранение —
  кнопкой).
- «go» — только шаги авторизации (переход вперёд), «search» — поиск,
  «done» — завершение формы/поля.
- Атрибуты — на месте вызова через существующий passthrough, компоненты
  `TextField`/`SearchField` не расширяются новыми пропсами.

## Steps

1. `search-field.tsx`: `enterKeyHint="search"` до спреда.
2. `amount-field.tsx`: `enterKeyHint="done"`.
3. Четыре кодовых поля (Target): `autoComplete="one-time-code"`,
   `autoCapitalize="none"`, `autoCorrect="off"`, `enterKeyHint="done"`.
4. `PhoneStep.tsx`, `EmailStep.tsx`: `enterKeyHint="go"`.
5. `PhoneChangeScreen.tsx:238`, `EmailChangeScreen.tsx:331`:
   `enterKeyHint="done"`.

## Boundaries

- Do NOT трогать `text-field.tsx` и `search-field.tsx` сверх внутреннего
  дефолта поиска.
- Do NOT размечать поля, где Enter ничего не делает (контакт-форма, инвайты,
  атрибуты объектов, суммы внутри больших форм) — кроме `amount-field`
  (поле суммы — самодостаточный ввод с клавиатуры).
- Do NOT добавлять `enterKeyHint` на textarea-редакторы (autogrow) —
  клавиша там перенос строки.
- Если строки вызовов не совпадают с цитатами (дрейф от 6ea8bc4ed) — СТОП
  и отчёт.

## Verification

- **Mechanical**: `npx tsc --noEmit` и `npm run lint` — зелёные;
  `make frontend-test` — зелёный; в SSR/dev разметке кодового поля
  присутствует `autocomplete="one-time-code"`.
- **Feel check** (телефон, протокол карты #1416):
  - iOS: фокус в поиске — клавиша «Готово»/«Поиск» по-язычному; фокус в
    коде входа — iOS предлагает код из SMS/почты одним тапом; фокус в
    поле телефона входа — клавиша «OK»/«Далее» (go);
  - Android: подписи клавиш те же смыслы (Gboard рисует действие);
- **Done when**: атрибуты стоят на всех десяти местах; unit/e2e без красных.
