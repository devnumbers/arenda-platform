---
name: pre-merge
description: Final polish for a closed multi-ticket effort — one architecture pass scoped to the branch, then a code-review loop until it converges — before the branch merges into dev.
disable-model-invocation: true
---

# Pre-merge polish

The effort is done — every ticket closed, acceptance green — and the branch waits to merge into `dev`. The tickets were built as isolated vertical slices, so per-ticket reviews never saw the whole branch: cross-ticket duplication, shallow seams between slices, and near-miss abstractions stay invisible until one pass looks at the accumulated diff. This skill is that pass — the gate a branch clears before the owner merges.

It never merges and never pushes: the effort closes here with a report, and the merge into `dev` stays the owner's explicit word.

Invoke it with the effort's map (URL or number). A map-less effort works too — the spec source is then the tickets referenced in the commit messages.

## Process

### 1. Pin the gate

- Work in the effort branch's worktree. On the main checkout — nothing to gate; stop.
- Fixed point: `git merge-base origin/dev HEAD`. Verify it resolves and `git diff <fixed-point>...HEAD` is non-empty; capture `git log <fixed-point>..HEAD --oneline`.
- Spec source, in order: the map issue and its closed tickets (fetch per `docs/agents/issue-tracker.md`); ticket references from the commit messages; ask the owner.
- A `golangci-lint` run can be poisoned by a sibling worktree's cache and fail on code that is fine — `golangci-lint cache clean` before suspecting the diff.

Done when: the fixed point resolves, the diff is non-empty, and the spec source is pinned.

### 2. Green baseline

Fast loops for the sides the branch touched (`make backend-test`, `make frontend-test`, `make admin-test`, `make tools-test`), then the full `make test`. A red suite is `/diagnosing-bugs` territory, not polish — stop and hand it there.

Done when: the full suite is green.

### 3. Architecture pass — scoped to the branch

Run `/improve-codebase-architecture`'s Explore and HTML-report steps, scoped: the Explore subagent walks the branch diff and the modules it touched — the whole codebase is out of scope here. Hunt what isolated slices produce: duplicated shapes across tickets, shallow modules with interface-heavy surfaces, the shared abstraction two tickets each half-built, missed `CONTEXT.md` vocabulary, missed `/codebase-design` terms. Apply the deletion test to suspects.

Present the candidates and grill with the owner — which to apply here, and for each pick, the shape of the deepened module and the tests that survive (that skill's grilling loop). A candidate beyond the branch's blast radius is a campaign, not a polish: file it as an issue for a future `/grill-with-docs` and record that it was filed.

Done when: the owner picked candidates — possibly zero, then skip to step 5.

### 4. Apply, one candidate at a time

Refactor under `/tdd` discipline: the suite is green before and after each candidate, behavior is preserved, each candidate is one commit. A "refactor" that changes behavior is a new ticket — file it as one instead.

Done when: every picked candidate is applied and the full suite is green.

### 5. Review gate

Run `/code-review` with the fixed point and spec source from step 1. Dispose of every finding: a documented-standard violation is fixed now; a judgement call (a baseline smell, a spec nuance) is fixed or declined with a one-line reason. Declines survive — they reach the report.

Done when: zero undisposed findings.

### 6. Converge

Fixes can surface new findings, so re-run the review gate. The gate passes when a round ends with zero undisposed findings and a green full suite. Three rounds is the ceiling: a branch still turning up new findings on the third round goes to the owner as a conscious leftover, recorded in the report.

Done when: the gate passes, or the ceiling surfaced leftovers.

### 7. Report and stop

Post the report as a comment on the map issue (the effort's tracking issue otherwise): candidates applied with their commits, per-round review verdicts, declines with reasons, issues filed for out-of-scope candidates. Link tickets by name, not bare ids. Then stop — the merge into `dev` and any push happen on the owner's word, outside this skill.
