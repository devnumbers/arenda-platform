# apps/landing/AGENTS.md

## Scope

Rules for the public landing in `apps/landing` — a standalone Next.js application (SSR/SSG, ADR 0063) serving the public site at `/`. Also follow the root `AGENTS.md` and relevant ADRs. This app has no FSD, no unit-test suite, and no backend of its own — form submissions proxy to the backend (`FEEDBACK_EMAIL` in deploy env; docs/deployment.md).

## Stack & Commands

- Next.js + Tailwind v4; versions live in `package.json` (single source). Next.js docs (`https://nextjs.org/docs`) take precedence over training data.
- Gates: `npm run lint`, `npm run typecheck`, `npm run build` — all from `apps/landing`. `next build` also validates the images config (the deploy gate runs the same code), so a green build is the photo-config check. There is no unit-test suite; acceptance is owner walkthrough on a stand (below).
- Start dev/stand binaries directly, not through `npm exec`: `npm exec` eats `-p` — use `./node_modules/.bin/next dev -p PORT` / `next start -p PORT`.

## Design Conventions

Before building or changing any landing section, read `DESIGN.md` (same directory): window-width tiers (mobile ≤480 stacks, `tab:` 481+, `desk:` 1200+), the strip/carousel engine canon and its Yandex reference measurements, the photo pipeline (original bytes → webp q100 → AVIF; the `deviceSizes`/`imageSizes` ladder is an owner decision — do not retune), the stand and standalone-rebuild recipe, walkthrough and Figma extraction recipes, tablet H2 sizing, the portable UI canon, and the Radix scroll-lock fix.

## Anatomy

- `components/sections/` — one file per landing section (hero, showcase, rentals, tariff-cards, faq, …).
- `components/` — site chrome and cross-section blocks: `site-header`, `site-footer`, `button` (LandingButton), `reveal`, `tablet-strip` + `strip-engine.ts` (the shared strip/carousel engine), `feedback-modal`, `json-ld`.
- `components/ui/` — portable product UI ported from `apps/frontend/shared/ui/design`: `modal` (Radix Dialog ≥768 / vaul sheet <768, with the managed `closing` phase — Radix Presence drops exit animations on React 19), `text-field`, `text-area`, `icon-button`, `sheet-drag-handle`. Keep these in sync with the frontend originals only via an explicit decision, not silent drift.
- `lib/` — small helpers (`cn`, nav, content, auth). `public/` — static assets; photo provenance is recorded in `ATTRIBUTIONS.md`.

## Acceptance (walkthrough)

Screens close through `/ui-walkthrough` like the frontend. The landing stand is the lightweight pattern: `next build` + `next start -p PORT` from `apps/landing` (PORT from the checkout's `.env` slot values — never a hardcoded port), verify on 375 / 768 / 1440, keep viewport screenshots only (full-page captures blank out Reveal sections — DESIGN.md). Browser automation goes through the playwright MCP (`docs/agents/mcp.md`); Figma work follows the server roles defined there (`figma` for context/screenshot, `figma-context` for file exports).

## Gotchas

- `next start`/`next build` strictly from `apps/landing` of the active checkout; in a fresh worktree run `npm ci` in `apps/landing` first.
- The standalone layout is FLAT: after a build copy `.next/static` → `.next/standalone/.next/` and `public` → `.next/standalone/`, then restart `node server.js` — a running server keeps serving 404s until restarted (symptoms: 404 on `/fonts/*.woff2` = `public` not copied; 400 "isn't a valid image" = `static` not copied).
- Write asset files atomically; a non-atomic replace under a running dev server fails decodes with "unable to decode image data" until restart.
- The header sits behind the feedback-modal overlay by design (`z-50`, owner decision) — do not raise it above the overlay.
