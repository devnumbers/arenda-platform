# 008 — Ряды и карточки: sweep `.pressable` вместо transition-all и opacity-диалектов

- **Status**: TODO
- **Commit**: 2d6925b76
- **Severity**: MEDIUM
- **Category**: Physicality & origin + Performance (аудит анимаций вне колеса, карта #1416 / тикет #1418)
- **Estimated scope**: ~12 файлов, механическая замена класс-строк

## Problem

Канон пресс-отклика, введённый планом 007 в дизайн-слое, не доходит до
сущностных рядов/карточек и самодельных нажимаемых блоков. Там три хвори:

1. **`transition-all`** (канон §5: «always a finding») на самых частых
   поверхностях — гонит нецелевые свойства мимо композитора:

```tsx
// apps/frontend/entities/payment/ui/payment-row-button.tsx:76 — current
        'transition-all focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-white',
        'hover:opacity-80 active:opacity-80 px-3',

// apps/frontend/entities/payment/ui/payment-card-button.tsx:56 — current
        'transition-all focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-white',
        'hover:opacity-80 active:opacity-80',

// apps/frontend/features/payments/ui/payments-add-sheet.tsx:179 — current
      className="flex cursor-pointer flex-col gap-4 rounded-card bg-surface-muted p-4 text-left outline-none transition-all focus-visible:ring-4 focus-visible:ring-primary hover:opacity-80 active:opacity-80"
```

2. **Пресс — только просадкой прозрачности**, при этом два значения одного
   интента (`opacity-80` ≈ 23 места, `opacity-90` ×3), разбросанных по
   виджетам: `participant-row-button.tsx:46`, `contact-row-button.tsx:54`,
   `ProfileHub.tsx:70`, `rental-group.tsx:50`, `rental-detail-body.tsx:324`,
   `payments-objects-screen.tsx:288`, `history-feed-screen.tsx:56`,
   `notification-row.tsx:32`, `search-pill.tsx:44` (hover:opacity-90
   active:opacity-90) и другие.

3. **active == hover** у цветовых рядов:

```tsx
// apps/frontend/widgets/property-detail/ui/PropertyDetailActions.tsx:77 — current
          className={`flex h-[52px] w-full cursor-pointer items-center rounded-button px-1 text-left outline-none transition-colors focus-visible:ring-2 focus-visible:ring-primary hover:bg-surface-muted-hover active:bg-surface-muted-hover ${
```

   (если план 007 исполнен, у PropertyDetailActions фикс уже стоит —
   пропусти.)

## Target

Единое правило для самодельных нажимаемых рядов/карточек:

1. `transition-all` → класс `.pressable` из плана 007 (он уже описывает
   transform+opacity+цвета+кольцо на токенах).
2. `active:opacity-80|90` снимается — пресс теперь скейл из `.pressable`.
3. `hover:opacity-80` **остаётся** (hover-затемнение — цветовой dialect
   наведения, DESIGN.md §8 «hover/press приглушают строку»; на десктопе
   ничего не меняется).
4. `active:bg-* == hover:bg-*` → `active:bg-*-active` (как в плане 007).

Образцы:

```tsx
// entities/payment/ui/payment-row-button.tsx:76-77 — target
        'pressable group/row flex w-full cursor-pointer items-center py-2 outline-none',
        'focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-white',
        'hover:opacity-80 px-3',

// entities/payment/ui/payment-card-button.tsx:55-58 — target
        'pressable flex w-[168.5px] shrink-0 cursor-pointer flex-col rounded-card bg-surface-muted p-4 outline-none',
        'focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-white',
        'hover:opacity-80',

// features/payments/ui/payments-add-sheet.tsx:179 — target
      className="pressable flex cursor-pointer flex-col gap-4 rounded-card bg-surface-muted p-4 text-left outline-none focus-visible:ring-4 focus-visible:ring-primary hover:opacity-80"
```

Перечень мест — по grep'у (актуализировать на момент исполнения):

```bash
# transition-all на интерактиве (все 4 места фронта):
grep -rn "transition-all" apps/frontend --include="*.tsx" | grep -v test
# opacity-пресс:
grep -rn "active:opacity-" apps/frontend --include="*.tsx" | grep -v test
# активные состояния, равные hover:
grep -rn "active:" apps/frontend --include="*.tsx" | grep -v test
```

Каждой найденной класс-строке — по правилам 1–4 выше. У строк с
`group-active/...`-вариантами (round-action-button.tsx:21) скейл не
добавлять, только оставить как есть: круглый FAB и календарные кнопки —
отдельная племенная группа, цветовой dialect для них осознанный.

## Repo conventions to follow

- `.pressable` введён планом 007 (`globals.css`, секция дизайн-слоя) —
  класс уже описывает полный переходный рецепт; новых transition-правил
  здесь не создавать.
- Hover-затемнение рядов остаётся: DESIGN.md §8 «hover/press приглушают
  строку» — меняется только press-половина.
- Exemplar правки — payment-row-button.tsx из Target.

## Steps

1. Выполнить три grep'а из Target, зафиксировать список мест (ожидание по
   коммиту 2d6925b76: transition-all — 4, active:opacity — ~26,
   active==hover — 1–2 вне плана 007).
2. Пройти список по правилам 1–4; образцы — из Target.
3. Для каждого правленого ряда — быстрая визуальная проверка по Feel check.

## Boundaries

- Do NOT трогать `payments-favorites-screen.tsx` (Reorder-ряды — план 016),
  `tab-bar-row.tsx`, `user-button.tsx`, контролы выбора, `wheel-picker.tsx`.
- Do NOT снимать `hover:opacity-*` — меняется только активная половина.
- Do NOT добавлять `.pressable` элементам без `cursor-pointer`/роли кнопки
  (ненажимаемым контейнерам).
- Do NOT вводить новые значения opacity (80 vs 90 разница остаётся только
  в hover, где она была; active-диалект один — скейл).
- Если grep'и дают сильно другие числа (дрейф) — СТОП и отчёт.

## Verification

- **Mechanical**: `npx tsc --noEmit`, `npm run lint`, `make frontend-test` —
  зелёные. Контрольные grep'и после правки: `transition-all` на
  интерактиве — 0; `active:opacity-` — 0.
- **Feel check** (телефон): тап по строке платежа, карточке платежа,
  чипу, строке истории — везде один и тот же мягкий скейл ≈3%;
  фокус-кольцо с клавиатуры проявляется плавно (250ms), hover на десктопе
  не изменился.
- **Done when**: контрольные grep'и чистые; пресс-отклик ряда/карточки
  визуально идентичен кнопкам дизайн-слоя.
