# 006 — transition-all → transition-opacity (3 частотных компонента)

- **Status**: DONE (14.10.2026; механика 001 уточнена при приёмке: доезд — criticalSettle без перелёта, резинка справа — margin-расширение диапазона, слева — трансформ; детали в коде и DESIGN.md §8)
- **Commit**: efe10fd46
- **Severity**: MEDIUM
- **Category**: Performance (AUDIT §5: «transition: all — always a finding»)
- **Estimated scope**: 3 файла, по одной строке

## Problem

`transition-all` анимирует любые меняющиеся свойства, включая фокус-кольцо
(box-shadow всплывает 250мс вместо мгновенной) и любые будущие правки классов:

- `entities/payment/ui/payment-card-button.tsx:62`
- `entities/payment/ui/payment-row-button.tsx:77`
- `features/payments/ui/payments-add-sheet.tsx:179` (ChoiceCard)

Фактически анимируется только opacity (hover/active/disabled); кривая и
длительность — домовые (`--default-transition-*` = `--dl-ease`/250ms из
`@theme`, globals.css:301-302).

## Target

```tsx
'transition-opacity focus-visible:ring-4 …'  /* кольцо встаёт мгновенно */
```

## Steps

1. В трёх файлах заменить `transition-all` → `transition-opacity`.
2. Убедиться, что других анимируемых свойств в этих компонентах нет
   (фон/цвет — константы классов, не меняются в рантайме).

## Boundaries

- Кривую/длительность не трогать (наследуют канон).
- Класс-хук `focus-visible:ring-*` не менять.

## Verification

- Механика: lint/build/test; e2e `payments-hub.spec.ts`.
- Feel-check: Tab-фокус по карточкам — кольцо мгновенно; hover — прежний фейд.
