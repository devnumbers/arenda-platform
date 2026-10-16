# Domain Docs

How the engineering skills should consume this repo's domain documentation when exploring the codebase.

## Before exploring, read these

- **`GLOSSARY-MAP.md`** at the repo root — the live index: it lists the bounded contexts, where each per-context `GLOSSARY.md` lives, and how the contexts relate. Read each `GLOSSARY.md` relevant to the topic.
- **`docs/adr/`** — read ADRs that touch the area you're about to work in.

If any of these files don't exist, **proceed silently**. Don't flag their absence; don't suggest creating them upfront. The `/domain-modeling` skill (reached via `/grill-with-docs` and `/improve-codebase-architecture`) creates them lazily when terms or decisions actually get resolved.

## File structure

Multi-context repo (this repo). The root `GLOSSARY-MAP.md` points at one `GLOSSARY.md` per bounded context, co-located with the context's code under `apps/backend/internal/<context>/`; system-wide decisions live in `docs/adr/`. Each `GLOSSARY.md` documents exactly one bounded context. Do not enumerate the contexts anywhere else — `GLOSSARY-MAP.md` is the only list, so it cannot go stale.

## Use the glossary's vocabulary

When your output names a domain concept (in an issue title, a refactor proposal, a hypothesis, a test name), use the term as defined in the relevant per-context `GLOSSARY.md` (or the shared kernel in `GLOSSARY-MAP.md`). Don't drift to synonyms the glossary explicitly avoids.

If the concept you need isn't in the glossary yet, that's a signal — either you're inventing language the project doesn't use (reconsider) or there's a real gap (note it for `/domain-modeling`).

## Flag ADR conflicts

If your output contradicts an existing ADR, surface it explicitly rather than silently overriding:

> _Contradicts ADR 0033 (unit of work) — but worth reopening because…_
