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

Each request has a minimal assertion so it can be run on its own from the
Bruno app or the Bruno CLI.

## E2E tests

System end-to-end tests, SQL checks, and the test orchestrator live in a
separate folder:

```
tools/e2e/
```

See `tools/e2e/README.md` for how to run the full E2E suite.

## Run a request with the CLI

```bash
cd tools/bruno/arenda-api
bru run auth/me.bru --env Local
```

Make sure `environments/Local.bru` exists (copy from `Local.bru.example` if
needed) and the backend is running.
