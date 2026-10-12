# ADR 0061: Action History (Product Action Journal)

## Status

Accepted (grilling ticket [#706](https://github.com/devnumbers/arenda-platform/issues/706), map [#704](https://github.com/devnumbers/arenda-platform/issues/704), 2026-09-22); amended 2026-10-09 (карта [#1324](https://github.com/devnumbers/arenda-platform/issues/1324), решения #1315–#1317: флип active↔maintenance журналируется, новый шаблон строки с `details`, живые ноги актёра, статус «Текущие/Удалённые» — §3, §4, §6, §7)

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
- **The active↔maintenance flip is recorded** (amended 2026-10-09, карта #1324, протокол D2 — the original settled MVP behavior was silence): the manual maintenance-status change (PATCH /properties/{id}, `cmd.Status`) writes one row inside the same transaction — `property.maintenance_started` («Изменение») entering maintenance, `property.maintenance_finished` («Выполнение») leaving it. The write guard is the **factual** before≠after inequality: the transition validator admits from==to (the idempotent PUT), and a no-op action still writes no row. A mixed PATCH (fields + status) writes two rows — the per-leg precedent of the rental completion pair; the SSE feed publication follows the journal (`journaled`), a status-only edit dirties the feed exactly when it wrote a row (ADR 0062). A generic `property.status_changed` id was considered and declined — the two dedicated maintenance ids carry the semantics, and the archive keeps its own ids.
- **One row per user action (amended 2026-10-12, карта #1324 — решение владельца: единые change-строки)**: every edit of one entity is one generic change row «<Сущность> изменён(а): [значение-ссылка]» — field-level headers («Название объекта изменено», «Адрес объекта изменён») are gone; what exactly changed travels in the `details` (§6), one gray line per changed field. For the property this folds the former granular ids — `renamed` / `address_changed` / `description_changed` / `attributes_changed` — into `property.updated`, and folds `photo_added` / `photo_deleted` into the same row («Фото: добавлено / удалено»); the granular ids stop being written (legacy rows keep theirs, §6). Amounts also travel in `context` on the money rows (created/updated rules and operations), in kopecks.
- `property.deleted` is never recorded: rows cascade with the property in the same transaction, so the row would be dead-born.
- Identity/profile/session changes, notifications, push subscriptions, popups are not property data — out of the journal.

### 4. Dictionary

Seven kinds (the mockup filter groups), stable `action` ids, base-action mapping, and the row-snapshot composition (exact Russian copy is approved at screen walkthroughs; all templates are passive voice — «Объект изменён: …», «Платёж оплачен» is NOT used for operations, the canon is «Операция оплачена», owner decision 2026-09-22). **Amended 2026-10-12 (карта #1324): every change row reads the generic «<Сущность> изменён(а): [значение-ссылка]» header with per-field diffs in `details`; the field-level headers and the granular property ids are retired from writes** (legacy rows keep theirs and render as recorded, §6):

| Kind | Action | Base | Row snapshot |
|---|---|---|---|
| Объект | property.created | added | название |
| | property.updated (any edit: название/адрес/описание/тип/характеристики/фото; absorbs renamed / address_changed / description_changed / attributes_changed / photo_added / photo_deleted) | changed | новое имя в заголовке; по строке details на каждое изменившееся поле, фото — «Фото: добавлено / удалено» |
| | property.maintenance_started / maintenance_finished (amended #1324) | changed / completed | — |
| | property.pinned / unpinned | changed | — |
| | property.archived / unarchived | changed | — |
| Аренда | rental.created | added | срок («11 месяцев») или дата начала («1 ноября») в заголовке; арендатор и период с годами — в details (D3) |
| | rental.updated (incl. extension) | changed | новый срок/дата в заголовке; диффы «Арендатор» и «Период: «с … по …» → «с … по …»» — в details |
| | rental.completed | completed | имя арендатора |
| | rental.deleted | deleted | имя арендатора |
| Платежи | payment.created / deleted | added / deleted | название |
| | payment.updated | changed | новое название в заголовке; диффы Название/Сумма/День списания/Дата окончания/Категория — в details |
| | payment.paused / resumed | changed | название |
| Операции | operation.created | added | название (дата — в context) |
| | operation.paid | completed | название (срок — в context) |
| | operation.deleted («Отменённая операция») | deleted | название (дата — в context) |
| Контакты | contact.created / deleted | added / deleted | ФИО |
| | contact.updated | changed | новое ФИО в заголовке; диффы всех полей полными значениями (ФИО/Телефон/Email/Мессенджер/Роль/Заметка) — в details |
| | contact.moved (cross-property move — a pair of rows, one per end) / contact.bound / contact.unbound (bind / unbind — the single existing end) | changed | ФИО |
| Задачи | task_rule.created / deleted | added / deleted | название |
| | task_rule.updated | changed | новое название в заголовке; диффы Название/Дата — в details |
| | task.completed | completed | название (срок — в context) |
| | task.uncompleted | changed | название |
| | task.completed_cleared (bulk) | deleted | N |
| Участники | member.invited / member.added | added | имя или email |
| | member.role_changed | changed | имя в заголовке; «Роль: «…» → «…»» — в details |
| | member.removed / invitation_cancelled / left / participant_removed (bulk) | deleted | имя или email |

Mapping rulings: archive/unarchive, pause/resume, task uncomplete, rental extension → **changed**; operation deletion (the «Отменённая» tombstone) → **deleted**; the contact rebind family (move/bind/unbind, ticket #856) → **changed**. The rebind anchors on every object the card is bound to at action time, so §3's «an unbound card writes no rows» stays about cards without any binding: a cross-property move writes the **pair of rows** — one per end (the source's «Контакт перенесён на другой объект: » leg first, then the destination's «Контакт перенесён с другого объекта: ») — mirroring the §3 per-leg pattern; a bind or an unbind owns a single end and writes its row there. A mixed unbind + details edit (the ФИО changed in the same PATCH) folds into the one source row — one manual action, one row per object — the ФИО traveling as the action-time snapshot, not as a name delta. Link targets (ticket #713): links point at the entity's existing page, so the completed/uncompleted task occurrence rows link the **producing rule** (the only task page is the rule's edit screen; a hard-deleted rule — `RuleID` nil — leaves the row linkless, the deleted-entity rule).

### 5. Privacy and money

- Recorded: entity label snapshots (names, titles, ФИО), participant names and **emails** (member emails are visible to all members since the 2026-09-20 decision, and research #705 designed email search in), attribute values, roles, periods. Amended #1324: the details' catalog labels (`AttributeChange.Label`, the RU field label the properties service resolves) travel in `context` alongside the raw values — metadata of the detail lines, not a second copy of the values.
- **Amended 2026-10-12, карта #1324 (решение владельца: «пояснения существуют, чтобы показать, что изменилось»)**: the `details` lines carry the **real values of every changed field** — contact PII included (телефон, email, мессенджер) and money («Сумма: «5 000 ₽» → «6 000 ₽»»). The former blanket bans («never a phone», «amounts never in segments») now hold **for the header only**: the header stays «действие + значение-ссылка» with no amounts and no PII beyond the snapshot it already carried. The exposure surface does not grow: the feed is visible to the property's participants, who can already open the contact cards and payment rules the diffs quote; the journal adds no audience, only history. Searchable folds the details in (§6) — «что видишь, то и ищешь» holds: a phone or an amount on screen is findable, by design.
- Still never recorded: nothing secret per the ADR 0020 PII discipline (tokens, raw payloads).
- Amounts: in the `details` as formatted rubles («5 000 ₽», the history domain's own formatter — display formatting left the frontend-only rule when the diffs moved server-side) and in `context` jsonb as kopecks integers for future use. Headers carry no amounts.

### 6. Row text = server-built segments

The server is the **single source of row text**: at write time the Recorder builds `segments` — an ordered array of `{text}` runs where a run may carry `link: {kind, id}` pointing at the target entity (the blue spans in the mockups). The frontend renders segments verbatim and wraps linked runs; at mapping time it strips the bare date tails (« (срок DD.MM.YYYY)» of operations and tasks, the rental period/date tails) the backend stopped writing in audit #876 — the strip cleans rows recorded before that (`entities/history/model/date-tails.ts`, `mappers.ts`). `searchable` is their plain-text concatenation (plus actor name/email). Frontend-owned templates were rejected: they would diverge from the searchable text («ищется не то, что показано») and duplicate the copy layer.

**New template (amended 2026-10-09, карта #1324, протокол D1–D3; change rows unified 2026-10-12)**: the rows read «действие: [сущность]» in the header — the link rides the header value — and the diffs moved into `details` (migration 000156, jsonb, NULL at legacy rows): ready gray lines built by the same catalog builders at write time (`[]string`, rendered verbatim, no links; amounts and PII allowed per the amended §5). The reading path carries no formatting catalog: what the builder assembled is what renders. Searchable folds the details in: **segments plain text + each detail line + actor name + actor email**; legacy rows keep the old formula byte-for-byte and are **not rebuilt** — «что видишь, то и ищешь» holds for both epochs (a legacy row renders the header alone, so its honest searchable is the header + actor; rebuilding would make old rows findable by text that is not on screen). The exact Russian copy of the extended catalog is approved at screen walkthroughs.

The actor is never inside the row text: it is a payload field, grouped as a header by the frontend (date → object → actor; per-scope screens pin one group level).

### 7. Read API

- `GET /history` — flat list, keyset `(created_at, id) DESC`, **bidirectional cursor** (`before_*` / `after_*` pairs, precedent `ListPaidOperationsGlobal`), `limit`; filters: `date_from/date_to`, `actions[]` (base), `kinds[]`, `actor_ids[]`, `property_ids[]`, `q` (search). The same endpoint serves all four screens: general feed, object history (`property_ids` = one), participant actions (`actor_ids` = one), participant actions within one object (`actor_ids` and `property_ids` = one each, AND-combined — ticket #841). Search is a single always-OR predicate — prefix-FTS (Russian morphology) OR ILIKE-trgm (substrings, emails) — evaluated on both legs of the same input (research #839; routing retired by #842: any routing is intent prediction). No «found N» counter. Grouping is frontend work.
- **Day grouping vs period filter — two calendars, accepted**: the feed's day chips («Сегодня»/«Вчера») group by the **viewer's local calendar day** (frontend `dateToIsoLocal`, #624 canon), while the `date_from`/`date_to` period filter is matched by the server as **UTC days** — from at midnight UTC, to exclusive +24h (the admin-audit period-filter canon). The divergence is bounded to the edge hours of a day — a night entry sits under the «Сегодня» chip yet outside an applied «сегодня» period — and accepted, by the precedent of the client-side «сегодня» of the payments projection (#453, `shared/lib/calendar.ts`).
- `GET /history/filters` — filter-sheet options: participants of the scope = **current members ∪ all actors present in the scope's journal** (revoked actors remain filterable), with names/emails/avatars; objects of the scope with name/address/photo. The N/M counters are computed client-side. (Deviation since #708: users carry no avatars in the platform yet — the participants options carry id, display name, email, first_name, is_owner, role and status; avatars joined the contract in #1286, ADR 0065.) **Live actor legs (amended #1324, D5; the ADR 0065 pattern)**: `actor_first_name`/`actor_last_name` on feed items read through the users JOIN at feed time — the card shows today's profile; the display-name composition («Имя Фамилия» / «(Вы)» / «Пользователь», the snapshot fallback by `actor_id IS NULL`) is frontend work — the server ships the parts, never mixing the composed snapshot into a bare name. **Participant status (D6)**: `status` = `current | removed`, computed at read time by `bool_or(is_live)` over the scope CTE — owner and membership legs (suspended included) are live, journal-only actors are not; re-adding a participant revives them automatically; no new table, nothing materialized.
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
- (~) `audit_log`, its Recorder, and its glossary entry stay as-is; the Audit context `GLOSSARY.md` _Avoid_ list no longer bans «история действий».

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
