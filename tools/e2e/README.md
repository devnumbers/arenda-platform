# E2E tests

This folder hosts the end-to-end tests of the Arenda platform. It currently
contains only the Playwright screen suite of the frontend; manual API request
collections live separately in `tools/bruno/arenda-api/`.

## What's inside

```
tools/e2e/
├── README.md                        # this file
└── frontend/                        # Playwright e2e of the frontend screens
    ├── run-frontend-e2e.sh          # `make frontend-e2e` orchestrator
    ├── seed.sql                     # deterministic seed (owner, session, properties)
    ├── live-overlay.sql             # overlay applied on top of the base seed
    └── e2e-crypto.mjs               # seed crypto: token HMAC, phone ciphertext
```

The frontend screen suite itself (specs and fixtures) lives with the app in
`apps/frontend/e2e`; see `frontend/README.md` here and the «Экранные e2e
(Playwright)» section of `docs/testing-strategy.md`.

## Prerequisites and running

The suite runs from the repository root via `make frontend-e2e` and requires
Docker plus the local Node and Go toolchains. The full run contract — stack
layout, seed, environment variables, reports — is described in
`frontend/README.md`.
