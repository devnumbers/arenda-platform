# ADR 0061: Action History (Product Action Journal)

## Status

Accepted (grilling ticket [#706](https://github.com/devnumbers/arenda-platform/issues/706), map [#704](https://github.com/devnumbers/arenda-platform/issues/704), 2026-09-22)

## Context

Property Sharing 2.0 (map #692) made participants first-class readers of a property's data, and the product now needs a user-facing «История действий» feed: every manual user action on a property recorded unconditionally, surviving member revocation and self-exit, dying with the property, readable by owners and members (viewer included), with search over years of accumulated volume.

The existing `audit_log` (ADR 0020) does not fit, and map #704 explicitly forbids building on it: it is a support/compliance journal read only in the admin panel, its PII policy strips the human-readable snapshots the feed needs (only `property.created` carries a name), it never stores amounts or emails, and its vocabulary («Журнал действий / Audit Log») is admin-facing.

Search over large volumes was settled by research ticket #705: neither pg_trgm nor FTS alone meets the requirements (measured); the hybrid — a `searchable` column materialized at write time with two GIN indexes — does. Numbers (1.5M rows, Postgres 18): frequent terms 92–161 ms via FTS vs 353–656 ms via trgm, indexes 17 MB + 124 MB, write overhead +8–13% (FTS) / +60–70% (trgm) at a human write pace.

One term collision had to be resolved: the Audit context glossary listed «история действий» as an _Avoid_ synonym of the audit log, while the product feature (and the mockups) are called «История действий».

## Decision

### 1. Separate bounded context and table

A new bounded context `internal/history` owns the product journal: table `action_journal`, a recording port, and the read API. Term split is canonical:

- **Журнал действий / Audit Log** — the admin-panel audit journal (ADR 0020), untouched.
- **История действий / Action History** — the product feed (this ADR), context «История».

### 2. Schema

Sketch (migration and names finalized in ticket #707; search design per research #705):

```sql
CREATE TABLE action_journal (
    id           uuid PRIMARY KEY,               -- app-generated UUIDv7 (ADR 0019)
    property_id  uuid NOT NULL REFERENCES properties (id) ON DELETE CASCADE,
    actor_id     uuid REFERENCES users (id) ON DELETE SET NULL,
    actor_role   text NOT NULL CHECK (actor_role IN ('owner','full_access','viewer')),  -- shared/policy mapper, ADR 0020 pattern
    actor_name   text NOT NULL DEFAULT '',       -- snapshot of the actor's display name at action time
    actor_email  text NOT NULL DEFAULT '',       -- snapshot of the actor's email (searchable; visible to members)
    kind         text NOT NULL CHECK (kind IN ('property','rental','payment','operation','contact','task','member')),  -- the seven mockup filter groups (§4)
    action       text NOT NULL,                  -- stable dotted id, e.g. property.renamed, operation.paid
    base_action  text NOT NULL CHECK (base_action IN ('added','changed','completed','deleted')),  -- filter + icon + color
    segments     jsonb NOT NULL,                 -- server-built row text: [{text, link?: {kind, id}}]
    searchable   text NOT NULL,                  -- segments plain text + actor_name + actor_email, materialized at write
    search_tsv   tsvector GENERATED ALWAYS AS (to_tsvector('russian', searchable)) STORED,
    context      jsonb NOT NULL DEFAULT '{}'::jsonb,  -- structured extras (amounts in kopecks, dates, old/new values)
    created_at   timestamptz NOT NULL            -- supplied by the application
);

CREATE INDEX ... ON action_journal (property_id, created_at DESC, id DESC);
CREATE INDEX ... ON action_journal (actor_id,    created_at DESC, id DESC);
CREATE INDEX ... ON action_journal (kind,        created_at DESC, id DESC);
CREATE INDEX ... ON action_journal USING gin (searchable gin_trgm_ops);
CREATE INDEX ... ON action_journal USING gin (search_tsv);
```

### 3. Recording

- Unconditional for all objects (map chart decision); written **in the action's transaction** through a Recorder port — ADR 0020 fail-safe canon; role of the actor resolved through `internal/shared/policy` (each owning module maps it, as audit does).
- One row per manual user action; **bulk operations write one row per affected property** (multi-object invite, participant bulk-remove, completed-journal cleanup), mirroring the audit per-leg pattern.
- **Every row anchors to a property** (`property_id NOT NULL`): mutations without an object binding — the property-less task book (ADR 0052), an unbound contact card — write no rows.
- Snapshots of human-readable labels are taken at action time and never re-resolved: the row survives renaming and deletion of the entity (a deleted entity leaves a row without a link).
- **Only manual user actions are recorded in MVP.** System writes are NOT recorded — including the slot coordinator's suspend/reactivate and invitation activation on registration (owner decision 2026-09-22, against the agent's recommendation; accepted cost: the member feed does not explain limit-driven disappearances). Auto-ticks (auto-payments, auto-tasks) stay excluded per the map charter.
- **Noise excluded**: payment favorite flag, favorite-order save, invitation resend. Photo add/delete and property pin are recorded. A no-op action writes no row: an idempotent re-pin of an already pinned object, and a completed-journal clear that removed nothing.
- **The active↔maintenance flip is not recorded**: the manual maintenance-status change (PATCH /properties/{id}, `cmd.Status`) writes no row in MVP — a status-only edit is a silent no-op, a mixed PATCH records only the other changed groups. A dedicated `property.status_changed` action id was considered and declined — the dictionary keeps no id for the flip, and silent no-entry is the settled MVP behavior.
- **One row per user action**: a PATCH touching several field groups at once gets one `property.updated` row naming the groups; single-group edits get their precise ids. Amounts travel in `context` on the money rows (created/updated rules and operations), in kopecks.
- `property.deleted` is never recorded: rows cascade with the property in the same transaction, so the row would be dead-born.
- Identity/profile/session changes, notifications, push subscriptions, popups are not property data — out of the journal.

### 4. Dictionary

Seven kinds (the mockup filter groups), stable `action` ids, base-action mapping, and the row-snapshot composition (exact Russian copy is approved at screen walkthroughs; all templates are passive voice — «Название объекта изменено: …», «Платёж оплачен» is NOT used for operations, the canon is «Операция оплачена», owner decision 2026-09-22):

| Kind | Action | Base | Row snapshot |
|---|---|---|---|
| Объект | property.created | added | название |
| | property.renamed / address_changed / description_changed / attributes_changed | changed | old → new (name) / label + value |
| | property.updated (several field groups in one edit) | changed | the changed groups' labels |
| | property.photo_added / photo_deleted | added / deleted | — |
| | property.pinned / unpinned | changed | — |
| | property.archived / unarchived | changed | — |
| Аренда | rental.created / updated (incl. extension) | added / changed | имя арендатора + период |
| | rental.completed | completed | имя арендатора |
| | rental.deleted | deleted | имя арендатора |
| Платежи | payment.created / updated / deleted | added / changed / deleted | название |
| | payment.paused / resumed | changed | название |
| Операции | operation.created | added | название + дата |
| | operation.paid | completed | название + срок |
| | operation.deleted («Отменённая операция») | deleted | название + дата |
| Контакты | contact.created / updated / deleted | added / changed / deleted | ФИО (updated — old → new) |
| | contact.moved (cross-property move — a pair of rows, one per end) / contact.bound / contact.unbound (bind / unbind — the single existing end) | changed | ФИО |
| Задачи | task_rule.created / updated / deleted | added / changed / deleted | название |
| | task.completed | completed | название + срок |
| | task.uncompleted | changed | название |
| | task.completed_cleared (bulk) | deleted | N |
| Участники | member.invited / member.added | added | имя или email |
| | member.role_changed | changed | имя, роль old → new |
| | member.removed / invitation_cancelled / left / participant_removed (bulk) | deleted | имя или email |

Mapping rulings: archive/unarchive, pause/resume, task uncomplete, rental extension → **changed**; operation deletion (the «Отменённая» tombstone) → **deleted**; the contact rebind family (move/bind/unbind, ticket #856) → **changed**. The rebind anchors on every object the card is bound to at action time, so §3's «an unbound card writes no rows» stays about cards without any binding: a cross-property move writes the **pair of rows** — one per end (the source's «ушёл на другой объект» leg first, then the destination's) — mirroring the §3 per-leg pattern; a bind or an unbind owns a single end and writes its row there. A mixed unbind + details edit (the ФИО changed in the same PATCH) folds into the one source row — one manual action, one row per object — the ФИО traveling as the action-time snapshot, not as a name delta. Link targets (ticket #713): links point at the entity's existing page, so the completed/uncompleted task occurrence rows link the **producing rule** (the only task page is the rule's edit screen; a hard-deleted rule — `RuleID` nil — leaves the row linkless, the deleted-entity rule).

### 5. Privacy and money

- Recorded: entity label snapshots (names, titles, ФИО), participant names and **emails** (member emails are visible to all members since the 2026-09-20 decision, and research #705 designed email search in), attribute values, roles, periods.
- Never recorded: contact phone numbers; nothing secret per the ADR 0020 PII discipline (tokens, raw payloads).
- Amounts: never in segments and therefore never searchable; stored in `context` jsonb as kopecks integers for future use. Display formatting stays frontend (`formatMoneyKopecks`).

### 6. Row text = server-built segments

The server is the **single source of row text**: at write time the Recorder builds `segments` — an ordered array of `{text}` runs where a run may carry `link: {kind, id}` pointing at the target entity (the blue spans in the mockups). The frontend renders segments verbatim and wraps linked runs; `searchable` is their plain-text concatenation (plus actor name/email). Frontend-owned templates were rejected: they would diverge from the searchable text («ищется не то, что показано») and duplicate the copy layer.

The actor is never inside the row text: it is a payload field, grouped as a header by the frontend (date → object → actor; per-scope screens pin one group level).

### 7. Read API

- `GET /history` — flat list, keyset `(created_at, id) DESC`, **bidirectional cursor** (`before_*` / `after_*` pairs, precedent `ListPaidOperationsGlobal`), `limit`; filters: `date_from/date_to`, `actions[]` (base), `kinds[]`, `actor_ids[]`, `property_ids[]`, `q` (search). The same endpoint serves all four screens: general feed, object history (`property_ids` = one), participant actions (`actor_ids` = one), participant actions within one object (`actor_ids` and `property_ids` = one each, AND-combined — ticket #841). Search is a single always-OR predicate — prefix-FTS (Russian morphology) OR ILIKE-trgm (substrings, emails) — evaluated on both legs of the same input (research #839; routing retired by #842: any routing is intent prediction). No «found N» counter. Grouping is frontend work.
- **Day grouping vs period filter — two calendars, accepted**: the feed's day chips («Сегодня»/«Вчера») group by the **viewer's local calendar day** (frontend `dateToIsoLocal`, #624 canon), while the `date_from`/`date_to` period filter is matched by the server as **UTC days** — from at midnight UTC, to exclusive +24h (the admin-audit period-filter canon). The divergence is bounded to the edge hours of a day — a night entry sits under the «Сегодня» chip yet outside an applied «сегодня» period — and accepted, by the precedent of the client-side «сегодня» of the payments projection (#453, `shared/lib/calendar.ts`).
- `GET /history/filters` — filter-sheet options: participants of the scope = **current members ∪ all actors present in the scope's journal** (revoked actors remain filterable), with names/emails/avatars; objects of the scope with name/address/photo. The N/M counters are computed client-side. (Deviation since #708: users carry no avatars in the platform yet — the participants options carry id, display name, email, first_name, is_owner and role; avatars join the contract when the platform grows them.)
- Authorization: property owner and members (viewer included — chart decision #9) via the derived-access rule (ADR 0028); suspended access — inaccessible; no access — private 404.
- **No detail endpoint**: the row payload (segments, actor, property, entity ids, context) is complete; «detail pages» are the existing entity pages reached through segment links (ticket #713); a deleted entity leaves a linkless row.

### 8. Retention: unbounded

Entries live while the property lives; the only deletion channel is the property CASCADE. No age- or size-based capping (owner decision 2026-09-22). Research numbers keep this comfortable for years (1.5M rows ≈ 0.8 GB table + ~141 MB indexes; write path human-paced); rotation/partitioning is revisited by a separate ADR if volume ever demands it.

## Consequences

- (+) Product feed semantics stay clean and separate from the admin audit; the term clash in the glossaries is resolved («Журнал действий» = audit, «История действий» = this context).
- (+) Snapshots make rows readable forever, independent of renames, deletions, and membership changes.
- (+) Search meets all measured requirements (morphology, substrings, emails, multi-word) at predictable latencies.
- (−) One more write per mutation with GIN overhead (+8–70% on batch inserts — microseconds at a human pace, inside an already-open transaction).
- (−) The feed cannot explain limit-driven access disappearances (system rows excluded) — accepted for MVP.
- (−) Row copy lives in Go templates; wording changes are code changes.
- (~) `audit_log`, its Recorder, and its glossary entry stay as-is; the Audit context `CONTEXT.md` _Avoid_ list no longer bans «история действий».

## Rejected alternatives

- **Building on `audit_log`** — forbidden by the map charter; lacks snapshots, emails, amounts-in-context, and product reading paths.
- **Frontend-owned text templates** (structured fields, client assembles) — search/text divergence and a duplicated copy layer.
- **Age- or size-based retention cap** — contradicts «search finds years-old records»; revisit together with partitioning if ever needed.
- **Recording system access transitions in MVP** — considered and declined by the owner for MVP (see Recording).

## See also

- [ADR 0020](./0020-audit-log.md) — the audit journal this is deliberately separate from; the Recorder/fail-safe canon reused here.
- [ADR 0028](./0028-object-data-access-model.md) — derived object access: who may read a property's history.
- [ADR 0019](./0019-uuid-v7-app-generated-ids.md) — application-generated UUIDv7 ids.
- Research report `docs/research/2026-09-15-history-search.md` (branch `research/history-search`, ticket #705) — search measurement and schema rationale.
- Map [#704](https://github.com/devnumbers/arenda-platform/issues/704), tickets #706–#713.
