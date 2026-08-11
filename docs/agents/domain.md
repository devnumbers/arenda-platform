# Domain Docs

How the engineering skills should consume this repo's domain documentation when exploring the codebase.

## Before exploring, read these

- **`CONTEXT-MAP.md`** at the repo root — it lists the bounded contexts, where each per-context `CONTEXT.md` lives, and how the contexts relate. Read each `CONTEXT.md` relevant to the topic.
- **`docs/adr/`** — read ADRs that touch the area you're about to work in.

If any of these files don't exist, **proceed silently**. Don't flag their absence; don't suggest creating them upfront. The `/domain-modeling` skill (reached via `/grill-with-docs` and `/improve-codebase-architecture`) creates them lazily when terms or decisions actually get resolved.

## File structure

Multi-context repo (this repo). The root `CONTEXT-MAP.md` points at one `CONTEXT.md` per bounded context, co-located with the context's code:

```
/
├── CONTEXT-MAP.md
├── docs/adr/                                      ← system-wide decisions
└── apps/backend/internal/
    ├── identity/CONTEXT.md
    ├── properties/CONTEXT.md                      ← rental super-context (ADR 0029)
    ├── billing/CONTEXT.md
    ├── access/CONTEXT.md
    ├── audit/CONTEXT.md
    └── popups/CONTEXT.md
```

`properties/CONTEXT.md` documents the rental super-context (properties + leases + notifications) as one glossary until the bidirectional domain cycles are broken via ports (ADR 0029).

## Use the glossary's vocabulary

When your output names a domain concept (in an issue title, a refactor proposal, a hypothesis, a test name), use the term as defined in the relevant per-context `CONTEXT.md` (or the shared kernel in `CONTEXT-MAP.md`). Don't drift to synonyms the glossary explicitly avoids.

If the concept you need isn't in the glossary yet, that's a signal — either you're inventing language the project doesn't use (reconsider) or there's a real gap (note it for `/domain-modeling`).

## Flag ADR conflicts

If your output contradicts an existing ADR, surface it explicitly rather than silently overriding:

> _Contradicts ADR-0007 (event-sourced orders) — but worth reopening because…_
