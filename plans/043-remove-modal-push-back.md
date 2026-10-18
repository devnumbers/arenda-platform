# 043 — Вырез push-back фона модалок

- **Status**: DONE
- **Baseline**: ветка multi-scroll, догонка feel-check 16.10 (039–042 DONE)
- **Severity**: HIGH (решение владельца, feel-check 16.10)
- **Origin**: карта #1437, догонка feel-check владельца
- **Estimated scope**: globals.css, modal.tsx, screen-layout.tsx, scroll-shift.spec.ts, контракт-тест, ~60 строк

## Problem

Feel-check владельца 16.10 на живом стенде: отодвигание страницы вглубь
при открытии модалок/попапов (push-back фона: `scale(0.96)` + сдвиг 8px +
радиус, решение владельца 14.10, канон apple-design §12) не принято —
«страница отдаляется, вырезать». Слово владельца перекрывает более
раннее решение 14.10: это второй разворот вкуса по модалкам за конвейер
(первый — лок фона без прыжка, план 041).

Единственное место эффекта — `app/globals.css:641-672`: хук
`body:has(.modal-overlay[data-state='open'])` на `.screen-shell`
(ScreenLayout), transition-блок 400ms на оболочке и его reduced-motion
ветка. Хук `.modal-overlay` стоит на оверлеях и Dialog (десктоп), и
vaul-Drawer (мобильный) в modal.tsx — эффект накрывает все Modal сразу.

## Target

**Шаг 1 — `app/globals.css:641-672`**: удалить блок целиком — правило
`body:has(...) .screen-shell` (transform + border-radius + overflow),
transition-блок `.screen-shell` (существовал только ради push-back) и
его `prefers-reduced-motion` ветку. Страница под скримом остаётся
полностью статичной. Вход самой карточки (`modal-card` 0.95→1) и оверлея
не трогается (решение владельца 16.10: «карточку оставить»).

**Шаг 2 — `shared/ui/design/modal.tsx:156-157,187-190`**: комментарии
«крючок push-back фона» переписать: класс остаётся как единый носитель
стиля оверлея (bg-overlay), без отсылок к удалённому эффекту.

**Шаг 3 — `widgets/screen-layout/ui/screen-layout.tsx:64-70`**:
комментарий-крючок над `.screen-shell` удалить (класс остаётся просто
стилевой точкой монтирования, ссылок на эффект больше нет).

**Шаг 4 — `e2e/scroll-shift.spec.ts:137-142`**: инжекция
`.screen-shell { transform: none !important; ... }` с комментарием «это
фича» становится ложью — убрать инжекцию; гард замеряет компенсацию
гюттера на статичной странице, как до 14.10.

**Шаг 5 — grep-проход**: `push-back`, `отодвиг`, `scale(0.96)` по
`apps/frontend` — 0 употреблений вне plans/ и истории git. Plan 012
(«Do NOT трогать push-back») и README-таблицы — история, не правится.

## TDD seams

- Новый `shared/ui/design/modal-push-back-removed.test.ts` —
  контракт-тест читает `app/globals.css` (прецедент
  press-feedback.test.ts): в файле нет `body:has(.modal-overlay`
  (хук push-back не возвращается), нет `translateY(8px) scale(0.96)`;
  блок `.modal-card[data-state='open']` существует (вход карточки
  сохранён решением владельца). Сначала красный (сейчас хук есть),
  потом шаги 1–4, зелёный.
- `e2e/scroll-shift.spec.ts`, `e2e/scroll-isolation.spec.ts` — прогнать:
  позиционные гарантии 041 не зависят от push-back (страница теперь
  статична и подавно не прыгает).

## Boundaries

- Do NOT трогать вход/выход карточки модалки (`modal-card` scale 0.95→1)
  и оверлея — решение владельца 16.10: «карточку оставить».
- Do NOT трогать vaul-шит «Ещё» (`.more-sheet-*`), полноэкранные
  поверхности (`.fullscreen-surface`) и дропдауны (`menu-in`/`menu-out`)
  — push-back их никогда не задевал, границы плана 012 в силе.
- Do NOT трогать useBodyScrollLock (план 041) и календарь (план 042) —
  соседние зоны догонки.
- Do NOT править plan-файлы 012/023 (границы «Do NOT трогать push-back»
  там — история решений, не живой код).

## Verification

- `npx tsc --noEmit`, `npm run lint`, `make frontend-test` — зелёные.
- `make frontend-e2e TESTS=-g scroll` — scroll-shift + scroll-isolation
  зелёные (позиция фона при модалках теперь тривиально статична).
- Feel: открытие модалки на десктопе и телефоне — скрим и карточка
  появляются, страница за ними не масштабируется и не сдвигается.

## Done when

`.screen-shell` не трансформируется никогда; хук `body:has(.modal-overlay`
удалён вместе с transition-блоком и reduced-motion веткой; комментарии
modal.tsx/screen-layout.tsx и scroll-shift.spec.ts не лгут; вход
карточки сохранён; тесты зелёные.
