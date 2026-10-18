# 021 — Viewport: `interactive-widget=resizes-content` (клавиатура Android сжимает лейаут)

- **Status**: TODO
- **Commit**: 6ea8bc4ed
- **Severity**: MEDIUM
- **Category**: mobile-native Baseline (viewport-мета) — поведение софт-клавиатуры; карта #1416 / тикет #1421
- **Estimated scope**: 1 файл, одна опция + комментарий

## Problem

Viewport-экспорт корневого лейаута (`app/layout.tsx:47-66`) не задаёт
`interactiveWidget` — Android Chrome работает в дефолте `resizes-visual`:
клавиатура рисуется ПОВЕРХ layout viewport, а не сжимает его. Следствия на
Android:

- фиксированный низ (StickyBottomBar, TabBar, поля в нижних панелях) остаётся
  под клавиатурой — на iOS тот же лейаут поднимается, потому что там клавиатура
  всегда сжимает viewport;
- `dvh`-единицы и `100dvh`-шелл на клавиатуру не реагируют, хотя всё остальное
  приложение уже живёт на dvh (`LoginShell.tsx:49,56`, `modal.tsx:171,206`,
  `wizard-chrome.tsx:252` и др.).

Директива — бейслайн скилла mobile-native (блок Baseline): она делает
поведение Android таким же, как у iOS, «so `100dvh` and bottom-pinned inputs
react to it the way they do on iOS». Next 16.4 опцию типизирует:
`interactiveWidget?: 'resizes-visual' | 'resizes-content' | 'overlays-content'`
(`node_modules/next/dist/lib/metadata/types/extra-types.d.ts:53`).

## Target

```ts
// apps/frontend/app/layout.tsx — target (внутри export const viewport)
export const viewport: Viewport = {
    width: 'device-width',
    initialScale: 1,
    // Android Chrome: клавиатура сжимает layout viewport (как на iOS), а не
    // рисуется поверх него — fixed-низ (StickyBottomBar, TabBar) и dvh-шелл
    // реагируют на клавиатуру. WebKit директиву игнорирует: iOS и так
    // сжимает. Канон mobile-native (Baseline), тикет #1421.
    interactiveWidget: 'resizes-content',
    // maximumScale is intentionally omitted: … (дальше без изменений)
```

## Repo conventions to follow

- Комментарии viewport-экспорта — «почему», с запретами на регресс
  (образец — комментарий про intentionally omitted maximumScale, `:50-53`).
- PullToRefresh не затрагивается: жест гейтится `isIosDevice()`
  (`PullToRefresh.tsx:96`), директива — Chromium-only.

## Steps

1. `apps/frontend/app/layout.tsx`: добавить `interactiveWidget: 'resizes-content'`
   в `viewport` с комментарием (Target).

## Boundaries

- Do NOT трогать остальные опции viewport (`themeColor`, `viewportFit`,
  `colorScheme`, отсутствие `maximumScale`).
- Do NOT добавлять дублирующую мета-строку руками в `metadata.other` —
  Next рендерит директиву в ту же `meta[name=viewport]` из типизированной
  опции.
- Если в `layout.tsx:47-66` состав опций не совпадает с цитатой (дрейф) —
  СТОП и отчёт.

## Verification

- **Mechanical**: `npx tsc --noEmit` и `npm run lint` — зелёные; на поднятом
  dev-стенде в SSR-разметке `<meta name="viewport">` содержит
  `interactive-widget=resizes-content`; `make frontend-test` — зелёный.
- **Feel check** (Android-железо, протокол карты #1416):
  - фокус в поле экрана с нижней панелью — клавиатура поднимает панель над
    собой (низ не накрыт), сворачивание возвращает;
  - экран со списком — при открытой клавиатуре список скроллится в сжатом
    viewport без «плавающего» низа;
  - iOS — поведение без изменений (директива игнорируется).
- **Done when**: мета в SSR-разметке; unit/e2e без красных.
