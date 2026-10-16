# apps/admin/AGENTS.md

## Scope

Rules for the react-admin back-office SPA in `apps/admin`. Also follow the root `AGENTS.md`, the relevant per-context `GLOSSARY.md` (index in `GLOSSARY-MAP.md`), relevant product docs, and ADRs.

## Stack & References

- Versions live in `package.json` (single source): Vite, React, TypeScript, react-admin 5 (`ra-i18n-polyglot`, `ra-language-russian`), MUI 7 with Emotion.
- Backend integration lives in `src/dataProvider.ts`, authentication in `src/authProvider.ts`; resources are declared per domain (anatomy in `CODING_STANDARDS.md`).
- Russian UI localization via `ra-i18n-polyglot` + `ra-language-russian`; keep new user-facing strings localized.
- Official react-admin (https://marmelab.com/react-admin/documentation.html) and MUI (https://mui.com/material-ui/) docs take precedence over training data; for non-obvious third-party behavior use `context7`.
- Skills: `typescript` for type design; browser automation goes through the playwright MCP (`docs/agents/mcp.md`).

Before adding resources, fields, inputs, helpers, or API wrappers, search existing resources and call sites to avoid duplicate patterns.

## Coding Standards

Before implementing or reviewing admin code, read `CODING_STANDARDS.md` (same directory): resource file anatomy, the read-only design and its dataProvider conventions, `fields.tsx` choice-catalog sync rules, i18n practice, the quality bar, testing patterns, and the review rubric used by the Standards axis of `/code-review`.

## Architecture

- This is a standalone Vite SPA, not Next.js: no App Router, no Server Components, no file-based routing. Do not apply the Next.js rules from `apps/frontend/AGENTS.md` here.
- Follow react-admin conventions: declare resources on the `<Admin>` component, keep list/edit/create/show views colocated per resource, and route all backend calls through `dataProvider` and all auth state through `authProvider`. Enforced by ESLint `no-restricted-globals` on `fetch`: raw `fetch` is allowed only in the sanctioned HTTP boundary files — `src/dataProvider.ts`, `src/authProvider.ts`, `src/lib/report-error.ts`.
- The quality bar is tool-enforced (details and canonical spellings in `CODING_STANDARDS.md`): `typescript-eslint` `recommendedTypeChecked` (plus `prefer-nullish-coalescing`) and `eslint-plugin-react-hooks` `recommended` at error level over all manual code — the HTTP boundary files included.
- Keep components small and explicit; prefer react-admin and MUI building blocks over custom widgets.
- Map backend DTOs at the `dataProvider` boundary; do not leak API response shapes into resource components.
- Money values are integer kopecks; display them via `MoneyField` / `formatKopecks` from `src/fields.tsx` (ru-RU, RUB), never ad-hoc formatting — money selectors in `eslint.config.mjs` ban raw `/100`/`*100` and `.toFixed` outside `src/fields.tsx` (generated code excepted).
- The property attributes catalog (`src/lib/generated/`) is generated from `tools/property-attributes/catalog.json` (`make attributes-gen`; `make attributes-check` fails in CI on a stale catalog). Do not hand-edit `generated/`; only the `src/lib/propertyAttributes.ts` seam may import them (ESLint `no-restricted-imports`).
- Runtime configuration comes from Vite env vars (see `.env.example`); never hardcode backend URLs or secrets. ESLint `no-restricted-syntax` bans absolute `http(s)://` and `/api/...` literals — request URLs are built from `import.meta.env.VITE_API_PREFIX`.
- Zero tolerance for suppressions in manual code (`src` outside `src/lib/generated/`): `eslint-disable*`, explicit `any`, `@ts-ignore`/`@ts-expect-error` — fix the code, never suppress (the blocking counter: `make ts-suppressions`).

## Quality Gates

Before claiming admin work complete: `make admin-typecheck`, `npm --prefix apps/admin run lint`, `make admin-build`, `make admin-test`.
