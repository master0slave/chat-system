# ADR 0002: Monorepo: Go API + Next.js Web + E2E

## Status

Accepted

## Context

The system has three parts that change together: the Go API, the Next.js web client, and the Cucumber E2E tests that drive both. The reference projects (`oddsteam/api.odds-worklog`, `oddsteam/web.odds-worklog`) live in separate repositories. For a learning project with one developer, a change to one REST route in separate repositories means three pull requests, and the contract files can drift apart.

## Decision

1. One repository with three top-level folders: `api/` (Go module), `web/` (Next.js), `e2e/cucumber/`.

2. Contracts live in `docs/` and are shared by every part:

   ✅ `docs/openapi.yaml` for REST, `docs/events.md` for WebSocket events
   ❌ a copy of the contract inside `web/`

3. The root `Makefile` is the only entry point. Every command a developer or CI needs is a `make` target (`up`, `down`, `test`, `run-api`, and later `run-web`, `e2e`).

   ✅ `make test`
   ❌ a README that says "cd api && go test ./... then cd ../web && npm test"

4. `make test` must pass before any change counts as done (see `.cursor/rules/verification.mdc`).

## Consequences

### Positive

- One commit can change the API, the client, and the E2E test for a feature together.
- Contract files cannot drift, because there is only one copy.

### Negative

- Go and Node tooling sit in one repository. Editors and CI must handle both.
- Deploying the API and web separately later needs path-based CI triggers.
