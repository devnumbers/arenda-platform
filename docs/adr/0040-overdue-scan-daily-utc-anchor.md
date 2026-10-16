# ADR 0040: Overdue operation scan anchored to daily 00:00 UTC

The operation overdue worker used a 24h ticker anchored to process start, so each deploy redefined the "daily" scan time: overdue status flips and (for owners whose local 10:00 dispatch slot had already passed) overdue notifications drifted with the last deploy. We replaced the interval with a wall-clock anchor — one scan per day at 00:00 UTC, hardcoded, with no scan at startup.

00:00 UTC is the earliest instant the scan can find any new work: overdue candidacy is date-based on the server's UTC clock (`ListAllOverdueCandidates` compares `operation_date` against the UTC date), so new candidates become visible exactly at UTC midnight. Every Russian timezone at UTC+10 and further west then reaches its owner-local 10:00 reminder slot (notifications `dispatchHour`) at or after the scan — UTC+10 lands exactly on the scan instant and dispatches within the minute — so reminders fire at 10:00 local regardless of deploy times.

## Considered Options

- **Hourly interval** — would also pin dispatch at 10:00 local, but runs 24 cheap scans a day to do the work of one; rejected as needless polling.
- **Keep the 24h interval + startup scan** — rejected: the schedule keeps drifting with every deploy, which was the original complaint.
- **Per-timezone candidacy** (scan at each owner's local midnight) — would deliver 10:00-local reminders to UTC+11/UTC+12 (Magadan, Kamchatka) too; rejected as a disproportionate change: with UTC-date candidacy those zones' candidates do not exist before 00:00 UTC anyway, so they get the reminder at scan time (11:00/12:00 local, same day).

## Consequences

- A run missed because downtime spans 00:00 UTC waits for the next anchor — overdue transitions for that day are delayed by up to 24h. Accepted: deploys are 5–15 s and rarely hit midnight UTC; there is no startup catch-up by design.
- UTC+11/UTC+12 owners receive overdue reminders at 11:00/12:00 local instead of 10:00.
- `OVERDUE_OPERATION_WORKER_INTERVAL` is gone; the schedule is code, not configuration.
