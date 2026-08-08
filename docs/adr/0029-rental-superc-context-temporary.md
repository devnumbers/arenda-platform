# ADR 0029: (properties, leases, notifications) — временно один контекст

## Status

Accepted

## Context

ADR 0001 провозглашает DDD modular monolith с изолированными bounded contexts, границы которых выдерживаются через package dependencies и application ports. Фактически три контекста — `properties`, `leases`, `notifications` — образуют bidirectional циклы на уровне domain и application слоёв:

- `properties ↔ leases`: импортируют domain и application типы друг друга (Property, Lease, Operation).
- `leases ↔ notifications`: импортируют domain и application типы друг друга (Lease, Reminder, NotificationPreference).

Это означает, что ubiquitous language этих трёх контекстов неразделим на уровне компиляции: термины Property, Lease, Operation, Reminder переплетены. Попытка описать их как три отдельных ubiquitous language противоречит реальной архитектуре.

## Decision

Документировать (properties, leases, notifications) как один супер-контекст "rental" в `CONTEXT-MAP.md` и в per-context `CONTEXT.md` — до тех пор, пока циклы не будут разорваны через ports (interfaces в consuming-контексте, реализация в source-контексте). Разрыв отложен как задача для `/improve-codebase-architecture`.

## Consequences

- Per-context CONTEXT.md для этого кластера — один файл, не три.
- Разрыв циклов через ports — будущая deepening opportunity (кандидат на `/improve-codebase-architecture`).
- Контексты `identity`, `billing`, `audit`, `popups`, `access` — действительно изолированы и описываются отдельными per-context CONTEXT.md.
- ADR 0001 остаётся в силе как цель; этот ADR фиксирует временное отклонение.
