# Context Map

Arenda Platform is a fintech platform for rental-property management: owners manage property cards; the rental records domain (leases, operations) is being redesigned from scratch and its previous implementation is fully removed (ADR 0046) — the record-keeping money vocabulary will be defined by that future domain. The platform does not move money: the only payment processing is the SaaS subscription via T-Kassa (ADR 0036).

DDD modular monolith (Go, `apps/backend/internal/`). Domain glossary is split by bounded context. See ADR 0001 (modular monolith) and ADR 0046 (leases removal; the former rental super-context of ADR 0029 no longer exists).

## Contexts

- [Identity](./apps/backend/internal/identity/CONTEXT.md) — accounts, sessions, user roles (Owner, Admin).
- [Properties](./apps/backend/internal/properties/CONTEXT.md) — property cards only: photos, contacts, attributes, archive. The leases/operations domain was removed for a full rewrite (ADR 0046).
- [Notifications](./apps/backend/internal/notifications/CONTEXT.md) — delivery channels (email, Web Push), per-channel preferences, the `subscription_grace` event with direct sending, and the grace worker.
- [Billing](./apps/backend/internal/billing/CONTEXT.md) — tariffs, subscriptions, payment methods, T-Kassa integration.
- [Access](./apps/backend/internal/access/CONTEXT.md) — property sharing, member roles, derived object access.
- [Audit](./apps/backend/internal/audit/CONTEXT.md) — audit log of user/admin/system actions (append-only; records about the removed leases/operations domain are kept as history).
- [Popups](./apps/backend/internal/popups/CONTEXT.md) — onboarding popups, popup views.

## Relationships

- **Identity → all**: Owner/Admin roles thread through every context.
- **Identity → Access**: Identity emits `UserRegistered` when a new account is created; Access consumes it to activate shares issued to a previously-unregistered email.
- **Properties ↔ Access**: Access governs who can view/edit properties.
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

**Money vocabularies** (ADR 0036, updated by ADR 0046) — never mix them:

- **Record-keeping**: empty until the new rental records domain is designed. The previous vocabulary (Операция, Доход, Расход) left the active language together with the removed domain.
- **Processing (Billing only)**: Оплата подписки, Возврат, PaymentId, CIT/MIT — the T-Kassa payment world. The «операция» in CIT/MIT definitions is a quoted provider term, not a rental record.

**Money / Деньги**: all product amounts are in Russian rubles (RUB); there is no multi-currency support. Storage, transport, and formatting rules: root `AGENTS.md` and ADR 0008.
