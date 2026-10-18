# 036 — Прогресс загрузки фото: XHR-транспорт с onprogress + кольцо на круге

- **Status**: TODO
- **Commit**: 40d303a87
- **Severity**: MEDIUM
- **Category**: Feedback / upload UX (ADR 0065; слоты #1228/#1229/#1230)
- **Estimated scope**: 1 транспорт-хелпер + 1 кольцо design-слоя + 3 API-функции + 3 слота

## Problem

Ровно три слота загрузки (четвёртого нет: `type="file"` — 3 вхождения,
`FormData` — 3 API-функции), и ни в одном нет прогресса: отправка —
голый `fetch` (`shared/api/client.ts:18-23`), `onprogress`/
`onUploadProgress`/`ReadableStream` по фронту — 0 совпадений. Файл до
5 МиБ на мобильной сети летит секунды — десятки секунд: круг просто
глушится (`busy`), превью стоит неподвижно, работает ли отправка —
неизвестно; повторный тап ничего не скажет (кнопки заглушены машинно —
канон #1119, здесь корректен, но нем).

`fetch` не отдаёт upload-progress; стриминг запроса (`duplex: 'half'`)
— без Safari/WebKit. Единственный канонический механизм — XHR
`xhr.upload.onprogress`.

## Target

1. **Транспорт** — `shared/api/upload.ts`, единственное место XHR в
   продукте; семантика ошибок — та же, что у fetch-транспорта, через
   переиспользование `parseTransportResponse`:

```ts
/* shared/api/upload.ts — эскиз */
export async function uploadFile<T>(
  path: string,
  file: File,
  onProgress?: (percent: number) => void,
): Promise<T> {
  // XHR, POST `/api${path}`, body = FormData с полем `file`.
  // xhr.upload.onprogress → Math.round((loaded / total) * 100), гард
  // lengthComputable; сетевой провал → ApiError('network_error', …) с тем
  // же текстом, что client.ts:25; ответ → new Response(xhr.response,
  // { status: xhr.status }) → parseTransportResponse<T> (problem+json,
  // 204 — как в transport-wire.ts:34-44).
}
```

2. **Кольцо** — `PhotoProgressRing` в design-слое: SVG-окружность поверх
   круга (`absolute inset-0`), `stroke-dashoffset = f(percent)`, цвет
   `--color-primary`, фон-дорожка полупрозрачная; при 100% держит полный
   круг до факта успеха/отказа (сервер ещё обрабатывает). Без текста
   процентов (круг 96/64 — мелко), без спиннера (кнопочный канон #1119 —
   «спиннера нет»).

3. **Проводка** — переменные фото-мутаций получают `onProgress`
   (tanstack-переменная — один объект, замыкание пишует state экрана):

```ts
/* features/profile/api/hooks.ts — useUploadMePhoto: сигнатура */
useGuardedMutation<User, ApiError, { file: File; onProgress?: (p: number) => void }>
/* features/properties, features/contacts — то же c { id, file, onProgress? } */
```

Слоты получают `progress: number | null` (null — нет кольца): профиль —
кольцо поверх data-URL-превью всё время мутации (autosave); staged
(объект/контакт) — во время `photoBusy`-фазы сабмита (кольцо на круге,
кнопка «Сохранить» глушится как сегодня). DELETE — без кольца (быстро).
Превью-стейдж не гасится до факта (сегодняшняя семантика сохраняется).

## Repo conventions to follow

- Ошибки — `shared/api/problem.ts` + `transport-wire.ts:34-44`
  (переиспользовать, не дублировать).
- Тост отказа — существующие сценарии (`scenarios.ts:75,137,169`), не
  новые.
- Токены — `tokens.css:101-103` (`--dl-ease` на росте дуги не нужен —
  dashoffset без transition, шаги и так частые).
- Тестовый стиль — `features/properties/api/queries.test.ts:34-76`
  (FormData-контракт) + jsdom-XHR для прогресса.

## Steps

1. TDD: `shared/api/upload.test.ts` на мок-XHR: (a) путь и FormData
   (поле `file`); (b) onprogress отдаёт проценты по событиям; (c) ответ
   200 → распарсенный JSON; (d) problem+json 400 → ApiError как у
   client; (e) сетевой провал → `network_error` с каноническим текстом.
   Красный → шаг 2.
2. `upload.ts` — Target.1; проводка API-функций
   (`features/properties/api/queries.ts:124-136`,
   `features/contacts/api/queries.ts:159-170`,
   `features/profile/api/hooks.ts:241-248`) на `uploadFile` — контракты
   эндпоинтов не меняются (тесты queries.test — зелёные с дополнением
   onProgress-ветки).
3. TDD: `photo-progress-ring.test.tsx` — рендер процентов в
   stroke-dashoffset; null — не рендерится. Красный → шаг 4.
4. `PhotoProgressRing` — Target.2; экспорт из design-index.
5. Проводка хуков (Target.3) и трёх слотов: `profile-photo-slot.tsx`
   (кольцо поверх превью), `property-photo-slot.tsx` и
   `contact-photo-slot.tsx` (кольцо на photoBusy-фазе сабмита; экраны
   держат `photoProgress`-стейт и гасят его в onSuccess/onError).
6. `make frontend-test`; регресс e2e (визарды с фото-шагами).

## Boundaries

- Do NOT трогать `client.ts`/`server-client.ts` — XHR только в `upload.ts`
  и только для трёх фото-эндпоинтов.
- Do NOT менять серверный контракт (multipart `file`, ответ = сущность).
- Do NOT рисовать кольцо на удалении и на пречеке 5 МиБ.
- Retry-семантика слотов — план 037, исполнить 036 → 037 (оба правят
  профильный слот).
- Если XHR в jsdom-тестах не даёт управлять прогрессом без костылей —
  вынести событийную часть в тестируемую чистую функцию, не тащить
  полифилы.
- Если API-функции/хуки не совпали с цитатами шага 2 — STOP и доложить.

## Verification

- **Mechanical**: новые тесты upload/ring — зелёные; `make frontend-test`
  — зелёный.
- **Feel check** (раздел #1423, throttled-сеть): выбрать фото профиля —
  кольцо растёт по прогрессу и держит 100 до ответа; провал сети — тост,
  кольцо гаснет; staged-объект: «Сохранить» с фото — кольцо на круге,
  кнопка глушится, по успеху — тост и круг с новым фото.
- **Done when**: все три слота показывают ход отправки; 0 fetch-загрузок
  файлов мимо `uploadFile`.
