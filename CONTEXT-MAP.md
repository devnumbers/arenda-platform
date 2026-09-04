# Context Map

Arenda Platform is a fintech platform for rental-property management: owners manage property cards; the record-keeping of recurring income and expenses per property is designed in the Payments context (ADR 0047), built on the prototype domain model after the former leases domain was fully removed (ADR 0046). The platform does not move money: the only payment processing is the SaaS subscription via T-Kassa (ADR 0036).

DDD modular monolith (Go, `apps/backend/internal/`). Domain glossary is split by bounded context. See ADR 0001 (modular monolith) and ADR 0046 (leases removal; the former rental super-context of ADR 0029 no longer exists).

## Contexts

- [Identity](./apps/backend/internal/identity/CONTEXT.md) — accounts, sessions, user roles (Owner, Admin).
- [Properties](./apps/backend/internal/properties/CONTEXT.md) — property cards only: photos, attributes, archive. The leases/operations domain was removed for a full rewrite (ADR 0046).
- [Rentals](./apps/backend/internal/rentals/CONTEXT.md) — rental tenancy of a property: period, terms, payment day, utilities, deposit (the clean-slate successor of the removed leases domain, ADR 0046). Each rental manages exactly one rent payment in the Payments context.
- [Contacts](./apps/backend/internal/contacts/CONTEXT.md) — the owner's contact book: cards of people for a property (plumber, management company, concierge); the property link is optional (ADR 0051).
- [Payments](./apps/backend/internal/payments/CONTEXT.md) — payment rules and operation occurrences per property: income/expense record-keeping, auto pay, overdue debt, pauses (ADR 0047).
- [Tasks](./apps/backend/internal/tasks/CONTEXT.md) — task rules and task occurrences per property: manual to-do tracking; overdue and «undated» are computed states, the completed journal survives rule deletion (ADR 0051).
- [Notifications](./apps/backend/internal/notifications/CONTEXT.md) — delivery channels (email, Web Push), per-channel preferences, the `subscription_grace` event with direct sending, and the grace worker.
- [Billing](./apps/backend/internal/billing/CONTEXT.md) — tariffs, subscriptions, payment methods, T-Kassa integration.
- [Access](./apps/backend/internal/access/CONTEXT.md) — property sharing, member roles, derived object access.
- [Audit](./apps/backend/internal/audit/CONTEXT.md) — audit log of user/admin/system actions (append-only; records about the removed leases/operations domain are kept as history).
- [Popups](./apps/backend/internal/popups/CONTEXT.md) — onboarding popups, popup views.

## Relationships

- **Identity → all**: Owner/Admin roles thread through every context.
- **Identity → Access**: Identity emits `UserRegistered` when a new account is created; Access consumes it to activate shares issued to a previously-unregistered email.
- **Properties ↔ Access**: Access governs who can view/edit properties.
- **Contacts → Properties**: a contact lives in its owner's contact book; the property link is optional (nullable `property_id`). Deleting a property — either ADR 0025 mode — nulls the link; the contact survives in the book (ADR 0051). Property-bound contacts are visible/editable to members by the property access roles (ADR 0028), as with payments; contacts without a property are owner-only.
- **Properties → Payments**: a payment and its operations belong to exactly one property (`property_id`); access to them follows the property access roles (ADR 0028). Property lifecycle interplay (archive/delete) is decided with the payments schema (ADR 0047, ticket #446).
- **Properties → Rentals**: a rental belongs to exactly one property (`property_id`); access to it follows the property access roles (ADR 0028).
- **Rentals → Payments**: a rental manages exactly one rent payment (income, category `rent`): creating, editing terms, extending, completing, and deleting the rental atomically drive the payment; the payment cannot be deleted past the rental.
- **Rentals → Contacts**: a rental may reference a contact from the owner's book as the tenant; deleting the contact nulls the reference (ADR 0051).
- **Properties → Tasks**: a task rule and its tasks belong to exactly one property (`property_id`); access to them follows the property access roles (ADR 0028). Archive is read-only for tasks (the tick skips archived properties); deletion is a total cascade (ADR 0051).
- **Properties → Billing**: Active property count feeds subscription tariff limits.
- **Billing → Access**: Downgrade/grace-period may suspend shared access when limits shrink.
- **All → Audit**: Every context records user/admin/system actions to the audit log.

## Shared kernel

Terms shared across contexts, documented once here:

**Owner / Собственник**: see Identity context — the user role that owns properties and manages rental data.

**Admin / Админ**: see Identity context — internal support user.

**Owner/Admin roles** (canonical home: `internal/shared/actor`, ADR 0034; mirrored by shared/policy for authorization and identity/domain for the account model):

- **Owner / Собственник** — see Identity context; the user role that owns properties and manages rental data.
- **Admin / Админ** — see Identity context; internal support user.

**Property access roles** (defined in shared/policy, consumed by Access + Properties + Billing):

- **Property Owner / Владелец объекта** — the user a property belongs to; sole lifecycle control (archive, delete); access cannot be revoked by members.
- **Full Access / Полный доступ** — member role: full data operations + member management.
- **Viewer / Просмотр** — member role: read-only access to all property data.
- **Suspended Access / Приостановленный доступ** — shared access hidden due to recipient tariff limit exhaustion.

**Money vocabularies** (ADR 0036, updated by ADR 0046 and ADR 0047) — never mix them:

- **Record-keeping (Payments)**: Платёж (правило), Операция (вхождение), Доход/Расход, Автоплатёж, Просрочка, Долг, Пауза, Категория, Форма оплаты — the world defined by the Payments context (ADR 0047). «Платёж» returned to the active language as the rule; «транзакция» stays banned here.
- **Processing (Billing only)**: Оплата подписки, Способ оплаты, Возврат, PaymentId, CIT/MIT — the T-Kassa payment world. The «операция» in CIT/MIT definitions is a quoted provider term, not a Payments Операция. «Способ оплаты» (card) is Billing-only — the Payments-side term is «Форма оплаты». UI-copy exception: the mobile payment-edit screen carries the mockup's field label «Способ оплаты» for its Форма оплаты field (Figma 705:10034, owner decision 2026-08-31); the domain term remains «Форма оплаты».

**Money / Деньги**: all product amounts are in Russian rubles (RUB); there is no multi-currency support. Storage, transport, and formatting rules: root `AGENTS.md` and ADR 0008.
