# ADR 0012: Operation Status Model

## Status

Accepted

## Context

Operations used to be implicitly completed once they were created. This made it impossible to distinguish money that had actually moved from money that was only planned, and the system could not tell whether a past-due expense or income had really been paid or received.

We need to support three business needs:

1. Plan future income and expenses separately from realized cash flow.
2. Surface past-due operations that still require action.
3. Send reminders for operations that have not been completed by their due date.

## Decision

Introduce four statuses for an operation:

- `pending` — planned income or expense, not yet completed.
- `overdue` — a `pending` operation whose date has passed and which is still not completed.
- `paid` — a completed expense operation.
- `received` — a completed income operation.

A newly created operation starts as `pending`. The owner completes an operation through an explicit action; the terminal status depends on the operation type (`paid` for expenses, `received` for income).

A background worker scans `pending` operations whose date has passed and transitions them to `overdue`. This transition happens once per operation. When an operation becomes `overdue`, a reminder is created and delivered through the existing reminders subsystem, which handles retry, audit, and SMS notification.

## Consequences

- The dashboard separates actual cash flow from planned or overdue obligations.
- New operations are created as planned by default, so historical reporting does not treat them as completed automatically.
- Completing an operation is an explicit step, which better matches the owner’s workflow.
- Overdue notifications reuse the existing reminder infrastructure, including retry and audit trails.
- Recurring operations continue to generate `pending` operation instances for future dates.
