# Support Chat — Design Spec

- **Date:** 2026-09-26
- **Status:** Draft — waiting for user review
- **Repo:** `class-practical-techniques/` (monorepo)

## 1. Goal

A support chat system in which customers open questions and the support team (agents) answers them in real time. A case can have more than one agent.

The project has two goals:

1. Build a working support chat.
2. Practise the project structure that ODDS uses in `oddsteam/web.odds-worklog` and `oddsteam/api.odds-worklog`: ADRs (`docs/adr`), Cursor rules (`.cursor/rules`), TDD, Clean Architecture with ports, and E2E tests with Cucumber, Playwright and Page Objects.

**Developer background:** already knows Next.js and Go. NestJS and Angular are not used, to keep the amount of new material small.

## 2. Scope (MVP)

### Roles

| Role | Can do |
|---|---|
| **customer** | Open a case. Sees only their own cases. Sends messages in their own cases. |
| **agent** | Sees every case. Joins any case (many agents per case is allowed). Replies in cases they have joined. Closes cases. |

### Case lifecycle

```
 customer opens case       first agent joins          agent closes case
───────────────► [waiting] ───────────────► [open] ───────────────► [closed]
                                      ▲  more agents can join
```

- `waiting`: the customer has opened the case with a first question, and no agent has joined yet.
- `open`: at least one agent has joined. Every participant can send messages.
- `closed`: read-only. Nobody can send messages. Reopening is not supported.

### User stories (one feature file each)

1. A customer opens a case with a first question.
2. An agent sees the list of waiting cases and joins one.
3. Everyone in a case sends messages and sees each other's messages in real time (one customer and several agents).
4. A participant who opens a case again sees the message history.
5. An agent closes a case, and after that nobody can send messages.
6. A customer cannot access another customer's case.

### Out of scope (Future)

These are candidates for later rounds. Each one should get its own spec or ADR when its turn comes:

- File and image attachments
- Typing indicator and read receipts
- Notifications outside the app (email, LINE)
- Case assignment, SLAs, agent workload view
- Reopening closed cases
- Shared topic rooms (community-style support)
- Message search
- Keycloak (replaces the simple login; see ADR 0005)
- Running more than one API instance (Redis or RabbitMQ adapter for `MessageBroadcaster`; see ADR 0004)

## 3. Tech stack

| Part | Choice |
|---|---|
| Frontend | Next.js (App Router), used only as a client UI |
| Backend | Go + Echo + WebSocket (`github.com/coder/websocket`) |
| Architecture | Clean Architecture with driving and driven ports (same as api.odds-worklog ADR 0001) |
| Database | MongoDB (docker-compose) |
| Auth | Simple login that issues a JWT (name + role), behind an `Authenticator` port |
| Unit tests | `go test` + testify; Vitest for `web/lib` |
| E2E | Cucumber + Playwright + Page Object Pattern |
| Entry point | `Makefile` at the repo root |

## 4. Repository structure

```
class-practical-techniques/
├── Makefile                   # up / down / test / run-api / run-web / e2e
├── .cursor/rules/
│   ├── tdd.mdc                # alwaysApply
│   ├── verification.mdc       # alwaysApply
│   ├── clean-architecture.mdc # globs: api/**/*.go
│   ├── openapi.mdc            # globs: api/handlers/**
│   ├── realtime-events.mdc    # globs: api/realtime/**, web/lib/socket*
│   └── e2e-cucumber.mdc       # globs: e2e/**
├── docs/
│   ├── adr/                   # README.md index + NNNN-kebab-title.md
│   ├── openapi.yaml           # REST contract (source of truth)
│   └── events.md              # WebSocket event contract (source of truth)
├── api/                       # Go module
│   ├── main.go                # wiring only
│   ├── business/
│   │   ├── models/            # entities + domain errors; no internal imports
│   │   └── usecases/          # one file per use case
│   │                          # + <usecase>_driving_ports.go / _driven_ports.go
│   ├── handlers/              # Echo HTTP handlers + WebSocket endpoints
│   ├── repositories/          # Mongo adapters (implement driven ports)
│   ├── realtime/              # in-memory Hub (implements MessageBroadcaster)
│   ├── pkg/auth/              # JWT (implements Authenticator)
│   └── deployment/local/docker-compose.yaml
├── web/                       # Next.js
│   ├── app/                   # /login, /cases, /cases/[id]
│   └── lib/                   # api.ts, socket.ts, messages.ts; the only code that talks to the backend
└── e2e/cucumber/
    ├── features/  pages/  steps/  support/
```

### Dependency rule (ADR 0001)

```
handlers ───────┐
repositories ───┼──► business/usecases ──► business/models
realtime ───────┤      (declares ports)
pkg/auth ───────┘
```

- `business/models` has zero internal imports.
- `business/usecases` imports only `business/models`. It reaches infrastructure only through interfaces that it declares itself.
- Outer layers depend inward and never the other way.
- In `web`, components must not call `fetch` or open sockets directly. They go through `web/lib`.

### Ports

| Port | Kind | Implemented by |
|---|---|---|
| `OpenCase`, `JoinCase`, `SendMessage`, `CloseCase`, `ListCases`, `GetCase`, `ListMessages` | driving | `business/usecases` (called by `handlers`) |
| `CaseRepository` | driven | `repositories` (Mongo) |
| `MessageRepository` | driven | `repositories` (Mongo) |
| `MessageBroadcaster` | driven | `realtime.Hub` |
| `Clock`, `IDGenerator` | driven | `pkg` (real) / fakes in tests |
| `Authenticator` | driven | `pkg/auth` (JWT) |

## 5. Data model

```go
type Role string       // "customer" | "agent"
type User struct { ID, Name string; Role Role }

type CaseStatus string // "waiting" | "open" | "closed"
type Case struct {
    ID           string
    Subject      string        // first question, cut to 80 characters
    CustomerID   string
    Participants []Participant // the customer plus agents who joined
    Status       CaseStatus
    CreatedAt, UpdatedAt time.Time
    ClosedAt     *time.Time
}
type Participant struct {
    UserID, Name string
    Role         Role
    JoinedAt     time.Time
}

type Message struct {
    ID, CaseID, SenderID, SenderName string
    SenderRole Role
    Kind       string // "text" | "system"
    Body       string // text messages: 1–2000 characters after trimming
    CreatedAt  time.Time
}
```

- Collections: `cases` with indexes `{status, updatedAt}` and `{customerId}`. `messages` with index `{caseId, createdAt}`.
- Messages are stored in their own collection, not embedded in the case, so a busy case cannot hit the document size limit and paging stays simple.
- System messages are created on join ("Bob joined the case") and on close ("Case closed by Bob").

### Business rules (enforced in use cases)

| Rule | Error |
|---|---|
| Only a customer can open a case. The first question is 1–2000 characters. | `ErrForbidden` / `ErrInvalidInput` |
| A customer can only see their own cases. | `ErrForbidden` |
| Only an agent can join. Joining twice has no effect (idempotent). The first join moves `waiting` to `open`. | `ErrForbidden` |
| A closed case cannot be joined. | `ErrCaseClosed` |
| Only participants can send messages. An agent must join first. | `ErrForbidden` |
| Nobody can send messages to a closed case. | `ErrCaseClosed` |
| Only an agent who has joined can close the case. Closing an already closed case returns an error. | `ErrForbidden` / `ErrCaseClosed` |
| The case does not exist. | `ErrNotFound` |

## 6. API contract

### REST (`docs/openapi.yaml`)

All routes except `/v1/login` need `Authorization: Bearer <jwt>`.

| Method | Path | Who | Behaviour |
|---|---|---|---|
| POST | `/v1/login` | anyone | `{name, role}` → `{token, user}` |
| POST | `/v1/cases` | customer | `{question}` → a `waiting` case plus its first message |
| GET | `/v1/cases?status=` | both | A customer gets their own cases. An agent gets all cases. Newest `updatedAt` first. |
| GET | `/v1/cases/:id` | owner / agent | The case and its participants |
| POST | `/v1/cases/:id/join` | agent | Join the case (idempotent) |
| GET | `/v1/cases/:id/messages?before=&limit=` | owner / agent | Newest first. Default `limit` 50, max 100. |
| POST | `/v1/cases/:id/messages` | participant | `{body}` → the created message |
| POST | `/v1/cases/:id/close` | joined agent | Close the case |

Mapping from domain errors to HTTP status, done in one place (`handlers/errors.go`):

| Domain error | HTTP |
|---|---|
| `ErrInvalidInput` | 400 |
| missing or invalid token | 401 |
| `ErrForbidden` | 403 |
| `ErrNotFound` | 404 |
| `ErrCaseClosed` | 409 |
| anything else | 500 (logged; no internal details in the response) |

### WebSocket (`docs/events.md`), server-to-client only

| Endpoint | Who | Events |
|---|---|---|
| `GET /v1/cases/:id/events?token=` | anyone allowed to view the case | `message.created`, `participant.joined`, `case.closed` |
| `GET /v1/cases/events?token=` | agent | `case.created`, `case.status_changed` |

Event envelope: `{"type": "message.created", "caseId": "…", "data": {…}}`

The token is sent as a query parameter because browsers cannot set headers on a WebSocket. An invalid token or missing permission is rejected before the upgrade.

## 7. Data flow: sending a message

1. `web` sends `POST /v1/cases/:id/messages`.
2. The handler reads the user from the JWT and calls the `SendMessage` use case.
3. The use case loads the case, checks the rules, and saves the message through `MessageRepository`.
4. The use case calls `MessageBroadcaster.Publish(caseID, event)`.
5. The Hub sends the event to every socket connected to that case, including the sender's.
6. The handler returns `201` with the message.
7. The client stores messages in a map keyed by `id` and sorted by `createdAt`, so the HTTP response and the event show as one message.

## 8. Error handling and resilience

- **The database is the source of truth. The socket only notifies.** If broadcasting fails after the save, the send still succeeds, and the failure is logged.
- **Slow clients:** each connection has a buffered send channel (size 16). When it is full, the Hub drops that connection instead of waiting, so one slow client cannot hold up a room.
- **Reconnect (web):** exponential backoff of 1s, 2s, 4s and so on, up to 30s, with a "Reconnecting…" banner. After reconnecting, the client fetches the latest messages and merges them by `id`.
- **Failed send (web):** the text stays in the input box and a "Retry" button appears.

## 9. Testing strategy

TDD (Red → Green → Refactor) at every layer. `make test` runs all Go tests and Vitest tests.

| Layer | Tool | What it covers |
|---|---|---|
| `business/usecases` | `go test` + in-memory fakes | All business rules in section 5. This is the core of the TDD work. |
| `handlers` | `httptest` + fake use cases | Request parsing, JSON shape, error-to-status mapping, auth |
| `repositories` | `go test` + Mongo from docker | Queries, indexes, paging |
| `realtime` | `go test` + a real WebSocket client | Subscribe, broadcast to the right case only, dropping slow clients |
| `web/lib` | Vitest | Merge and dedupe messages, reconnect backoff |
| E2E | Cucumber + Playwright + Page Objects | The 6 user stories, using **two browser contexts** (customer and agent) to show that real time works |

Example feature file (declarative, following ADR 0007):

```gherkin
Feature: Real-time support conversation
    As a customer
    I want the support team to answer me in the chat
    So that I can solve my problem without waiting for email

    Scenario: Agent answers a waiting customer in real time
        Given customer "Ann" has asked "How do I reset my password?"
        When agent "Bob" joins Ann's case
        And agent "Bob" replies "Click 'Forgot password' on the login page"
        Then Ann should see Bob's reply without refreshing
```

## 10. ADRs to write

Same format as ODDS: `# ADR NNNN: Title`, then `## Status`, `## Context` (the problem, with a concrete example), `## Decision` (numbered rules with ✅ / ❌ examples), and `## Consequences` with `### Positive` and `### Negative`. `docs/adr/README.md` holds the index table.

| # | Title | Based on |
|---|---|---|
| 0001 | Clean Architecture Dependency Rule | api.odds-worklog ADR 0001 |
| 0002 | Monorepo: Go API + Next.js Web + E2E | new |
| 0003 | REST for Commands, WebSocket for Server Events | new |
| 0004 | In-memory Hub behind `MessageBroadcaster` Port | new |
| 0005 | Simple JWT Login behind `Authenticator` Port | new |
| 0006 | Page Object Pattern for E2E Tests | web.odds-worklog ADR 0001 |
| 0007 | Declarative Feature Files | web.odds-worklog ADR 0002 |

## 11. Cursor rules to write

Frontmatter: `description`, plus either `alwaysApply: true` or `globs:`. Every rule links to its ADR.

| File | Scope | Content |
|---|---|---|
| `tdd.mdc` | always | Red → Green → Refactor, run `make test` at each step |
| `verification.mdc` | always | `make test` must pass before a change counts as done |
| `clean-architecture.mdc` | `api/**/*.go` | The import rules from ADR 0001, with ✅/❌ examples |
| `openapi.mdc` | `api/handlers/**` | Route changes must update `docs/openapi.yaml` |
| `realtime-events.mdc` | `api/realtime/**`, `web/lib/socket*` | Event changes must update `docs/events.md`; the socket is server-to-client only |
| `e2e-cucumber.mdc` | `e2e/**` | Page Objects plus declarative features (ADR 0006/0007) |

## 12. Build order (input for the implementation plan)

1. Repo skeleton, `Makefile`, docker-compose (Mongo), ADRs, Cursor rules
2. `business/models` + use cases, test-first with fakes
3. Mongo repositories + integration tests
4. JWT auth + Echo handlers + `openapi.yaml`
5. Realtime Hub + WebSocket endpoints + `events.md`
6. Next.js: login, case list, case room (via `web/lib`)
7. E2E: the 6 user stories

## 13. Handoff note

This spec was written in the folder `jua odd`. The user will rename that folder to `class-practical-techniques`, then run `git init` and commit this spec there. Next step: the user reviews this spec, and after approval, create the implementation plan with the `writing-plans` skill.

Reference repos: https://github.com/oddsteam/web.odds-worklog (`.cursor/rules`, `docs/adr`) and https://github.com/oddsteam/api.odds-worklog (`.cursor/rules`, `docs/adr`, `business/usecases/*_ports.go`).
