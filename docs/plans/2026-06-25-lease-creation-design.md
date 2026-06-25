# Страница создания аренды — Design

## Goal

Реализовать `/leases/new` как двухшаговый wizard создания аренды 1‑в‑1 по макетам Figma, с использованием HeroUI v3 и существующих проектных UI‑компонентов.

## Constraints

- Аренда создаётся только со страницы объекта; в текущем коде отдельной страницы объекта ещё нет, поэтому временно добавляем кнопку «Сдать» на карточку свободного объекта в `/properties`.
- `property_id` передаётся через query‑параметр: `/leases/new?propertyId=<uuid>`.
- Арендатор пока не создаётся в этом flow (`tenant_contact_id` не передаётся).
- Период аренды в API не передаётся — бэкенд всегда считает его ежемесячным; в UI отображаются только даты начала/конца.

## Architecture

- **Single URL**, состояние шага хранится в `sessionStorage` (как у `/properties/new`).
- **Layout**: тот же centered container (`max-width: 560px`), что и у создания объекта.
- **Header**: переиспользуем паттерн `PropertyCreateHeader` — стрелка назад, centered title «Создание аренды», крестик, бейдж «1 из 2» / «2 из 2», segmented progress bar.
- **Wizard**: клиентский компонент `LeaseCreateWizard`, читает `propertyId` из `searchParams`, загружает объект через `useProperty`, хранит черновик в `sessionStorage`, валидирует и сабмитит через `useCreateLease`.
- **Date picking**: `HeroUI DatePicker` (`@heroui/react/date-picker`) для выбора начала/конца аренды.
- **Payment day**: custom `PaymentDayPicker` — на десктопе popover с сеткой 1–31, на мобильных bottom drawer (можно использовать `@heroui/react/drawer`).
- **Money inputs**: reuse `TextField` с `type="number"`, значения вводятся в рублях и конвертируются в копейки (`*100`) перед отправкой.

## Components

| Component | Location | Purpose |
|-----------|----------|---------|
| `LeaseCreateWizard` | `widgets/leases/ui/LeaseCreateWizard.tsx` | Контейнер wizard, логика шагов, submit, success. |
| `LeaseCreateHeader` | `widgets/leases/ui/LeaseCreateHeader.tsx` | Шапка с title, back/cancel, badge, progress. |
| `LeasePriceStep` | `widgets/leases/ui/LeasePriceStep.tsx` | Шаг 1: арендная плата и залог. |
| `LeaseDatesStep` | `widgets/leases/ui/LeaseDatesStep.tsx` | Шаг 2: день оплаты, дата начала, дата конца. |
| `LeaseSuccessStep` | `widgets/leases/ui/LeaseSuccessStep.tsx` | Экран успеха с иконкой Key 96×96, кнопки «Добавить позже» и «Добавить арендатора». |
| `PaymentDayPicker` | `widgets/leases/ui/PaymentDayPicker.tsx` | Пикер дня оплаты 1–31. |
| `useLeaseCreateDraft` | `widgets/leases/lib/use-lease-create-draft.ts` | sessionStorage‑hook черновика. |

## Data Flow

1. Пользователь нажимает «Сдать» на карточке свободного объекта → `/leases/new?propertyId=...`.
2. `LeaseCreateWizard`:
   - Если `propertyId` отсутствует/невалиден → редирект `/properties`.
   - Загружает объект `useProperty(propertyId)`; если объект не найден или занят → редирект `/properties`.
   - Инициализирует `draft.step = 1`.
3. Step 1: ввод `rentAmount` (required) и `depositAmount` (optional). Кнопка «Продолжить» disabled пока rent пуст.
4. Step 2: выбор `paymentDay` (required), `startDate` (required), `endDate` (optional). Кнопка «Создать аренду» disabled пока не заполнены обязательные поля.
5. Submit: конвертация рублей → копейки, форматирование дат в `YYYY-MM-DD`, `POST /leases`.
6. Success: показан экран успеха; очистка sessionStorage.

## Validation

- `rentAmount` > 0.
- `depositAmount` ≥ 0 (optional).
- `paymentDay` ∈ 1..31.
- `startDate` ≤ `endDate` (если end указана).
- Бэкенд валидирует `ErrOpenLeaseExists` — показывать ошибку пользователю.

## Routes

- Add `leaseNew: '/leases/new'` to `apps/frontend/shared/config/routes.ts`.
- Add temporary entry button in `PropertyCard` for `occupancy === 'free'` linking to `${ROUTES.leaseNew}?propertyId=${property.id}`.

## Responsive

- Desktop: content max-width 560px, centered, padding `96px 0`.
- Tablet: padding `84px 40px 96px`.
- Mobile: padding `84px 20px 96px`, primary button fixed/sticky at bottom.
- Success card: rounded 32px, centered icon, stack buttons on mobile, row on desktop.

## Testing / Verification

- `npm run lint` — 0 errors.
- `npm run build` — successful.
- `npm run generate:api` if OpenAPI unchanged (no backend changes expected).
- Playwright screenshots mobile/desktop for step 1, step 2, success.
