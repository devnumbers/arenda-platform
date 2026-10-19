# 035 — Раскатка примитива фото: 15 ручных сайтов, бастер контакт-детали, lazy лент

- **Status**: TODO
- **Commit**: 40d303a87
- **Severity**: MEDIUM
- **Category**: Consistency / фото-канон отображения (ADR 0065, #1275, #1229)
- **Estimated scope**: ~14 файлов, 15 raw-`<img>`-сайтов (16-й вне канонов —
  `notification-category-icon`, исключение в Target.1) + баг-фикс бастера
  на сайте 8

## Problem

После 034 канон «глиф под фото + onLoad» живёт в примитиве, но 16 сайтов
за пределами двух канон-компонентов держат собственные копии
`img + onError + useState(photoBroken)` (инвентаризация рекогносцировки,
проветрена по месту на 40d303a87); 15 из них конвертятся здесь, 16-й —
исключение (Target.1):

| # | file:line (img) | Поверхность |
| --- | --- | --- |
| 1 | `widgets/history/ui/history-feed-screen.tsx:775` | ActorFeedAvatar, лента, 24 (строки 763–786; рекогносцировка тикета звала 782–805 — дрейф) |
| 2 | `widgets/history/ui/history-filters-sheet.tsx:240` | ParticipantFilterAvatar, 36 |
| 3 | `widgets/notifications/ui/notification-detail-screen.tsx:275` | ActorAvatar (снимок уведомления) |
| 4 | `widgets/notifications/ui/notification-detail-screen.tsx:343` | PropertyCardAvatar (снимок объекта) |
| 5 | `widgets/participants/ui/participant-fragments.tsx:60` | ObjectAvatarGlyph, 44 |
| 6 | `widgets/properties/ui/PropertyCard.tsx:111` | OwnerChipAvatar, 24 |
| 7 | `widgets/contacts/ui/contact-photo-slot.tsx:105` | слот фото контакта, 96, режим показа |
| 8 | `widgets/contacts/ui/contact-detail-screen.tsx:203-208` | **баг**: фото привязанного объекта |
| 9 | `widgets/contacts/ui/contact-object-select.tsx:144` | выбор объекта |
| 10 | `widgets/payments/ui/operations-objects-select-screen.tsx:221` | SelectAvatar |
| 11 | `widgets/payments/ui/payments-sections.tsx:123` | аватар объекта платежей, 44 |
| 12 | `widgets/payments/ui/operation-create-wizard/operation-property-step.tsx:84` | шаг объекта |
| 13 | `widgets/tasks/ui/task-property-select.tsx:119` | выбор объекта задачи |
| 14 | `widgets/tasks/ui/tasks-property-select.tsx:150` | мультивыбор |
| 15 | `widgets/profile/ui/tariff/tariff-disable-screen.tsx:206` | PropertyPickRow, 44 |

Сайт 8 — единственный отход от канона: фото привязанного объекта на
карточке контакта рисуется без `onError` (битое = пустой круг, мимо
фолбэка #1275) и без бастера (мимо #1229: после правки фото контактом
владельца объект покажет прежние байты до 5 минут):

```tsx
/* contact-detail-screen.tsx:203-208 — нет onError, нет ?v=N */
{boundProperty.photoUrl ? (
  <img src={boundProperty.photoUrl} alt="" className="h-full w-full object-cover" />
) : BoundPropertyGlyph !== undefined ? ( … ) : null}
```

Второе измерение — атрибуты подачи: `decoding`/`loading`/`fetchpriority`
на пользовательских фото — 0 совпадений по фронту. Ленты и списки грузят
аватары всех смонтированных рядов сразу (лента истории — все загруженные
страницы), соревнуясь с данными.

## Target

1. Каждый сайт из таблицы — на `CirclePhoto` (034) со своим глифом и
   кругом: структура span-родителей не меняется, копии
   `useState(photoBroken)`+`onError` удаляются вместе с гардами `''`
   (гард теперь в примитиве). Поверхностные особенности сохраняются:
   у снимков уведомлений (#3–4) и `notification-category-icon`
   фолбэки свои — глиф в примитив передаётся как есть.

   Исключение: `entities/notification/ui/notification-category-icon.tsx:107`
   — фото врезано в 3D-PNG-композит эталона категории (свой клип и слои);
   механика примитива туда не ложится без переделки композита. Сайт
   остаётся на собственном `onError` (фолбэк каноничен #1275); в примечании
   докблока ссылка на примитив. Возврат — отдельным словом при ревью.

2. Сайт 8 — плюс баг-фикс: `usePropertyPhotoBuster(boundPropertyId)`
   (`features/properties/api/hooks.ts:357-366`, ключ `propertyKeys.photoBuster`,
   `shared/api/query-keys.ts:95`) и `photoDisplayUrl`; глиф — уже
   вычисленный `BoundPropertyGlyph`. Проще: заменить весь span на
   `PropertyAvatar surface="row"`? Нет — у детали контакта круг на белой
   карточке без канта (`bg-white`, не `circleIconPair.white`) — оставить
   span, внутрь `CirclePhoto`.

3. `loading="lazy"` — список потребует решения на уровне вызова: примитив
   получает опциональный `loading?: 'lazy'` (дефолт — eager, атрибут не
   пишется). Канон поверхностей: ленивые — ряды списков и лент (surface
   `card`/`row`/`feed`/`filter` у PropertyAvatar; `row` у UserAvatar;
   select-списки шитов и визардов, таблица выше #1–6, #9–15); жадные —
   hero-круги шапок и фото-слоты (surface `hero`, слот #7, аватары шапок
   истории/уведомлений). Прицел: 20–30 аватаров хаба и бесконечная лента
   перестают лить запросы стрима за пределами вьюпорта (браузерный
   rootMargin подхватывает заранее — поп-ин на быстром скролле не виден).

## Repo conventions to follow

- Примитив и его докблок — `shared/ui/design/circle-photo.tsx` (034).
- Бастер-канон — `contact-detail-screen.tsx:159-166` (зеркало для
  property-бастера), докблок `shared/lib/photo.ts:21-30`.
- ADR 0065 — same-origin стрим; ADR 0031/конвенция #1417 — не при делах
  (lazy не трогает overscroll).

## Steps

1. Круги таблицы #1–7, #9–15 → `CirclePhoto` (механика 034 уже зелёная);
   каждый сайт — отдельный микро-коммит в рамках плана, правка только
   внутренностей клипающего круга.
2. Сайт 8: buster-хук + `photoDisplayUrl` + `CirclePhoto` (глиф
   `BoundPropertyGlyph`).
3. `loading?: 'lazy'` в примитив (тест в `circle-photo.test.tsx`: проп
   прокидывается в атрибут, дефолт — атрибута нет); расставить по канону
   поверхностей из Target.3.
4. `make frontend-test`; регресс e2e лент и хабов.

## Boundaries

- Do NOT трогать `notification-category-icon.tsx` (см. Target.1).
- Do NOT переводить сайты на `next/image` — ADR 0065.
- Do NOT менять фото-слоты (`property-photo-slot`, `profile-photo-slot`) —
  их display-img (#7 — только контактный слот; property/profile-слоты
  рисуют `PropertyAvatar`/`ProfileAvatar` и приедут с 034/035 сами).
  Прогресс/retry слотов — планы 036/037, не здесь.
- Пересечение с 007/008 (pressable на рядах): PropertyCard и ряды истории
  правятся и там — на одном экране не параллелить.
- Если какой-то сайт не ложится на примитив (неожиданная структура) —
  STOP и доложить, не выкраивать.

## Verification

- **Mechanical**: `make frontend-test` зелёный; grep `onError` по
  widgets/entities — остаётся только `notification-category-icon` и
  не-фото-потребители; grep `<img` в пользовательских фото — 0 вне
  примитива и категории-иконки.
- **Feel check** (раздел #1423): лента истории на throttled-сети —
  аватары за экраном не запрашиваются (Network), при подлёте — глиф и
  проявление; карточка контакта с привязанным объектом: битое фото
  объекта → глиф типа; правка фото объекта → на контакте новый кадр
  без перезахода.
- **Done when**: 15 сайтов таблицы на примитиве; у контакт-детали бастер
  и onError; ленивая подача на рядовых поверхностях.
