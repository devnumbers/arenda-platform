# 031 — ErrorCard: фидбек повтора + унификация локальных копий

- **Status**: TODO
- **Commit**: 2a37f5bc5
- **Severity**: MEDIUM
- **Category**: Feedback / button async-канон (#1119, #1114; решение владельца 30.09)
- **Estimated scope**: 1 канон-компонент + тест + ~8 экранов

## Problem

Канон кнопок (#1420): `Button` глушит сам — `disabled || loading`,
`aria-busy`, `data-loading` (`shared/ui/design/button.tsx:83,95,104-107`).
Канон error-состояний (§7, #697) — `ErrorCard` с «Повторить». Но `ErrorCard`
не пробрасывает фазу повтора (`shared/ui/design/error-card.tsx:9-34`,
текущий код):

```tsx
export type ErrorCardProps = {
  readonly title: string;
  readonly onRetry: () => void;
  readonly className?: string;
};

export function ErrorCard({ title, onRetry, className }: ErrorCardProps): JSX.Element {
  return (
    <section className={cn('mx-6 rounded-card bg-surface-muted px-6 py-6', className)}>
      <h2 className="text-xl font-semibold leading-6 text-content">{title}</h2>
      <p className="mt-2 text-sm leading-4 text-content-secondary">
        Проверьте подключение и попробуйте еще раз
      </p>
      <div className="mt-4">
        <Button size="small" variant="secondary" onClick={onRetry}>
          Повторить
        </Button>
      </div>
    </section>
  );
}
```

На повторе карточка остаётся, кнопка активна, фидбека ноль — на всех
канон-потребителях. Машинный канон уже умеет фазу (`loading` Button),
граница #1114 для retry записана решением #1420: «retry-кнопки
error-состояний подставляют isFetching — это работа этой кнопки в полёте».
Живой прецедент проводки — легаси `PropertiesErrorState`:
`PropertiesPage.tsx:213` → `onRetry={() => void refetch()} isLoading={isFetching}`.

Вторая часть — построчные копии ErrorCard-разметки (докблок самого канона:
«Экстракция #697: одинаковые карточки… жили поэкранно» — копии вернулись):

- `widgets/notifications/ui/notifications-states.tsx:70-91` —
  `NotificationsErrorCard` (копия с дефолтным тайтлом);
- `widgets/tasks/ui/tasks-of-property-screen.tsx:307-327` — `TasksStateCard`
  (копия, без className);
- `widgets/payments/ui/payments-of-property-screen.tsx:247-265` и родня —
  `PaymentsStateCard`-обёртки (копия с `hint`/`action`-слотами);
- `widgets/properties/ui/properties-search-screen.tsx:145-161` —
  `PropertiesSearchErrorCard` (копия с фиксированным тайтлом).

Копии не получат фидбек повтора, если их не свести к канону.

## Target

`ErrorCard` получает опциональную фазу повтора (Button-канон #1119 без
изменений внутри кнопки):

```tsx
export type ErrorCardProps = {
  readonly title: string;
  readonly onRetry: () => void;
  /** Повтор в полёте (#1114, граница из решения #1420): кнопка глушится
   * машинно (disabled || loading, aria-busy), текст видим, спиннера нет. */
  readonly loading?: boolean;
  readonly className?: string;
};

export function ErrorCard({ title, onRetry, loading = false, className }: ErrorCardProps): JSX.Element {
  return (
    <section className={cn('mx-6 rounded-card bg-surface-muted px-6 py-6', className)}>
      <h2 className="text-xl font-semibold leading-6 text-content">{title}</h2>
      <p className="mt-2 text-sm leading-4 text-content-secondary">
        Проверьте подключение и попробуйте еще раз
      </p>
      <div className="mt-4">
        <Button size="small" variant="secondary" onClick={onRetry} loading={loading}>
          Повторить
        </Button>
      </div>
    </section>
  );
}
```

Копии сводятся к канону с сохранением публичного API (тонкая делегация —
вызовы не трогаем):

```tsx
/* notifications-states.tsx — target */
export function NotificationsErrorCard({
  onRetry,
  className,
  title = 'Не удалось загрузить уведомления',
  loading = false,
}: {
  readonly onRetry: () => void;
  readonly className?: string;
  readonly title?: string;
  readonly loading?: boolean;
}): JSX.Element {
  return <ErrorCard title={title} onRetry={onRetry} loading={loading} className={className} />;
}
```

(`TasksStateCard`, `PaymentsStateCard`, `PropertiesSearchErrorCard` — та же
делегация со своими тайтлами/слотами; у `PaymentsStateCard` слоты
`hint`/`action` сохраняются в пропсах и передаются в канон: `hint` →
канон-пояснение пока передаётся словом «Проверьте подключение…» — сверить
вызовы: если хоть один передаёт нестандартный hint, оставить разметку
карточки в каноне расширяемой — см. Boundaries.)

## Repo conventions to follow

- Async-канон кнопок — `CODING_STANDARDS.md` «Mutations and buttons» +
  `button.tsx:83-107` (глушение, `aria-busy`, `data-loading`, текст видим,
  спиннера нет — решение владельца 30.09).
- Граница retry — `PropertiesPage.tsx:213` (проводка `isFetching`).
- Тестовый стиль — `shared/ui/design/button.test.ts`.

## Steps

1. TDD: создать `shared/ui/design/error-card.test.ts` по образцу
   `button.test.ts`: (a) рендер с `loading` → кнопка `disabled`,
   `aria-busy="true"`, `data-loading="true"`, текст «Повторить» видим;
   (b) без `loading` → кнопка активна; (c) клик вызывает `onRetry`.
   Сначала тест — красный, затем шаг 2 — зелёный.
2. `error-card.tsx` — Target-код выше.
3. Проводка фазы на канон-потребителях: в каждом экране, где ErrorCard
   рендерится с `onRetry={() => void query.refetch()}`, добавить
   `loading={query.isFetching}`. Список проветренных мест: 
   `widgets/history/ui/history-feed-screen.tsx:526`, 
   `widgets/participants/ui/participants-list-screen.tsx:269`
   (`ErrorCard` с `className="mt-6"`), `widgets/contacts/ui/contacts-of-property-screen.tsx:180`,
   `widgets/contacts/ui/contact-book-search-screen.tsx:82` —
   через `ContactsErrorCard` (прокинуть `loading` в обёртку
   `features/contacts/ui/contact-search-states.tsx:18-28`), 
   `widgets/notifications/ui/notifications-feed-screen.tsx:201`,
   `widgets/notifications/ui/notification-detail-screen.tsx:104-108` —
   через `NotificationsErrorCard`. Правило: `loading={query.isFetching}`
   того запроса, чей `refetch` стоит в `onRetry`.
4. Свести копии к канону (делегация): `notifications-states.tsx`,
   `tasks-of-property-screen.tsx` (`TasksStateCard`), 
   `properties-search-screen.tsx` (`PropertiesSearchErrorCard`), 
   `payments-of-property-screen.tsx` (`PaymentsStateCard`; если у вызовов
   встречаются нестандартные `hint` — остановиться на делегации без hint и
   доложить).
5. `make frontend-test`; регресс e2e (`support-modal`, ленты).

## Boundaries

- Do NOT менять `Button` — его канона достаточно.
- Do NOT трогать легаси без карточки (`widgets/profile/ui/PaymentList.tsx:73-96`
  — текст+кнопка; `widgets/rentals/ui/rental-history-screen.tsx:114-131` —
  то же) и легаси с другой темой (`widgets/properties/ui/PropertiesErrorState`
  — белая карточка с рамкой, `role="alert"`): это визуальные решения —
  отдельное слово владельца при ревью тикета; план их не переносит.
- Do NOT трогать ветвления ошибок (план 030) и хвосты (план 032).
- Порядок: файлово независим от 026–030; 032 требует этот план
  (`loading` ErrorCard) — исполнить до 032.
- Если код не совпал с цитатами — STOP и доложить.

## Verification

- **Mechanical**: новый тест ErrorCard — зелёный; `make frontend-test` —
  зелёный.
- **Feel check**: оборвать сеть, открыть раздел с ошибкой, нажать
  «Повторить» — кнопка глушится на время запроса (текст остаётся, спиннера
  нет), по завершении либо данные, либо карточка с активной кнопкой;
  спам-клик не даёт параллельных запросов (Network: один).
- **Done when**: кнопка «Повторить» на всех канон-карточках глохнет в
  полёте; построчных копий разметки ErrorCard в widgets не осталось.
