# 015 — text-field: плавающий лейбл на transform, без transition-all и layout-свойств

- **Status**: TODO
- **Commit**: 2d6925b76
- **Severity**: MEDIUM
- **Category**: Performance (аудит анимаций вне колеса, карта #1416 / тикет #1418)
- **Estimated scope**: 1 файл (text-field.tsx), ~8 строк

## Problem

Плавающий лейбл базового поля всех форм (`titleIn` — визарды операций и
платежей, контакты, профиль, фильтры) анимирует layout-свойства через
`transition-all` и играет при каждом фокусе каждого поля:

```tsx
// apps/frontend/shared/ui/design/text-field.tsx:188-192 — current
                className={cn(
                  'pointer-events-none absolute left-0 text-content-secondary transition-all duration-300',
                  'top-[19px] text-base leading-[18px]',
                  'peer-focus:top-[10px] peer-focus:text-[13px] peer-focus:leading-[15px]',
                  'peer-[:not(:placeholder-shown)]:top-[10px] peer-[:not(:placeholder-shown)]:text-[13px] peer-[:not(:placeholder-shown)]:leading-[15px]',
                )}
```

Канон (AUDIT.md §5): анимировать только `transform`/`opacity`;
`top` — в явном списке layout-триггеров (layout+paint+composite);
«`transition: all` — always a finding». Частота: заполнение записей —
ежедневная работа, форма = 8+ полей = 8+ анимаций на визит
(категория «Purpose & frequency» аудита отмечает поверхность как
частую; длительность 300ms и сам морф — задокументированное решение
(tokens.css:100) и не пересматриваются — меняется механизм).

## Target

Один morф вместо трёх layout-свопов: лейбл живёт в «плавающей» позиции
(top-[10px], 16px), покоя стоит transform-смещением вниз; всплытие —
`translateY(-9px) scale(0.8125)` (13/16) с origin в левый верхний угол:

```tsx
// apps/frontend/shared/ui/design/text-field.tsx:188-192 — target
                className={cn(
                  'pointer-events-none absolute left-0 top-[10px] origin-top-left text-base text-content-secondary transition-transform duration-300 ease-apple motion-reduce:duration-150',
                  'translate-y-[9px]',
                  'peer-focus:-translate-y-[9px] peer-focus:scale-[0.8125]',
                  'peer-[:not(:placeholder-shown)]:-translate-y-[9px] peer-[:not(:placeholder-shown)]:scale-[0.8125]',
                )}
```

Расчёт соответствия текущему виду:

- позиция: покой `top 10 + 9 = 19px` (как сейчас), float `top 10px` (как
  сейчас);
- кегль: `16 × 0.8125 = 13px` (как сейчас; строки масштабируются вместе с
  кеглем — межстрочный 18px → визуально ≈14.6px против текущих 15px,
  расхождение 0.4px на однострочном лейбле незаметно);
- ошибка в лейбле (span `top-[10px] text-[13px]`, строка 182) — статична,
  не меняется.

Честный трейд-офф: масштабированный текст чуть легче нативного 13px
(штрихи масштабируются). На 13px это на грани различимости; feel-check —
обязателен. Если владелец забракует чёткость — СТОП и отчёт (альтернатива
«scope-ить transition-all до top/font-size/line-height» хуже канону и не
предлагается).

## Repo conventions to follow

- Длительность 300ms — задокументирована в комментарии токена
  (`tokens.css:100`, «Плавающий лейбл поля — 300ms»); кривая — дефолт
  `--dl-ease` (явно писать `ease-apple`, раз `transition-all` снят и
  дефолт больше не единственный источник).
- Reduced — `motion-reduce:duration-150` (канон §8; у лейбла его не было —
  попутно закрывается дыра).
- Прецедент transform-переходов поля — skeleton/switch на
  `transition-transform`.

## Steps

1. `text-field.tsx:188-192` — классы по Target.
2. Прогнать формы с `titleIn` (контакт-форма, визарды, профиль) по Feel
   check.

## Boundaries

- Do NOT трогать `titleOut`-вариант и статичный error-span (строка 182).
- Do NOT менять длительность 300ms и точки морфа (19px↔10px, 16→13).
- Do NOT трогать autogrow-textarea (`text-field.tsx` вне лейбла) — вне
  класса (конвенция плана 006).
- Если код не совпадает с цитатами — СТОП и отчёт.

## Verification

- **Mechanical**: `npx tsc --noEmit`, `npm run lint`, `make frontend-test`
  — зелёные.
- **Feel check** (телефон + десктоп, контакт-форма из 8 полей):
  - фокус/ввод: морф визуально идентичен коммиту (тот же путь 19→10px,
    16→13), скорость та же;
  - блик чёткости: floated-лейбл на 13px выглядит резким на iPhone
    (масштаб vs нативный кегль) — при замечании СТОП и отчёт;
  - автозаполнение/вставка значения (placeholder-shown → float) — морф
    тот же;
  - DevTools → Rendering → prefers-reduced-motion: reduce — морф 150ms;
  - DevTools → Performance: запись фокуса — без layout в треке (только
    composite/paint).
- **Done when**: grep `transition-all` по text-field.tsx — 0; морф
  визуально неотличим; reduced укорочен.
