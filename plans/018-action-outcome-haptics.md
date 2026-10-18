# 018 — Хаптика исходов действия (Android): модуль + проводка в каналы исходов

- **Status**: TODO
- **Commit**: 34b3ea465
- **Severity**: MEDIUM
- **Category**: Multimodal feedback (apple-design §13; тикет #1420, карта #1416)
- **Estimated scope**: 6 файлов (новые action-haptics.ts + тест, base.ts + тест, success-popup.tsx, participant-success-popup.tsx), ~90 строк

## Problem

Тактилен только тик колеса (`wheel-haptics.ts`: 8мс/40мс, Android-only);
исходы действий для рук молчат. Канон Apple §13 отводит вибрации значимые
моменты — успех/ошибка/коммит — при трёх правилах: каузальность (вибрация
на том же событии, что видимый сигнал), harmony (один кадр с визуалом),
utility (не переусыпа́ть).

**Решение владельца (14.10.2026, тикет #1420) — скоуп B: хаптика исходов
через готовые каналы, коммит без видимого сигнала не хаптится.** Вариант A
(отказ — только тик колеса) и вариант C (дополнительно коммит-тик на
onSuccess primary CTA — свип 52 экранов StickyBottomBar) отклонены: 51
вызов мутаций мимо тостов — переусыпание и дубль с уже летящим тостом.

Каналы исходов уже сошлись в немногие точки (проверено на 34b3ea465):

1. **Тосты** — все сценарии идут через `notify` (`scenarios.ts`: 38
   success / 15 error), единые точки — `base.ts`:

```ts
// shared/lib/notifications/base.ts:100-112 — current
  success: (title, options): NotificationKey =>
    showDisplacingToast(() =>
      String(
        toast.success(...
```

```ts
// shared/lib/notifications/base.ts:114 — current (перегрузки сведены в error)
  error,
```

   Путь `promise` (base.ts:166+) идёт мимо `success`/`error`-замыканий
   (внутри `toast.promise` со своими render-замыканиями) — ему хаптика
   на сеттлменте промиса отдельно.

2. **Попапы успеха** — два канонных компонента: `SuccessPopup`
   (success-popup.tsx:36 — центр уведомлений, «Прочитать все»/«Удалить
   все») и `ParticipantSuccessPopup` (participant-success-popup.tsx:16 —
   четыре экрана участников, #698; монтируется в момент показа, `Modal
   open` всегда true).

Двойных каналов на одном действии нет: попап-действия успех тостом не
дублируют (notifications-feed-screen.tsx:84-89 — onSuccess ставит попап,
ошибки уходят тостом; участники — попап без тоста). Страховка от будущих
синхронных соседей — гэп-троттлинг в модуле (канон паузы тика колеса).

Платформа: `navigator.vibrate` есть только на Android Chrome; iOS
Safari/PWA не реализует — честный no-op (тот же договор, что у тика
колеса, решение владельца 14.10; iOS-аудитория хаптики не увидит —
осознано).

## Target

**Шаг 1 — модуль** `apps/frontend/shared/ui/design/action-haptics.ts`
(рядом с `wheel-haptics.ts` — один тактильный словарь, параллельные имена
`tickRow`/`shouldTickRow` → `outcomeHaptic`/`shouldFireOutcomeHaptic`):

```ts
/** Тактильный исход действия (решение владельца 2026-10-14, тикет
 * #1420 — скоуп B: вибрируют исходы успеха/ошибки вместе с видимым
 * сигналом; коммит без сигнала не хаптится; пресс не хаптится). Канон
 * Apple §13: каузальность (тот же момент, что тост/попап) и utility
 * (только успех/ошибка — info/warning/loading молчат). Веб-вибрация
 * есть только на Android Chrome: iOS Safari не реализует
 * navigator.vibrate — честный no-op (тот же договор, что у тика колеса
 * wheel-haptics.ts). Паттерны: успех — один короткий 16мс «готово»
 * (длиннее тика детента, читается как исход, не как прокрутку); ошибка
 * — двойной [32, 60, 32], читается как отказ и не спутывается с
 * одиночками. Пауза ≥100мс между исходами: синхронные соседи одного
 * действия (тост + попап) сливаются в один тик, а не в «брр». */

/** Длительность тика успеха (мс). */
export const ACTION_SUCCESS_MS = 16;

/** Паттерн тика ошибки: вибро-пауза-вибро (мс). */
export const ACTION_ERROR_PATTERN = [32, 60, 32] as const;

/** Минимальная пауза между исходами (мс). */
export const ACTION_HAPTIC_GAP_MS = 100;

let lastFireAt: number | null = null;

/** Пора ли вибрировать: первый исход всегда, дальше — не чаще GAP. */
export function shouldFireOutcomeHaptic(
  now: number,
  lastAt: number | null,
): boolean {
  return lastAt === null || now - lastAt >= ACTION_HAPTIC_GAP_MS;
}

/** Вибрация исхода; отсутствие API — тишина (no-op, включая SSR). */
export function outcomeHaptic(outcome: 'success' | 'error'): void {
  if (typeof navigator !== 'undefined' && typeof navigator.vibrate === 'function') {
    const now = Date.now();
    if (shouldFireOutcomeHaptic(now, lastFireAt)) {
      lastFireAt = now;
      navigator.vibrate(outcome === 'success' ? ACTION_SUCCESS_MS : [...ACTION_ERROR_PATTERN]);
    }
  }
}

/** Сброс троттлинга — шов для тестов. */
export function resetOutcomeHaptic(): void {
  lastFireAt = null;
}
```

**Шаг 2 — тосты** `apps/frontend/shared/lib/notifications/base.ts`
(прецедент слоения — файл уже импортирует `@/shared/ui/toast/*`):

```ts
// base.ts:100 — target: хаптика первым стейтментом, до показа тоста —
// один кадр с появлением визуала (harmony §13)
  success: (title, options): NotificationKey => {
    outcomeHaptic('success');
    return showDisplacingToast(() =>
      ...как сейчас...
    );
  },
```

```ts
// base.ts:65-79 — target: в терминальной реализации error, до показа
function error(
  titleOrError: string | unknown,
  options?: ScenarioOptions,
): NotificationKey {
  outcomeHaptic('error');
  const title = ...
```

```ts
// base.ts: promise — target: хаптика на сеттлменте промиса (тост
// меняет вариант на тот же момент); void — новый промис никому не нужен
    void promise.then(
      () => outcomeHaptic('success'),
      () => outcomeHaptic('error'),
    );
    return showDisplacingToast(() => { ...как сейчас... });
```

`info`/`warning`/`loading` — молчат (utility §13).

**Шаг 3 — попапы успеха:**

```tsx
// shared/ui/design/success-popup.tsx — target: эффект на открытие
export function SuccessPopup({ open, onOpenChange, message }: SuccessPopupProps): JSX.Element {
  useEffect(() => {
    if (open) {
      outcomeHaptic('success');
    }
  }, [open]);
  return (
    <DialogRoot open={open} ...как сейчас...
```

```tsx
// widgets/participants/ui/participant-success-popup.tsx — target:
// монтируется в момент показа (Modal open всегда true) — эффект на маунт
export function ParticipantSuccessPopup({ title, onClose }): JSX.Element {
  useEffect(() => {
    outcomeHaptic('success');
  }, []);
  return (
    <Modal open ...как сейчас...
```

**Шаг 4 — тесты (TDD, швы pre-agreed):**

- `shared/ui/design/action-haptics.test.ts` — зеркало
  `wheel-haptics.test.ts` (тот же приём `Reflect.set(navigator,
  'vibrate', …)`): гейт (первый всегда / чаще паузы глушится / по паузе
  тикает), паттерны (success → 16, error → [32, 60, 32]), no-op без API
  и при отсутствии `navigator`, троттлинг сквозной на оба исхода
  (success, через 50мс error — тишина), `resetOutcomeHaptic` открывает.
- `shared/lib/notifications/base.test.ts` — добавить
  `vi.mock('@/shared/ui/design/action-haptics', …)` с `outcomeHaptic:
  vi.fn()` (по образцу мока ToastProvider в шапке): success/error зовут
  хаптику, info/warning/loading — нет; promise — на resolved зовёт
  `'success'`, на rejected — `'error'` (промис-фикстура + микрофлотч).
- Попапы — точка feel-check, не unit: канон тестов репо — node без DOM,
  прецедент тика колеса (план 004: модуль тестируется, интеграция в
  пикере — feel-check владельца).

## Repo conventions to follow

- Тактильный словарь и имена — по канону `wheel-haptics.ts`
  (константы-паттерны, чистый гейт, no-op-гард API).
- Решения 14.10: тик колеса — единственный прежний тактильный момент;
  скоуп B этого тикета — развитие того же принципа, не отмена.
- Порядок импортов base.ts: `@/shared/ui/design/action-haptics` — в
  группу `@/shared/ui/*` (существующая).
- e2e не расширяются: вибрация вне DOM-контракта (у колеса — так же).

## Steps

1. `action-haptics.test.ts` — красные тесты модуля по шагу 4.
2. `action-haptics.ts` — модуль по шагу 1, зелёный.
3. `base.test.ts` — красные тесты проводки; `base.ts` — шаг 2, зелёный.
4. `success-popup.tsx`, `participant-success-popup.tsx` — шаг 3.
5. `npx tsc --noEmit`, `npm run lint`, `make frontend-test`.

## Boundaries

- Do NOT хаптить `info`/`warning`/`loading` тосты и успехи мимо каналов
  (onSuccess напрямую) — скоуп B, решение владельца 14.10.
- Do NOT трогать `wheel-haptics.ts` и колёсный пикер.
- Do NOT вводить хаптику на пресс/клик кнопок — модель тикета #1420 её
  явно отвергла.
- Do NOT менять тексты/варианты/длительности тостов.
- Паттерны (16 / [32, 60, 32] / 100) — стартовые значения; правка по
  feel-check владельца — не дрейф, правится константами модуля.
- Если код в указанных местах не совпадает с цитатами (дрейф от коммита
  34b3ea465) — СТОП и отчёт, не импровизировать.

## Verification

- **Mechanical**: `cd apps/frontend && npx tsc --noEmit` — без ошибок;
  `npm run lint` — зелёный; `make frontend-test` — зелёный. Контрольный
  grep: `outcomeHaptic` — base.ts (3 точки), success-popup.tsx,
  participant-success-popup.tsx; больше нигде.
- **Feel check** (реальный Android, Chrome/PWA): сабмит формы с ошибкой —
  двойной тик в момент красного тоста; успех с тостом — одиночный;
  «Прочитать все» в центре уведомлений — одиночный с попапом; приглашение
  участника — одиночный с попапом; быстрый повторный сабмит с ошибкой —
  без слипшихся «брр» (гэп 100мс); iOS (Safari и standalone) — тишина
  без ошибок консоли; десктоп — тишина.
- **Done when**: модуль + три проводки стоят и покрыты тестами; хаптика
  только на успехе/ошибке каналов исходов; Android тикает, iOS молчит.
