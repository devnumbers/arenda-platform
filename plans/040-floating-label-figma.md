# 040 — Плавающий лейбл: вёрстка по Figma (откат трансформа)

- **Status**: TODO
- **Baseline**: bfd817d54; правится исход плана 015 (dc6c3be8d)
- **Severity**: MEDIUM→HIGH (решение владельца, feel-check 16.10)
- **Origin**: карта #1437, догонка feel-check владельца; макеты Figma
  «Рентли. Сервис и лендинг» 948-46863 / 948-46874 / 948-46915 / 948-46926
- **Estimated scope**: text-field.tsx (4 строки класса), контракт-тест, ~60 строк

## Problem

План 015 перевёл морф плавающего лейбла на transform: приподнятый лейбл —
это 16px, смасштабированный до 0.8125, а не настоящий 13px. Сам коммит 015
называл это «честным трейд-оффом» и отдавал чёткость feel-check'у
владельца. Feel-check 16.10 провален: вёрстка приподнятого лейбла
разошлась с макетом — в Figma приподнятый лейбл (пустой, сфокусированный,
заполненный) есть настоящий мелкий текст сверху.

Решение владельца (16.10, вариант «Б»): вернуть настоящую вёрстку
(геометрию до 015), сохранив гигиену переходов из 015 — переход только
нужных свойств (не `transition-all`) и reduced-motion-сокращение.

## Target

`shared/ui/design/text-field.tsx:188-191` — блок лейбла:

```tsx
// target
'pointer-events-none absolute left-0 text-content-secondary transition-[top,font-size,line-height] duration-300 motion-reduce:duration-150',
'top-[19px] text-base leading-[18px]',
'peer-focus:top-[10px] peer-focus:text-[13px] peer-focus:leading-[15px]',
'peer-[:not(:placeholder-shown)]:top-[10px] peer-[:not(:placeholder-shown)]:text-[13px] peer-[:not(:placeholder-shown)]:leading-[15px]',
```

Отличия от до-015 состояния: `transition-all` →
`transition-[top,font-size,line-height]` (переходятся только морфящиеся
свойства — гигиена 015), `motion-reduce:duration-150` (reduced-пробел,
закрытый 015). Отличия от текущего (015): убираются `origin-top-left`,
`translate-y-[9px]`, `scale-[0.8125]`, `ease-apple`,
`transition-transform` — кривая возвращается к дефолту Tailwind, как до
015 (визуал «как было»).

## TDD seams

- Новый `shared/ui/design/text-field.test.tsx`: рендер `TextField`
  (пустого и со значением), у лейбла в покое — `top-[19px]`, `text-base`,
  `leading-[18px]`; в floated-состояниях (`peer-focus:…`,
  `peer-[:not(:placeholder-shown)]:…`) — `top-[10px]`, `text-[13px]`,
  `leading-[15px]`; в классах лейбла нет `scale-`, `translate-y-`,
  `origin-top-left`, `transition-all`; есть
  `transition-[top,font-size,line-height]` и `motion-reduce:duration-150`.
  Красный на текущем коде (там scale/translate), зелёный после правки.

## Boundaries

- Do NOT трогать `search-field.tsx` (его кегль — план 024, coarse-флоор),
  `amount-field`/`amount-input`, textarea-редакторы.
- Do NOT трогать titleOut и статичный error-span (граница плана 015).
- Do NOT менять длительность 300ms и точки морфа (решение владельца в
  tokens.css не пересматривается).

## Verification

- `npx tsc --noEmit`, `npm run lint`, `make frontend-test` — зелёные.
- Feel (десктоп + телефон): фокус и ввод в поле — лейбл поднимается как
  настоящий мелкий текст, чёткий на всём морфе (сверка с кадрами Figma);
  на reduced-motion морф 150ms.

## Done when

Лейбл в приподнятом состоянии — настоящие 13px/15px на top 10px (как в
макете и до 015); `transition-all` не вернулся; контракт-тест зелёный.
