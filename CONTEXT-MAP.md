# Context Map

Arenda Platform — DDD modular monolith (Go, `apps/backend/internal/`). Domain glossary is split by bounded context. See ADR 0001 (modular monolith) and ADR 0029 (rental super-context).

## Contexts

- [Identity](./apps/backend/internal/identity/CONTEXT.md) — accounts, sessions, user roles (Owner, Admin).
- [Rental](./apps/backend/internal/properties/CONTEXT.md) — properties, leases, operations, reminders. Super-context: properties + leases + notifications are tightly coupled (bidirectional domain imports); documented as one glossary until cycles are broken (ADR 0029).
- [Billing](./apps/backend/internal/billing/CONTEXT.md) — tariffs, subscriptions, payment methods, T-Kassa integration.
- [Access](./apps/backend/internal/access/CONTEXT.md) — property sharing, member roles, derived object access.
- [Audit](./apps/backend/internal/audit/CONTEXT.md) — audit log of user/admin/system actions.
- [Popups](./apps/backend/internal/popups/CONTEXT.md) — onboarding popups, popup views.

## Relationships

- **Identity → all**: Owner/Admin roles thread through every context.
- **Rental ↔ Access**: Access governs who can view/edit properties and their derived data (operations, reminders).
- **Rental → Billing**: Active property count feeds subscription tariff limits.
- **Billing → Access**: Downgrade/grace-period may suspend shared access when limits shrink.
- **All → Audit**: Every context records user/admin/system actions to the audit log.

## Shared kernel

Terms shared across contexts, documented once here:

**Owner / Собственник**: see Identity context — the user role that owns properties and manages rental data.

**Admin / Админ**: see Identity context — internal support user.

**Property access roles** (defined in shared/policy, consumed by Access + Rental + Billing):

- **Property Owner / Владелец объекта** — the user a property belongs to; sole lifecycle control (archive, delete); access cannot be revoked by members.
- **Full Access / Полный доступ** — member role: full data operations + member management.
- **Viewer / Просмотр** — member role: read-only access to all property data.
- **Suspended Access / Приостановленный доступ** — shared access hidden due to recipient tariff limit exhaustion.
