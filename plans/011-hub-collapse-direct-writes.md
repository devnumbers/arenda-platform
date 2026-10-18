# 011 — hub-collapse: прямые записи стилей потребителям вместо переменной на :root каждый кадр

- **Status**: TODO
- **Commit**: 2d6925b76
- **Severity**: HIGH
- **Category**: Performance (аудит анимаций вне колеса, карта #1416 / тикет #1418)
- **Estimated scope**: 2 файла (hub-collapse.tsx, globals.css), ~40 строк

## Problem

Сворачивание хаб-шапки пишет прогресс скролла в CSS-переменную на `:root`
каждый анимационный кадр скролла:

```tsx
// apps/frontend/shared/ui/design/hub-collapse.tsx:40-42 — current
      last = progress;
      root.style.setProperty('--hub-collapse', String(Math.round(progress * 1000) / 1000));
      root.classList.toggle('hub-collapse-on', progress > 0.04);
```

(`root = document.documentElement`; rAF-троттлинг есть, delta-гейт 0.004
есть — но во время активного скролла запись идёт каждый кадр.)

Канон (AUDIT.md §5) прямо называет этот паттерн: «Don't drive child
transforms via a CSS variable on the parent — it recalcs styles for all
children», `setProperty('--x', …) driving child transforms` — в списке
охоты. Переменная на корне документа наследуется всему дереву: на каждый
кадр инвалидируются унаследованные custom properties по всему документу,
хотя потребителей двое (`.hub-compact` — opacity, `.hub-compact-bar` —
translateY), оба живут в TopNav хаба:

```css
/* apps/frontend/app/globals.css:492-495 — current */
.hub-compact {
    opacity: var(--hub-collapse, 0);
    pointer-events: none;
}

/* globals.css:502-504 — current */
.hub-compact-bar {
    transform: translateY(calc((1 - var(--hub-collapse, 0)) * -100%));
}
```

Хаб-страницы — длинные списки объектов/задач (основной скролл продукта),
таргет — слабые Android. Решение владельца 2026-09-09 (визуал сворачивания,
Figma 1603-93157) не пересматривается — меняется только механизм доставки
прогресса: те же значения, тем же rAF-путём, но прямыми записями в style
двух классов потребителей (канон: «Set transform directly on the element»).

## Target

```tsx
// apps/frontend/shared/ui/design/hub-collapse.tsx — target (тело update + очистка)
    const update = () => {
      frame = 0;
      const rect = anchor.getBoundingClientRect();
      const restTop = rect.top + window.scrollY;
      const range = restTop + rect.height - BAR_HEIGHT;
      let progress = range > 0 ? window.scrollY / range : 1;
      progress = Math.min(1, Math.max(0, progress));
      if (reduced.matches) progress = progress > 0.5 ? 1 : 0;
      if (Math.abs(progress - last) < 0.004) return;
      last = progress;
      // Прогресс — прямыми записями в потребителей (аудит #1418, канон
      // перфа: никаких custom properties на :root каждый кадр — инвалидирует
      // всё дерево). Потребителей двое, оба в TopNav.
      for (const el of document.querySelectorAll<HTMLElement>('.hub-compact')) {
        el.style.opacity = String(progress);
      }
      for (const el of document.querySelectorAll<HTMLElement>('.hub-compact-bar')) {
        el.style.transform = `translateY(${((1 - progress) * -100).toFixed(3)}%)`;
      }
      root.classList.toggle('hub-collapse-on', progress > 0.04);
    };
```

Очистка в анмаунте — снять inline-стили (сейчас снимает переменную):

```tsx
// hub-collapse.tsx, cleanup в return useEffect — target
      root.classList.remove('hub-collapse-on');
      for (const el of document.querySelectorAll<HTMLElement>('.hub-compact, .hub-compact-bar')) {
        el.style.removeProperty('opacity');
        el.style.removeProperty('transform');
      }
```

CSS-потребители получают статичные rest-значения (прогресс 0) вместо var:

```css
/* apps/frontend/app/globals.css:490-506 — target (комментарий обновлён) */
/* Компакт-бар хаба (решение владельца 2026-09-09, Figma 1603-93157 →
   1733-93740): видимость управляется прогрессом прокрутки, который
   HubCollapseAnchor пишет прямыми style-записями в .hub-compact/.hub-compact-bar
   (аудит #1418: переменная на :root каждый кадр инвалидирует всё дерево).
   Ниже — rest-состояние (прогресс 0); CSS-переходов нет, плавность даёт
   сам скролл. Класс hub-collapse-on включает pointer-events, когда
   компакт уже виден. */
.hub-compact {
    opacity: 0;
    pointer-events: none;
}

:root.hub-collapse-on .hub-compact {
    pointer-events: auto;
}

/* Мобильный клон компакт-бара выезжает сверху вместе с прогрессом. */
.hub-compact-bar {
    transform: translateY(-100%);
}
```

JSDoc хука — обновить абзац про механизм (строки 20-27): «пишет прогресс
прямыми style-записями в потребителей .hub-compact/.hub-compact-bar, минуя
React-состояние и CSS-переменные» — суть и гарантии (rAF-троттлинг,
passive-listener, reduced-порог, только opacity/transform) сохраняются.

## Repo conventions to follow

- Внутренний эталон прямых style-записей на кадр —
  `applyContentTransform` в `shared/ui/pull-to-refresh/PullToRefresh.tsx:130-140`
  (тот же приём: style вместо React-состояния, transition не участвует).
- Визуал и пороги не меняются: delta-гейт 0.004, порог класса 0.04,
  reduced-снап 0.5 — всё остаётся (решение 09.09).
- `hub-collapse-on` остаётся на `:root` — переключается редко (переход
  порога), не на каждый кадр.

## Steps

1. `hub-collapse.tsx` — update() по Target; cleanup; JSDoc.
2. `globals.css` — блок `.hub-compact*` по Target (с новым комментарием).

## Boundaries

- Do NOT трогать пороги, длительности, разметку TopNav, другие consumers
  скролла (`wheel-picker.tsx` — исключение 001-004).
- Do NOT заменять rAF на scroll-timeline/intersection-observer — механизм
  доставки меняется минимально.
- Do NOT кешировать querySelectorAll: список потребителей статичен, но
  запрос дешевле инвалидации кеша; превышать два запроса на кадр нельзя.
- Если у `.hub-compact*` появятся новые потребители вне TopNav (дрейф) —
  СТОП и отчёт.

## Verification

- **Mechanical**: `npx tsc --noEmit`, `npm run lint`, `make frontend-test`
  — зелёные. Профиль: DevTools → Performance, скролл длинной хаб-страницы
  (объекты) — в записях style recalc исчезает полное дерево (до правки
  Recalculate Style каждый кадр с большим scope, после — точечный).
- **Feel check** (телефон + десктоп):
  - скролл хаба: шапка сворачивается в компакт-бар плавно, как до правки
    (порыв вверх/вниз, отпускание — без прыжков);
  - компакт-бар кликабелен в видимой фазе, не кликабелен в скрытой
    (pointer-events гейт жив);
  - после ухода со страницы inline-стили сняты (инспектором: на TopNav
    следующей страницы нет style="opacity…");
  - reduced-motion: сворачивание по порогу 0/1, без промежуточных кадров;
  - resize окна: прогресс пересчитывается (invalidate жив).
- **Done when**: `--hub-collapse` больше не пишется на `:root` (grep
  `hub-collapse` по setProperty — 0); сворачивание визуально идентично
  коммиту 2d6925b76; перф-профиль скролла хаба без doc-wide recalc.
