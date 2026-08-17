# ADR 0036: Fintech domain language: record-keeping vs payment processing

## Status

Accepted

## Context

Arenda Platform was developed without an explicit industry framing; it is now recognized as a fintech platform for rental-property finance management. The product is **record-keeping, not money movement**: owners record and plan income/expense operations by hand; the platform does not acquire, hold, or transfer tenants' money. The only payment processing is the SaaS subscription via T-Kassa in the Billing context (ADR 0007, 0010, 0016, 0017).

The two domains share overlapping words («операция», «платёж», «транзакция») with different meanings, and the collision has already bitten: `learning-records/0007-misconception-payment-vs-operation.md` documents the payment-vs-operation confusion, CIT/MIT in `billing/CONTEXT.md` were defined through the word «Операция» (reserved by Rental for money records), and the leases code uses `PaymentDay`/`nextPaymentDate` against its own glossary's avoid-list.

## Decision

Two money vocabularies are kept deliberately separate:

- **Record-keeping world (Rental)**: Операция, Регулярная операция, Доход, Расход, Категория дохода/расхода. «Платёж» and «транзакция» are banned as synonyms here. Planned and overdue operations are the accrual view; completed operations (`paid`/`received`) are the cash fact — the product combines both accounting methods.
- **Processing world (Billing, T-Kassa)**: Оплата подписки, Способ оплаты, Возврат, PaymentId, RebillId, CIT/MIT. In the T-Kassa provider section «операция» appears only as a quoted provider term and never means a Rental Операция.
- **UI exception**: «Платежи» stays the name of the user-facing section for operations — a navigation label, not a domain entity.

All amounts are in Russian rubles (RUB); multi-currency is not supported, and the term «Валюта» is not introduced preemptively. Money-handling rules (integer kopecks across all layers, no floats, formatting only at the UI layer) live in the root and per-app `AGENTS.md`; storage stays `BIGINT` kopecks (ADR 0008).

## Considered Options

- **Unified vocabulary** (one word set for both worlds) — rejected: processing semantics pollute record-keeping and reproduce the exact confusion that learning-records 0007 resolved.
- **No fintech framing** (keep generic "finance tracker" language) — rejected: the glossary then lacks the domain terms (доход/расход, метод учёта, дебиторка, аванс) that product discussions and agents already need.
- **Rename `PaymentDay`/`nextPaymentDate` in the same iteration** — rejected: this iteration is docs-only; the drift is documented in `properties/CONTEXT.md`, and the rename is a separate code task.

## Consequences

- (+) Agents and product docs get a canonical term set: new glossary entries in `properties/CONTEXT.md` (доход/расход/прибыль/денежный поток, кассовый метод/метод начисления, дебиторская/кредиторская задолженность, аванс/незаработанный доход, арендная плата) and `billing/CONTEXT.md` (Возврат), plus the two-worlds rule in the `CONTEXT-MAP.md` shared kernel.
- (+) «Финтех» carries no false promise of payment processing — the positioning states "record-keeping, not money movement", which keeps licensing/regulatory scope (115-ФЗ and the like) out of the product.
- (-) The split must be actively maintained: T-Касса provider terms keep their native wording under an explicit disclaimer, and the known `PaymentDay` drift remains in code until a dedicated rename.
- CapEx/OpEx/depreciation and investor metrics (NOI, cap rate) are intentionally not in the glossary yet; they arrive lazily via `/domain-modeling` when reports/analytics land.

## See also

- [`docs/adr/0008-subscription-lifecycle.md`](./0008-subscription-lifecycle.md) — money as `BIGINT` kopecks.
- [`docs/adr/0029-rental-superc-context-temporary.md`](./0029-rental-superc-context-temporary.md) — the rental super-context whose glossary holds the record-keeping terms.
- `learning-records/0007-misconception-payment-vs-operation.md` — the confusion this split prevents.
- `reference/glossary-core.html` — the owner's finance-course glossary these terms draw on.
