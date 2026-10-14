# 008 — Микропоявления хаба: «+»-фейд, LiveValue на суммах, иллюстрации; константа кривой LiveValue

- **Status**: DONE (14.10.2026; механика 001 уточнена при приёмке: доезд — criticalSettle без перелёта, резинка справа — margin-расширение диапазона, слева — трансформ; детали в коде и DESIGN.md §8)
- **Commit**: efe10fd46
- **Severity**: LOW + упущенные возможности A/B/C (решение владельца: делать всё)
- **Category**: Missed opportunities; когезия (#10)
- **Estimated scope**: 4 файла

## Problem

- Кнопка «+» телепортируется в шапку хаба после резолва `propertiesQuery`
  (`payments-global-screen.tsx:122,192`; loading-скелетон её не рисует
  намеренно — `payments-loading.tsx:32-34`).
- Суммы карточек хаба меняются без оживления при рефетче/realtime-инвалидации
  (`entities/payment/ui/payment-card-button.tsx:74`): канон C существует
  (`shared/ui/live-value`), в payments не потребляется ни разу.
- Иллюстрации пустых секций появляются разом без проявления
  (`payments-global-screen.tsx:683-689`).
- `shared/ui/live-value/LiveValue.tsx:297-299` — вторая рукописная копия
  кривой `--dl-ease` строкой в WAAPI (вынужденная: WAAPI не читает `var()`),
  молча разъедется при смене токена.

## Target

1. **«+»**: однократный fade-in 250мс при появлении — CSS-анимация
   `fade-in 250ms var(--dl-ease) both` (keyframes `opacity: 0 → 1`) классом на
   кнопке; fade-семейство канона — reduced-motion не гейтится.
2. **Суммы**: обёртка суммы в `<LiveValue valueKey={amountKopecks} mode="flash">`
   (режим B — плотные плитки, только вспышка 1200мс; дим/кроссфейд на плитке
   168px избыточен). Вспышка на коробке с компенсирующими полями — механика
   готового компонента. valueKey — число копеек: смена суммы = свап.
3. **Иллюстрации**: однократный fade-in 350мс (`--dl-duration-move`) той же
   утилитой.
4. **Кривая**: `LiveValue.tsx` — константа `DL_EASE_WAAPI =
   'cubic-bezier(0.32, 0.72, 0, 1)'` с комментарием-привязкой к `--dl-ease`
   (единственное место дублирования, байт-в-байт токен).

## Repo conventions

- Канон C/B: DESIGN.md §8, `shared/ui/live-value` (consumers: participants,
  properties — взять экземпляр потребления для сверки пропсов).
- Keyframes дизайн-слоя живут в `app/globals.css`; класс-хук — в компоненте.

## Steps

1. `LiveValue.tsx`: константа кривой + комментарий.
2. `payment-card-button.tsx`: сумму в `LiveValue mode="flash"` (компонент
   entities → shared разрешён).
3. `globals.css`: keyframes `dl-fade-in`; `payments-global-screen.tsx`:
   класс на кнопке «+» (250мс) и на иллюстрации плейсхолдера (350мс).

## Boundaries

- Смена целых поверхностей (скелетон→контент, пусто→данные) НЕ анимировать — канон.
- LiveValue только на сумму; заголовок/подпись не трогать.

## Verification

- Механика: lint/build/test; e2e `payments-hub.spec.ts`.
- Feel-check: загрузка хаба — «+» мягко проявляется; правка суммы платежа на
  подэкране → возврат на хаб — сумма вспыхивает (режим B, ~1200мс→250мс
  при reduced-motion токенами).
