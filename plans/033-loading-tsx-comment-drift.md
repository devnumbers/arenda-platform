# 033 — 8 loading.tsx: докблоки без Suspense-границы

- **Status**: TODO
- **Commit**: 2a37f5bc5
- **Severity**: LOW
- **Category**: Documentation drift (DESIGN.md §7 #609, §15)
- **Estimated scope**: 8 файлов, только комментарии

## Problem

Восемь `loading.tsx` несут шаблонный докблок «…тот же кадр, что и fallback
Suspense-границы страницы», хотя Suspense-границы в их `page.tsx` нет —
экран держит pending-гейт сам, `loading.tsx` покрывает только окно
RSC/await `params|searchParams`. Поведение корректно (кадр совпадает —
проветрено аудитом #1422), документация ссылается на несуществующую
границу.

Текущий блок во всех восьми (пример — `operations/new/loading.tsx:4-6`):

```tsx
/**
 * Route-loading сегмента (#609): архетип живёт в зоне виджета — тот же кадр, что и fallback Suspense-границы страницы.
 */
```

Файлы (проверка «страница без Suspense» — по аудиту #1422, группа
«голая страница»):

| Файл | Чем на деле держится кадр |
| --- | --- |
| `app/(screens)/contacts/[contactId]/edit/loading.tsx:5` | in-screen гейт `contact-edit-screen.tsx:125-130` |
| `app/(screens)/contacts/search/loading.tsx:5` | hint-кадр `contact-book-search-screen.tsx:84-85` |
| `app/(screens)/operations/new/loading.tsx:5` | гейт визарда `operation-create-wizard-screen.tsx:83-95` |
| `app/(screens)/payments/favorites/loading.tsx:5` | гейт `payments-favorites-screen.tsx:260-264` |
| `app/(screens)/payments/objects/loading.tsx:5` | гейт `payments-objects-screen.tsx:145-150` |
| `app/(screens)/payments/overdue/loading.tsx:5` | гейт `payment-overdue-global-screen.tsx:93-113` |
| `app/(screens)/tasks/[ruleId]/edit/loading.tsx:5` | гейт `task-edit-screen.tsx:186-194` |
| `app/(screens)/tasks/new/loading.tsx:5` | гейт `task-create-screen.tsx:186-189` |

Эталон точной формулировки — `properties/search/loading.tsx:5` (описывает
совпадение с пустым/начальным кадром экрана, а не с несуществующей
границей) и `profile/notifications/loading.tsx:6-9`.

## Target

Единая замена шаблона — на честную формулировку группы (в каждом файле
своя вторая половина предложения, см. таблицу):

```tsx
/**
 * Route-loading сегмента (#609): архетип живёт в зоне виджета — тот же
 * кадр, что у экрана в фазе загрузки; Suspense-границы в page.tsx нет,
 * экран держит pending-гейт сам.
 */
```

## Repo conventions to follow

- Докблок `loading.tsx` описывает, чем на деле держится кадр сегмента:
  «тот же кадр, что и fallback Suspense-границы страницы» — только для
  маршрутов с реальной Suspense-границей в `page.tsx` (группы
  Suspense+ServerPrefetchBoundary и plain-Suspense); для «голых» страниц —
  формулировка про внутренний гейт экрана (эталон — `properties/search/
  loading.tsx:5` и `profile/notifications/loading.tsx:6-9`).

## Steps

1. В восьми файлах заменить одну строку докблока (выше) на формулировку
   Target; для `contacts/search` вторая фраза — «…кадр, что у экрана в
   idle-фазе поиска (hint)»; для `operations/new` — «…кадр, что у первого
   шага визарда (форма шага = скелетон, §7 #607)».
2. Поведение не меняется — проверка компиляции достаточна.

## Boundaries

- Do NOT менять код/разметку — только комментарии.
- Do NOT трогать `loading.tsx` с корректными докблоками (маршруты
  Suspense+ServerPrefetchBoundary и plain-Suspense группы, а также
  `properties/search` и `profile/notifications`).
- Если файл на месте исполнения не совпал с таблицей (дрейф от коммита) —
  STOP и доложить.

## Verification

- **Mechanical**: `make frontend-test` — зелёный (компиляция).
- **Feel check**: неприменим — поведение не меняется, план правит только
  комментарии; пользовательская поверхность отсутствует.
- **Done when**: grep `fallback Suspense-границы` по `app/**/loading.tsx`
  возвращает только маршруты с реальной Suspense-границей в `page.tsx`.
