# Frontend AGENTS.md Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Rewrite `apps/frontend/AGENTS.md` as a concise, FSD-based rules document aligned with `apps/backend/AGENTS.md` style.

**Architecture:** Single Markdown file replacing the existing minimal file. No application code changes; only documentation and verification.

**Tech Stack:** Markdown; existing Next.js `16.2.9` / React `19` / TypeScript `5` project.

---

### Task 1: Back up the current `apps/frontend/AGENTS.md`

**Files:**
- Read: `apps/frontend/AGENTS.md`

**Step 1: Read the current file**

Run: `cat apps/frontend/AGENTS.md`
Expected: The file contains only the Next.js warning block.

**Step 2: Note the warning content**

Expected content to preserve as a short note:
```markdown
<!-- BEGIN:nextjs-agent-rules -->
# This is NOT the Next.js you know

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` before writing any code. Heed deprecation notices.
<!-- END:nextjs-agent-rules -->
```

---

### Task 2: Rewrite `apps/frontend/AGENTS.md`

**Files:**
- Overwrite: `apps/frontend/AGENTS.md`

**Step 1: Write the new file**

```markdown
# apps/frontend/AGENTS.md

## Scope

Rules for the Next.js frontend in `apps/frontend`. Also follow the root `AGENTS.md`, `CONTEXT.md`, relevant product docs, and ADRs.

## Stack & References

- Next.js `16.2.9`, React `19.2.4`, TypeScript `^5`.
- React Compiler enabled in `next.config.ts`.
- Official Next.js docs (`https://nextjs.org/docs`) take precedence over training data.
- For non-obvious third-party behavior, use Context7 for current docs.

## Required Skills

- For all frontend work, invoke `$next-best-practices` and `$vercel-react-best-practices`.
- For component composition and design patterns, invoke `$vercel-composition-patterns`.
- For view transitions, invoke `$vercel-react-view-transitions`.
- For TypeScript questions and type design, invoke `$typescript`.
- For current library docs before relying on non-obvious APIs, use Context7 (`mcp:context7`).

## TypeScript & Linting

- Keep `strict: true` and the strictest practical compiler options in `tsconfig.json`.
- Use `eslint-config-next` (`core-web-vitals` + `typescript` presets) in `eslint.config.mjs`.
- Avoid `any`; prefer `unknown` with narrowing. If `any` is unavoidable, add a comment and consider an ADR.
- Use explicit return types for functions exported from `shared/`, `entities/`, `features/`, and `widgets/`.
- Prefer `readonly`, `ReadonlyArray`, and `as const` for immutable data.

## Architecture (FSD)

- `app/` — Next.js App Router pages, layouts, loading/error boundaries, and route handlers.
- `pages/` — only if an explicit ADR adds the Pages Router; the default is App Router.
- `widgets/` — self-contained page blocks composed of features, entities, and shared UI.
- `features/` — user scenarios and use cases (for example: "create operation", "pay subscription").
- `entities/` — domain models mapped from the backend: owner, property, lease, operation, subscription.
- `shared/` — reusable infrastructure: UI kit, API client, config, helpers, types, and hooks not tied to a specific feature.
- Dependency direction is inward only: `app/widgets` → `features` → `entities` → `shared`. No imports upward or sideways between slices.
- Keep UI dumb; business logic lives in `features/` and `entities/`. Server calls live in `shared/api` or Next.js route handlers.

## API & Data Flow

- Use Next.js server-side data fetching (Server Components and route handlers) by default.
- Browser-side state only when interactivity requires it.
- Map backend DTOs to entity models at the API boundary; do not leak generated DTOs into widgets or features.
- Reuse backend types from OpenAPI where possible; keep frontend entity types explicit and minimal.

## Components & State

- Server Components by default; use `'use client'` only for interactivity, browser APIs, or hooks that require a client boundary.
- Keep components small, explicit, and focused on one responsibility.
- Avoid prop drilling; prefer composition and, when necessary, feature-scoped context.
- Do not use global state for local UI state. Use URL state for shareable page state.
- Prefer explicit event handlers and reducers over implicit side effects.

## Testing & Quality Gates

- Add tests for features and entities that contain business logic.
- Run before claiming frontend work complete:

```bash
cd apps/frontend && npm run lint
cd apps/frontend && npm run build
```

- Keep tests deterministic; mock network and browser APIs at `shared/api` boundaries.

## Commands

```bash
# from apps/frontend
npm install
npm run dev
npm run build
npm run lint
```
```

**Step 2: Verify the file was written**

Run: `cat apps/frontend/AGENTS.md`
Expected: The file contains the sections above and the Next.js warning is no longer a standalone block.

---

### Task 3: Update `CHANGELOG.md`

**Files:**
- Modify: `CHANGELOG.md` (add entry under `## 2026-06-23`)

**Step 1: Add entry**

Under the existing `## 2026-06-23` section, add:

```markdown
### Изменено
- Добавлены правила разработки фронтенда: навыки, строгий TypeScript, линтер, FSD-архитектура и порядок зависимостей.
```

**Step 2: Verify**

Run: `head -n 25 CHANGELOG.md`
Expected: The new entry appears under `## 2026-06-23`.

---

### Task 4: Verify lint and build still pass

**Files:**
- Verify: `apps/frontend/`

**Step 1: Run linter**

Run: `cd apps/frontend && npm run lint`
Expected: No errors.

**Step 2: Run build**

Run: `cd apps/frontend && npm run build`
Expected: Build succeeds.

---

### Task 5: Commit the changes

**Step 1: Stage files**

Run:
```bash
git add apps/frontend/AGENTS.md CHANGELOG.md docs/plans/2026-06-23-frontend-agents-design.md docs/plans/2026-06-23-frontend-agents-implementation-plan.md
```

**Step 2: Commit**

Run:
```bash
git commit -m "docs(frontend): add AGENTS.md with FSD, TypeScript, and skill rules"
```

Expected: Commit succeeds with the two changed files and two plan documents.
