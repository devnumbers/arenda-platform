---
name: pre-merge
description: Final gate for a finished multi-ticket effort — the whole polish runs as one dynamic workflow (architecture pass scoped to the branch, a review loop that auto-fixes every finding until none remain, full test suites), then integrates fresh dev, merges the branch into local dev and reports to the map issue. Never pushes.
disable-model-invocation: true
---

# Pre-merge gate (dynamic workflow)

The effort is done — every ticket closed, acceptance green — and the branch waits in its worktree to merge into `dev`. The tickets were built as isolated vertical slices, so per-ticket reviews never saw the whole branch: cross-ticket duplication, shallow seams, and doc-rot stay invisible until one pass looks at the accumulated diff. This skill is that pass, and it runs as **one dynamic workflow** (the `/workflow` machinery): the workflow reviews, fixes, tests, integrates fresh `dev`, and — when everything is clean — merges into `dev` and posts the report. Invoke it with the effort's map (URL or number); a map-less effort works too (the spec source then comes from ticket references in the commit messages).

The owner's involvement is three deliberate moments: confirming the run, answering escalations, and pushing.

Boundaries that never move:

- **Never pushes.** Push to `dev` deploys stage; it stays the owner's explicit word.
- **Never tears down.** Worktree removal, stand shutdown (`compose down -v`), slot teardown — outside the workflow, per `docs/agents/parallel-dev.md`, on the owner's word.
- **Merges only when clean.** Zero blocking findings (rounds 1–2: any severity blocks; from round 3: high/medium block, lows become recorded leftover) plus green full suites on the branch and on merged `dev`. Anything less is a blocked report, not a merge.
- **Never aborts a merge, never invents behavior** — the `/resolving-merge-conflicts` rules, baked into the integrator persona.

## Preparation (live session, before the workflow)

1. Load the `dynamic-workflows` skill via the Skill tool — `CreateWorkflow` refuses to run without it.
2. Pin the gate. The effort branch must sit in its worktree (invoked on the main checkout's branch means there is nothing to gate — stop). Verify `git merge-base dev HEAD` resolves and `git diff <base>...HEAD` is non-empty; pin the map issue. Fresh `dev` integration is the workflow's job, not yours. Lint runs through `make` (per-checkout `GOLANGCI_LINT_CACHE`); a bare `golangci-lint` outside `make` shares the machine-global cache.
3. Copy the template and edit only its CONFIG block:
   ```
   cp .agents/skills/pre-merge/pre-merge.dwf.ts .zcode/workflow-drafts/premerge-<branch>.dwf.ts
   ```
   Fill in: `WT` (absolute worktree path), `BRANCH`, `MAIN` (absolute main-checkout path — where `dev` lives and where the merge happens), `MAP_ISSUE`, `AREAS`, `DISPO`. `AREAS` is the hand-composed area split of the diff (id/title/paths/standards/focus) — reading the diff's shape and the per-app `AGENTS.md` is what makes this good; an empty array falls back to a mechanical directory split. `DISPO` lists pins and dispositions from ticket reviews and map comments that reviewers must not re-open. Two flags: `E2E_AFTER_MERGE` (run `make frontend-e2e` on merged `dev` in the main checkout after the merge — safe there, disposable slot-0 stack; +10–20 min) and `ASK_BEFORE_MERGE` (put one final yes/no to the owner before the merge even on a clean gate).
4. If a previous gate run on this branch was blocked or errored, **harvest its dispositions** into `DISPO` before submitting: declines with reasons, applied cluster titles with commit hashes, low-leftovers, owner pins from the blocked report. A fresh run that omits them re-opens closed work in round 1.
5. Submit: `CreateWorkflow { path: ".zcode/workflow-drafts/premerge-<branch>.dwf.ts", name: "Pre-merge: <branch>" }`. The owner approves the run by its phase graph. If the file was already submitted once and needs a fix, edit the draft file and resubmit with `path` — never paste the script inline again.

## What the workflow does (the contract)

1. **Branch check + fresh `dev` integration.** If `dev` moved, the workflow merges it into the branch; conflicts go to an integrator agent (see state, find primary sources, preserve both intents per hunk; the classic collision class — migration and ADR numbers — is renumbered on the branch side). Ambiguous calls escalate to the owner instead of guessing.
2. **Diff pinning** against the post-integration merge-base; file list fanned out over hand-set (or mechanical) areas.
3. **Full baseline**: `make -C <worktree> test-log` (reuses the `test` target; verbose output goes to `.make-test.log` inside the checkout, the recipe prints only the summary — and a tail on failure, which feeds the repair round). Red → bounded repair (2 rounds, the repairer reads the full log) → still red → blocked report; review never starts.
4. **Review sweep per area, up to 3 rounds.** Each area gets a read-only reviewer: defects + documented-standard violations + the Fowler smell baseline (judgement calls; the repo's documented standards win) + a doc-rot sweep (comments describing behavior the branch removed) + architecture candidates — the `/improve-codebase-architecture` method scoped to the diff (shallow modules, deletion test, cross-ticket duplication, missed `CONTEXT.md` vocabulary; `/codebase-design` terms). With a map issue set, a spec reviewer runs alongside (missing/partial/wrong/scope-creep against the map and its tickets). **Every finding is independently confirmed** by a fresh agent (Подтверждено / Опровергнуто / Неясно) before it counts; `DISPO` pins ride in every prompt. **Blocking rule:** in rounds 1–2 every confirmed finding of any severity blocks; from round 3 on, low findings (and unclear lows) stop blocking — they are recorded as осознанный остаток in the report and merged with it. High/medium block in every round.
5. **Triage into file-disjoint fix clusters.** A planner agent returns clusters (concrete files, per-finding task, commit message in repo canon), declines (Speculative arch candidates, canon conflicts — with reasons), issues to file (candidates beyond the branch's blast radius), and escalates behavior-changing fixes to the owner for sanction. The script itself validates file disjointness and finding coverage deterministically and repairs gaps with a reserve cluster.
6. **Parallel fixers per cluster** (minimal, style-following, test-first for behavior-adjacent changes, targeted tests only), then **fast gates** — tsc, eslint, vitest, go test — with bounded repair. Red after 3 rounds means **no commits**: the tree stays dirty for live handling, by design.
7. **Serial commits**: one cluster, one commit; hashes land on the board.
8. Loop back to 4 until a round ends with zero blocking findings (see the rule in step 4) — or the 3-round ceiling stops the gate (осознанный остаток → owner, **no merge**).
9. **Final full `make test-log`**, pre-merge checks (main is on `dev`, `dev` unmoved since integration, `git merge-tree` dry-run clean, worktree clean), then — if `ASK_BEFORE_MERGE` — one final question to the owner through an escalation (hold = clean stop, the branch stays ready to merge). **Merge `--no-ff` into `dev` in the main checkout**; the merge message is composed in repo canon from the map and the commit log.
10. **Full `make test-log` on merged `dev`**, then — if `E2E_AFTER_MERGE` — `make frontend-e2e` on merged `dev` (the joined tree is exactly where integration regressions live). Then the report: markdown artifact for the owner, a comment on the map issue, issue close.

The full suite runs via `world.run` through the `test-log` make target — a real exit code with no `echo $?` masking, and an output small enough to never hit the workflow's stream cap (the verbose log stays on disk for repair agents to read). If the cap is ever hit anyway, the script falls back to per-suite targets and reports anything still unverifiable as honestly not covered (which blocks the merge).

## During the run (main agent duties)

- **Do not poll.** The completion notification arrives on its own; `TaskOutput` only when the owner asks you to wait.
- **Answer escalations.** Subagents raise blocking questions (`dwfq-…`): ambiguous conflict calls, behavior-change sanctions, unclear findings. Escalations of the same round go to the owner **batched in one `AskUserQuestion`** (several questions, each with options and a recommendation). The moment an answer lands, send its `ResolveWorkflowQuestion` **before touching the next escalation** — an answer you have not resolved is not delivered, and the subagent keeps waiting. Before handling a new escalation, glance at `GetWorkflowRun` for pending «questions awaiting» and clear them first. Nothing answers on your behalf; a dropped question parks the subagent forever.
- **Repair, don't restart.** A run heading the wrong way is fixed by editing the run's script file (the notification and `GetWorkflowRun` name it) and calling `AmendWorkflow` with that `path` — never by starting a new `CreateWorkflow`, never by `TaskStop` first. errored → fix the script, resubmit via `AmendWorkflow`. stopped → resume only on the owner's word (`ResumeWorkflowRun`); a `user` stop means leave it alone.
- **Ceiling stop → amend, not a fresh gate.** A run stopped by the `SWEEP_ROUNDS` ceiling with live blockers ends cleanly and needs no rerun from zero: on the owner's word to continue, bump `SWEEP_ROUNDS` in the draft's CONFIG and `AmendWorkflow` the **completed** run — its finished steps (integration, baseline, rounds 1–N) replay from the journal for free, only the new rounds are paid. A fresh `CreateWorkflow` re-pays the whole pipeline (the history night of 25.09: +2h45m and ~190M tokens for one extra review round).

## After the run

- Verify live (this has bitten before): `git -C <main> log --oneline -3` shows the merge; the worktree is clean; the gh comment landed on the map issue; the workflow report and the artifact agree.
- Left to the owner's word: push; worktree/branch/stand teardown (order per `docs/agents/parallel-dev.md` — stands down before `worktree remove`); a live `/ui-walkthrough` for UI-heavy branches; `make frontend-e2e` if wanted.

## Gotchas (each paid for in a real campaign)

- **Absolute paths only in `world.run`** — relative `make -C`/`git -C` once failed every gate round silently and once lied about a clean tree. The template is absolute everywhere; keep it that way.
- **The merge happens in the main checkout** (`git -C <main>`), never from the worktree — a merge once went into the worktree from a persistent shell cwd («Already up to date»).
- **e2e is out of the gate**: running e2e from a worktree against its `.env` can tear down a live stand's slot (`down -v` deletes volumes).
- **Comments in fixed code** are Russian, written for the next reader, and never mention the review, the agents, or the run.
- **The command set is fixed literals**: git, make, node, npm, gh — no push, no destructive docker commands anywhere in the script.
- `make test` does not include e2e. The honest exit code is the one `world.run` returns — never a piped `echo $?` after it.
- **An owner's answer without its `ResolveWorkflowQuestion` is a lost answer.** During the history gate (24.09) the run stood parked for 1h45m because the next escalation's notification arrived mid-handling and swallowed the previous resolve. The sequence is always: owner answers → resolve immediately → only then the next escalation.
- **Amend replays journaled git observations verbatim.** After a manual environment change between attempts (a branch switch of the main checkout, a rebuild), a resumed run's cached `git` checks answer with the old world and deterministic code throws on stale data — twice on the realtime gate (25.09) the merge-readiness check re-played «frontend-foundation» after the checkout to `dev`; reflog proved nobody switched back. The cure: a live `world.run` with a fresh journal key before the checks (a new call site or new args) — it executes live and invalidates the cache of every observation after it.
- **The blocked report's low list is cumulative.** Lows recorded in rounds 1..N are never pruned when a later round's fix clusters close them, so a ceiling-stop report names items already fixed at HEAD — half of the 26-item list was stale when the owner said «почини тоже» (realtime gate, 25.09). Verify each item against HEAD before acting on it.

## Files

- `pre-merge.dwf.ts` — the workflow template. Edit only the CONFIG block; the mechanics are battle-tested (the #692 доводка: 3 sweeps 24→12→7 findings, every one independently confirmed and fixed).
