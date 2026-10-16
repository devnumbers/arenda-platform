# Manual API collections

This folder contains manually callable API request collections for exploring the
Arenda backend endpoints.

## Current collection

- `arenda-api/` — everyday CRUD requests for:
  - `auth`
  - `properties`
  - `leases`
  - `operations`
  - `recurring-operations`
  - `tenant-contacts`
  - `reminders`
  - `subscription`
  - `webhooks`

Each request has a minimal assertion so it can be run on its own from the
Bruno app or the Bruno CLI.

## E2E tests

The Playwright screen suite of the frontend lives in `tools/e2e/frontend/`
and runs from the repository root via `make frontend-e2e`. See
`tools/e2e/README.md` and `tools/e2e/frontend/README.md` for details.

## Run a request with the CLI

```bash
cd tools/bruno/arenda-api
bru run auth/me.bru --env Local
```

Make sure `environments/Local.bru` exists (copy from `Local.bru.example` if
needed) and the backend is running.
