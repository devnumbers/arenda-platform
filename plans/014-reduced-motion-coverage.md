# 014 — Reduced-motion: остаточное покрытие — switch, PTR-спиннер, tariff-faq, кривая скелетона

- **Status**: TODO
- **Commit**: 2d6925b76
- **Severity**: MEDIUM
- **Category**: Accessibility + Cohesion & tokens (аудит анимаций вне колеса, карта #1416 / тикет #1418)
- **Estimated scope**: 4 файла (switch.tsx, PullToRefresh.module.css, tariff-faq.tsx, globals.css), ~20 строк

## Problem

Стратегия reduced-motion зафиксирована глобально (DESIGN.md §8: «анимации
не отключает, а укорачивает до ~150ms») и применена в 8 местах, но sweep
обнаружил хвост поверхностей, где движение никак не покрыто:

```tsx
// apps/frontend/shared/ui/design/switch.tsx:26 — current (ползунок 350ms, без reduced)
      <SwitchThumb className="block h-6 w-9 rounded-pill bg-white shadow-[0_2px_8px_rgba(0,0,0,0.16)] transition-transform ease-apple duration-(--dl-duration-move) data-[state=checked]:translate-x-6" />
```

```tsx
// apps/frontend/widgets/profile/ui/tariff/tariff-faq.tsx:171,183 — current
            'shrink-0 text-content-tertiary transition-transform duration-300 ease-out',
// …
          'grid transition-[grid-template-rows,opacity] duration-300 ease-out',
```

```css
/* apps/frontend/shared/ui/pull-to-refresh/PullToRefresh.module.css:32-34 — current
   (бесконечное вращение без reduced-ветки; функциональный индикатор —
   но вращение канон называет классическим триггером вестибулярной реакции) */
.refreshing svg {
    animation: ptr-spin 0.8s linear infinite;
}
```

Плюс когезионный хвост: единственная регулярная не-Apple кривая продукта —
дефолтный pulse скелетона (Tailwind: `2s cubic-bezier(0.4, 0, 0.6, 1)`),
мимо токен-канона (все transition-* переведены на `--dl-ease`,
`globals.css:301-303`, но keyframe-дефолты пролетают мимо):

```tsx
// apps/frontend/shared/ui/design/skeleton.tsx:17 — current
  return <div aria-hidden className={cn('animate-pulse rounded-pill bg-surface-muted', className)} {...props} />;
```

Не находим: `prefers-reduced-transparency`/`prefers-contrast` — 0
использований. Вне motion-канона (аудит движения) — фиксировано как
известный пробел, планом не закрывается.

## Target

**Шаг 1 — switch**: Tailwind-вариант `motion-reduce:` локально у носителя:

```tsx
// apps/frontend/shared/ui/design/switch.tsx:26 — target
      <SwitchThumb className="block h-6 w-9 rounded-pill bg-white shadow-[0_2px_8px_rgba(0,0,0,0.16)] transition-transform ease-apple duration-(--dl-duration-move) motion-reduce:duration-150 data-[state=checked]:translate-x-6" />
```

**Шаг 2 — PTR-спиннер**: в `PullToRefresh.module.css` после `.refreshing svg`:

```css
/* Reduced-motion: вращение останавливаем (функциональный индикатор остаётся
   видимым на месте — пониманию помогает, вестибулярно нейтрален;
   стратегия §8 «укоротить, не отключить» к бесконечным циклам неприменима). */
@media (prefers-reduced-motion: reduce) {
    .refreshing svg {
        animation: none;
    }
}
```

**Шаг 3 — tariff-faq**: тем же `motion-reduce:` (кривую ease-out и 300ms
НЕ трогать — задокументированное решение владельца 28.09, комментарий
в шапке файла):

```tsx
// apps/frontend/widgets/profile/ui/tariff/tariff-faq.tsx:171 — target
            'shrink-0 text-content-tertiary transition-transform duration-300 ease-out motion-reduce:duration-150',
// tariff-faq.tsx:183 — target
          'grid transition-[grid-template-rows,opacity] duration-300 ease-out motion-reduce:duration-150',
```

**Шаг 4 — кривая скелетона на токен**: в `@theme`-блоке `app/globals.css`
(рядом с переопределением дефолтов переходов, строки ~295-303):

```css
    /* Скелетон-пульс на кривой дома (аудит #1418): единственный регулярный
       keyframe-дефолт Tailwind шёл с чужой кривой (0.4, 0.6, 1) мимо
       токен-канона. Длительность 2s и opacity-природа — без изменений;
       reduced-motion не укорачивает: это бесконечный индикатор статуса,
       укорачивание дало бы строб. */
    --animate-pulse: pulse 2s var(--dl-ease) infinite;
```

(Ключевые кадры `pulse` Tailwind эмитит сам при использовании утилиты —
переопределяется только shorthand.)

## Repo conventions to follow

- Стратегия §8 — «укоротить до ~150ms, не отключить»; бесконечные
  индикаторы (спиннер, pulse) — исключение: у спиннера вращение гасится,
  opacity-пульс скелетона остаётся.
- Локальные reduced-правила — Tailwind-вариант `motion-reduce:`;
  глобальные — медиа-блоки в CSS (прецеденты: `globals.css:578-595`,
  reduced-блок `ToastProvider.module.css`).
- Кривые — только `--dl-ease` (решение 26.08).

## Steps

1. `switch.tsx:26` — `motion-reduce:duration-150` (шаг 1).
2. `PullToRefresh.module.css` — reduced-блок (шаг 2).
3. `tariff-faq.tsx:171,183` — `motion-reduce:duration-150` (шаг 3).
4. `globals.css` @theme — `--animate-pulse` (шаг 4).

## Boundaries

- Do NOT менять тайминги/кривые вне reduced-веток (350ms switch и 300ms
  tariff-faq — действующие решения).
- Do NOT трогать `animate-spin` (ToastBody, функциональный индикатор в
  тосте — по духу шага 2 гасить его не нужно: живёт <5s, решение
  вынесено владельцу при ревью, по умолчанию не трогать).
- Do NOT укорачивать pulse скелетона под reduced — строб.
- Если код не совпадает с цитатами — СТОП и отчёт.

## Verification

- **Mechanical**: `npx tsc --noEmit`, `npm run lint`, `make frontend-test`
  — зелёные.
- **Feel check** (DevTools → Rendering → prefers-reduced-motion: reduce):
  - switch: ползунок переезжает за 150ms, состояние читается;
  - tariff-faq: раскрытие/шеврон 150ms;
  - pull-to-refresh (iOS standalone): спиннер виден и статичен во время
    refresh, после — как раньше;
  - скелетоны: пульс живой, мягкий разгон/послевкусие кривой `--dl-ease`
    (DevTools → Animations 10% — кривая не 0.4/0.6);
  - без reduced ничего не изменилось.
- **Done when**: grep по четырём правкам на месте; reduced-проход по
  switch/FAQ/PTR/скелетону чист.
