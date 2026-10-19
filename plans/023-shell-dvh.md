# 023 — Шелл: `min-h-dvh` вместо единственного `min-h-screen`

- **Status**: TODO
- **Commit**: 6ea8bc4ed
- **Severity**: LOW
- **Category**: mobile-native §3 (высота лейаута: 100vh — largest viewport, шелл — на dvh); карта #1416 / тикет #1421
- **Estimated scope**: 3 файла, один класс + три комментария

## Problem

Оболочка экранов — единственное место шелла на large-viewport единице:
`screen-layout.tsx:71` — `<div className="screen-shell flex min-h-screen
flex-col tablet:pt-[72px]">`. `min-h-screen` = `100vh` = наибольший вьюпорт
(хром браузера схлопнут): пока URL-бар виден, оболочка минимум на его высоту
выше видимой области — лишний недоскролл и нижний запас, посчитанный от
невидимой земли. Весь остальной продукт уже на dvh: `LoginShell.tsx:49,56`,
`modal.tsx:171,206`, `wizard-chrome.tsx:252`,
`operations-categories-screen.tsx:132` и др. Канон §3: «App shell … should
track the visible area as chrome shows/hides — `100dvh`».

Аудит ссылался на `:64` — строка дрейфнула к `:71` (рекогносцировка
снималась с другого коммита); суть не изменилась.

## Target

```tsx
// apps/frontend/widgets/screen-layout/ui/screen-layout.tsx:71 — target
<div className="screen-shell flex min-h-dvh flex-col tablet:pt-[72px]">
```

И три комментария, которые ссылаются на старое имя класса, чтобы не врали:

- `widgets/history/ui/history-feed-screen.tsx:93` — «ScreenLayout уже flex-1
  от min-h-screen оболочки …» → «…от min-h-dvh оболочки…»;
- `widgets/history/ui/history-feed-screen.tsx:500` — «…контента ScreenLayout
  (уже flex-1 от min-h-screen) …» → «…(уже flex-1 от min-h-dvh)…»;
- `e2e/history-feed.spec.ts:392` — комментарий «min-h-screen оболочки,
  лента с flex-1 тянется до нижнего края…» → «min-h-dvh оболочки…».

## Repo conventions to follow

- dvh — установившийся канон продукта (см. Problem); `min-h-screen` в других
  местах по фронту после фикса не остаётся (grep `min-h-screen` — единственное
  вхождение в продуктивном коде).
- Комментарии правятся точечно, только упоминание класса; смысловую часть
  не трогать.

## Steps

1. `screen-layout.tsx:71`: `min-h-screen` → `min-h-dvh`.
2. `history-feed-screen.tsx:93` и `:500`, `e2e/history-feed.spec.ts:392`:
   обновить упоминания класса.

## Boundaries

- Do NOT трогать `screen-shell` CSS (`globals.css:566`) — transition-блок
  push-back к высоте отношения не имеет.
- Do NOT переводить герои/лэндинг-подобные экраны на svh — таких поверхностей
  в кабинете нет; LoginShell уже на dvh.
- Do NOT трогать `min-h-[calc(100dvh-…)]`-производные — они уже dvh.
- Если `min-h-screen` в `screen-layout.tsx` не на строке `:71` (дрейф) —
  СТОП и отчёт.

## Verification

- **Mechanical**: `grep -rn "min-h-screen" apps/frontend` —
  ноль вхождений (класс и все комментарии правятся планом); `npx tsc --noEmit`, `npm run lint`,
  `make frontend-test` — зелёные; регресс
  `make frontend-e2e TESTS='-g frame-shell'` — зелёный.
- **Feel check** (iPhone, Safari-браузер, не standalone — там URL-бар жив):
  низ страницы (TabBar-зона, контент над ним) доходит до видимого низа без
  недоскролла; схлопывание/раскрытие URL-бара скроллом не оставляет пустой
  полосы под контентом.
- **Done when**: класс заменён, комментарии правдивы, регресс зелёный.
