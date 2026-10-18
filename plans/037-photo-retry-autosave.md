# 037 — Retry фото autosave-профиля: файл переживает провал

- **Status**: TODO
- **Commit**: 40d303a87
- **Severity**: MEDIUM
- **Category**: Feedback / upload UX (#1230; toast-action канон)
- **Estimated scope**: 1 stage-тип + 1 экран + 1 слот + тесты lib

## Problem

Retry-модели трёх слотов разъехались (проветрено на 40d303a87):

- **staged (объект #1228, контакт #1229)** — провал переживает stage:
  тост «Не удалось загрузить фото», экран остаётся, повтор — кнопкой
  «Сохранить» (`property-edit-screen.tsx:386-388` — сброс только при
  `photoSaved`). Модель рабочая, ниже канонизируется без машинерии.
- **autosave (профиль #1230)** — провал гасит stage: файл теряется,
  круг откатывается к серверному фото, ретрай = заново открывать пикер
  и выбирать файл (`AccountScreen.tsx:184-187`):

```tsx
onError: (error) => {
  setPhotoStage(null);          // выбранный с камеры файл погиб
  notify.scenarios.profile.photoUpdateError(error);
},
```

На мобильной сети (тот случай, где загрузка падает чаще всего) это
худший ответ: файл на 3 МиБ, выбранный с камеры, нужно выбирать заново.
- **контакт-создание** — провал фото не отменяет созданную карточку,
  экран уходит (`contact-create-screen.tsx:148-158`, комментарий
  документирует почему: повторный сабмит создал бы дубль). Осознанное
  ограничение — остаётся, ретрай через правку созданной карточки.

Механика «Повторить» в тосте уже канон и не используется ни разу:
`NotificationAction { label, onPress }` (`shared/lib/notifications/types.ts:1-5`)
рендерится канонной кнопкой (`shared/ui/toast/ToastBody.tsx:57-80`,
закрывает тост по клику) — `notify.*` с `action` в вызовах — 0.

## Target

1. **Stage с памятью провала** — `MePhotoStage` получает флаг
   (`features/profile/lib/photo.ts:19-23`):

```ts
export type MePhotoStage = {
  kind: 'file';
  file: File;
  previewUrl: string;
  /** Провал прошлой попытки: круг предлагает повтор той же загрузки. */
  failed?: boolean;
};
```

`mePhotoDisplay` флаг игнорирует (превью показывается как раньше) —
тест lib дополняется веткой.

2. **Провал не гасит stage** (`AccountScreen.tsx:182-188`): stage
   остаётся с `failed: true`; тост получает действие — первый потребитель
   action-канона:

```tsx
onError: (error) => {
  setPhotoStage({ kind: 'file', file, previewUrl, failed: true });
  notify.scenarios.profile.photoUpdateError(error, {
    action: { label: 'Повторить', onPress: () => void retryUpload(file) },
  });
},
```

Ретрай — та же мутация с тем же `file` (крючок на
`useUploadMePhoto`), `onSuccess`/`onError` — те же ветки (успех гасит
stage, провал снова помечает и тостует).

3. **Аффорданс на слоте** (`profile-photo-slot.tsx`): пока
   `stage?.failed` и не busy — тап по кругу не открывает пикер, а
   повторяет загрузку (`aria-label="Повторить загрузку фото"`, новый
   проп `onRetry`); ссылка «Обновить фото» остаётся пикером (замена
   файла — легитимный путь). Кольцо полёта — план 036 (исполнить 036 →
   037). При успехе или выборе другого файла флаг умирает вместе со
   stage.

## Repo conventions to follow

- Action-канон — `shared/ui/toast/ToastBody.tsx:49-80` (докблок
  «оба закрывают тост по клику»), типы `notifications/types.ts`;
  `ErrorScenarioFn` уже принимает options (`types.ts:19-22`) — сценарий
  принимает action без расширения канона.
- Решение владельца об autosave (#1230, `AccountScreen.tsx:164-169`) —
  не пересматривается: фото применяется сразу; меняется только ответ
  на провал.
- Stage-канон staged-слотов — `features/properties/lib/photo.ts:18-20`,
  `features/contacts/lib/photo.ts:19-21` (не трогается).
- Тестовый стиль — `features/profile/lib/photo.test.ts`.

## Steps

1. TDD: `features/profile/lib/photo.test.ts` — ветка `failed: true` в
   stage: превью показывается, флаг на дисплей не влияет. Красный →
   шаг 2.
2. `MePhotoStage` + `mePhotoDisplay` — Target.1.
3. `AccountScreen` — Target.2 (stage переживает провал, тост с
   «Повторить», ретрай тем же файлом).
4. `ProfilePhotoSlot` — Target.3 (проп `onRetry`, аффорданс тапа при
   failed; докблок слота дописать: «провал держит файл, тап — повтор,
   ссылка — замена»).
5. `make frontend-test`; регресс e2e профиля.

## Boundaries

- Do NOT трогать staged-слоты (объект/контакт) — их retry уже держит
  stage; канон запишет план 038.
- Do NOT менять решение об autosave (мгновенное применение) и контракт
  `POST /me/photo`.
- Do NOT добавлять action в другие тосты — первый потребитель только
  здесь (остальные — по мере надобности, не в этом плане).
- Если stage-тип или хендлеры `AccountScreen` не совпали с цитатами
  Problem/Target — STOP и доложить.

## Verification

- **Mechanical**: тесты lib — зелёные; `make frontend-test` — зелёный.
- **Feel check** (раздел #1423): оборвать сеть на загрузке фото профиля —
  тост с «Повторить», круг держит превью; восстановить сеть — «Повторить»
  (или тап по кругу) досылает тот же файл без пикера; замена файла через
  ссылку работает; staged-объект: провал фото-части — «Сохранить»
  досылает, как раньше.
- **Done when**: провал autosave не теряет файл; action-канон тостов
  получил первого потребителя.
