# 009 — Меню: скейл от триггера, свой вход/выход, контакт-кебаб на общий MenuContent

- **Status**: TODO
- **Commit**: 2d6925b76
- **Severity**: MEDIUM
- **Category**: Physicality & origin + Cohesion (аудит анимаций вне колеса, карта #1416 / тикет #1418)
- **Estimated scope**: 3 файла (menu.tsx, globals.css, contact-kebab-menu.tsx), ~30 строк

## Problem

Дропдаун-меню — единственный анимированный якорный попап продукта —
скейлится от своего геометрического центра, а не от триггера. Канон (AUDIT.md
§3): «Popovers/dropdowns/tooltips scale from their trigger, not center» —
модалки исключение, меню нет. Radix кладёт точку входа в переменную
`--radix-dropdown-menu-content-transform-origin`, фронт её не использует
(grep `transform-origin` по фронту — единственное совпадение `left` у
toast-прогресса, `ToastProvider.module.css:98`). Через общий `MenuContent`
скейл-от-наследуется 11 потребителями (picker-menu, PropertyDetailKebab,
contact-step, participants ×3, payment-history, tasks ×2, notifications,
menu.tsx сам).

```tsx
// apps/frontend/shared/ui/design/menu.tsx:35 — current
          'data-[state=open]:animate-[modal-card-in_150ms_var(--dl-ease)]',
```

Три подчинённые хвори:

1. Меню ре-использует **модальный кейфрейм** `modal-card-in` (scale 0.95
   от центра, `globals.css:329-339`) со своей длительностью 150ms —
   «один кейфрейм, три характера» (модалка 400ms, SuccessPopup 400ms,
   меню 150ms); при ретюне модалки меню разъедется молча.
2. **Выхода нет**: `data-[state=closed]`-класса у MenuContent нет — меню
   исчезает мгновенно, в то время как всё остальное в продукте уходит
   анимированно.
3. Контакт-кебаб (`widgets/contacts/ui/contact-kebab-menu.tsx:30-35`) —
   второй сырой `DropdownMenu.Content` мимо дизайн-слоя: появляется
   мгновенно, без движения и origin, с чужим радиусом/паддингом —
   несогласованность с MenuContent.

```tsx
// apps/frontend/widgets/contacts/ui/contact-kebab-menu.tsx:30-35 — current
        <DropdownMenu.Content
          align="end"
          sideOffset={4}
          collisionPadding={24}
          className="z-50 min-w-[250px] rounded-3xl bg-white p-2 shadow-[0_8px_32px_rgba(23,26,28,0.16)] outline-none"
        >
```

## Target

**Шаг 1 — собственные кейфреймы меню** в `app/globals.css`, рядом с
`modal-card-in/out` (после строки 371):

```css
/* Дропдаун-меню (аудит #1418): свой кейфрейм вместо modal-card-in —
   скейл 0.95 от ТОЧКИ ТРИГГЕРА (origin ставит Radix в переменную
   --radix-dropdown-menu-content-transform-origin). Длительности — бюджет
   дропдаунов канона: вход 150ms, выход 120ms, обе на --dl-ease. */
@keyframes menu-in {
    from {
        opacity: 0;
        transform: scale(0.95);
    }

    to {
        opacity: 1;
        transform: scale(1);
    }
}

@keyframes menu-out {
    from {
        opacity: 1;
        transform: scale(1);
    }

    to {
        opacity: 0;
        transform: scale(0.97);
    }
}
```

**Шаг 2 — MenuContent**: origin от Radix + свой вход + выход:

```tsx
// apps/frontend/shared/ui/design/menu.tsx:33-37 — target
          'z-50 flex w-fit flex-col gap-0.5 rounded-card bg-surface px-0.5 py-3 font-sans shadow-[0_8px_24px_rgba(23,26,28,0.12)] outline-none',
          '[transform-origin:var(--radix-dropdown-menu-content-transform-origin,center)]',
          'data-[state=open]:animate-[menu-in_150ms_var(--dl-ease)]',
          'data-[state=closed]:animate-[menu-out_120ms_var(--dl-ease)]',
```

**Шаг 3 — контакт-кебаб на MenuContent** (кебаб получает общий вход/выход/
origin; собственные размеры оставить className-дополнением):

```tsx
// apps/frontend/widgets/contacts/ui/contact-kebab-menu.tsx:30-35 — target
        <MenuContent className="min-w-[250px]">
          <MenuItem onSelect={onDelete} icon={<TrashBin className="h-[18px] w-[17px]" />} label="Удалить" />
        </MenuContent>
```

(импорт `MenuContent` из `shared/ui/design/menu`; убрать `DropdownMenu.Portal`
— Portal уже внутри MenuContent; `collisionPadding={24}` потерять нельзя —
проверь, есть ли проп у MenuContent, если нет — добавь
`collisionPadding = 24` в его пропсы по образцу `sideOffset`.)

**Шаг 4 — reduced-motion**: в существующий reduced-контекст фронта добавить
укорочение (150ms-стратегия §8): в `app/globals.css` рядом с другими
reduced-блоками (после блока `.more-sheet-*`, строка 595):

```css
@media (prefers-reduced-motion: reduce) {
    [data-radix-dropdown-menu-content] {
        animation-duration: 150ms;
    }
}
```

## Repo conventions to follow

- Значения — только из канона бюджета (дропдауны 150–250ms) и токена
  `--dl-ease`; новых кривых нет.
- Radix сам вычисляет origin по стороне/выравниванию (`align="end"`,
  `sideOffset=4`) — не хардкодить проценты.
- Прецедент вынесенного кейфрейма в globals.css — `modal-card-in/out`
  (`globals.css:329-346`).
- Exemplar потребителя MenuContent — `PropertyDetailKebab.tsx`.

## Steps

1. `globals.css` — кейфреймы `menu-in/out` (шаг 1) + reduced-правило (шаг 4).
2. `menu.tsx` — классы по Target (шаг 2).
3. `contact-kebab-menu.tsx` — перевод на MenuContent (шаг 3), при
   необходимости добавить `collisionPadding` в MenuContent.

## Boundaries

- Do NOT трогать длительность 150ms входа (в бюджете дропдаунов; тайминги
  `--dl-*` — решение 26.08, не ре-литигировать).
- Do NOT трогать picker-menu.tsx / mobile-пикеры — их «попапы» на мобиле
  реализованы как Modal/шит, origin-вопроса нет.
- Do NOT менять стили пунктов меню.
- Если `--radix-dropdown-menu-content-transform-origin` не появился в
  computed style при feel-проверке (версия Radix дрейфанула) — СТОП и
  отчёт, не хардкодить origin.

## Verification

- **Mechanical**: `npx tsc --noEmit`, `npm run lint`, `make frontend-test` —
  зелёные. `make frontend-e2e TESTS='-g кебаб|меню'` — если есть профильные
  спеки, зелёные.
- **Feel check** (десктоп + телефон):
  - кебаб объекта: меню разворачивается ИЗ точки кебаба (правый верхний
    угол растёт влево-вниз), не из центра карточки;
  - меню слева от края экрана — origin уходит в правый нижний угол;
  - закрытие по выбору пункта и по Esc — короткий обратный скейл-фейд
    120ms, не мгновенное исчезновение;
  - spam-тест: открыть/закрыть 10 раз подряд — без застреваний (Radix
    Presence + animationend);
  - DevTools → Rendering → prefers-reduced-motion: reduce — вход/выход
    укорочены до 150ms;
  - контакт-кебаб на странице контакта — выглядит и анимируется как все
    меню (radius/паддинг по MenuContent), пункт «Удалить» работает,
    меню не обрезается у правого края (collisionPadding жив).
- **Done when**: grep `modal-card-in` по menu.tsx — 0; contact-kebab не
  содержит сырого DropdownMenu.Content; все 11 потребителей MenuContent
  скейлятся от триггера.
