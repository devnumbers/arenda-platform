---
name: go
description: Go rules of this repository — repo constraints and coding-standard pointers plus primary sources (official docs, Google, Uber, 100go.co). Use for all Go backend work in apps/backend.
---

# Go (this repository)

A thin orientation skill: the rules live in repo documents, and this skill points at them and at the primary sources. It replaces generic community Go skills — their defaults (`pkg/` layouts, stdlib `ServeMux` routing, microservice framing) do not apply to this codebase.

## Read first (repo sources of truth)

1. `apps/backend/AGENTS.md` — stack, mandatory tools, architecture rules, API & persistence contract, quality gates.
2. `apps/backend/CODING_STANDARDS.md` — architecture inside a bounded context, errors, concurrency, tests, and the review rubric (distilled from 100go.co, filtered against `.golangci.yml`).
3. `.golangci.yml` (repo root) — if lint catches it, review does not re-report it.
4. `docs/research/2026-08-18-go-projects-benchmark.md` — how this ruleset was benchmarked against Google/Uber/Kratos/go-zero/Three Dots Labs/Loki/Tailscale/K8s.

## Sharpest repo rules agents get wrong

- DDD modular monolith: `internal/<context>/{domain,application,adapters}`, dependencies inward only (depguard). Not microservices, not a `pkg/` layout, no `cmd/`-per-service sprawl.
- Ports are declared by consumers, not providers; never declare an interface beside its only implementation.
- Time arrives through the `internal/shared/clock.Clock` port; transactions go through `internal/transaction` UoW (ADR 0033).
- Sentinel errors per context, returned unwrapped; `errors.Is` matching happens at the transport edge only.
- Every goroutine has an owner responsible for its exit.
- Fakes over mocks; table-driven tests with `t.Parallel()` and testify.
- IDs are app-generated UUIDv7 (`uuid.NewV7()`); `uuid.New*` variants are banned by forbidigo. Money is `int64` kopecks — integer arithmetic only.
- Contract-first: change `api/openapi/openapi.yaml` and `db/queries` before behavior, then regenerate; never hand-edit generated files.

## Primary sources (verify here, in this order)

1. Official Go: `https://go.dev/ref/spec`, `https://pkg.go.dev`, `https://go.dev/doc/modules/layout`, and the go.dev release notes for the target version.
2. Go Code Review Comments: `https://go.dev/wiki/CodeReviewComments`.
3. Google Go Style Guide: `https://google.github.io/styleguide/go/decisions` and `https://google.github.io/styleguide/go/best-practices`.
4. Uber Go Style Guide: `https://github.com/uber-go/guide/blob/master/style.md`.
5. 100 Go Mistakes: `https://100go.co`.

Official Go documentation, repo ADRs, and `CODING_STANDARDS.md` outrank this skill on any conflict. For the modern-idiom catalog by Go version, invoke `use-modern-go`.
