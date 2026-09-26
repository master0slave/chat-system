# Support Chat API Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the repo skeleton, ADRs, Cursor rules and a complete Go API (REST + WebSocket) for the support chat, tested at every layer.

**Architecture:** Clean Architecture (ADR 0001). `business/models` holds entities and domain errors. `business/usecases` holds one file per use case and declares driving and driven ports. Adapters (`handlers` with Echo, `repositories` with MongoDB, `realtime` in-memory Hub, `pkg/auth` JWT) implement those ports, and `main` wires them together. Commands are REST. WebSockets only push events after a save (ADR 0003).

**Tech Stack:** Go 1.26, Echo v4.15.4, github.com/coder/websocket v1.8.15, mongo-driver v2.9.1, golang-jwt v5.3.1, google/uuid v1.6.0 (UUIDv7), testify v1.12.1, MongoDB 8 in docker compose, GNU Make.

**Spec:** `docs/superpowers/specs/2026-09-26-support-chat-design.md`

**This is plan 1 of 3.** It covers spec build-order steps 1–5. Plan 2 (Next.js web, step 6) and plan 3 (Cucumber E2E, step 7) will be written against the API this plan ships. When this plan is done, `make test` passes and the whole support conversation works over HTTP and WebSocket (`api/app_test.go` proves it).

## Global Constraints

- Go module name: `supportchat`, rooted at `api/`. Every import path in this plan starts with `supportchat/`.
- `business/models` imports only the standard library. `business/usecases` imports only `business/models` and the standard library (ADR 0001, enforced by `api/architecture_test.go`).
- Message and first-question bodies: 1–2000 characters after trimming, counted as Unicode code points.
- Subject: the first question on one line, cut to 80 characters.
- Message list: newest first, default `limit` 50, max 100.
- Hub buffer per connection: 16 events. When the buffer is full, the Hub drops the connection. Publishing never blocks.
- Error responses are always `{"error": "<message>"}`. The domain-error → HTTP mapping lives only in `api/handlers/errors.go`: InvalidInput 400, Unauthorized 401, Forbidden 403, NotFound 404, CaseClosed 409, anything else 500 with the body `internal error`.
- WebSocket token goes in `?token=`. The token and the permission are checked before the upgrade.
- `make test` must pass at the end of every task (`.cursor/rules/verification.mdc`). It needs Docker running, because it starts MongoDB.
- Run every `go` command from `api/`. After a step adds a new third-party import, run `go mod tidy` before testing.
- Commit messages: Conventional Commits (`feat:`, `test:`, `docs:`, `chore:`), ending with the line `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

### Decisions this plan adds to the spec

The spec leaves these open or states them loosely. Each one is pinned here so every task agrees:

| Topic | Decision | Why |
|---|---|---|
| Message paging cursor | `before` is a **message id**. IDs are UUIDv7, which sort as strings in creation order. Mongo sorts and filters on `_id`. | Paging on `createdAt` skips or repeats messages that share one millisecond. |
| Indexes | `cases`: `{updatedAt:-1}`, `{status:1, updatedAt:-1}`, `{customerId:1, updatedAt:-1}`. `messages`: `{caseId:1, _id:-1}` | These match the actual sort keys. The spec's `{customerId}` and `{caseId, createdAt}` would not cover the sorts. |
| Concurrent updates | `Case.Version` + `CaseRepository.Update` only saves when the stored version matches, otherwise it returns `ErrConflict`. Use cases retry up to 3 times. If all attempts fail, the result is **409**. | Two agents joining at the same moment would otherwise lose one participant. A message could also be saved after a close. |
| Broadcaster port | `PublishCase(caseID, event)` and `PublishAgents(event)` instead of a single `Publish` | There are two audiences: the people in a case, and the agent feed. |
| User ID | `role + ":" + lower-case name` with whitespace collapsed | Same person gets the same cases on every login (ADR 0005). |
| JSON shape | Models carry `json` tags. List endpoints return a bare array, never `null`. | Less mapping code. Recorded in ADR 0001 consequences. |
| Event payloads | Defined in `docs/events.md` (Task 12) | The spec only named the event types. |

## Review Focus

Inputs and conditions the spec implies but does not spell out. Each one has a test in the task named:

1. **Two agents join the same waiting case at the same moment.** Both must end up as participants. Covered by `TestConcurrentJoinsKeepBothAgents` (Task 4).
2. **A message is sent while an agent is closing the case.** The message must be rejected with 409, not saved after the close. Covered by `TestSendMessageLosesRaceWithClose` (Task 6).
3. **One browser tab stops reading its WebSocket.** Everyone else in the case keeps receiving messages. Covered by `TestSlowSubscriberIsDroppedWithoutBlockingOthers` (Task 10).
4. **Several messages are saved in the same millisecond, then paged with `limit=2`.** No gaps and no repeats. Covered by `TestMessageRepositoryPagesMessagesFromTheSameMillisecond` (Task 9).
5. **A customer logs in as "Ann Lee" once and " ann  lee" the next time.** They must see the same cases. Covered by `TestLoginGivesSameIDRegardlessOfCaseAndSpaces` (Task 8).

Also covered: Thai text counts characters, not bytes (Task 1). Internal errors do not leak details (Task 11). `alg: none` JWTs are rejected (Task 8). WebSocket connections from other origins are refused (Task 12).

## File Map

```
Makefile, .gitignore                                  Task 1
api/go.mod, api/architecture_test.go                  Task 1
api/deployment/local/docker-compose.yaml              Task 1
api/business/models/{errors,user,case,message,event}.go (+ tests)   Task 1
docs/adr/README.md, docs/adr/0001…0007-*.md           Task 2
.cursor/rules/*.mdc (6 files)                         Task 2
api/business/usecases/
  cases_driving_ports.go, cases_driven_ports.go       Task 3
  case_service.go                                     Task 3, replaced in Task 4
  unimplemented.go (temporary)                        Task 3, shrinks in Tasks 4–6, deleted in Task 7
  fakes_test.go, open_case.go (+ test)                Task 3
  join_case.go (+ test)                               Task 4
  close_case.go (+ test)                              Task 5
  send_message.go (+ test)                            Task 6
  list_cases.go, get_case.go, list_messages.go, read_cases_test.go   Task 7
  auth_driving_ports.go, auth_driven_ports.go, auth_service.go (+ test)   Task 8
api/pkg/auth/jwt.go (+ test)                          Task 8
api/pkg/clock, api/pkg/ids                            Task 9
api/repositories/{mongo,case_repository,message_repository}.go (+ tests)   Task 9
api/realtime/hub.go (+ test)                          Task 10
api/handlers/{errors,auth,cases,router}.go (+ tests), docs/openapi.yaml   Task 11
api/handlers/events.go (+ test), router.go update, docs/events.md          Task 12
api/app.go, api/main.go, api/app_test.go              Task 13
```

---

### Task 1: Repo skeleton, MongoDB, domain models and the architecture guard

**Files:**
- Create: `Makefile`, `.gitignore`, `api/go.mod`, `api/deployment/local/docker-compose.yaml`
- Create: `api/architecture_test.go`
- Create: `api/business/models/errors.go`, `user.go`, `case.go`, `message.go`, `event.go`
- Test: `api/business/models/case_test.go`, `api/business/models/message_test.go`

**Interfaces:**
- Consumes: nothing
- Produces (used by every later task):
  - `models.ErrInvalidInput`, `ErrUnauthorized`, `ErrForbidden`, `ErrNotFound`, `ErrCaseClosed`, `ErrConflict` (sentinel `error` values)
  - `models.Role` (`RoleCustomer`, `RoleAgent`, `Valid() bool`), `models.User{ID, Name string; Role Role}`
  - `models.CaseStatus` (`StatusWaiting`, `StatusOpen`, `StatusClosed`, `Valid() bool`), `models.Participant{UserID, Name string; Role Role; JoinedAt time.Time}`
  - `models.Case{ID, Subject, CustomerID string; Participants []Participant; Status CaseStatus; CreatedAt, UpdatedAt time.Time; ClosedAt *time.Time; Version int}` with `CanView(User) bool`, `HasParticipant(userID string) bool`
  - `models.SubjectFrom(question string) string`, `models.MaxSubjectLength = 80`
  - `models.MessageKind` (`KindText`, `KindSystem`), `models.Message{ID, CaseID, SenderID, SenderName string; SenderRole Role; Kind MessageKind; Body string; CreatedAt time.Time}`, `models.NormalizeBody(raw string) (string, error)`, `models.MaxBodyLength = 2000`
  - `models.EventType` constants `EventMessageCreated`, `EventParticipantJoined`, `EventCaseClosed`, `EventCaseCreated`, `EventCaseStatusChanged`, and `models.Event{Type EventType; CaseID string; Data any}`
  - Make targets `up`, `down`, `test`, `test-api`, `run-api`

- [ ] **Step 1: Put the repo under git and commit the spec**

The spec says the user would run `git init`. Check first:

```bash
cd /Users/tanamasgunpai/class-practical-techniques
git status || git init
git add docs/superpowers
git commit -m "docs: add support chat design spec and API plan

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

Expected: a commit containing the spec and this plan. If `git status` already works and they are committed, skip the commit.

- [ ] **Step 2: Create the Makefile, .gitignore and docker compose file**

`Makefile` (recipe lines must start with a TAB):

```makefile
COMPOSE = docker compose -f api/deployment/local/docker-compose.yaml

.PHONY: up down test test-api run-api

up: ## Start MongoDB and wait until it is healthy
	$(COMPOSE) up -d --wait

down: ## Stop MongoDB (data is kept in a volume)
	$(COMPOSE) down

test: up test-api ## Run every test. A change is done only when this passes.

test-api:
	cd api && go vet ./... && go test -race -count=1 ./...

run-api: up ## Run the API on :8080
	cd api && go run .
```

`.gitignore`:

```text
.DS_Store
.env
node_modules/
.next/
api/bin/
```

`api/deployment/local/docker-compose.yaml`:

```yaml
name: supportchat

services:
  mongo:
    image: mongo:8
    ports:
      - "27017:27017"
    volumes:
      - mongo-data:/data/db
    healthcheck:
      test: ["CMD", "mongosh", "--quiet", "--eval", "db.adminCommand('ping').ok"]
      interval: 2s
      timeout: 5s
      retries: 30

volumes:
  mongo-data:
```

- [ ] **Step 3: Start MongoDB**

Run: `make up`
Expected: ends with `Container supportchat-mongo-1 Healthy`. If port 27017 is taken, stop the other MongoDB first. Do not change the port, because the tests default to `mongodb://localhost:27017`.

- [ ] **Step 4: Create the Go module and the architecture guard**

```bash
mkdir -p api/business/models && cd api
go mod init supportchat
go get github.com/stretchr/testify@v1.12.1
```

`api/architecture_test.go`:

```go
package main

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
)

const module = "supportchat/"

// allowedInternalImports is the dependency rule from docs/adr/0001-clean-architecture-dependency-rule.md.
// Packages listed here may import only the internal packages in their list, plus the standard library.
var allowedInternalImports = map[string][]string{
	module + "business/models":   {},
	module + "business/usecases": {module + "business/models"},
}

func TestDependencyRule(t *testing.T) {
	out, err := exec.Command("go", "list", "-f", `{{.ImportPath}}{{range .Imports}} {{.}}{{end}}`, "./...").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		pkg, imports := fields[0], fields[1:]
		allowed, ruled := allowedInternalImports[pkg]
		if !ruled {
			continue
		}
		for _, imp := range imports {
			internal := strings.HasPrefix(imp, module)
			stdlib := !strings.Contains(strings.Split(imp, "/")[0], ".")
			if (internal && !slices.Contains(allowed, imp)) || (!internal && !stdlib) {
				t.Errorf("%s must not import %s (ADR 0001)", pkg, imp)
			}
		}
	}
}
```

- [ ] **Step 5: Write the failing model tests**

`api/business/models/case_test.go`:

```go
package models_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"supportchat/business/models"
)

var (
	ann = models.User{ID: "customer:ann", Name: "Ann", Role: models.RoleCustomer}
	cat = models.User{ID: "customer:cat", Name: "Cat", Role: models.RoleCustomer}
	bob = models.User{ID: "agent:bob", Name: "Bob", Role: models.RoleAgent}
)

func TestCaseCanView(t *testing.T) {
	c := models.Case{ID: "c1", CustomerID: ann.ID}

	assert.True(t, c.CanView(ann), "owner can view")
	assert.True(t, c.CanView(bob), "any agent can view")
	assert.False(t, c.CanView(cat), "another customer cannot view")
	assert.False(t, c.CanView(models.User{ID: ann.ID, Role: "admin"}), "unknown role cannot view")
}

func TestCaseHasParticipant(t *testing.T) {
	c := models.Case{Participants: []models.Participant{{UserID: ann.ID}, {UserID: bob.ID}}}

	assert.True(t, c.HasParticipant(bob.ID))
	assert.False(t, c.HasParticipant("agent:dan"))
}

func TestSubjectFrom(t *testing.T) {
	assert.Equal(t, "How do I reset my password?", models.SubjectFrom("How do I\n  reset my password?"))
	assert.Equal(t, strings.Repeat("ก", 80), models.SubjectFrom(strings.Repeat("ก", 81)))
}

func TestValidRoleAndStatus(t *testing.T) {
	assert.True(t, models.RoleAgent.Valid())
	assert.False(t, models.Role("admin").Valid())
	assert.True(t, models.StatusClosed.Valid())
	assert.False(t, models.CaseStatus("archived").Valid())
}
```

`api/business/models/message_test.go`:

```go
package models_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"supportchat/business/models"
)

func TestNormalizeBody(t *testing.T) {
	body, err := models.NormalizeBody("  hello \n")
	require.NoError(t, err)
	assert.Equal(t, "hello", body)

	for name, raw := range map[string]string{
		"empty":           "",
		"only whitespace": " \n\t ",
		"2001 characters": strings.Repeat("a", 2001),
	} {
		_, err := models.NormalizeBody(raw)
		assert.ErrorIs(t, err, models.ErrInvalidInput, name)
	}
}

func TestNormalizeBodyCountsCharactersNotBytes(t *testing.T) {
	thai := strings.Repeat("ส", 2000) // 6000 bytes, 2000 characters

	body, err := models.NormalizeBody(thai)

	require.NoError(t, err)
	assert.Equal(t, thai, body)
}
```

- [ ] **Step 6: Run the tests to verify they fail**

Run: `cd api && go test ./business/models/`
Expected: FAIL with compile errors such as `undefined: models.Case` and `undefined: models.NormalizeBody`.

- [ ] **Step 7: Write the models**

`api/business/models/errors.go`:

```go
package models

import "errors"

// Domain errors. handlers/errors.go maps each one to an HTTP status.
var (
	ErrInvalidInput = errors.New("invalid input")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrNotFound     = errors.New("not found")
	ErrCaseClosed   = errors.New("case is closed")
	ErrConflict     = errors.New("case was changed by another request, try again")
)
```

`api/business/models/user.go`:

```go
package models

type Role string

const (
	RoleCustomer Role = "customer"
	RoleAgent    Role = "agent"
)

func (r Role) Valid() bool { return r == RoleCustomer || r == RoleAgent }

type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role Role   `json:"role"`
}
```

`api/business/models/case.go`:

```go
package models

import (
	"strings"
	"time"
)

type CaseStatus string

const (
	StatusWaiting CaseStatus = "waiting"
	StatusOpen    CaseStatus = "open"
	StatusClosed  CaseStatus = "closed"
)

func (s CaseStatus) Valid() bool {
	return s == StatusWaiting || s == StatusOpen || s == StatusClosed
}

const MaxSubjectLength = 80

type Participant struct {
	UserID   string    `json:"userId"`
	Name     string    `json:"name"`
	Role     Role      `json:"role"`
	JoinedAt time.Time `json:"joinedAt"`
}

type Case struct {
	ID           string        `json:"id"`
	Subject      string        `json:"subject"`
	CustomerID   string        `json:"customerId"`
	Participants []Participant `json:"participants"`
	Status       CaseStatus    `json:"status"`
	CreatedAt    time.Time     `json:"createdAt"`
	UpdatedAt    time.Time     `json:"updatedAt"`
	ClosedAt     *time.Time    `json:"closedAt,omitempty"`
	// Version supports optimistic concurrency: CaseRepository.Update only saves
	// when the stored version still equals this one.
	Version int `json:"-"`
}

// CanView reports whether u may read this case: every agent, or the customer who owns it.
func (c Case) CanView(u User) bool {
	return u.Role == RoleAgent || (u.Role == RoleCustomer && c.CustomerID == u.ID)
}

func (c Case) HasParticipant(userID string) bool {
	for _, p := range c.Participants {
		if p.UserID == userID {
			return true
		}
	}
	return false
}

// SubjectFrom turns the first question into a one-line subject of at most MaxSubjectLength characters.
func SubjectFrom(question string) string {
	oneLine := []rune(strings.Join(strings.Fields(question), " "))
	if len(oneLine) > MaxSubjectLength {
		oneLine = oneLine[:MaxSubjectLength]
	}
	return string(oneLine)
}
```

`api/business/models/message.go`:

```go
package models

import (
	"strings"
	"time"
	"unicode/utf8"
)

type MessageKind string

const (
	KindText   MessageKind = "text"
	KindSystem MessageKind = "system"
)

const MaxBodyLength = 2000

type Message struct {
	ID         string      `json:"id"`
	CaseID     string      `json:"caseId"`
	SenderID   string      `json:"senderId"`
	SenderName string      `json:"senderName"`
	SenderRole Role        `json:"senderRole"`
	Kind       MessageKind `json:"kind"`
	Body       string      `json:"body"`
	CreatedAt  time.Time   `json:"createdAt"`
}

// NormalizeBody trims the text and checks that it has 1 to MaxBodyLength characters.
// Characters are counted as Unicode code points, so Thai text is not penalised for using 3 bytes each.
func NormalizeBody(raw string) (string, error) {
	body := strings.TrimSpace(raw)
	if n := utf8.RuneCountInString(body); n == 0 || n > MaxBodyLength {
		return "", ErrInvalidInput
	}
	return body, nil
}
```

`api/business/models/event.go`:

```go
package models

type EventType string

// Event types. docs/events.md is the contract for their payloads.
const (
	EventMessageCreated    EventType = "message.created"
	EventParticipantJoined EventType = "participant.joined"
	EventCaseClosed        EventType = "case.closed"
	EventCaseCreated       EventType = "case.created"
	EventCaseStatusChanged EventType = "case.status_changed"
)

// Event is the envelope sent to WebSocket clients.
type Event struct {
	Type   EventType `json:"type"`
	CaseID string    `json:"caseId"`
	Data   any       `json:"data"`
}
```

- [ ] **Step 8: Run all tests**

Run: `cd api && go mod tidy && cd .. && make test`
Expected: `ok  supportchat` and `ok  supportchat/business/models`.

- [ ] **Step 9: Prove the architecture guard catches a violation**

Create a temporary file `api/business/models/zz_violation.go`:

```go
package models

import _ "github.com/stretchr/testify/assert"
```

Run: `cd api && go test -run TestDependencyRule .`
Expected: FAIL with `supportchat/business/models must not import github.com/stretchr/testify/assert (ADR 0001)`.

Delete `api/business/models/zz_violation.go`, then run `make test` again. Expected: PASS.

- [ ] **Step 10: Commit**

```bash
git add Makefile .gitignore api/
git commit -m "feat: add repo skeleton, MongoDB compose and domain models

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: ADRs and Cursor rules

**Files:**
- Create: `docs/adr/README.md`, `docs/adr/0001-clean-architecture-dependency-rule.md`, `0002-monorepo-go-api-nextjs-web-e2e.md`, `0003-rest-for-commands-websocket-for-server-events.md`, `0004-in-memory-hub-behind-message-broadcaster-port.md`, `0005-simple-jwt-login-behind-authenticator-port.md`, `0006-page-object-pattern-for-e2e-tests.md`, `0007-declarative-feature-files.md`
- Create: `.cursor/rules/tdd.mdc`, `verification.mdc`, `clean-architecture.mdc`, `openapi.mdc`, `realtime-events.mdc`, `e2e-cucumber.mdc`

**Interfaces:**
- Consumes: the names from Task 1 (`architecture_test.go`, `make test`)
- Produces: the ADR numbers and file names that later code comments cite (`ADR 0001` … `ADR 0007`). ADR 0006 and 0007 are written now but first used in plan 3.

This task is documentation, so it has no Red step. Its check is Step 3.

- [ ] **Step 1: Write the ADRs**

`docs/adr/README.md`:

```markdown
# Architecture Decision Records

Each ADR records one decision: the problem, the rule we chose, and what it costs us.
Format: `# ADR NNNN: Title`, then `## Status`, `## Context`, `## Decision` (numbered rules with ✅ / ❌ examples), and `## Consequences` with `### Positive` and `### Negative`.

To change a decision, write a new ADR that supersedes the old one. Do not rewrite an accepted ADR.

| # | Title | Status |
|---|---|---|
| [0001](0001-clean-architecture-dependency-rule.md) | Clean Architecture Dependency Rule | Accepted |
| [0002](0002-monorepo-go-api-nextjs-web-e2e.md) | Monorepo: Go API + Next.js Web + E2E | Accepted |
| [0003](0003-rest-for-commands-websocket-for-server-events.md) | REST for Commands, WebSocket for Server Events | Accepted |
| [0004](0004-in-memory-hub-behind-message-broadcaster-port.md) | In-memory Hub behind `MessageBroadcaster` Port | Accepted |
| [0005](0005-simple-jwt-login-behind-authenticator-port.md) | Simple JWT Login behind `Authenticator` Port | Accepted |
| [0006](0006-page-object-pattern-for-e2e-tests.md) | Page Object Pattern for E2E Tests | Accepted |
| [0007](0007-declarative-feature-files.md) | Declarative Feature Files | Accepted |
```

`docs/adr/0001-clean-architecture-dependency-rule.md`:

````markdown
# ADR 0001: Clean Architecture Dependency Rule

## Status

Accepted

## Context

Business rules such as "nobody can send messages to a closed case" are the part of the system we most want to test and keep stable. If a use case imports the Mongo driver or Echo directly, testing that rule needs a database and an HTTP server, and switching the database later means editing business code.

For example, this use case cannot be tested without MongoDB:

```go
func (s *caseService) SendMessage(...) {
    s.mongo.Collection("cases").FindOne(ctx, bson.M{"_id": caseID}) // business code knows about Mongo
}
```

## Decision

1. `business/models` imports nothing from this module and nothing outside the standard library.

   ✅ `import "time"`
   ❌ `import "go.mongodb.org/mongo-driver/v2/bson"`

2. `business/usecases` imports only `business/models` and the standard library. It reaches infrastructure through interfaces it declares itself: **driving ports** (what callers may use, in `*_driving_ports.go`) and **driven ports** (what it needs, in `*_driven_ports.go`).

   ✅ `type CaseRepository interface { Get(ctx context.Context, id string) (models.Case, error) }` in `cases_driven_ports.go`
   ❌ `import "supportchat/repositories"` in a use case

3. Adapters (`handlers`, `repositories`, `realtime`, `pkg/...`) depend inward and implement the ports. They never import each other, except that `main` wires them together.

   ✅ `repositories.MongoCaseRepository` implements `usecases.CaseRepository`
   ❌ `handlers` calling `repositories` directly and skipping the use case

4. `handlers` call only driving ports (`usecases.CaseService`, `usecases.AuthService`).

5. `architecture_test.go` enforces rules 1 and 2 on every `make test`.

6. In `web`, components never call `fetch` or open a WebSocket. Only `web/lib` talks to the backend.

## Consequences

### Positive

- Every business rule is tested in milliseconds with in-memory fakes.
- MongoDB, the WebSocket hub, or the login method can be replaced by writing one new adapter.
- A reviewer can tell where a piece of code belongs from its imports.

### Negative

- More files: each port has an interface plus at least one adapter and one fake.
- Mapping between models and storage documents (`caseDoc`) is written by hand.
- As a pragmatic shortcut, models carry `json` tags so handlers can return them directly. If the API shape needs to differ from the model, add response types in `handlers` instead of changing the model.
````

`docs/adr/0002-monorepo-go-api-nextjs-web-e2e.md`:

```markdown
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
```

`docs/adr/0003-rest-for-commands-websocket-for-server-events.md`:

```markdown
# ADR 0003: REST for Commands, WebSocket for Server Events

## Status

Accepted

## Context

Participants must see new messages in real time. One option is to send everything, including "send message", over a WebSocket. That makes each action harder to test (no status codes), harder to document (no OpenAPI), and harder to retry safely. It also duplicates the auth and validation that REST already has.

## Decision

1. Every command and query is a REST call described in `docs/openapi.yaml`. Its response is the result.

   ✅ `POST /v1/cases/:id/messages` returns `201` with the saved message
   ❌ the client sends `{"type":"send","body":"hi"}` over the socket

2. WebSockets are **server-to-client only**. They carry notifications described in `docs/events.md`. The server closes a socket that sends data.

3. The database is the source of truth. A use case saves first, then publishes. If publishing fails, the command still succeeds.

4. Clients merge REST responses and events by message `id`, so a message that arrives both ways appears once. After a reconnect, the client refetches over REST.

5. Browsers cannot set headers on a WebSocket, so socket endpoints take `?token=`. The token and the permission are checked before the upgrade.

## Consequences

### Positive

- Commands get status codes, validation errors and OpenAPI docs for free.
- Real-time delivery can fail or lag without losing data.
- Handlers can be tested with `httptest` alone.

### Negative

- The sender receives its own message twice (response and event), so clients must deduplicate.
- A token in a query string can end up in access logs. Tokens are short-lived (12 hours), and Keycloak will replace this (ADR 0005).
```

`docs/adr/0004-in-memory-hub-behind-message-broadcaster-port.md`:

```markdown
# ADR 0004: In-memory Hub behind `MessageBroadcaster` Port

## Status

Accepted

## Context

Use cases must notify connected clients after they save. A message broker (Redis pub/sub, RabbitMQ) is only needed when more than one API instance runs, and the MVP runs one. Calling a broker, or the WebSocket library, directly from a use case would also break ADR 0001.

## Decision

1. Use cases publish through the driven port `usecases.MessageBroadcaster` (`PublishCase(caseID, event)`, `PublishAgents(event)`).

2. `realtime.Hub` implements it in memory: topics map to subscriber channels, protected by one mutex.

3. Publishing never blocks. Each subscriber has a buffer of `realtime.DefaultBufferSize` (16) events. When it is full, the Hub removes that subscriber and closes its channel. The WebSocket endpoint then closes the socket with status 1013 (try again later), and the client reconnects and refetches.

   ✅ `select { case s.ch <- e: default: h.remove(topic, s) }`
   ❌ `s.ch <- e` (one frozen browser tab stops the whole case)

4. Running more than one API instance requires a new adapter (Redis or RabbitMQ) behind the same port, recorded in a new ADR.

## Consequences

### Positive

- No extra infrastructure for the MVP. `make up` starts only MongoDB.
- Use cases are tested with a recording fake. The Hub is tested with plain channels.

### Negative

- Events are lost on restart. Clients recover by refetching over REST (ADR 0003).
- The API cannot scale horizontally until the broker adapter exists.
```

`docs/adr/0005-simple-jwt-login-behind-authenticator-port.md`:

```markdown
# ADR 0005: Simple JWT Login behind `Authenticator` Port

## Status

Accepted

## Context

The system needs to know who is calling (name and role) to enforce the rules. A real identity provider (Keycloak) is planned but would slow down the first round. The code that checks permissions should not change when Keycloak arrives.

## Decision

1. `POST /v1/login` takes `{name, role}` with no password and returns a JWT. This is for development and teaching only.

2. The user ID is `role + ":" + lower-case name` with runs of spaces collapsed, so the same person gets the same cases on every login.

   ✅ "Ann Lee" and " ann  lee" both become `customer:ann lee`

3. Tokens are HS256 JWTs signed with `JWT_SECRET`, valid for 12 hours. `pkg/auth.JWT` implements the driven port `usecases.Authenticator`. Verification accepts only HS256 and requires `exp`.

   ❌ accepting `alg: none` or tokens without `exp`

4. Handlers get the user from `usecases.AuthService.Authenticate`, never by parsing the token themselves.

5. Keycloak will be a new `Authenticator` adapter plus a new ADR that supersedes rule 1.

## Consequences

### Positive

- No identity infrastructure needed to start.
- Every permission check is already written against `models.User`, so Keycloak changes only the adapter and the login screen.

### Negative

- **Anyone can log in as anyone.** This must never be deployed where real customers can reach it.
- There is no logout or token revocation before expiry.
```

`docs/adr/0006-page-object-pattern-for-e2e-tests.md`:

```markdown
# ADR 0006: Page Object Pattern for E2E Tests

## Status

Accepted

## Context

E2E step definitions that use selectors directly break in many places when one screen changes. For example, if three steps use `page.locator('#send')`, renaming that button means editing all three.

## Decision

1. Each screen has one Page Object class in `e2e/cucumber/pages/` (`LoginPage`, `CaseListPage`, `CaseRoomPage`). It is the only code that knows that screen's selectors.

2. Page Object methods describe what a user does or sees, not how.

   ✅ `await caseRoom.sendMessage("Thanks")`
   ❌ `await page.fill('[data-testid="message-input"]', "Thanks"); await page.click('#send')` inside a step

3. Selectors use `data-testid` attributes or accessible roles, never CSS classes.

4. Step definitions in `e2e/cucumber/steps/` only call Page Objects and assertions.

5. Each actor (customer, agent) gets its own Playwright browser context, so one scenario can show real-time delivery between two people.

## Consequences

### Positive

- A UI change is fixed in one Page Object.
- Steps read like the feature files.

### Negative

- One more layer to write before the first E2E test runs.
```

`docs/adr/0007-declarative-feature-files.md`:

````markdown
# ADR 0007: Declarative Feature Files

## Status

Accepted

## Context

Feature files that list clicks and field names ("imperative" style) are long, break when the UI changes, and hide the business rule. For example:

```gherkin
When I click "Login"
And I type "Ann" into "name"
And I click "New case"
```

This says nothing about *why* Ann is there.

## Decision

1. Feature files describe behaviour in business language: who, what, and the outcome.

   ✅ `Given customer "Ann" has asked "How do I reset my password?"`
   ❌ `When I type "How do I reset my password?" into the "question" field`

2. Each feature file starts with `As a / I want / So that`.

3. Each user story in the spec has one feature file in `e2e/cucumber/features/`.

4. UI details live in Page Objects (ADR 0006), never in `.feature` files.

## Consequences

### Positive

- Non-developers can read and review the scenarios.
- Scenarios survive UI redesigns.

### Negative

- Step definitions do more work, because one step may cover several UI actions.
````

- [ ] **Step 2: Write the Cursor rules**

`.cursor/rules/tdd.mdc`:

```markdown
---
description: Test-driven development cycle for every change
alwaysApply: true
---

# TDD

Work in Red → Green → Refactor steps:

1. **Red:** write one failing test for the next behaviour. Run it and confirm it fails for the expected reason (not a typo or compile error elsewhere).
2. **Green:** write the least code that makes it pass.
3. **Refactor:** clean up while the tests stay green.

Run `make test` after each step.

- Use-case tests use the in-memory fakes in `api/business/usecases/fakes_test.go`, never MongoDB.
- Bug fixes start with a test that reproduces the bug.
- Do not write production code that no failing test asked for.
```

`.cursor/rules/verification.mdc`:

```markdown
---
description: What "done" means
alwaysApply: true
---

# Verification

A change is done only when `make test` passes on your machine, and you have read its output.

- Do not say "should work" or "tests pass" without running `make test` in this session.
- If a test fails, report the failure with its output. Do not skip, delete, or weaken the test to make it pass.
- If `make test` cannot run (for example, Docker is not running), say so. Do not report the change as done.
```

`.cursor/rules/clean-architecture.mdc`:

```markdown
---
description: Dependency rule for the Go API (ADR 0001)
globs: api/**/*.go
---

# Clean Architecture

See [ADR 0001](../../docs/adr/0001-clean-architecture-dependency-rule.md). `api/architecture_test.go` enforces the import rules.

| Package | May import |
|---|---|
| `business/models` | standard library only |
| `business/usecases` | `business/models` + standard library |
| `handlers`, `repositories`, `realtime`, `pkg/...` | `business/...` + libraries; never each other |
| `main` | everything (wiring only) |

✅ Declare what a use case needs as an interface in `<name>_driven_ports.go`, and implement it in an adapter.
❌ `import "go.mongodb.org/mongo-driver/v2/mongo"` in `business/`.

✅ A handler calls `usecases.CaseService`.
❌ A handler calls `repositories.MongoCaseRepository`.

- One use case per file in `business/usecases/` (`open_case.go`, `join_case.go`, …).
- Business rules and their errors (`models.ErrForbidden`, …) live in use cases, not in handlers or repositories.
- Map errors to HTTP statuses only in `handlers/errors.go`.
```

`.cursor/rules/openapi.mdc`:

```markdown
---
description: REST routes must match docs/openapi.yaml
globs: api/handlers/**
---

# OpenAPI contract

`docs/openapi.yaml` is the source of truth for REST (ADR 0003).

- Adding, removing or changing a route, a request field, a response field, or a status code means updating `docs/openapi.yaml` in the same commit.
- Routes are registered only in `api/handlers/router.go`.
- Errors are always `{"error": "<message>"}` with the status from `handlers/errors.go`.
- Never put internal details (driver errors, stack traces) in a response. Unknown errors are `500 {"error":"internal error"}` and are logged.
```

`.cursor/rules/realtime-events.mdc`:

```markdown
---
description: WebSocket events must match docs/events.md
globs: api/realtime/**,api/handlers/events*.go,web/lib/socket*
---

# Real-time events

`docs/events.md` is the source of truth for WebSocket events (ADR 0003, ADR 0004).

- Adding or changing an event type or its `data` means updating `docs/events.md` in the same commit.
- Sockets are **server-to-client only**. Commands go through REST.
- Event types are constants in `api/business/models/event.go`. Never write the string by hand.
- Publishing must never block: keep the Hub's drop-slow-subscriber behaviour.
- Clients merge events and REST responses by message `id`, and refetch after reconnecting.
```

`.cursor/rules/e2e-cucumber.mdc`:

```markdown
---
description: Cucumber + Playwright E2E conventions (ADR 0006, ADR 0007)
globs: e2e/**
---

# E2E with Cucumber

See [ADR 0006](../../docs/adr/0006-page-object-pattern-for-e2e-tests.md) and [ADR 0007](../../docs/adr/0007-declarative-feature-files.md).

- One feature file per user story in `e2e/cucumber/features/`, starting with `As a / I want / So that`.
- Steps are declarative business language.

  ✅ `When agent "Bob" joins Ann's case`
  ❌ `When I click the "Join" button`

- Selectors live only in Page Objects in `e2e/cucumber/pages/`, using `data-testid` or roles.
- Step definitions call Page Objects. They never call `page.locator` directly.
- Each actor gets its own browser context, so real-time scenarios use two or more contexts.
```

- [ ] **Step 3: Check the links and the index**

Run:

```bash
ls docs/adr | sort
grep -o '([0-9]\{4\}-[a-z0-9-]*\.md)' docs/adr/README.md | tr -d '()' | while read f; do test -f "docs/adr/$f" || echo "missing $f"; done
grep -L '^## Consequences' docs/adr/0*.md
```

Expected: eight files are listed (README + 0001–0007). There are no `missing` lines, and the last command prints nothing, because every ADR has a Consequences section.

- [ ] **Step 4: Commit**

```bash
git add docs/adr .cursor
git commit -m "docs: add ADRs 0001-0007 and Cursor rules

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Case ports, test fakes and OpenCase

**Files:**
- Create: `api/business/usecases/cases_driving_ports.go`, `cases_driven_ports.go`, `case_service.go`, `unimplemented.go`, `open_case.go`
- Test: `api/business/usecases/fakes_test.go`, `api/business/usecases/open_case_test.go`

**Interfaces:**
- Consumes: everything in `models` from Task 1
- Produces:
  - Driving ports: `usecases.OpenCase`, `JoinCase`, `SendMessage`, `CloseCase`, `ListCases`, `GetCase`, `ListMessages` (one-method interfaces) and `usecases.CaseService` (embeds all seven). The exact signatures are in `cases_driving_ports.go` below.
  - Driven ports: `usecases.CaseRepository` (`Insert`, `Get`, `Update`, `List(ctx, CaseFilter)`), `usecases.CaseFilter{CustomerID string; Status models.CaseStatus}`, `usecases.MessageRepository` (`Insert`, `List(ctx, MessageQuery)`), `usecases.MessageQuery{CaseID, BeforeID string; Limit int}`, `usecases.MessageBroadcaster` (`PublishCase(caseID string, e models.Event)`, `PublishAgents(e models.Event)`), `usecases.Clock` (`Now() time.Time`), `usecases.IDGenerator` (`NewID() string`)
  - `usecases.CaseDeps{Cases, Messages, Broadcaster, Clock, IDs}` and `usecases.NewCaseService(CaseDeps) CaseService`
  - Test helpers in `fakes_test.go` that Tasks 4–7 use: `newEnv()`, `env.openCase/join/close`, `env.cases.stored`, `env.cases.beforeUpdate`, `env.messages.inCase`, `env.events.types()`, `env.clock.now`, users `ann`, `cat` (customers), `bob`, `dan` (agents), time `t0`, and the helper `bodies`

`caseService` must satisfy the whole `CaseService` interface before every use case exists. `unimplemented.go` holds temporary stubs. Tasks 4–6 delete one stub each, and Task 7 deletes the file.

- [ ] **Step 1: Write the ports**

`api/business/usecases/cases_driving_ports.go`:

```go
package usecases

import (
	"context"

	"supportchat/business/models"
)

// Driving ports: what handlers may call. Each one is implemented in its own file.

type OpenCase interface {
	OpenCase(ctx context.Context, actor models.User, question string) (models.Case, models.Message, error)
}

type JoinCase interface {
	JoinCase(ctx context.Context, actor models.User, caseID string) (models.Case, error)
}

type SendMessage interface {
	SendMessage(ctx context.Context, actor models.User, caseID, body string) (models.Message, error)
}

type CloseCase interface {
	CloseCase(ctx context.Context, actor models.User, caseID string) (models.Case, error)
}

type ListCases interface {
	// ListCases returns the cases the actor may see, newest UpdatedAt first. An empty status means any status.
	ListCases(ctx context.Context, actor models.User, status models.CaseStatus) ([]models.Case, error)
}

type GetCase interface {
	GetCase(ctx context.Context, actor models.User, caseID string) (models.Case, error)
}

type ListMessages interface {
	// ListMessages returns messages older than beforeID (all if empty), newest first.
	// limit 0 means DefaultMessageLimit; values above MaxMessageLimit are capped.
	ListMessages(ctx context.Context, actor models.User, caseID, beforeID string, limit int) ([]models.Message, error)
}

type CaseService interface {
	OpenCase
	JoinCase
	SendMessage
	CloseCase
	ListCases
	GetCase
	ListMessages
}
```

`api/business/usecases/cases_driven_ports.go`:

```go
package usecases

import (
	"context"
	"time"

	"supportchat/business/models"
)

// Driven ports: what the case use cases need from the outside world.

type CaseFilter struct {
	CustomerID string            // empty means every customer
	Status     models.CaseStatus // empty means every status
}

type CaseRepository interface {
	Insert(ctx context.Context, c models.Case) error
	// Get returns models.ErrNotFound when the case does not exist.
	Get(ctx context.Context, id string) (models.Case, error)
	// Update saves c only if the stored version equals c.Version, and stores c.Version+1.
	// It returns models.ErrConflict when another request saved first, and models.ErrNotFound for an unknown id.
	Update(ctx context.Context, c models.Case) error
	// List returns matching cases, newest UpdatedAt first.
	List(ctx context.Context, f CaseFilter) ([]models.Case, error)
}

type MessageQuery struct {
	CaseID   string
	BeforeID string // empty means start from the newest message
	Limit    int
}

type MessageRepository interface {
	Insert(ctx context.Context, m models.Message) error
	// List returns messages whose ID is lower than BeforeID, newest first.
	// This works because IDGenerator returns IDs that sort in creation order.
	List(ctx context.Context, q MessageQuery) ([]models.Message, error)
}

// MessageBroadcaster notifies connected clients. It must not block: the database is the source of truth.
type MessageBroadcaster interface {
	PublishCase(caseID string, e models.Event)
	PublishAgents(e models.Event)
}

type Clock interface {
	Now() time.Time
}

// IDGenerator returns unique IDs that sort (as strings) in creation order.
type IDGenerator interface {
	NewID() string
}
```

- [ ] **Step 2: Write the fakes and the failing OpenCase tests**

`api/business/usecases/fakes_test.go`. The fake repository enforces the same version check as the Mongo one:

```go
package usecases_test

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"supportchat/business/models"
	"supportchat/business/usecases"
)

var (
	ann = models.User{ID: "customer:ann", Name: "Ann", Role: models.RoleCustomer}
	cat = models.User{ID: "customer:cat", Name: "Cat", Role: models.RoleCustomer}
	bob = models.User{ID: "agent:bob", Name: "Bob", Role: models.RoleAgent}
	dan = models.User{ID: "agent:dan", Name: "Dan", Role: models.RoleAgent}

	t0 = time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC)
)

// fakeCaseRepo is an in-memory CaseRepository with the same version check as the Mongo one.
type fakeCaseRepo struct {
	mu    sync.Mutex
	cases map[string]models.Case
	// beforeUpdate, if set, runs inside Update before the version check.
	// Tests use it to simulate another request saving first.
	beforeUpdate func()
}

func newFakeCaseRepo() *fakeCaseRepo { return &fakeCaseRepo{cases: map[string]models.Case{}} }

func (r *fakeCaseRepo) Insert(_ context.Context, c models.Case) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cases[c.ID] = clone(c)
	return nil
}

func (r *fakeCaseRepo) Get(_ context.Context, id string) (models.Case, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.cases[id]
	if !ok {
		return models.Case{}, models.ErrNotFound
	}
	return clone(c), nil
}

func (r *fakeCaseRepo) Update(_ context.Context, c models.Case) error {
	if hook := r.beforeUpdate; hook != nil {
		r.beforeUpdate = nil
		hook()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	stored, ok := r.cases[c.ID]
	if !ok {
		return models.ErrNotFound
	}
	if stored.Version != c.Version {
		return models.ErrConflict
	}
	c = clone(c)
	c.Version++
	r.cases[c.ID] = c
	return nil
}

func (r *fakeCaseRepo) List(_ context.Context, f usecases.CaseFilter) ([]models.Case, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.Case
	for _, c := range r.cases {
		if (f.CustomerID == "" || c.CustomerID == f.CustomerID) && (f.Status == "" || c.Status == f.Status) {
			out = append(out, clone(c))
		}
	}
	slices.SortFunc(out, func(a, b models.Case) int { return b.UpdatedAt.Compare(a.UpdatedAt) })
	return out, nil
}

func (r *fakeCaseRepo) stored(t *testing.T, id string) models.Case {
	t.Helper()
	c, err := r.Get(context.Background(), id)
	require.NoError(t, err)
	return c
}

func clone(c models.Case) models.Case {
	c.Participants = slices.Clone(c.Participants)
	return c
}

type fakeMessageRepo struct {
	mu       sync.Mutex
	messages []models.Message
}

func (r *fakeMessageRepo) Insert(_ context.Context, m models.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.messages = append(r.messages, m)
	return nil
}

func (r *fakeMessageRepo) List(_ context.Context, q usecases.MessageQuery) ([]models.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.Message
	for _, m := range slices.Backward(r.messages) {
		if m.CaseID == q.CaseID && (q.BeforeID == "" || m.ID < q.BeforeID) && len(out) < q.Limit {
			out = append(out, m)
		}
	}
	return out, nil
}

func (r *fakeMessageRepo) inCase(caseID string) []models.Message {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.Message
	for _, m := range r.messages {
		if m.CaseID == caseID {
			out = append(out, m)
		}
	}
	return out
}

type published struct {
	Topic string // "case:<id>" or "agents"
	Event models.Event
}

type recordingBroadcaster struct {
	mu     sync.Mutex
	events []published
}

func (b *recordingBroadcaster) PublishCase(caseID string, e models.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, published{Topic: "case:" + caseID, Event: e})
}

func (b *recordingBroadcaster) PublishAgents(e models.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, published{Topic: "agents", Event: e})
}

// types returns "topic type" pairs, which keeps assertions short.
func (b *recordingBroadcaster) types() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []string
	for _, p := range b.events {
		out = append(out, p.Topic+" "+string(p.Event.Type))
	}
	return out
}

func (b *recordingBroadcaster) reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = nil
}

type fixedClock struct{ now time.Time }

func (c *fixedClock) Now() time.Time { return c.now }

// sequentialIDs returns id-0001, id-0002, … which sort in creation order like UUIDv7.
type sequentialIDs struct {
	mu sync.Mutex
	n  int
}

func (g *sequentialIDs) NewID() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.n++
	return fmt.Sprintf("id-%04d", g.n)
}

type env struct {
	cases    *fakeCaseRepo
	messages *fakeMessageRepo
	events   *recordingBroadcaster
	clock    *fixedClock
	svc      usecases.CaseService
}

func newEnv() *env {
	e := &env{
		cases:    newFakeCaseRepo(),
		messages: &fakeMessageRepo{},
		events:   &recordingBroadcaster{},
		clock:    &fixedClock{now: t0},
	}
	e.svc = usecases.NewCaseService(usecases.CaseDeps{
		Cases:       e.cases,
		Messages:    e.messages,
		Broadcaster: e.events,
		Clock:       e.clock,
		IDs:         &sequentialIDs{},
	})
	return e
}

// openCase is a Given step: Ann opens a case, and the recorded events are cleared.
func (e *env) openCase(t *testing.T, customer models.User, question string) models.Case {
	t.Helper()
	c, _, err := e.svc.OpenCase(context.Background(), customer, question)
	require.NoError(t, err)
	e.events.reset()
	return c
}

func (e *env) join(t *testing.T, agent models.User, caseID string) {
	t.Helper()
	_, err := e.svc.JoinCase(context.Background(), agent, caseID)
	require.NoError(t, err)
	e.events.reset()
}

func (e *env) close(t *testing.T, agent models.User, caseID string) {
	t.Helper()
	_, err := e.svc.CloseCase(context.Background(), agent, caseID)
	require.NoError(t, err)
	e.events.reset()
}

func bodies(ms []models.Message) []string {
	var out []string
	for _, m := range ms {
		out = append(out, m.Body)
	}
	return out
}
```

`api/business/usecases/open_case_test.go`:

```go
package usecases_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"supportchat/business/models"
)

func TestOpenCase(t *testing.T) {
	e := newEnv()

	c, m, err := e.svc.OpenCase(context.Background(), ann, "  How do I reset my password?  ")

	require.NoError(t, err)
	assert.Equal(t, models.StatusWaiting, c.Status)
	assert.Equal(t, "How do I reset my password?", c.Subject)
	assert.Equal(t, ann.ID, c.CustomerID)
	assert.Equal(t, []models.Participant{{UserID: ann.ID, Name: "Ann", Role: models.RoleCustomer, JoinedAt: t0}}, c.Participants)
	assert.Equal(t, t0, c.CreatedAt)
	assert.Equal(t, t0, c.UpdatedAt)

	assert.Equal(t, c.ID, m.CaseID)
	assert.Equal(t, models.KindText, m.Kind)
	assert.Equal(t, "How do I reset my password?", m.Body)
	assert.Equal(t, ann.ID, m.SenderID)

	assert.Equal(t, c, e.cases.stored(t, c.ID))
	assert.Equal(t, []models.Message{m}, e.messages.inCase(c.ID))
	assert.Equal(t, []string{"agents case.created"}, e.events.types())
}

func TestOpenCaseRejectsAgents(t *testing.T) {
	e := newEnv()

	_, _, err := e.svc.OpenCase(context.Background(), bob, "hello")

	assert.ErrorIs(t, err, models.ErrForbidden)
	assert.Empty(t, e.events.types())
}

func TestOpenCaseRejectsEmptyQuestion(t *testing.T) {
	e := newEnv()

	_, _, err := e.svc.OpenCase(context.Background(), ann, "   ")

	assert.ErrorIs(t, err, models.ErrInvalidInput)
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `cd api && go test ./business/usecases/`
Expected: FAIL with `undefined: usecases.NewCaseService` and `undefined: usecases.CaseDeps`.

- [ ] **Step 4: Write the service shell, the stubs and OpenCase**

`api/business/usecases/case_service.go`:

```go
package usecases

import (
	"time"

	"supportchat/business/models"
)

type CaseDeps struct {
	Cases       CaseRepository
	Messages    MessageRepository
	Broadcaster MessageBroadcaster
	Clock       Clock
	IDs         IDGenerator
}

type caseService struct {
	CaseDeps
}

func NewCaseService(deps CaseDeps) CaseService {
	return &caseService{CaseDeps: deps}
}

func (s *caseService) newMessage(caseID string, sender models.User, kind models.MessageKind, body string, now time.Time) models.Message {
	return models.Message{
		ID:         s.IDs.NewID(),
		CaseID:     caseID,
		SenderID:   sender.ID,
		SenderName: sender.Name,
		SenderRole: sender.Role,
		Kind:       kind,
		Body:       body,
		CreatedAt:  now,
	}
}
```

`api/business/usecases/unimplemented.go`:

```go
package usecases

import (
	"context"
	"errors"

	"supportchat/business/models"
)

// Temporary: caseService must satisfy CaseService before every use case exists.
// Each later task deletes the stub for the use case it implements, and Task 7 deletes this file.

var errNotImplemented = errors.New("not implemented")

func (s *caseService) JoinCase(context.Context, models.User, string) (models.Case, error) {
	return models.Case{}, errNotImplemented
}

func (s *caseService) SendMessage(context.Context, models.User, string, string) (models.Message, error) {
	return models.Message{}, errNotImplemented
}

func (s *caseService) CloseCase(context.Context, models.User, string) (models.Case, error) {
	return models.Case{}, errNotImplemented
}

func (s *caseService) ListCases(context.Context, models.User, models.CaseStatus) ([]models.Case, error) {
	return nil, errNotImplemented
}

func (s *caseService) GetCase(context.Context, models.User, string) (models.Case, error) {
	return models.Case{}, errNotImplemented
}

func (s *caseService) ListMessages(context.Context, models.User, string, string, int) ([]models.Message, error) {
	return nil, errNotImplemented
}
```

`api/business/usecases/open_case.go`:

```go
package usecases

import (
	"context"

	"supportchat/business/models"
)

func (s *caseService) OpenCase(ctx context.Context, actor models.User, question string) (models.Case, models.Message, error) {
	if actor.Role != models.RoleCustomer {
		return models.Case{}, models.Message{}, models.ErrForbidden
	}
	body, err := models.NormalizeBody(question)
	if err != nil {
		return models.Case{}, models.Message{}, err
	}

	now := s.Clock.Now()
	c := models.Case{
		ID:         s.IDs.NewID(),
		Subject:    models.SubjectFrom(body),
		CustomerID: actor.ID,
		Participants: []models.Participant{
			{UserID: actor.ID, Name: actor.Name, Role: actor.Role, JoinedAt: now},
		},
		Status:    models.StatusWaiting,
		CreatedAt: now,
		UpdatedAt: now,
		Version:   1,
	}
	m := s.newMessage(c.ID, actor, models.KindText, body, now)

	if err := s.Cases.Insert(ctx, c); err != nil {
		return models.Case{}, models.Message{}, err
	}
	if err := s.Messages.Insert(ctx, m); err != nil {
		return models.Case{}, models.Message{}, err
	}
	s.Broadcaster.PublishAgents(models.Event{Type: models.EventCaseCreated, CaseID: c.ID, Data: c})
	return c, m, nil
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `make test`
Expected: PASS, including `TestOpenCase`, `TestOpenCaseRejectsAgents` and `TestOpenCaseRejectsEmptyQuestion`. The architecture test still passes, because `usecases` imports only `models` and the standard library.

- [ ] **Step 6: Commit**

```bash
git add api/business/usecases
git commit -m "feat: add case ports, in-memory fakes and OpenCase use case

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: JoinCase with optimistic concurrency

**Files:**
- Modify: `api/business/usecases/case_service.go` (replace the whole file: this adds `updateCase` and `errNoChange`)
- Modify: `api/business/usecases/unimplemented.go` (delete the `JoinCase` stub)
- Create: `api/business/usecases/join_case.go`
- Test: `api/business/usecases/join_case_test.go`

**Interfaces:**
- Consumes: `CaseDeps`, the fakes and `newMessage` from Task 3
- Produces: `(*caseService).updateCase(ctx, id string, change func(*models.Case) error) (models.Case, error)` and `errNoChange`. Tasks 5 and 6 use both. `updateCase` retries on `models.ErrConflict` up to `maxUpdateAttempts` (3) times. When `change` returns an error, `updateCase` returns the loaded case together with that error.

Rules from spec section 5: only agents can join (`ErrForbidden`). Joining twice has no effect. The first join moves `waiting` to `open`. A closed case cannot be joined (`ErrCaseClosed`, tested in Task 5 once CloseCase exists).

- [ ] **Step 1: Write the failing tests**

`api/business/usecases/join_case_test.go`:

```go
package usecases_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"supportchat/business/models"
)

func TestFirstAgentJoinOpensTheCase(t *testing.T) {
	e := newEnv()
	opened := e.openCase(t, ann, "help")
	e.clock.now = t0.Add(time.Minute)

	c, err := e.svc.JoinCase(context.Background(), bob, opened.ID)

	require.NoError(t, err)
	assert.Equal(t, models.StatusOpen, c.Status)
	assert.True(t, c.HasParticipant(bob.ID))
	assert.Equal(t, t0.Add(time.Minute), c.UpdatedAt)
	assert.Equal(t, c, e.cases.stored(t, opened.ID))

	msgs := e.messages.inCase(opened.ID)
	last := msgs[len(msgs)-1]
	assert.Equal(t, models.KindSystem, last.Kind)
	assert.Equal(t, "Bob joined the case", last.Body)

	assert.Equal(t, []string{
		"case:" + opened.ID + " participant.joined",
		"case:" + opened.ID + " message.created",
		"agents case.status_changed",
	}, e.events.types())
}

func TestSecondAgentJoinKeepsCaseOpenAndDoesNotRepeatStatusEvent(t *testing.T) {
	e := newEnv()
	opened := e.openCase(t, ann, "help")
	e.join(t, bob, opened.ID)

	c, err := e.svc.JoinCase(context.Background(), dan, opened.ID)

	require.NoError(t, err)
	assert.Equal(t, models.StatusOpen, c.Status)
	assert.Len(t, c.Participants, 3)
	assert.NotContains(t, e.events.types(), "agents case.status_changed")
}

func TestJoinTwiceHasNoEffect(t *testing.T) {
	e := newEnv()
	opened := e.openCase(t, ann, "help")
	e.join(t, bob, opened.ID)
	before := e.messages.inCase(opened.ID)

	c, err := e.svc.JoinCase(context.Background(), bob, opened.ID)

	require.NoError(t, err)
	assert.Len(t, c.Participants, 2)
	assert.Equal(t, before, e.messages.inCase(opened.ID))
	assert.Empty(t, e.events.types())
}

func TestJoinCaseErrors(t *testing.T) {
	e := newEnv()
	opened := e.openCase(t, ann, "help")

	_, err := e.svc.JoinCase(context.Background(), ann, opened.ID)
	assert.ErrorIs(t, err, models.ErrForbidden, "customers cannot join")

	_, err = e.svc.JoinCase(context.Background(), bob, "missing")
	assert.ErrorIs(t, err, models.ErrNotFound)
}

// Review Focus: two agents join the same waiting case at the same moment.
func TestConcurrentJoinsKeepBothAgents(t *testing.T) {
	e := newEnv()
	opened := e.openCase(t, ann, "help")
	// Dan's join saves between Bob's read and Bob's write.
	e.cases.beforeUpdate = func() { e.join(t, dan, opened.ID) }

	c, err := e.svc.JoinCase(context.Background(), bob, opened.ID)

	require.NoError(t, err)
	assert.True(t, c.HasParticipant(bob.ID))
	assert.True(t, c.HasParticipant(dan.ID))
	assert.Len(t, e.cases.stored(t, opened.ID).Participants, 3)
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd api && go test ./business/usecases/ -run 'Join'`
Expected: FAIL with `not implemented` from the stub.

- [ ] **Step 3: Implement**

Replace `api/business/usecases/case_service.go` with:

```go
package usecases

import (
	"context"
	"errors"
	"time"

	"supportchat/business/models"
)

type CaseDeps struct {
	Cases       CaseRepository
	Messages    MessageRepository
	Broadcaster MessageBroadcaster
	Clock       Clock
	IDs         IDGenerator
}

type caseService struct {
	CaseDeps
}

func NewCaseService(deps CaseDeps) CaseService {
	return &caseService{CaseDeps: deps}
}

const maxUpdateAttempts = 3

// errNoChange lets a change function say "nothing to save" (for example, joining twice).
var errNoChange = errors.New("no change")

// updateCase loads a case, applies change and saves it. When another request saved the
// case in between, it reloads and tries again, so two agents joining at once both end up
// as participants. If change returns an error, updateCase returns the loaded case and that error.
func (s *caseService) updateCase(ctx context.Context, id string, change func(*models.Case) error) (models.Case, error) {
	for attempt := 1; ; attempt++ {
		c, err := s.Cases.Get(ctx, id)
		if err != nil {
			return models.Case{}, err
		}
		if err := change(&c); err != nil {
			return c, err
		}
		err = s.Cases.Update(ctx, c)
		if err == nil {
			c.Version++
			return c, nil
		}
		if !errors.Is(err, models.ErrConflict) || attempt == maxUpdateAttempts {
			return models.Case{}, err
		}
	}
}

func (s *caseService) newMessage(caseID string, sender models.User, kind models.MessageKind, body string, now time.Time) models.Message {
	return models.Message{
		ID:         s.IDs.NewID(),
		CaseID:     caseID,
		SenderID:   sender.ID,
		SenderName: sender.Name,
		SenderRole: sender.Role,
		Kind:       kind,
		Body:       body,
		CreatedAt:  now,
	}
}
```

Delete the `JoinCase` function from `api/business/usecases/unimplemented.go`.

`api/business/usecases/join_case.go`:

```go
package usecases

import (
	"context"
	"errors"

	"supportchat/business/models"
)

func (s *caseService) JoinCase(ctx context.Context, actor models.User, caseID string) (models.Case, error) {
	if actor.Role != models.RoleAgent {
		return models.Case{}, models.ErrForbidden
	}

	now := s.Clock.Now()
	var wasWaiting bool
	c, err := s.updateCase(ctx, caseID, func(c *models.Case) error {
		if c.Status == models.StatusClosed {
			return models.ErrCaseClosed
		}
		if c.HasParticipant(actor.ID) {
			return errNoChange
		}
		wasWaiting = c.Status == models.StatusWaiting
		c.Participants = append(c.Participants, models.Participant{
			UserID: actor.ID, Name: actor.Name, Role: actor.Role, JoinedAt: now,
		})
		c.Status = models.StatusOpen
		c.UpdatedAt = now
		return nil
	})
	if errors.Is(err, errNoChange) {
		return c, nil
	}
	if err != nil {
		return models.Case{}, err
	}

	joined := c.Participants[len(c.Participants)-1]
	m := s.newMessage(c.ID, actor, models.KindSystem, actor.Name+" joined the case", now)
	if err := s.Messages.Insert(ctx, m); err != nil {
		return models.Case{}, err
	}
	s.Broadcaster.PublishCase(c.ID, models.Event{Type: models.EventParticipantJoined, CaseID: c.ID, Data: joined})
	s.Broadcaster.PublishCase(c.ID, models.Event{Type: models.EventMessageCreated, CaseID: c.ID, Data: m})
	if wasWaiting {
		s.Broadcaster.PublishAgents(models.Event{Type: models.EventCaseStatusChanged, CaseID: c.ID, Data: c})
	}
	return c, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `make test`
Expected: PASS. If `TestConcurrentJoinsKeepBothAgents` fails with 2 participants, `updateCase` is not reloading after `ErrConflict`.

- [ ] **Step 5: Commit**

```bash
git add api/business/usecases
git commit -m "feat: add JoinCase with optimistic-concurrency retry

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: CloseCase

**Files:**
- Modify: `api/business/usecases/unimplemented.go` (delete the `CloseCase` stub)
- Create: `api/business/usecases/close_case.go`
- Test: `api/business/usecases/close_case_test.go`

**Interfaces:**
- Consumes: `updateCase`, `newMessage` (Task 4), fakes (Task 3)
- Produces: `CloseCase` behaviour that Task 6's tests rely on (through `env.close`)

Rules: only an agent who has joined can close (`ErrForbidden`). Closing a closed case fails with `ErrCaseClosed`. Closing adds the system message "Case closed by Bob". Events are sent in this order: `message.created`, then `case.closed` to the case, then `case.status_changed` to agents.

- [ ] **Step 1: Write the failing tests**

`api/business/usecases/close_case_test.go`:

```go
package usecases_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"supportchat/business/models"
)

func TestJoinedAgentClosesCase(t *testing.T) {
	e := newEnv()
	opened := e.openCase(t, ann, "help")
	e.join(t, bob, opened.ID)
	closedAt := t0.Add(time.Hour)
	e.clock.now = closedAt

	c, err := e.svc.CloseCase(context.Background(), bob, opened.ID)

	require.NoError(t, err)
	assert.Equal(t, models.StatusClosed, c.Status)
	require.NotNil(t, c.ClosedAt)
	assert.Equal(t, closedAt, *c.ClosedAt)
	assert.Equal(t, models.StatusClosed, e.cases.stored(t, opened.ID).Status)

	msgs := e.messages.inCase(opened.ID)
	assert.Equal(t, "Case closed by Bob", msgs[len(msgs)-1].Body)
	assert.Equal(t, models.KindSystem, msgs[len(msgs)-1].Kind)

	assert.Equal(t, []string{
		"case:" + opened.ID + " message.created",
		"case:" + opened.ID + " case.closed",
		"agents case.status_changed",
	}, e.events.types())
}

func TestCloseCaseErrors(t *testing.T) {
	e := newEnv()
	opened := e.openCase(t, ann, "help")
	e.join(t, bob, opened.ID)

	_, err := e.svc.CloseCase(context.Background(), ann, opened.ID)
	assert.ErrorIs(t, err, models.ErrForbidden, "customers cannot close")

	_, err = e.svc.CloseCase(context.Background(), dan, opened.ID)
	assert.ErrorIs(t, err, models.ErrForbidden, "agents must join before closing")

	_, err = e.svc.CloseCase(context.Background(), bob, "missing")
	assert.ErrorIs(t, err, models.ErrNotFound)

	e.close(t, bob, opened.ID)
	_, err = e.svc.CloseCase(context.Background(), bob, opened.ID)
	assert.ErrorIs(t, err, models.ErrCaseClosed, "closing twice")
}

func TestClosedCaseCannotBeJoined(t *testing.T) {
	e := newEnv()
	opened := e.openCase(t, ann, "help")
	e.join(t, bob, opened.ID)
	e.close(t, bob, opened.ID)

	_, err := e.svc.JoinCase(context.Background(), dan, opened.ID)

	assert.ErrorIs(t, err, models.ErrCaseClosed)
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd api && go test ./business/usecases/ -run 'Close'`
Expected: FAIL with `not implemented`.

- [ ] **Step 3: Implement**

Delete the `CloseCase` function from `api/business/usecases/unimplemented.go`.

`api/business/usecases/close_case.go`:

```go
package usecases

import (
	"context"

	"supportchat/business/models"
)

func (s *caseService) CloseCase(ctx context.Context, actor models.User, caseID string) (models.Case, error) {
	if actor.Role != models.RoleAgent {
		return models.Case{}, models.ErrForbidden
	}

	now := s.Clock.Now()
	c, err := s.updateCase(ctx, caseID, func(c *models.Case) error {
		if !c.HasParticipant(actor.ID) {
			return models.ErrForbidden
		}
		if c.Status == models.StatusClosed {
			return models.ErrCaseClosed
		}
		c.Status = models.StatusClosed
		c.ClosedAt = &now
		c.UpdatedAt = now
		return nil
	})
	if err != nil {
		return models.Case{}, err
	}

	m := s.newMessage(c.ID, actor, models.KindSystem, "Case closed by "+actor.Name, now)
	if err := s.Messages.Insert(ctx, m); err != nil {
		return models.Case{}, err
	}
	s.Broadcaster.PublishCase(c.ID, models.Event{Type: models.EventMessageCreated, CaseID: c.ID, Data: m})
	s.Broadcaster.PublishCase(c.ID, models.Event{Type: models.EventCaseClosed, CaseID: c.ID, Data: c})
	s.Broadcaster.PublishAgents(models.Event{Type: models.EventCaseStatusChanged, CaseID: c.ID, Data: c})
	return c, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `make test`
Expected: PASS, including `TestClosedCaseCannotBeJoined`.

- [ ] **Step 5: Commit**

```bash
git add api/business/usecases
git commit -m "feat: add CloseCase use case

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: SendMessage

**Files:**
- Modify: `api/business/usecases/unimplemented.go` (delete the `SendMessage` stub)
- Create: `api/business/usecases/send_message.go`
- Test: `api/business/usecases/send_message_test.go`

**Interfaces:**
- Consumes: `updateCase`, `newMessage`, `models.NormalizeBody`
- Produces: `SendMessage`, which Task 7's paging tests use to create messages

Rules: only participants may send (`ErrForbidden`). An agent must join first. Nobody can send to a closed case (`ErrCaseClosed`). The body is 1–2000 characters (`ErrInvalidInput`). Sending bumps the case's `UpdatedAt` so it rises in the list.

- [ ] **Step 1: Write the failing tests**

`api/business/usecases/send_message_test.go`:

```go
package usecases_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"supportchat/business/models"
)

func TestParticipantsSendMessages(t *testing.T) {
	e := newEnv()
	opened := e.openCase(t, ann, "help")
	e.join(t, bob, opened.ID)
	e.clock.now = t0.Add(2 * time.Minute)

	m, err := e.svc.SendMessage(context.Background(), bob, opened.ID, " Click 'Forgot password' ")

	require.NoError(t, err)
	assert.Equal(t, "Click 'Forgot password'", m.Body)
	assert.Equal(t, models.KindText, m.Kind)
	assert.Equal(t, bob.ID, m.SenderID)
	assert.Equal(t, "Bob", m.SenderName)
	assert.Equal(t, models.RoleAgent, m.SenderRole)
	assert.Equal(t, t0.Add(2*time.Minute), e.cases.stored(t, opened.ID).UpdatedAt)
	assert.Contains(t, e.messages.inCase(opened.ID), m)
	assert.Equal(t, []string{"case:" + opened.ID + " message.created"}, e.events.types())

	_, err = e.svc.SendMessage(context.Background(), ann, opened.ID, "Thanks!")
	assert.NoError(t, err, "the customer can reply too")
}

func TestSendMessageErrors(t *testing.T) {
	e := newEnv()
	opened := e.openCase(t, ann, "help")
	e.join(t, bob, opened.ID)
	closed := e.openCase(t, ann, "old question")
	e.join(t, bob, closed.ID)
	e.close(t, bob, closed.ID)

	cases := []struct {
		name   string
		actor  models.User
		caseID string
		body   string
		want   error
	}{
		{"agent who has not joined", dan, opened.ID, "hi", models.ErrForbidden},
		{"another customer", cat, opened.ID, "hi", models.ErrForbidden},
		{"closed case", ann, closed.ID, "hi", models.ErrCaseClosed},
		{"empty body", ann, opened.ID, "  ", models.ErrInvalidInput},
		{"body too long", ann, opened.ID, strings.Repeat("a", 2001), models.ErrInvalidInput},
		{"unknown case", ann, "missing", "hi", models.ErrNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := e.svc.SendMessage(context.Background(), tc.actor, tc.caseID, tc.body)
			assert.ErrorIs(t, err, tc.want)
		})
	}
	assert.Empty(t, e.events.types())
}

func TestSendMessageLosesRaceWithClose(t *testing.T) {
	e := newEnv()
	opened := e.openCase(t, ann, "help")
	e.join(t, bob, opened.ID)
	// Bob closes the case between Ann's read and Ann's write.
	e.cases.beforeUpdate = func() { e.close(t, bob, opened.ID) }

	_, err := e.svc.SendMessage(context.Background(), ann, opened.ID, "one more thing")

	assert.ErrorIs(t, err, models.ErrCaseClosed)
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd api && go test ./business/usecases/ -run 'Send'`
Expected: FAIL with `not implemented`.

- [ ] **Step 3: Implement**

Delete the `SendMessage` function from `api/business/usecases/unimplemented.go`.

`api/business/usecases/send_message.go`:

```go
package usecases

import (
	"context"

	"supportchat/business/models"
)

func (s *caseService) SendMessage(ctx context.Context, actor models.User, caseID, body string) (models.Message, error) {
	body, err := models.NormalizeBody(body)
	if err != nil {
		return models.Message{}, err
	}

	now := s.Clock.Now()
	// Touching UpdatedAt through updateCase makes "is the case still open?" and the save
	// one versioned step, so a message cannot slip in after a close that saved first.
	c, err := s.updateCase(ctx, caseID, func(c *models.Case) error {
		if !c.HasParticipant(actor.ID) {
			return models.ErrForbidden
		}
		if c.Status == models.StatusClosed {
			return models.ErrCaseClosed
		}
		c.UpdatedAt = now
		return nil
	})
	if err != nil {
		return models.Message{}, err
	}

	m := s.newMessage(c.ID, actor, models.KindText, body, now)
	if err := s.Messages.Insert(ctx, m); err != nil {
		return models.Message{}, err
	}
	s.Broadcaster.PublishCase(c.ID, models.Event{Type: models.EventMessageCreated, CaseID: c.ID, Data: m})
	return m, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `make test`
Expected: PASS, including `TestSendMessageLosesRaceWithClose`.

- [ ] **Step 5: Commit**

```bash
git add api/business/usecases
git commit -m "feat: add SendMessage use case

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: Read use cases: ListCases, GetCase, ListMessages

**Files:**
- Delete: `api/business/usecases/unimplemented.go`
- Create: `api/business/usecases/list_cases.go`, `get_case.go`, `list_messages.go`
- Test: `api/business/usecases/read_cases_test.go`

**Interfaces:**
- Consumes: ports and fakes from Task 3
- Produces: `usecases.DefaultMessageLimit = 50`, `usecases.MaxMessageLimit = 100`. `CaseService` is now fully implemented.

Rules: a customer sees only their own cases, and an agent sees all of them, newest `UpdatedAt` first. An unknown `status` filter is `ErrInvalidInput`. Viewing someone else's case is `ErrForbidden`. `limit` 0 means 50, anything above 100 is capped, and a negative limit is `ErrInvalidInput`.

- [ ] **Step 1: Write the failing tests**

`api/business/usecases/read_cases_test.go`:

```go
package usecases_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"supportchat/business/models"
	"supportchat/business/usecases"
)

func TestListCases(t *testing.T) {
	e := newEnv()
	first := e.openCase(t, ann, "first")
	e.clock.now = t0.Add(time.Minute)
	catsCase := e.openCase(t, cat, "cat's question")
	e.clock.now = t0.Add(2 * time.Minute)
	e.join(t, bob, first.ID) // first is now the most recently updated

	agentView, err := e.svc.ListCases(context.Background(), bob, "")
	require.NoError(t, err)
	assert.Equal(t, []string{first.ID, catsCase.ID}, ids(agentView), "agents see all cases, newest update first")

	annView, err := e.svc.ListCases(context.Background(), ann, "")
	require.NoError(t, err)
	assert.Equal(t, []string{first.ID}, ids(annView), "customers see only their own cases")

	waiting, err := e.svc.ListCases(context.Background(), bob, models.StatusWaiting)
	require.NoError(t, err)
	assert.Equal(t, []string{catsCase.ID}, ids(waiting))

	_, err = e.svc.ListCases(context.Background(), bob, "archived")
	assert.ErrorIs(t, err, models.ErrInvalidInput)
}

func TestGetCase(t *testing.T) {
	e := newEnv()
	opened := e.openCase(t, ann, "help")

	c, err := e.svc.GetCase(context.Background(), ann, opened.ID)
	require.NoError(t, err)
	assert.Equal(t, opened.ID, c.ID)

	_, err = e.svc.GetCase(context.Background(), bob, opened.ID)
	assert.NoError(t, err, "agents can view any case, even before joining")

	_, err = e.svc.GetCase(context.Background(), cat, opened.ID)
	assert.ErrorIs(t, err, models.ErrForbidden)

	_, err = e.svc.GetCase(context.Background(), ann, "missing")
	assert.ErrorIs(t, err, models.ErrNotFound)
}

func TestListMessagesPagesNewestFirst(t *testing.T) {
	e := newEnv()
	opened := e.openCase(t, ann, "message 0")
	for i := 1; i <= 4; i++ {
		_, err := e.svc.SendMessage(context.Background(), ann, opened.ID, fmt.Sprintf("message %d", i))
		require.NoError(t, err)
	}

	page1, err := e.svc.ListMessages(context.Background(), ann, opened.ID, "", 2)
	require.NoError(t, err)
	assert.Equal(t, []string{"message 4", "message 3"}, bodies(page1))

	page2, err := e.svc.ListMessages(context.Background(), ann, opened.ID, page1[1].ID, 2)
	require.NoError(t, err)
	assert.Equal(t, []string{"message 2", "message 1"}, bodies(page2))
}

func TestListMessagesLimits(t *testing.T) {
	e := newEnv()
	opened := e.openCase(t, ann, "help")
	for i := range usecases.MaxMessageLimit + 10 {
		_, err := e.svc.SendMessage(context.Background(), ann, opened.ID, fmt.Sprint(i))
		require.NoError(t, err)
	}

	def, err := e.svc.ListMessages(context.Background(), ann, opened.ID, "", 0)
	require.NoError(t, err)
	assert.Len(t, def, usecases.DefaultMessageLimit)

	capped, err := e.svc.ListMessages(context.Background(), ann, opened.ID, "", 1000)
	require.NoError(t, err)
	assert.Len(t, capped, usecases.MaxMessageLimit)

	_, err = e.svc.ListMessages(context.Background(), ann, opened.ID, "", -1)
	assert.ErrorIs(t, err, models.ErrInvalidInput)

	_, err = e.svc.ListMessages(context.Background(), cat, opened.ID, "", 0)
	assert.ErrorIs(t, err, models.ErrForbidden)
}

func ids(cs []models.Case) []string {
	var out []string
	for _, c := range cs {
		out = append(out, c.ID)
	}
	return out
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd api && go test ./business/usecases/ -run 'List|Get'`
Expected: FAIL with `not implemented`.

- [ ] **Step 3: Implement**

Delete `api/business/usecases/unimplemented.go`.

`api/business/usecases/list_cases.go`:

```go
package usecases

import (
	"context"

	"supportchat/business/models"
)

func (s *caseService) ListCases(ctx context.Context, actor models.User, status models.CaseStatus) ([]models.Case, error) {
	if status != "" && !status.Valid() {
		return nil, models.ErrInvalidInput
	}
	filter := CaseFilter{Status: status}
	switch actor.Role {
	case models.RoleAgent:
	case models.RoleCustomer:
		filter.CustomerID = actor.ID
	default:
		return nil, models.ErrForbidden
	}
	return s.Cases.List(ctx, filter)
}
```

`api/business/usecases/get_case.go`:

```go
package usecases

import (
	"context"

	"supportchat/business/models"
)

func (s *caseService) GetCase(ctx context.Context, actor models.User, caseID string) (models.Case, error) {
	c, err := s.Cases.Get(ctx, caseID)
	if err != nil {
		return models.Case{}, err
	}
	if !c.CanView(actor) {
		return models.Case{}, models.ErrForbidden
	}
	return c, nil
}
```

`api/business/usecases/list_messages.go`:

```go
package usecases

import (
	"context"

	"supportchat/business/models"
)

const (
	DefaultMessageLimit = 50
	MaxMessageLimit     = 100
)

func (s *caseService) ListMessages(ctx context.Context, actor models.User, caseID, beforeID string, limit int) ([]models.Message, error) {
	if limit < 0 {
		return nil, models.ErrInvalidInput
	}
	if limit == 0 {
		limit = DefaultMessageLimit
	}
	limit = min(limit, MaxMessageLimit)

	if _, err := s.GetCase(ctx, actor, caseID); err != nil {
		return nil, err
	}
	return s.Messages.List(ctx, MessageQuery{CaseID: caseID, BeforeID: beforeID, Limit: limit})
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `make test`
Expected: PASS. Also run `grep -rn errNotImplemented api/`. Expected: no output.

- [ ] **Step 5: Commit**

```bash
git add -A api/business/usecases
git commit -m "feat: add ListCases, GetCase and ListMessages use cases

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 8: Login: AuthService and the JWT adapter

**Files:**
- Create: `api/business/usecases/auth_driving_ports.go`, `auth_driven_ports.go`, `auth_service.go`
- Create: `api/pkg/auth/jwt.go`
- Test: `api/business/usecases/auth_service_test.go`, `api/pkg/auth/jwt_test.go`

**Interfaces:**
- Consumes: `models.User`, `models.Role`, `models.ErrInvalidInput`, `models.ErrUnauthorized`
- Produces:
  - `usecases.AuthService` with `Login(ctx, name string, role models.Role) (token string, user models.User, err error)` and `Authenticate(ctx, token string) (models.User, error)`, built by `usecases.NewAuthService(Authenticator) AuthService`
  - `usecases.Authenticator` with `Issue(models.User) (string, error)` and `Verify(token string) (models.User, error)`
  - `usecases.MaxNameLength = 50`
  - `auth.NewJWT(secret []byte, ttl time.Duration) *auth.JWT`, which implements `usecases.Authenticator`

- [ ] **Step 1: Write the failing use case tests**

`api/business/usecases/auth_service_test.go`:

```go
package usecases_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"supportchat/business/models"
	"supportchat/business/usecases"
)

// fakeAuthenticator issues "token-for:<id>|<name>|<role>" so tests can read what was issued.
type fakeAuthenticator struct{}

func (fakeAuthenticator) Issue(u models.User) (string, error) {
	return "token-for:" + u.ID + "|" + u.Name + "|" + string(u.Role), nil
}

func (fakeAuthenticator) Verify(token string) (models.User, error) {
	rest, ok := strings.CutPrefix(token, "token-for:")
	parts := strings.Split(rest, "|")
	if !ok || len(parts) != 3 {
		return models.User{}, models.ErrUnauthorized
	}
	return models.User{ID: parts[0], Name: parts[1], Role: models.Role(parts[2])}, nil
}

func TestLoginIssuesTokenForUser(t *testing.T) {
	svc := usecases.NewAuthService(fakeAuthenticator{})

	token, u, err := svc.Login(context.Background(), "  Ann  ", models.RoleCustomer)

	require.NoError(t, err)
	assert.Equal(t, models.User{ID: "customer:ann", Name: "Ann", Role: models.RoleCustomer}, u)
	assert.Equal(t, "token-for:customer:ann|Ann|customer", token)

	back, err := svc.Authenticate(context.Background(), token)
	require.NoError(t, err)
	assert.Equal(t, u, back)
}

// Review Focus: the same person types their name slightly differently on the next login.
func TestLoginGivesSameIDRegardlessOfCaseAndSpaces(t *testing.T) {
	svc := usecases.NewAuthService(fakeAuthenticator{})

	_, first, err := svc.Login(context.Background(), "Ann  Lee", models.RoleCustomer)
	require.NoError(t, err)
	_, again, err := svc.Login(context.Background(), " ann lee", models.RoleCustomer)
	require.NoError(t, err)

	assert.Equal(t, first.ID, again.ID)
	assert.Equal(t, "customer:ann lee", again.ID)
}

func TestLoginValidation(t *testing.T) {
	svc := usecases.NewAuthService(fakeAuthenticator{})

	for name, tc := range map[string]struct {
		name string
		role models.Role
	}{
		"empty name":   {"   ", models.RoleCustomer},
		"long name":    {strings.Repeat("a", usecases.MaxNameLength+1), models.RoleCustomer},
		"unknown role": {"Ann", "admin"},
	} {
		_, _, err := svc.Login(context.Background(), tc.name, tc.role)
		assert.ErrorIs(t, err, models.ErrInvalidInput, name)
	}
}

func TestAuthenticateRejectsEmptyToken(t *testing.T) {
	svc := usecases.NewAuthService(fakeAuthenticator{})

	_, err := svc.Authenticate(context.Background(), "")

	assert.ErrorIs(t, err, models.ErrUnauthorized)
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd api && go test ./business/usecases/ -run 'Login|Authenticate'`
Expected: FAIL with `undefined: usecases.NewAuthService`.

- [ ] **Step 3: Implement the ports and the service**

`api/business/usecases/auth_driving_ports.go`:

```go
package usecases

import (
	"context"

	"supportchat/business/models"
)

type AuthService interface {
	// Login returns a signed token for the user. Name and role are checked; there is no password (ADR 0005).
	Login(ctx context.Context, name string, role models.Role) (string, models.User, error)
	// Authenticate returns the user in a token, or models.ErrUnauthorized.
	Authenticate(ctx context.Context, token string) (models.User, error)
}
```

`api/business/usecases/auth_driven_ports.go`:

```go
package usecases

import "supportchat/business/models"

type Authenticator interface {
	Issue(u models.User) (string, error)
	// Verify returns models.ErrUnauthorized for a malformed, expired or tampered token.
	Verify(token string) (models.User, error)
}
```

`api/business/usecases/auth_service.go`:

```go
package usecases

import (
	"context"
	"strings"
	"unicode/utf8"

	"supportchat/business/models"
)

const MaxNameLength = 50

type authService struct {
	authenticator Authenticator
}

func NewAuthService(a Authenticator) AuthService {
	return &authService{authenticator: a}
}

func (s *authService) Login(_ context.Context, name string, role models.Role) (string, models.User, error) {
	name = strings.Join(strings.Fields(name), " ")
	if n := utf8.RuneCountInString(name); n == 0 || n > MaxNameLength || !role.Valid() {
		return "", models.User{}, models.ErrInvalidInput
	}
	// The ID comes from role + name, so "Ann" gets the same cases every time she logs in.
	u := models.User{ID: string(role) + ":" + strings.ToLower(name), Name: name, Role: role}
	token, err := s.authenticator.Issue(u)
	if err != nil {
		return "", models.User{}, err
	}
	return token, u, nil
}

func (s *authService) Authenticate(_ context.Context, token string) (models.User, error) {
	if token == "" {
		return models.User{}, models.ErrUnauthorized
	}
	return s.authenticator.Verify(token)
}
```

Run: `cd api && go test ./business/usecases/`
Expected: PASS.

- [ ] **Step 4: Write the failing JWT tests**

This is an internal test (`package auth`), so it can replace the clock.

`api/pkg/auth/jwt_test.go`:

```go
package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"supportchat/business/models"
)

var ann = models.User{ID: "customer:ann", Name: "Ann", Role: models.RoleCustomer}

func TestIssueThenVerify(t *testing.T) {
	j := NewJWT([]byte("secret"), time.Hour)

	token, err := j.Issue(ann)
	require.NoError(t, err)
	u, err := j.Verify(token)

	require.NoError(t, err)
	assert.Equal(t, ann, u)
}

func TestVerifyRejectsExpiredToken(t *testing.T) {
	j := NewJWT([]byte("secret"), time.Hour)
	issuedAt := time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC)
	j.now = func() time.Time { return issuedAt }
	token, err := j.Issue(ann)
	require.NoError(t, err)

	j.now = func() time.Time { return issuedAt.Add(time.Hour + time.Second) }
	_, err = j.Verify(token)

	assert.ErrorIs(t, err, models.ErrUnauthorized)
}

func TestVerifyRejectsOtherSecretAndGarbage(t *testing.T) {
	token, err := NewJWT([]byte("other"), time.Hour).Issue(ann)
	require.NoError(t, err)

	_, err = NewJWT([]byte("secret"), time.Hour).Verify(token)
	assert.ErrorIs(t, err, models.ErrUnauthorized)

	_, err = NewJWT([]byte("secret"), time.Hour).Verify("not-a-jwt")
	assert.ErrorIs(t, err, models.ErrUnauthorized)
}

func TestVerifyRejectsNoneAlgorithm(t *testing.T) {
	unsigned := jwt.NewWithClaims(jwt.SigningMethodNone, claims{
		Role:             models.RoleAgent,
		RegisteredClaims: jwt.RegisteredClaims{Subject: "agent:eve", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	})
	token, err := unsigned.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	_, err = NewJWT([]byte("secret"), time.Hour).Verify(token)

	assert.ErrorIs(t, err, models.ErrUnauthorized)
}
```

Run: `cd api && go get github.com/golang-jwt/jwt/v5@v5.3.1 && go test ./pkg/auth/`
Expected: FAIL with `undefined: NewJWT`.

- [ ] **Step 5: Implement the JWT adapter**

`api/pkg/auth/jwt.go`:

```go
// Package auth implements usecases.Authenticator with HS256 JWTs (ADR 0005).
package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"

	"supportchat/business/models"
)

type JWT struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

func NewJWT(secret []byte, ttl time.Duration) *JWT {
	return &JWT{secret: secret, ttl: ttl, now: time.Now}
}

type claims struct {
	Name string      `json:"name"`
	Role models.Role `json:"role"`
	jwt.RegisteredClaims
}

func (j *JWT) Issue(u models.User) (string, error) {
	now := j.now()
	c := claims{
		Name: u.Name,
		Role: u.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(j.ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(j.secret)
}

func (j *JWT) Verify(token string) (models.User, error) {
	var c claims
	_, err := jwt.ParseWithClaims(token, &c,
		func(*jwt.Token) (any, error) { return j.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(j.now),
	)
	if err != nil || c.Subject == "" || !c.Role.Valid() {
		return models.User{}, models.ErrUnauthorized
	}
	return models.User{ID: c.Subject, Name: c.Name, Role: c.Role}, nil
}
```

- [ ] **Step 6: Run all tests**

Run: `cd api && go mod tidy && cd .. && make test`
Expected: PASS, including `TestVerifyRejectsNoneAlgorithm` and `TestVerifyRejectsExpiredToken`.

- [ ] **Step 7: Commit**

```bash
git add api/
git commit -m "feat: add login use case and HS256 JWT authenticator

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 9: MongoDB repositories, clock and ID generator

**Files:**
- Create: `api/pkg/clock/clock.go`, `api/pkg/ids/ids.go`
- Create: `api/repositories/mongo.go`, `case_repository.go`, `message_repository.go`
- Test: `api/pkg/clock/clock_test.go`, `api/pkg/ids/ids_test.go`, `api/repositories/testdb_test.go`, `case_repository_test.go`, `message_repository_test.go`

**Interfaces:**
- Consumes: `usecases.CaseRepository`, `usecases.MessageRepository`, `usecases.CaseFilter`, `usecases.MessageQuery` (Task 3)
- Produces:
  - `clock.System{}`, which implements `usecases.Clock` (UTC, cut to milliseconds)
  - `ids.UUIDv7{}`, which implements `usecases.IDGenerator`
  - `repositories.Connect(ctx, uri string) (*mongo.Client, error)` and `repositories.EnsureIndexes(ctx, *mongo.Database) error`
  - `repositories.NewMongoCaseRepository(*mongo.Database) *MongoCaseRepository` and `repositories.NewMongoMessageRepository(*mongo.Database) *MongoMessageRepository`

These tests need MongoDB. `make test` starts it. Each test uses its own database (`test_<nanos>`) and drops it afterwards. Set `MONGO_URI` to use a MongoDB that is not on localhost:27017.

- [ ] **Step 1: Write the failing clock and ID tests**

`api/pkg/clock/clock_test.go`:

```go
package clock_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"supportchat/pkg/clock"
)

func TestNowIsUTCMilliseconds(t *testing.T) {
	now := clock.System{}.Now()

	assert.Equal(t, time.UTC, now.Location())
	assert.Zero(t, now.Nanosecond()%int(time.Millisecond))
}
```

`api/pkg/ids/ids_test.go`:

```go
package ids_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"supportchat/pkg/ids"
)

func TestIDsSortInCreationOrder(t *testing.T) {
	gen := ids.UUIDv7{}
	prev := gen.NewID()
	for range 10_000 { // many IDs land in the same millisecond
		next := gen.NewID()
		assert.Less(t, prev, next)
		prev = next
	}
}
```

Run: `cd api && go test ./pkg/...`
Expected: FAIL with `no non-test Go files` / `undefined: clock.System`.

- [ ] **Step 2: Implement clock and IDs**

`api/pkg/clock/clock.go`:

```go
// Package clock implements usecases.Clock.
package clock

import "time"

type System struct{}

// Now returns UTC time cut to milliseconds, the precision MongoDB stores,
// so a value reads back from the database exactly as it was written.
func (System) Now() time.Time {
	return time.Now().UTC().Truncate(time.Millisecond)
}
```

`api/pkg/ids/ids.go`:

```go
// Package ids implements usecases.IDGenerator.
package ids

import "github.com/google/uuid"

// UUIDv7 IDs start with a millisecond timestamp and increase within one process,
// so sorting them as strings sorts them by creation time. MessageRepository pages on this.
type UUIDv7 struct{}

func (UUIDv7) NewID() string {
	return uuid.Must(uuid.NewV7()).String()
}
```

Run: `cd api && go get github.com/google/uuid@v1.6.0 && go test ./pkg/...`
Expected: PASS.

- [ ] **Step 3: Write the failing repository tests**

`api/repositories/testdb_test.go`:

```go
package repositories_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"supportchat/repositories"
)

var t0 = time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC)

// testDB connects to the Mongo started by `make up` and gives each test its own database.
func testDB(t *testing.T) *mongo.Database {
	t.Helper()
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := repositories.Connect(ctx, uri)
	require.NoError(t, err, "cannot reach MongoDB at %s; run `make up`", uri)

	db := client.Database(fmt.Sprintf("test_%d", time.Now().UnixNano()))
	require.NoError(t, repositories.EnsureIndexes(ctx, db))
	t.Cleanup(func() {
		ctx := context.Background()
		_ = db.Drop(ctx)
		_ = client.Disconnect(ctx)
	})
	return db
}
```

`api/repositories/case_repository_test.go`:

```go
package repositories_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"supportchat/business/models"
	"supportchat/business/usecases"
	"supportchat/repositories"
)

func sampleCase(id, customerID string, status models.CaseStatus, updatedAt time.Time) models.Case {
	return models.Case{
		ID:         id,
		Subject:    "help",
		CustomerID: customerID,
		Participants: []models.Participant{
			{UserID: customerID, Name: "Ann", Role: models.RoleCustomer, JoinedAt: t0},
		},
		Status:    status,
		CreatedAt: t0,
		UpdatedAt: updatedAt,
		Version:   1,
	}
}

func TestCaseRepositoryInsertAndGet(t *testing.T) {
	repo := repositories.NewMongoCaseRepository(testDB(t))
	ctx := context.Background()
	closedAt := t0.Add(time.Hour)
	c := sampleCase("c1", "customer:ann", models.StatusClosed, t0)
	c.ClosedAt = &closedAt

	require.NoError(t, repo.Insert(ctx, c))
	got, err := repo.Get(ctx, "c1")

	require.NoError(t, err)
	assert.Equal(t, c, got)
}

func TestCaseRepositoryGetUnknown(t *testing.T) {
	repo := repositories.NewMongoCaseRepository(testDB(t))

	_, err := repo.Get(context.Background(), "missing")

	assert.ErrorIs(t, err, models.ErrNotFound)
}

func TestCaseRepositoryUpdateChecksVersion(t *testing.T) {
	repo := repositories.NewMongoCaseRepository(testDB(t))
	ctx := context.Background()
	c := sampleCase("c1", "customer:ann", models.StatusWaiting, t0)
	require.NoError(t, repo.Insert(ctx, c))

	c.Status = models.StatusOpen
	require.NoError(t, repo.Update(ctx, c))

	stored, err := repo.Get(ctx, "c1")
	require.NoError(t, err)
	assert.Equal(t, models.StatusOpen, stored.Status)
	assert.Equal(t, 2, stored.Version)

	stale := c // still version 1
	stale.Subject = "overwritten"
	assert.ErrorIs(t, repo.Update(ctx, stale), models.ErrConflict)

	assert.ErrorIs(t, repo.Update(ctx, sampleCase("missing", "x", models.StatusOpen, t0)), models.ErrNotFound)
}

func TestCaseRepositoryListFiltersAndSorts(t *testing.T) {
	repo := repositories.NewMongoCaseRepository(testDB(t))
	ctx := context.Background()
	require.NoError(t, repo.Insert(ctx, sampleCase("old", "customer:ann", models.StatusWaiting, t0)))
	require.NoError(t, repo.Insert(ctx, sampleCase("new", "customer:ann", models.StatusOpen, t0.Add(2*time.Minute))))
	require.NoError(t, repo.Insert(ctx, sampleCase("cats", "customer:cat", models.StatusWaiting, t0.Add(time.Minute))))

	all, err := repo.List(ctx, usecases.CaseFilter{})
	require.NoError(t, err)
	assert.Equal(t, []string{"new", "cats", "old"}, caseIDs(all))

	anns, err := repo.List(ctx, usecases.CaseFilter{CustomerID: "customer:ann"})
	require.NoError(t, err)
	assert.Equal(t, []string{"new", "old"}, caseIDs(anns))

	waiting, err := repo.List(ctx, usecases.CaseFilter{Status: models.StatusWaiting})
	require.NoError(t, err)
	assert.Equal(t, []string{"cats", "old"}, caseIDs(waiting))

	none, err := repo.List(ctx, usecases.CaseFilter{CustomerID: "customer:nobody"})
	require.NoError(t, err)
	assert.NotNil(t, none)
	assert.Empty(t, none)
}

func caseIDs(cs []models.Case) []string {
	var out []string
	for _, c := range cs {
		out = append(out, c.ID)
	}
	return out
}
```

`api/repositories/message_repository_test.go`:

```go
package repositories_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"supportchat/business/models"
	"supportchat/business/usecases"
	"supportchat/pkg/ids"
	"supportchat/repositories"
)

func TestMessageRepositoryInsertAndList(t *testing.T) {
	repo := repositories.NewMongoMessageRepository(testDB(t))
	ctx := context.Background()
	m := models.Message{
		ID: "m1", CaseID: "c1", SenderID: "customer:ann", SenderName: "Ann",
		SenderRole: models.RoleCustomer, Kind: models.KindText, Body: "hello", CreatedAt: t0,
	}
	require.NoError(t, repo.Insert(ctx, m))
	require.NoError(t, repo.Insert(ctx, models.Message{ID: "m2", CaseID: "other-case", Body: "x", CreatedAt: t0}))

	got, err := repo.List(ctx, usecases.MessageQuery{CaseID: "c1", Limit: 10})

	require.NoError(t, err)
	assert.Equal(t, []models.Message{m}, got)
}

// Review Focus: several messages in the same millisecond must page without gaps or repeats.
func TestMessageRepositoryPagesMessagesFromTheSameMillisecond(t *testing.T) {
	repo := repositories.NewMongoMessageRepository(testDB(t))
	ctx := context.Background()
	gen := ids.UUIDv7{}
	for i := range 5 {
		require.NoError(t, repo.Insert(ctx, models.Message{
			ID: gen.NewID(), CaseID: "c1", Body: fmt.Sprint(i), CreatedAt: t0, // same timestamp for all
		}))
	}

	var seen []string
	before := ""
	for {
		page, err := repo.List(ctx, usecases.MessageQuery{CaseID: "c1", BeforeID: before, Limit: 2})
		require.NoError(t, err)
		if len(page) == 0 {
			break
		}
		for _, m := range page {
			seen = append(seen, m.Body)
		}
		before = page[len(page)-1].ID
	}

	assert.Equal(t, []string{"4", "3", "2", "1", "0"}, seen)
}

func TestMessageRepositoryEmptyListIsNotNil(t *testing.T) {
	repo := repositories.NewMongoMessageRepository(testDB(t))

	got, err := repo.List(context.Background(), usecases.MessageQuery{CaseID: "none", Limit: 10})

	require.NoError(t, err)
	assert.NotNil(t, got)
}
```

Run: `cd api && go test ./repositories/`
Expected: FAIL with `undefined: repositories.Connect`.

- [ ] **Step 4: Implement the repositories**

`api/repositories/mongo.go`:

```go
// Package repositories implements the driven repository ports with MongoDB.
package repositories

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	casesCollection    = "cases"
	messagesCollection = "messages"
)

// Connect opens a client and pings the server, so a wrong URI fails at startup.
func Connect(ctx context.Context, uri string) (*mongo.Client, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(ctx)
		return nil, err
	}
	return client, nil
}

// EnsureIndexes creates the indexes the queries in this package rely on. It is safe to call on every start.
func EnsureIndexes(ctx context.Context, db *mongo.Database) error {
	_, err := db.Collection(casesCollection).Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "updatedAt", Value: -1}}},
		{Keys: bson.D{{Key: "status", Value: 1}, {Key: "updatedAt", Value: -1}}},
		{Keys: bson.D{{Key: "customerId", Value: 1}, {Key: "updatedAt", Value: -1}}},
	})
	if err != nil {
		return err
	}
	_, err = db.Collection(messagesCollection).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "caseId", Value: 1}, {Key: "_id", Value: -1}},
	})
	return err
}
```

`api/repositories/case_repository.go`:

```go
package repositories

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"supportchat/business/models"
	"supportchat/business/usecases"
)

type participantDoc struct {
	UserID   string    `bson:"userId"`
	Name     string    `bson:"name"`
	Role     string    `bson:"role"`
	JoinedAt time.Time `bson:"joinedAt"`
}

type caseDoc struct {
	ID           string           `bson:"_id"`
	Subject      string           `bson:"subject"`
	CustomerID   string           `bson:"customerId"`
	Participants []participantDoc `bson:"participants"`
	Status       string           `bson:"status"`
	CreatedAt    time.Time        `bson:"createdAt"`
	UpdatedAt    time.Time        `bson:"updatedAt"`
	ClosedAt     *time.Time       `bson:"closedAt,omitempty"`
	Version      int              `bson:"version"`
}

type MongoCaseRepository struct {
	col *mongo.Collection
}

var _ usecases.CaseRepository = (*MongoCaseRepository)(nil)

func NewMongoCaseRepository(db *mongo.Database) *MongoCaseRepository {
	return &MongoCaseRepository{col: db.Collection(casesCollection)}
}

func (r *MongoCaseRepository) Insert(ctx context.Context, c models.Case) error {
	_, err := r.col.InsertOne(ctx, toCaseDoc(c))
	return err
}

func (r *MongoCaseRepository) Get(ctx context.Context, id string) (models.Case, error) {
	var d caseDoc
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&d)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return models.Case{}, models.ErrNotFound
	}
	if err != nil {
		return models.Case{}, err
	}
	return d.toModel(), nil
}

func (r *MongoCaseRepository) Update(ctx context.Context, c models.Case) error {
	d := toCaseDoc(c)
	d.Version = c.Version + 1
	res, err := r.col.ReplaceOne(ctx, bson.M{"_id": c.ID, "version": c.Version}, d)
	if err != nil {
		return err
	}
	if res.MatchedCount == 1 {
		return nil
	}
	n, err := r.col.CountDocuments(ctx, bson.M{"_id": c.ID})
	if err != nil {
		return err
	}
	if n == 0 {
		return models.ErrNotFound
	}
	return models.ErrConflict
}

func (r *MongoCaseRepository) List(ctx context.Context, f usecases.CaseFilter) ([]models.Case, error) {
	filter := bson.M{}
	if f.CustomerID != "" {
		filter["customerId"] = f.CustomerID
	}
	if f.Status != "" {
		filter["status"] = string(f.Status)
	}
	cur, err := r.col.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "updatedAt", Value: -1}}))
	if err != nil {
		return nil, err
	}
	var docs []caseDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]models.Case, 0, len(docs))
	for _, d := range docs {
		out = append(out, d.toModel())
	}
	return out, nil
}

func toCaseDoc(c models.Case) caseDoc {
	ps := make([]participantDoc, 0, len(c.Participants))
	for _, p := range c.Participants {
		ps = append(ps, participantDoc{UserID: p.UserID, Name: p.Name, Role: string(p.Role), JoinedAt: p.JoinedAt})
	}
	return caseDoc{
		ID:           c.ID,
		Subject:      c.Subject,
		CustomerID:   c.CustomerID,
		Participants: ps,
		Status:       string(c.Status),
		CreatedAt:    c.CreatedAt,
		UpdatedAt:    c.UpdatedAt,
		ClosedAt:     c.ClosedAt,
		Version:      c.Version,
	}
}

func (d caseDoc) toModel() models.Case {
	ps := make([]models.Participant, 0, len(d.Participants))
	for _, p := range d.Participants {
		ps = append(ps, models.Participant{UserID: p.UserID, Name: p.Name, Role: models.Role(p.Role), JoinedAt: p.JoinedAt.UTC()})
	}
	var closedAt *time.Time
	if d.ClosedAt != nil {
		t := d.ClosedAt.UTC()
		closedAt = &t
	}
	return models.Case{
		ID:           d.ID,
		Subject:      d.Subject,
		CustomerID:   d.CustomerID,
		Participants: ps,
		Status:       models.CaseStatus(d.Status),
		CreatedAt:    d.CreatedAt.UTC(),
		UpdatedAt:    d.UpdatedAt.UTC(),
		ClosedAt:     closedAt,
		Version:      d.Version,
	}
}
```

`api/repositories/message_repository.go`:

```go
package repositories

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"supportchat/business/models"
	"supportchat/business/usecases"
)

type messageDoc struct {
	ID         string    `bson:"_id"`
	CaseID     string    `bson:"caseId"`
	SenderID   string    `bson:"senderId"`
	SenderName string    `bson:"senderName"`
	SenderRole string    `bson:"senderRole"`
	Kind       string    `bson:"kind"`
	Body       string    `bson:"body"`
	CreatedAt  time.Time `bson:"createdAt"`
}

type MongoMessageRepository struct {
	col *mongo.Collection
}

var _ usecases.MessageRepository = (*MongoMessageRepository)(nil)

func NewMongoMessageRepository(db *mongo.Database) *MongoMessageRepository {
	return &MongoMessageRepository{col: db.Collection(messagesCollection)}
}

func (r *MongoMessageRepository) Insert(ctx context.Context, m models.Message) error {
	_, err := r.col.InsertOne(ctx, messageDoc{
		ID:         m.ID,
		CaseID:     m.CaseID,
		SenderID:   m.SenderID,
		SenderName: m.SenderName,
		SenderRole: string(m.SenderRole),
		Kind:       string(m.Kind),
		Body:       m.Body,
		CreatedAt:  m.CreatedAt,
	})
	return err
}

// List pages on _id rather than createdAt: many messages can share one millisecond,
// but IDs are unique and sort in creation order (UUIDv7).
func (r *MongoMessageRepository) List(ctx context.Context, q usecases.MessageQuery) ([]models.Message, error) {
	filter := bson.M{"caseId": q.CaseID}
	if q.BeforeID != "" {
		filter["_id"] = bson.M{"$lt": q.BeforeID}
	}
	opts := options.Find().SetSort(bson.D{{Key: "_id", Value: -1}}).SetLimit(int64(q.Limit))
	cur, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	var docs []messageDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]models.Message, 0, len(docs))
	for _, d := range docs {
		out = append(out, models.Message{
			ID:         d.ID,
			CaseID:     d.CaseID,
			SenderID:   d.SenderID,
			SenderName: d.SenderName,
			SenderRole: models.Role(d.SenderRole),
			Kind:       models.MessageKind(d.Kind),
			Body:       d.Body,
			CreatedAt:  d.CreatedAt.UTC(),
		})
	}
	return out, nil
}
```

- [ ] **Step 5: Run all tests**

Run: `cd api && go get go.mongodb.org/mongo-driver/v2@v2.9.1 && go mod tidy && cd .. && make test`
Expected: PASS, including `TestCaseRepositoryUpdateChecksVersion` and `TestMessageRepositoryPagesMessagesFromTheSameMillisecond`. If you see `cannot reach MongoDB`, run `make up`.

- [ ] **Step 6: Commit**

```bash
git add api/
git commit -m "feat: add MongoDB repositories, UTC clock and UUIDv7 ids

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 10: Realtime Hub

**Files:**
- Create: `api/realtime/hub.go`
- Test: `api/realtime/hub_test.go`

**Interfaces:**
- Consumes: `models.Event`, `usecases.MessageBroadcaster`
- Produces:
  - `realtime.NewHub(bufferSize int) *Hub` and `realtime.DefaultBufferSize = 16`
  - `(*Hub).PublishCase(caseID string, e models.Event)` and `(*Hub).PublishAgents(e models.Event)`, which implement `usecases.MessageBroadcaster`
  - `(*Hub).SubscribeCase(caseID string) (<-chan models.Event, func())` and `(*Hub).SubscribeAgents() (<-chan models.Event, func())`. Task 12's WebSocket endpoints use these. The channel is closed when cancel runs (cancel is idempotent) or when the Hub drops a slow subscriber.

- [ ] **Step 1: Write the failing tests**

`api/realtime/hub_test.go`:

```go
package realtime_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"supportchat/business/models"
	"supportchat/realtime"
)

func event(caseID, body string) models.Event {
	return models.Event{Type: models.EventMessageCreated, CaseID: caseID, Data: body}
}

func receive(t *testing.T, ch <-chan models.Event) models.Event {
	t.Helper()
	select {
	case e, ok := <-ch:
		require.True(t, ok, "channel was closed")
		return e
	case <-time.After(time.Second):
		t.Fatal("no event received")
		return models.Event{}
	}
}

func assertNothingWaiting(t *testing.T, ch <-chan models.Event) {
	t.Helper()
	select {
	case e := <-ch:
		t.Fatalf("unexpected event %+v", e)
	default:
	}
}

func TestPublishCaseReachesEverySubscriberOfThatCaseOnly(t *testing.T) {
	hub := realtime.NewHub(realtime.DefaultBufferSize)
	ann, cancelAnn := hub.SubscribeCase("c1")
	defer cancelAnn()
	bob, cancelBob := hub.SubscribeCase("c1")
	defer cancelBob()
	other, cancelOther := hub.SubscribeCase("c2")
	defer cancelOther()

	hub.PublishCase("c1", event("c1", "hi"))

	assert.Equal(t, event("c1", "hi"), receive(t, ann))
	assert.Equal(t, event("c1", "hi"), receive(t, bob))
	assertNothingWaiting(t, other)
}

func TestPublishAgentsReachesAgentSubscribersOnly(t *testing.T) {
	hub := realtime.NewHub(realtime.DefaultBufferSize)
	agents, cancelAgents := hub.SubscribeAgents()
	defer cancelAgents()
	caseSub, cancelCase := hub.SubscribeCase("c1")
	defer cancelCase()

	created := models.Event{Type: models.EventCaseCreated, CaseID: "c1"}
	hub.PublishAgents(created)

	assert.Equal(t, created, receive(t, agents))
	assertNothingWaiting(t, caseSub)
}

func TestCancelClosesChannelAndCanBeCalledTwice(t *testing.T) {
	hub := realtime.NewHub(realtime.DefaultBufferSize)
	ch, cancel := hub.SubscribeCase("c1")

	cancel()
	cancel()
	hub.PublishCase("c1", event("c1", "after cancel"))

	_, ok := <-ch
	assert.False(t, ok)
}

// Review Focus: a client that stops reading must not block the others in the case.
func TestSlowSubscriberIsDroppedWithoutBlockingOthers(t *testing.T) {
	hub := realtime.NewHub(2)
	slow, cancelSlow := hub.SubscribeCase("c1")
	defer cancelSlow()
	fast, cancelFast := hub.SubscribeCase("c1")
	defer cancelFast()

	for i := range 3 { // one more than the buffer
		hub.PublishCase("c1", event("c1", string(rune('a'+i))))
		receive(t, fast)
	}

	// slow still holds the 2 buffered events, then its channel is closed.
	assert.Equal(t, event("c1", "a"), receive(t, slow))
	assert.Equal(t, event("c1", "b"), receive(t, slow))
	_, ok := <-slow
	assert.False(t, ok, "slow subscriber should be dropped")

	hub.PublishCase("c1", event("c1", "d"))
	assert.Equal(t, event("c1", "d"), receive(t, fast), "fast subscriber keeps receiving")
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd api && go test ./realtime/`
Expected: FAIL with `undefined: realtime.NewHub`.

- [ ] **Step 3: Implement**

`api/realtime/hub.go`:

```go
// Package realtime fans events out to WebSocket connections in this process (ADR 0004).
package realtime

import (
	"sync"

	"supportchat/business/models"
	"supportchat/business/usecases"
)

// DefaultBufferSize is how many events a connection may fall behind before the Hub drops it.
const DefaultBufferSize = 16

const agentsTopic = "agents"

type subscriber struct {
	ch chan models.Event
}

// Hub implements usecases.MessageBroadcaster. Publish never blocks: a subscriber whose
// buffer is full is removed and its channel closed, so one slow client cannot hold up a case.
type Hub struct {
	mu         sync.Mutex
	bufferSize int
	topics     map[string]map[*subscriber]struct{}
}

var _ usecases.MessageBroadcaster = (*Hub)(nil)

func NewHub(bufferSize int) *Hub {
	return &Hub{bufferSize: bufferSize, topics: map[string]map[*subscriber]struct{}{}}
}

func (h *Hub) PublishCase(caseID string, e models.Event) { h.publish(caseTopic(caseID), e) }

func (h *Hub) PublishAgents(e models.Event) { h.publish(agentsTopic, e) }

// SubscribeCase returns a channel of events for one case and a cancel function.
// The channel is closed when cancel is called or when the Hub drops a slow subscriber.
func (h *Hub) SubscribeCase(caseID string) (<-chan models.Event, func()) {
	return h.subscribe(caseTopic(caseID))
}

// SubscribeAgents returns a channel of case-list events for agents. See SubscribeCase.
func (h *Hub) SubscribeAgents() (<-chan models.Event, func()) {
	return h.subscribe(agentsTopic)
}

func caseTopic(caseID string) string { return "case:" + caseID }

func (h *Hub) subscribe(topic string) (<-chan models.Event, func()) {
	s := &subscriber{ch: make(chan models.Event, h.bufferSize)}
	h.mu.Lock()
	if h.topics[topic] == nil {
		h.topics[topic] = map[*subscriber]struct{}{}
	}
	h.topics[topic][s] = struct{}{}
	h.mu.Unlock()

	cancel := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.remove(topic, s)
	}
	return s.ch, cancel
}

// remove must be called with h.mu held. It is safe to call more than once.
func (h *Hub) remove(topic string, s *subscriber) {
	subs := h.topics[topic]
	if _, ok := subs[s]; !ok {
		return
	}
	delete(subs, s)
	close(s.ch)
	if len(subs) == 0 {
		delete(h.topics, topic)
	}
}

func (h *Hub) publish(topic string, e models.Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.topics[topic] {
		select {
		case s.ch <- e:
		default:
			h.remove(topic, s)
		}
	}
}
```

- [ ] **Step 4: Run all tests with the race detector**

Run: `make test` (it uses `-race`)
Expected: PASS with no `DATA RACE` output.

- [ ] **Step 5: Commit**

```bash
git add api/realtime
git commit -m "feat: add in-memory realtime hub that drops slow subscribers

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 11: REST handlers and the OpenAPI contract

**Files:**
- Create: `docs/openapi.yaml`
- Create: `api/handlers/errors.go`, `auth.go`, `cases.go`, `router.go`
- Test: `api/handlers/stubs_test.go`, `auth_test.go`, `cases_test.go`

**Interfaces:**
- Consumes: `usecases.AuthService` (Task 8), `usecases.CaseService` (Tasks 3–7)
- Produces:
  - `handlers.Config{Auth usecases.AuthService; Cases usecases.CaseService; AllowedOrigins []string; Logger *slog.Logger}` (Task 12 adds `Events`)
  - `handlers.NewRouter(Config) *echo.Echo` and `handlers.ErrorHandler(*slog.Logger) echo.HTTPErrorHandler`
  - Test helpers `stubAuth` (tokens `ann-token`, `bob-token`), `stubCases`, `newRouter(auth, cases)` and `call(t, e, method, path, token, body)`

- [ ] **Step 1: Write the contract**

`docs/openapi.yaml`:

```yaml
openapi: 3.0.3
info:
  title: Support Chat API
  version: 1.0.0
  description: |
    REST contract for commands and queries (ADR 0003). WebSocket events are in events.md.
    Every route except POST /v1/login needs `Authorization: Bearer <token>`.
    Every error response is `{"error": "<message>"}`.
servers:
  - url: http://localhost:8080
security:
  - bearerAuth: []

paths:
  /v1/login:
    post:
      summary: Log in with a name and role (no password, ADR 0005)
      security: []
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [name, role]
              properties:
                name: { type: string, minLength: 1, maxLength: 50, example: Ann }
                role: { $ref: '#/components/schemas/Role' }
      responses:
        '200':
          description: Logged in
          content:
            application/json:
              schema:
                type: object
                required: [token, user]
                properties:
                  token: { type: string }
                  user: { $ref: '#/components/schemas/User' }
        '400': { $ref: '#/components/responses/InvalidInput' }

  /v1/cases:
    post:
      summary: Open a case with a first question (customers only)
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [question]
              properties:
                question: { type: string, description: '1–2000 characters after trimming' }
      responses:
        '201':
          description: The new case (status waiting) and its first message
          content:
            application/json:
              schema:
                type: object
                required: [case, message]
                properties:
                  case: { $ref: '#/components/schemas/Case' }
                  message: { $ref: '#/components/schemas/Message' }
        '400': { $ref: '#/components/responses/InvalidInput' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '403': { $ref: '#/components/responses/Forbidden' }
    get:
      summary: List cases, newest updatedAt first
      description: A customer gets only their own cases. An agent gets every case.
      parameters:
        - name: status
          in: query
          required: false
          schema: { $ref: '#/components/schemas/CaseStatus' }
      responses:
        '200':
          description: Cases (an empty array when there are none)
          content:
            application/json:
              schema:
                type: array
                items: { $ref: '#/components/schemas/Case' }
        '400': { $ref: '#/components/responses/InvalidInput' }
        '401': { $ref: '#/components/responses/Unauthorized' }

  /v1/cases/{id}:
    parameters:
      - $ref: '#/components/parameters/CaseId'
    get:
      summary: Get a case and its participants (its customer, or any agent)
      responses:
        '200':
          description: The case
          content:
            application/json:
              schema: { $ref: '#/components/schemas/Case' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '403': { $ref: '#/components/responses/Forbidden' }
        '404': { $ref: '#/components/responses/NotFound' }

  /v1/cases/{id}/join:
    parameters:
      - $ref: '#/components/parameters/CaseId'
    post:
      summary: Join a case (agents only, idempotent)
      description: The first join moves the case from waiting to open. Joining again returns the case unchanged.
      responses:
        '200':
          description: The case after joining
          content:
            application/json:
              schema: { $ref: '#/components/schemas/Case' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '403': { $ref: '#/components/responses/Forbidden' }
        '404': { $ref: '#/components/responses/NotFound' }
        '409': { $ref: '#/components/responses/Conflict' }

  /v1/cases/{id}/messages:
    parameters:
      - $ref: '#/components/parameters/CaseId'
    get:
      summary: List messages, newest first (its customer, or any agent)
      parameters:
        - name: before
          in: query
          required: false
          description: Return only messages older than the message with this id. Use the last id of the previous page.
          schema: { type: string }
        - name: limit
          in: query
          required: false
          description: Default 50. Values above 100 are treated as 100.
          schema: { type: integer, minimum: 0 }
      responses:
        '200':
          description: Messages (an empty array when there are none)
          content:
            application/json:
              schema:
                type: array
                items: { $ref: '#/components/schemas/Message' }
        '400': { $ref: '#/components/responses/InvalidInput' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '403': { $ref: '#/components/responses/Forbidden' }
        '404': { $ref: '#/components/responses/NotFound' }
    post:
      summary: Send a message (participants only)
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [body]
              properties:
                body: { type: string, description: '1–2000 characters after trimming' }
      responses:
        '201':
          description: The saved message. It is also sent as a message.created event.
          content:
            application/json:
              schema: { $ref: '#/components/schemas/Message' }
        '400': { $ref: '#/components/responses/InvalidInput' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '403': { $ref: '#/components/responses/Forbidden' }
        '404': { $ref: '#/components/responses/NotFound' }
        '409': { $ref: '#/components/responses/Conflict' }

  /v1/cases/{id}/close:
    parameters:
      - $ref: '#/components/parameters/CaseId'
    post:
      summary: Close a case (agents who have joined)
      responses:
        '200':
          description: The closed case
          content:
            application/json:
              schema: { $ref: '#/components/schemas/Case' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '403': { $ref: '#/components/responses/Forbidden' }
        '404': { $ref: '#/components/responses/NotFound' }
        '409': { $ref: '#/components/responses/Conflict' }

components:
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      bearerFormat: JWT

  parameters:
    CaseId:
      name: id
      in: path
      required: true
      schema: { type: string }

  schemas:
    Role:
      type: string
      enum: [customer, agent]
    CaseStatus:
      type: string
      enum: [waiting, open, closed]
    User:
      type: object
      required: [id, name, role]
      properties:
        id: { type: string, example: 'customer:ann' }
        name: { type: string, example: Ann }
        role: { $ref: '#/components/schemas/Role' }
    Participant:
      type: object
      required: [userId, name, role, joinedAt]
      properties:
        userId: { type: string }
        name: { type: string }
        role: { $ref: '#/components/schemas/Role' }
        joinedAt: { type: string, format: date-time }
    Case:
      type: object
      required: [id, subject, customerId, participants, status, createdAt, updatedAt]
      properties:
        id: { type: string }
        subject: { type: string, description: 'The first question on one line, at most 80 characters' }
        customerId: { type: string }
        participants:
          type: array
          items: { $ref: '#/components/schemas/Participant' }
        status: { $ref: '#/components/schemas/CaseStatus' }
        createdAt: { type: string, format: date-time }
        updatedAt: { type: string, format: date-time }
        closedAt: { type: string, format: date-time, description: 'Present only when status is closed' }
    Message:
      type: object
      required: [id, caseId, senderId, senderName, senderRole, kind, body, createdAt]
      properties:
        id: { type: string, description: 'Sorts in creation order (UUIDv7)' }
        caseId: { type: string }
        senderId: { type: string }
        senderName: { type: string }
        senderRole: { $ref: '#/components/schemas/Role' }
        kind: { type: string, enum: [text, system] }
        body: { type: string }
        createdAt: { type: string, format: date-time }
    Error:
      type: object
      required: [error]
      properties:
        error: { type: string }

  responses:
    InvalidInput:
      description: The request body or a parameter is invalid
      content:
        application/json:
          schema: { $ref: '#/components/schemas/Error' }
    Unauthorized:
      description: Missing, invalid or expired token
      content:
        application/json:
          schema: { $ref: '#/components/schemas/Error' }
    Forbidden:
      description: The user may not do this
      content:
        application/json:
          schema: { $ref: '#/components/schemas/Error' }
    NotFound:
      description: The case does not exist
      content:
        application/json:
          schema: { $ref: '#/components/schemas/Error' }
    Conflict:
      description: The case is closed, or it was changed by another request at the same moment (retry)
      content:
        application/json:
          schema: { $ref: '#/components/schemas/Error' }
```

Run: `npx --yes @redocly/cli@latest lint docs/openapi.yaml`
Expected: `Your API description is valid.` Warnings about missing `operationId` and `license` are fine.

- [ ] **Step 2: Write the failing handler tests**

`api/handlers/stubs_test.go`:

```go
package handlers_test

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"supportchat/business/models"
	"supportchat/handlers"
)

var (
	ann = models.User{ID: "customer:ann", Name: "Ann", Role: models.RoleCustomer}
	bob = models.User{ID: "agent:bob", Name: "Bob", Role: models.RoleAgent}
)

// stubAuth accepts two fixed tokens.
type stubAuth struct {
	login func(name string, role models.Role) (string, models.User, error)
}

func (s stubAuth) Login(_ context.Context, name string, role models.Role) (string, models.User, error) {
	return s.login(name, role)
}

func (stubAuth) Authenticate(_ context.Context, token string) (models.User, error) {
	switch token {
	case "ann-token":
		return ann, nil
	case "bob-token":
		return bob, nil
	}
	return models.User{}, models.ErrUnauthorized
}

// stubCases lets each test set only the use case it expects to be called.
type stubCases struct {
	openCase     func(actor models.User, question string) (models.Case, models.Message, error)
	joinCase     func(actor models.User, caseID string) (models.Case, error)
	sendMessage  func(actor models.User, caseID, body string) (models.Message, error)
	closeCase    func(actor models.User, caseID string) (models.Case, error)
	listCases    func(actor models.User, status models.CaseStatus) ([]models.Case, error)
	getCase      func(actor models.User, caseID string) (models.Case, error)
	listMessages func(actor models.User, caseID, beforeID string, limit int) ([]models.Message, error)
}

func (s *stubCases) OpenCase(_ context.Context, a models.User, q string) (models.Case, models.Message, error) {
	return s.openCase(a, q)
}
func (s *stubCases) JoinCase(_ context.Context, a models.User, id string) (models.Case, error) {
	return s.joinCase(a, id)
}
func (s *stubCases) SendMessage(_ context.Context, a models.User, id, body string) (models.Message, error) {
	return s.sendMessage(a, id, body)
}
func (s *stubCases) CloseCase(_ context.Context, a models.User, id string) (models.Case, error) {
	return s.closeCase(a, id)
}
func (s *stubCases) ListCases(_ context.Context, a models.User, st models.CaseStatus) ([]models.Case, error) {
	return s.listCases(a, st)
}
func (s *stubCases) GetCase(_ context.Context, a models.User, id string) (models.Case, error) {
	return s.getCase(a, id)
}
func (s *stubCases) ListMessages(_ context.Context, a models.User, id, before string, limit int) ([]models.Message, error) {
	return s.listMessages(a, id, before, limit)
}

func newRouter(auth stubAuth, cases *stubCases) *echo.Echo {
	return handlers.NewRouter(handlers.Config{
		Auth:           auth,
		Cases:          cases,
		AllowedOrigins: []string{"http://localhost:3000"},
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}

// call sends one request. token "" means no Authorization header.
func call(t *testing.T, e *echo.Echo, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}
```

`api/handlers/auth_test.go`:

```go
package handlers_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"supportchat/business/models"
)

func TestLogin(t *testing.T) {
	auth := stubAuth{login: func(name string, role models.Role) (string, models.User, error) {
		assert.Equal(t, "Ann", name)
		assert.Equal(t, models.RoleCustomer, role)
		return "ann-token", ann, nil
	}}
	e := newRouter(auth, &stubCases{})

	rec := call(t, e, http.MethodPost, "/v1/login", "", `{"name":"Ann","role":"customer"}`)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"token":"ann-token","user":{"id":"customer:ann","name":"Ann","role":"customer"}}`, rec.Body.String())
}

func TestLoginRejectsBadInput(t *testing.T) {
	auth := stubAuth{login: func(string, models.Role) (string, models.User, error) {
		return "", models.User{}, models.ErrInvalidInput
	}}
	e := newRouter(auth, &stubCases{})

	rec := call(t, e, http.MethodPost, "/v1/login", "", `{"name":"","role":"customer"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"invalid input"}`, rec.Body.String())

	rec = call(t, e, http.MethodPost, "/v1/login", "", `{not json`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestProtectedRoutesNeedAValidToken(t *testing.T) {
	e := newRouter(stubAuth{}, &stubCases{})

	assert.Equal(t, http.StatusUnauthorized, call(t, e, http.MethodGet, "/v1/cases", "", "").Code)
	assert.Equal(t, http.StatusUnauthorized, call(t, e, http.MethodGet, "/v1/cases", "forged", "").Code)
	assert.Equal(t, http.StatusUnauthorized, call(t, e, http.MethodPost, "/v1/cases/c1/messages", "", `{"body":"x"}`).Code)
}
```

`api/handlers/cases_test.go`:

```go
package handlers_test

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"supportchat/business/models"
)

var t0 = time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC)

func TestOpenCaseReturns201WithCaseAndMessage(t *testing.T) {
	cases := &stubCases{openCase: func(actor models.User, question string) (models.Case, models.Message, error) {
		assert.Equal(t, ann, actor)
		assert.Equal(t, "How do I reset my password?", question)
		return models.Case{ID: "c1", Status: models.StatusWaiting, CreatedAt: t0, UpdatedAt: t0},
			models.Message{ID: "m1", CaseID: "c1", Body: question, CreatedAt: t0}, nil
	}}
	e := newRouter(stubAuth{}, cases)

	rec := call(t, e, http.MethodPost, "/v1/cases", "ann-token", `{"question":"How do I reset my password?"}`)

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.JSONEq(t, `{
		"case": {"id":"c1","subject":"","customerId":"","participants":null,"status":"waiting",
		         "createdAt":"2026-09-26T09:00:00Z","updatedAt":"2026-09-26T09:00:00Z"},
		"message": {"id":"m1","caseId":"c1","senderId":"","senderName":"","senderRole":"","kind":"",
		            "body":"How do I reset my password?","createdAt":"2026-09-26T09:00:00Z"}
	}`, rec.Body.String())
}

func TestDomainErrorsMapToHTTPStatus(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		body   string
	}{
		{models.ErrInvalidInput, http.StatusBadRequest, `{"error":"invalid input"}`},
		{models.ErrForbidden, http.StatusForbidden, `{"error":"forbidden"}`},
		{models.ErrNotFound, http.StatusNotFound, `{"error":"not found"}`},
		{models.ErrCaseClosed, http.StatusConflict, `{"error":"case is closed"}`},
		{models.ErrConflict, http.StatusConflict, `{"error":"case was changed by another request, try again"}`},
		{errors.New("mongo: connection refused at 10.0.0.5"), http.StatusInternalServerError, `{"error":"internal error"}`},
	} {
		cases := &stubCases{sendMessage: func(models.User, string, string) (models.Message, error) {
			return models.Message{}, tc.err
		}}
		e := newRouter(stubAuth{}, cases)

		rec := call(t, e, http.MethodPost, "/v1/cases/c1/messages", "ann-token", `{"body":"hi"}`)

		assert.Equal(t, tc.status, rec.Code, tc.err.Error())
		assert.JSONEq(t, tc.body, rec.Body.String(), "internal details must not leak")
	}
}

func TestListCasesPassesStatusAndNeverReturnsNull(t *testing.T) {
	cases := &stubCases{listCases: func(actor models.User, status models.CaseStatus) ([]models.Case, error) {
		assert.Equal(t, bob, actor)
		assert.Equal(t, models.StatusWaiting, status)
		return nil, nil
	}}
	e := newRouter(stubAuth{}, cases)

	rec := call(t, e, http.MethodGet, "/v1/cases?status=waiting", "bob-token", "")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `[]`, rec.Body.String())
}

func TestCaseRoutesPassTheCaseID(t *testing.T) {
	open := models.Case{ID: "c1", Status: models.StatusOpen}
	cases := &stubCases{
		getCase:   func(_ models.User, id string) (models.Case, error) { return models.Case{ID: id}, nil },
		joinCase:  func(_ models.User, id string) (models.Case, error) { return open, nil },
		closeCase: func(_ models.User, id string) (models.Case, error) { return models.Case{ID: id, Status: models.StatusClosed}, nil },
		sendMessage: func(_ models.User, id, body string) (models.Message, error) {
			return models.Message{ID: "m9", CaseID: id, Body: body}, nil
		},
	}
	e := newRouter(stubAuth{}, cases)

	rec := call(t, e, http.MethodGet, "/v1/cases/c1", "bob-token", "")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"id":"c1"`)

	rec = call(t, e, http.MethodPost, "/v1/cases/c1/join", "bob-token", "")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"open"`)

	rec = call(t, e, http.MethodPost, "/v1/cases/c1/messages", "bob-token", `{"body":"hello"}`)
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Contains(t, rec.Body.String(), `"caseId":"c1"`)

	rec = call(t, e, http.MethodPost, "/v1/cases/c1/close", "bob-token", "")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"closed"`)
}

func TestListMessagesQueryParameters(t *testing.T) {
	cases := &stubCases{listMessages: func(_ models.User, id, before string, limit int) ([]models.Message, error) {
		assert.Equal(t, "c1", id)
		assert.Equal(t, "m5", before)
		assert.Equal(t, 20, limit)
		return nil, nil
	}}
	e := newRouter(stubAuth{}, cases)

	rec := call(t, e, http.MethodGet, "/v1/cases/c1/messages?before=m5&limit=20", "ann-token", "")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `[]`, rec.Body.String())

	rec = call(t, e, http.MethodGet, "/v1/cases/c1/messages?limit=abc", "ann-token", "")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `cd api && go get github.com/labstack/echo/v4@v4.15.4 && go test ./handlers/`
Expected: FAIL with `undefined: handlers.NewRouter`.

- [ ] **Step 4: Implement**

`api/handlers/errors.go`:

```go
package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"

	"supportchat/business/models"
)

type errorResponse struct {
	Error string `json:"error"`
}

// statusFor is the one place that maps domain errors to HTTP statuses (spec section 6).
func statusFor(err error) int {
	switch {
	case errors.Is(err, models.ErrInvalidInput):
		return http.StatusBadRequest
	case errors.Is(err, models.ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, models.ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, models.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, models.ErrCaseClosed), errors.Is(err, models.ErrConflict):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

// ErrorHandler writes every error returned by a handler as {"error": "..."}.
// Unknown errors become 500 with a generic message; the details go to the log only.
func ErrorHandler(logger *slog.Logger) echo.HTTPErrorHandler {
	return func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}
		status, msg := statusFor(err), err.Error()
		var he *echo.HTTPError
		if errors.As(err, &he) {
			status, msg = he.Code, fmt.Sprint(he.Message)
		}
		if status == http.StatusInternalServerError {
			logger.Error("request failed", "method", c.Request().Method, "path", c.Path(), "err", err)
			msg = "internal error"
		}
		_ = c.JSON(status, errorResponse{Error: msg})
	}
}
```

`api/handlers/auth.go`:

```go
package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"supportchat/business/models"
	"supportchat/business/usecases"
)

const userKey = "user"

// requireUser reads "Authorization: Bearer <token>" and stores the user for currentUser.
func requireUser(auth usecases.AuthService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			token, ok := strings.CutPrefix(c.Request().Header.Get("Authorization"), "Bearer ")
			if !ok {
				return models.ErrUnauthorized
			}
			u, err := auth.Authenticate(c.Request().Context(), token)
			if err != nil {
				return err
			}
			c.Set(userKey, u)
			return next(c)
		}
	}
}

func currentUser(c echo.Context) models.User {
	u, _ := c.Get(userKey).(models.User)
	return u
}

// decodeJSON reads the request body into v. Any malformed body is ErrInvalidInput.
func decodeJSON(c echo.Context, v any) error {
	if err := json.NewDecoder(c.Request().Body).Decode(v); err != nil {
		return models.ErrInvalidInput
	}
	return nil
}

type loginRequest struct {
	Name string      `json:"name"`
	Role models.Role `json:"role"`
}

type loginResponse struct {
	Token string      `json:"token"`
	User  models.User `json:"user"`
}

func login(auth usecases.AuthService) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req loginRequest
		if err := decodeJSON(c, &req); err != nil {
			return err
		}
		token, u, err := auth.Login(c.Request().Context(), req.Name, req.Role)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, loginResponse{Token: token, User: u})
	}
}
```

`api/handlers/cases.go`:

```go
package handlers

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"supportchat/business/models"
	"supportchat/business/usecases"
)

type caseHandlers struct {
	cases usecases.CaseService
}

type openCaseRequest struct {
	Question string `json:"question"`
}

type openCaseResponse struct {
	Case    models.Case    `json:"case"`
	Message models.Message `json:"message"`
}

type sendMessageRequest struct {
	Body string `json:"body"`
}

func (h caseHandlers) open(c echo.Context) error {
	var req openCaseRequest
	if err := decodeJSON(c, &req); err != nil {
		return err
	}
	cs, m, err := h.cases.OpenCase(c.Request().Context(), currentUser(c), req.Question)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, openCaseResponse{Case: cs, Message: m})
}

func (h caseHandlers) list(c echo.Context) error {
	status := models.CaseStatus(c.QueryParam("status"))
	cs, err := h.cases.ListCases(c.Request().Context(), currentUser(c), status)
	if err != nil {
		return err
	}
	if cs == nil {
		cs = []models.Case{}
	}
	return c.JSON(http.StatusOK, cs)
}

func (h caseHandlers) get(c echo.Context) error {
	cs, err := h.cases.GetCase(c.Request().Context(), currentUser(c), c.Param("id"))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, cs)
}

func (h caseHandlers) join(c echo.Context) error {
	cs, err := h.cases.JoinCase(c.Request().Context(), currentUser(c), c.Param("id"))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, cs)
}

func (h caseHandlers) listMessages(c echo.Context) error {
	limit := 0
	if raw := c.QueryParam("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return models.ErrInvalidInput
		}
		limit = n
	}
	ms, err := h.cases.ListMessages(c.Request().Context(), currentUser(c), c.Param("id"), c.QueryParam("before"), limit)
	if err != nil {
		return err
	}
	if ms == nil {
		ms = []models.Message{}
	}
	return c.JSON(http.StatusOK, ms)
}

func (h caseHandlers) send(c echo.Context) error {
	var req sendMessageRequest
	if err := decodeJSON(c, &req); err != nil {
		return err
	}
	m, err := h.cases.SendMessage(c.Request().Context(), currentUser(c), c.Param("id"), req.Body)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, m)
}

func (h caseHandlers) close(c echo.Context) error {
	cs, err := h.cases.CloseCase(c.Request().Context(), currentUser(c), c.Param("id"))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, cs)
}
```

`api/handlers/router.go`:

```go
// Package handlers is the HTTP and WebSocket adapter. It calls only driving ports (ADR 0001).
package handlers

import (
	"log/slog"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"supportchat/business/usecases"
)

type Config struct {
	Auth  usecases.AuthService
	Cases usecases.CaseService
	// AllowedOrigins are the web app origins, for example "http://localhost:3000".
	AllowedOrigins []string
	Logger         *slog.Logger
}

// NewRouter registers every route in docs/openapi.yaml.
func NewRouter(cfg Config) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.HTTPErrorHandler = ErrorHandler(cfg.Logger)
	e.Use(middleware.Recover())
	e.Use(middleware.BodyLimit("64K"))
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: cfg.AllowedOrigins,
		AllowHeaders: []string{"Authorization", "Content-Type"},
	}))

	auth := requireUser(cfg.Auth)
	cases := caseHandlers{cases: cfg.Cases}

	v1 := e.Group("/v1")
	v1.POST("/login", login(cfg.Auth))
	v1.POST("/cases", cases.open, auth)
	v1.GET("/cases", cases.list, auth)
	v1.GET("/cases/:id", cases.get, auth)
	v1.POST("/cases/:id/join", cases.join, auth)
	v1.GET("/cases/:id/messages", cases.listMessages, auth)
	v1.POST("/cases/:id/messages", cases.send, auth)
	v1.POST("/cases/:id/close", cases.close, auth)
	return e
}
```

- [ ] **Step 5: Run all tests**

Run: `cd api && go mod tidy && cd .. && make test`
Expected: PASS, including `TestDomainErrorsMapToHTTPStatus`, whose 500 case must return `{"error":"internal error"}`.

- [ ] **Step 6: Check routes against the contract**

Run: `grep -E 'v1\.(GET|POST)' api/handlers/router.go` and compare the result with the `paths:` in `docs/openapi.yaml`.
Expected: 8 routes, all present in the contract. Paths use Echo's `:id` in place of OpenAPI's `{id}`.

- [ ] **Step 7: Commit**

```bash
git add docs/openapi.yaml api/
git commit -m "feat: add REST handlers, error mapping and OpenAPI contract

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 12: WebSocket endpoints and the events contract

**Files:**
- Create: `docs/events.md`
- Create: `api/handlers/events.go`
- Modify: `api/handlers/router.go` (add `Events` to `Config`, the two socket routes, and `hosts`)
- Modify: `api/handlers/stubs_test.go` (`newRouter` gets a Hub, and `newRouterWithHub` is added)
- Test: `api/handlers/events_test.go`

**Interfaces:**
- Consumes: `realtime.Hub` subscribe methods (Task 10), `AuthService.Authenticate`, `CaseService.GetCase`
- Produces: `handlers.EventSubscriber` interface; `handlers.Config.Events EventSubscriber`; routes `GET /v1/cases/:id/events?token=` and `GET /v1/cases/events?token=`

- [ ] **Step 1: Write the contract**

`docs/events.md`:

````markdown
# WebSocket Events

This is the contract for server-to-client events (ADR 0003). REST is in `openapi.yaml`.

## Connecting

| Endpoint | Who may connect | Events |
|---|---|---|
| `GET /v1/cases/{id}/events?token=<jwt>` | anyone who may view the case (its customer, or any agent) | `message.created`, `participant.joined`, `case.closed` |
| `GET /v1/cases/events?token=<jwt>` | agents | `case.created`, `case.status_changed` |

The token and the permission are checked **before** the upgrade. A refused connection gets a normal HTTP response:

| Status | Meaning |
|---|---|
| 401 | missing, invalid or expired token |
| 403 | not allowed to view this case, or not an agent (agent feed), or the `Origin` is not an allowed web origin |
| 404 | the case does not exist |

The socket is server-to-client only. If the client sends a message, the server closes the socket.

If the client falls more than 16 events behind, the server closes the socket with status **1013 (try again later)**. The client should reconnect with backoff and refetch over REST.

## Envelope

Every event is one JSON text frame:

```json
{"type": "message.created", "caseId": "0192…", "data": { … }}
```

## Event types

### `message.created` (case topic)

Sent after a message is saved: text messages, and system messages for joins and closes. `data` is a Message, the same shape as in `openapi.yaml`:

```json
{
  "type": "message.created",
  "caseId": "0192f0c1-…",
  "data": {
    "id": "0192f0c2-…",
    "caseId": "0192f0c1-…",
    "senderId": "agent:bob",
    "senderName": "Bob",
    "senderRole": "agent",
    "kind": "text",
    "body": "Click 'Forgot password' on the login page",
    "createdAt": "2026-09-26T09:05:00.123Z"
  }
}
```

The sender receives this event too. Clients deduplicate by `data.id`.

### `participant.joined` (case topic)

Sent when an agent joins for the first time. `data` is a Participant:

```json
{"type": "participant.joined", "caseId": "0192f0c1-…",
 "data": {"userId": "agent:bob", "name": "Bob", "role": "agent", "joinedAt": "2026-09-26T09:04:00Z"}}
```

A `message.created` with the system message "Bob joined the case" follows it.

### `case.closed` (case topic)

Sent when an agent closes the case. `data` is the Case with `status: "closed"` and `closedAt`. After this, sending returns 409.

### `case.created` (agent feed)

Sent when a customer opens a case. `data` is the new Case (`status: "waiting"`).

### `case.status_changed` (agent feed)

Sent when a case moves `waiting → open` (first agent joins) or `→ closed`. `data` is the Case after the change.

## Order

Events for one case are delivered in the order they were published. For a join: `participant.joined`, then `message.created`. For a close: `message.created`, then `case.closed`.
````

- [ ] **Step 2: Update the test helper and write the failing socket tests**

Replace `api/handlers/stubs_test.go` with:

```go
package handlers_test

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"supportchat/business/models"
	"supportchat/handlers"
	"supportchat/realtime"
)

var (
	ann = models.User{ID: "customer:ann", Name: "Ann", Role: models.RoleCustomer}
	bob = models.User{ID: "agent:bob", Name: "Bob", Role: models.RoleAgent}
)

// stubAuth accepts two fixed tokens.
type stubAuth struct {
	login func(name string, role models.Role) (string, models.User, error)
}

func (s stubAuth) Login(_ context.Context, name string, role models.Role) (string, models.User, error) {
	return s.login(name, role)
}

func (stubAuth) Authenticate(_ context.Context, token string) (models.User, error) {
	switch token {
	case "ann-token":
		return ann, nil
	case "bob-token":
		return bob, nil
	}
	return models.User{}, models.ErrUnauthorized
}

// stubCases lets each test set only the use case it expects to be called.
type stubCases struct {
	openCase     func(actor models.User, question string) (models.Case, models.Message, error)
	joinCase     func(actor models.User, caseID string) (models.Case, error)
	sendMessage  func(actor models.User, caseID, body string) (models.Message, error)
	closeCase    func(actor models.User, caseID string) (models.Case, error)
	listCases    func(actor models.User, status models.CaseStatus) ([]models.Case, error)
	getCase      func(actor models.User, caseID string) (models.Case, error)
	listMessages func(actor models.User, caseID, beforeID string, limit int) ([]models.Message, error)
}

func (s *stubCases) OpenCase(_ context.Context, a models.User, q string) (models.Case, models.Message, error) {
	return s.openCase(a, q)
}
func (s *stubCases) JoinCase(_ context.Context, a models.User, id string) (models.Case, error) {
	return s.joinCase(a, id)
}
func (s *stubCases) SendMessage(_ context.Context, a models.User, id, body string) (models.Message, error) {
	return s.sendMessage(a, id, body)
}
func (s *stubCases) CloseCase(_ context.Context, a models.User, id string) (models.Case, error) {
	return s.closeCase(a, id)
}
func (s *stubCases) ListCases(_ context.Context, a models.User, st models.CaseStatus) ([]models.Case, error) {
	return s.listCases(a, st)
}
func (s *stubCases) GetCase(_ context.Context, a models.User, id string) (models.Case, error) {
	return s.getCase(a, id)
}
func (s *stubCases) ListMessages(_ context.Context, a models.User, id, before string, limit int) ([]models.Message, error) {
	return s.listMessages(a, id, before, limit)
}

func newRouter(auth stubAuth, cases *stubCases) *echo.Echo {
	return newRouterWithHub(auth, cases, realtime.NewHub(realtime.DefaultBufferSize))
}

func newRouterWithHub(auth stubAuth, cases *stubCases, hub *realtime.Hub) *echo.Echo {
	return handlers.NewRouter(handlers.Config{
		Auth:           auth,
		Cases:          cases,
		Events:         hub,
		AllowedOrigins: []string{"http://localhost:3000"},
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}

// call sends one request. token "" means no Authorization header.
func call(t *testing.T, e *echo.Echo, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}
```

`api/handlers/events_test.go`:

```go
package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"supportchat/business/models"
	"supportchat/realtime"
)

func wsURL(server *httptest.Server, path string) string {
	return "ws" + strings.TrimPrefix(server.URL, "http") + path
}

func readEvent(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var got map[string]any
	require.NoError(t, wsjson.Read(ctx, conn, &got))
	return got
}

func TestCaseEventsStreamOnlyThatCase(t *testing.T) {
	hub := realtime.NewHub(realtime.DefaultBufferSize)
	cases := &stubCases{getCase: func(_ models.User, id string) (models.Case, error) { return models.Case{ID: id}, nil }}
	server := httptest.NewServer(newRouterWithHub(stubAuth{}, cases, hub))
	defer server.Close()

	conn, _, err := websocket.Dial(context.Background(), wsURL(server, "/v1/cases/c1/events?token=ann-token"), nil)
	require.NoError(t, err)
	defer conn.CloseNow()

	hub.PublishCase("c2", models.Event{Type: models.EventMessageCreated, CaseID: "c2", Data: "not for you"})
	hub.PublishCase("c1", models.Event{Type: models.EventMessageCreated, CaseID: "c1", Data: map[string]string{"body": "hi"}})

	assert.Equal(t, map[string]any{
		"type": "message.created", "caseId": "c1", "data": map[string]any{"body": "hi"},
	}, readEvent(t, conn))
}

func TestCaseEventsRejectBeforeUpgrade(t *testing.T) {
	cases := &stubCases{getCase: func(models.User, string) (models.Case, error) { return models.Case{}, models.ErrForbidden }}
	server := httptest.NewServer(newRouter(stubAuth{}, cases))
	defer server.Close()

	_, resp, err := websocket.Dial(context.Background(), wsURL(server, "/v1/cases/c1/events?token=bad"), nil)
	require.Error(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	_, resp, err = websocket.Dial(context.Background(), wsURL(server, "/v1/cases/c1/events?token=ann-token"), nil)
	require.Error(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestAgentEvents(t *testing.T) {
	hub := realtime.NewHub(realtime.DefaultBufferSize)
	server := httptest.NewServer(newRouterWithHub(stubAuth{}, &stubCases{}, hub))
	defer server.Close()

	_, resp, err := websocket.Dial(context.Background(), wsURL(server, "/v1/cases/events?token=ann-token"), nil)
	require.Error(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "customers cannot watch the agent feed")

	conn, _, err := websocket.Dial(context.Background(), wsURL(server, "/v1/cases/events?token=bob-token"), nil)
	require.NoError(t, err)
	defer conn.CloseNow()

	hub.PublishAgents(models.Event{Type: models.EventCaseCreated, CaseID: "c1", Data: nil})

	assert.Equal(t, map[string]any{"type": "case.created", "caseId": "c1", "data": nil}, readEvent(t, conn))
}

func TestEventsRejectOtherOrigins(t *testing.T) {
	server := httptest.NewServer(newRouter(stubAuth{}, &stubCases{}))
	defer server.Close()

	_, resp, err := websocket.Dial(context.Background(), wsURL(server, "/v1/cases/events?token=bob-token"), &websocket.DialOptions{
		HTTPHeader: http.Header{"Origin": []string{"http://evil.example"}},
	})

	require.Error(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `cd api && go get github.com/coder/websocket@v1.8.15 && go test ./handlers/`
Expected: FAIL with `unknown field Events in struct literal of type handlers.Config`.

- [ ] **Step 4: Implement**

`api/handlers/events.go`:

```go
package handlers

import (
	"context"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/labstack/echo/v4"

	"supportchat/business/models"
	"supportchat/business/usecases"
)

// EventSubscriber is what the WebSocket endpoints need from realtime.Hub.
// Each returned channel is closed when cancel is called or the subscriber is dropped for being slow.
type EventSubscriber interface {
	SubscribeCase(caseID string) (events <-chan models.Event, cancel func())
	SubscribeAgents() (events <-chan models.Event, cancel func())
}

const writeTimeout = 5 * time.Second

type eventHandlers struct {
	auth           usecases.AuthService
	cases          usecases.CaseService
	events         EventSubscriber
	originPatterns []string
}

// caseEvents streams one case's events to anyone who may view the case (docs/events.md).
func (h eventHandlers) caseEvents(c echo.Context) error {
	ctx := c.Request().Context()
	u, err := h.auth.Authenticate(ctx, c.QueryParam("token"))
	if err != nil {
		return err
	}
	if _, err := h.cases.GetCase(ctx, u, c.Param("id")); err != nil {
		return err
	}
	events, cancel := h.events.SubscribeCase(c.Param("id"))
	defer cancel()
	return h.stream(c, events)
}

// agentEvents streams case-list events to agents.
func (h eventHandlers) agentEvents(c echo.Context) error {
	u, err := h.auth.Authenticate(c.Request().Context(), c.QueryParam("token"))
	if err != nil {
		return err
	}
	if u.Role != models.RoleAgent {
		return models.ErrForbidden
	}
	events, cancel := h.events.SubscribeAgents()
	defer cancel()
	return h.stream(c, events)
}

// stream upgrades the connection and writes events until the client leaves or the Hub drops it.
// The socket is server-to-client only (ADR 0003): CloseRead ends the stream if the client sends anything.
func (h eventHandlers) stream(c echo.Context, events <-chan models.Event) error {
	conn, err := websocket.Accept(c.Response(), c.Request(), &websocket.AcceptOptions{
		OriginPatterns: h.originPatterns,
	})
	if err != nil {
		return nil // Accept has already written the HTTP error response.
	}
	defer conn.CloseNow()

	ctx := conn.CloseRead(c.Request().Context())
	for {
		select {
		case <-ctx.Done():
			return nil
		case e, ok := <-events:
			if !ok {
				conn.Close(websocket.StatusTryAgainLater, "client too slow")
				return nil
			}
			wctx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := wsjson.Write(wctx, conn, e)
			cancel()
			if err != nil {
				return nil
			}
		}
	}
}
```

Replace `api/handlers/router.go` with:

```go
// Package handlers is the HTTP and WebSocket adapter. It calls only driving ports (ADR 0001).
package handlers

import (
	"log/slog"
	"net/url"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"supportchat/business/usecases"
)

type Config struct {
	Auth   usecases.AuthService
	Cases  usecases.CaseService
	Events EventSubscriber
	// AllowedOrigins are the web app origins, for example "http://localhost:3000".
	AllowedOrigins []string
	Logger         *slog.Logger
}

// NewRouter registers every route in docs/openapi.yaml and docs/events.md.
func NewRouter(cfg Config) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.HTTPErrorHandler = ErrorHandler(cfg.Logger)
	e.Use(middleware.Recover())
	e.Use(middleware.BodyLimit("64K"))
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: cfg.AllowedOrigins,
		AllowHeaders: []string{"Authorization", "Content-Type"},
	}))

	auth := requireUser(cfg.Auth)
	cases := caseHandlers{cases: cfg.Cases}
	events := eventHandlers{auth: cfg.Auth, cases: cfg.Cases, events: cfg.Events, originPatterns: hosts(cfg.AllowedOrigins)}

	v1 := e.Group("/v1")
	v1.POST("/login", login(cfg.Auth))
	v1.POST("/cases", cases.open, auth)
	v1.GET("/cases", cases.list, auth)
	v1.GET("/cases/:id", cases.get, auth)
	v1.POST("/cases/:id/join", cases.join, auth)
	v1.GET("/cases/:id/messages", cases.listMessages, auth)
	v1.POST("/cases/:id/messages", cases.send, auth)
	v1.POST("/cases/:id/close", cases.close, auth)
	// WebSocket endpoints read the token from ?token= because browsers cannot set headers on a WebSocket.
	v1.GET("/cases/events", events.agentEvents)
	v1.GET("/cases/:id/events", events.caseEvents)
	return e
}

// hosts turns "http://localhost:3000" into "localhost:3000", the form websocket.AcceptOptions expects.
func hosts(origins []string) []string {
	var out []string
	for _, o := range origins {
		if u, err := url.Parse(o); err == nil && u.Host != "" {
			out = append(out, u.Host)
		}
	}
	return out
}
```

- [ ] **Step 5: Run all tests**

Run: `cd api && go mod tidy && cd .. && make test`
Expected: PASS, including `TestCaseEventsRejectBeforeUpgrade` (401 and 403) and `TestEventsRejectOtherOrigins` (403).

- [ ] **Step 6: Commit**

```bash
git add docs/events.md api/
git commit -m "feat: add WebSocket event endpoints and events contract

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 13: Wiring, the end-to-end API test and a manual run

**Files:**
- Create: `api/app.go`, `api/main.go`
- Test: `api/app_test.go`

**Interfaces:**
- Consumes: every constructor above: `repositories.Connect/EnsureIndexes/NewMongo*Repository`, `realtime.NewHub`, `usecases.NewCaseService/NewAuthService`, `auth.NewJWT`, `clock.System`, `ids.UUIDv7`, `handlers.NewRouter`
- Produces: `newApp(ctx, config, *slog.Logger) (*echo.Echo, func(), error)` and `configFromEnv() config`. Environment variables are `PORT` (8080), `MONGO_URI` (mongodb://localhost:27017), `MONGO_DB` (supportchat), `JWT_SECRET` (dev default, logs a warning) and `WEB_ORIGINS` (comma-separated, default http://localhost:3000). Plan 2 depends on these defaults.

- [ ] **Step 1: Write the failing end-to-end test**

This test runs the spec's main story against real MongoDB and a real Hub. It covers user stories 1, 2, 3, 4, 5 and 6 at the API level.

`api/app_test.go`:

```go
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSupportConversation runs the whole API against real MongoDB: the spec's main story end to end.
func TestSupportConversation(t *testing.T) {
	cfg := configFromEnv()
	cfg.MongoDB = fmt.Sprintf("test_app_%d", time.Now().UnixNano())
	cfg.JWTSecret = "test-secret"
	ctx := context.Background()
	router, closeDB, err := newApp(ctx, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err, "run `make up` first")
	defer closeDB()
	server := httptest.NewServer(router)
	defer server.Close()
	api := client{t: t, base: server.URL}

	annToken := api.login("Ann", "customer")
	bobToken := api.login("Bob", "agent")

	agentFeed := api.dial("/v1/cases/events?token=" + bobToken)
	opened := api.do(annToken, http.MethodPost, "/v1/cases", `{"question":"How do I reset my password?"}`, http.StatusCreated)
	caseID := opened["case"].(map[string]any)["id"].(string)
	assert.Equal(t, "case.created", read(t, agentFeed)["type"])

	annRoom := api.dial("/v1/cases/" + caseID + "/events?token=" + annToken)
	api.do(bobToken, http.MethodPost, "/v1/cases/"+caseID+"/join", "", http.StatusOK)
	assert.Equal(t, "participant.joined", read(t, annRoom)["type"])
	assert.Equal(t, "message.created", read(t, annRoom)["type"]) // "Bob joined the case"

	api.do(bobToken, http.MethodPost, "/v1/cases/"+caseID+"/messages", `{"body":"Click 'Forgot password'"}`, http.StatusCreated)
	reply := read(t, annRoom)
	assert.Equal(t, "Click 'Forgot password'", reply["data"].(map[string]any)["body"], "Ann sees Bob's reply without refreshing")

	api.do(bobToken, http.MethodPost, "/v1/cases/"+caseID+"/close", "", http.StatusOK)
	api.do(annToken, http.MethodPost, "/v1/cases/"+caseID+"/messages", `{"body":"one more thing"}`, http.StatusConflict)

	catToken := api.login("Cat", "customer")
	api.do(catToken, http.MethodGet, "/v1/cases/"+caseID, "", http.StatusForbidden)

	history := api.doList(annToken, "/v1/cases/"+caseID+"/messages")
	var bodies []string
	for _, m := range history {
		bodies = append(bodies, m["body"].(string))
	}
	assert.Equal(t, []string{
		"Case closed by Bob", "Click 'Forgot password'", "Bob joined the case", "How do I reset my password?",
	}, bodies)
}

type client struct {
	t    *testing.T
	base string
}

func (c client) login(name, role string) string {
	res := c.do("", http.MethodPost, "/v1/login", fmt.Sprintf(`{"name":%q,"role":%q}`, name, role), http.StatusOK)
	return res["token"].(string)
}

func (c client) send(token, method, path, body string, wantStatus int) []byte {
	c.t.Helper()
	req, err := http.NewRequest(method, c.base+path, bytes.NewBufferString(body))
	require.NoError(c.t, err)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(c.t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(c.t, err)
	require.Equal(c.t, wantStatus, resp.StatusCode, "%s %s: %s", method, path, raw)
	return raw
}

func (c client) do(token, method, path, body string, wantStatus int) map[string]any {
	c.t.Helper()
	var out map[string]any
	require.NoError(c.t, json.Unmarshal(c.send(token, method, path, body, wantStatus), &out))
	return out
}

func (c client) doList(token, path string) []map[string]any {
	c.t.Helper()
	var out []map[string]any
	require.NoError(c.t, json.Unmarshal(c.send(token, http.MethodGet, path, "", http.StatusOK), &out))
	return out
}

func (c client) dial(path string) *websocket.Conn {
	c.t.Helper()
	conn, _, err := websocket.Dial(context.Background(), "ws"+strings.TrimPrefix(c.base, "http")+path, nil)
	require.NoError(c.t, err)
	c.t.Cleanup(func() { conn.CloseNow() })
	return conn
}

func read(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var e map[string]any
	require.NoError(t, wsjson.Read(ctx, conn, &e))
	return e
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd api && go test -run TestSupportConversation .`
Expected: FAIL with `undefined: configFromEnv` and `undefined: newApp`.

- [ ] **Step 3: Implement the wiring**

`api/app.go`:

```go
package main

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"supportchat/business/usecases"
	"supportchat/handlers"
	"supportchat/pkg/auth"
	"supportchat/pkg/clock"
	"supportchat/pkg/ids"
	"supportchat/realtime"
	"supportchat/repositories"
)

const devJWTSecret = "dev-only-secret-change-me"

type config struct {
	Port           string
	MongoURI       string
	MongoDB        string
	JWTSecret      string
	AllowedOrigins []string
}

func configFromEnv() config {
	return config{
		Port:           env("PORT", "8080"),
		MongoURI:       env("MONGO_URI", "mongodb://localhost:27017"),
		MongoDB:        env("MONGO_DB", "supportchat"),
		JWTSecret:      env("JWT_SECRET", devJWTSecret),
		AllowedOrigins: strings.Split(env("WEB_ORIGINS", "http://localhost:3000"), ","),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// newApp wires adapters to use cases. It is the only place that knows every concrete type.
// The returned function disconnects from MongoDB.
func newApp(ctx context.Context, cfg config, logger *slog.Logger) (*echo.Echo, func(), error) {
	client, err := repositories.Connect(ctx, cfg.MongoURI)
	if err != nil {
		return nil, nil, err
	}
	db := client.Database(cfg.MongoDB)
	if err := repositories.EnsureIndexes(ctx, db); err != nil {
		_ = client.Disconnect(ctx)
		return nil, nil, err
	}
	if cfg.JWTSecret == devJWTSecret {
		logger.Warn("JWT_SECRET is not set; using the development secret")
	}

	hub := realtime.NewHub(realtime.DefaultBufferSize)
	cases := usecases.NewCaseService(usecases.CaseDeps{
		Cases:       repositories.NewMongoCaseRepository(db),
		Messages:    repositories.NewMongoMessageRepository(db),
		Broadcaster: hub,
		Clock:       clock.System{},
		IDs:         ids.UUIDv7{},
	})
	authService := usecases.NewAuthService(auth.NewJWT([]byte(cfg.JWTSecret), 12*time.Hour))

	router := handlers.NewRouter(handlers.Config{
		Auth:           authService,
		Cases:          cases,
		Events:         hub,
		AllowedOrigins: cfg.AllowedOrigins,
		Logger:         logger,
	})
	return router, func() { _ = client.Disconnect(context.Background()) }, nil
}
```

`api/main.go`:

```go
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := configFromEnv()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	startCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	router, closeDB, err := newApp(startCtx, cfg, logger)
	cancel()
	if err != nil {
		logger.Error("startup failed", "err", err)
		os.Exit(1)
	}
	defer closeDB()

	go func() {
		logger.Info("api listening", "port", cfg.Port)
		if err := router.Start(":" + cfg.Port); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server stopped", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = router.Shutdown(shutdownCtx)
}
```

- [ ] **Step 4: Run all tests**

Run: `make test`
Expected: every package prints `ok`, including `supportchat` (`TestDependencyRule` and `TestSupportConversation`).

- [ ] **Step 5: Run the API by hand**

Run `make run-api` in one terminal. Expected log: `JWT_SECRET is not set; using the development secret`, then `api listening port=8080`.

In a second terminal:

```bash
TOKEN=$(curl -s -X POST localhost:8080/v1/login -H 'Content-Type: application/json' \
  -d '{"name":"Ann","role":"customer"}' | sed -E 's/.*"token":"([^"]+)".*/\1/')
curl -s -X POST localhost:8080/v1/cases -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"question":"How do I reset my password?"}'
curl -s localhost:8080/v1/cases -H "Authorization: Bearer $TOKEN"
curl -s localhost:8080/v1/cases
```

Expected, in order:
1. The second command returns `{"case":{…"status":"waiting"…},"message":{…}}`.
2. The third command returns a JSON array that contains that case.
3. The last command returns `{"error":"unauthorized"}`.

Stop the API with Ctrl+C. It should exit cleanly.

- [ ] **Step 6: Commit**

```bash
git add api/
git commit -m "feat: wire API with MongoDB, hub and JWT; add end-to-end API test

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

## After this plan

- **Plan 2: Next.js web** (spec step 6): `/login`, `/cases`, `/cases/[id]`. All backend calls go through `web/lib` (`api.ts`, `socket.ts`, `messages.ts`), with Vitest tests for the merge-by-`id` logic and the reconnect backoff (1s, 2s, 4s … 30s). `make test` will also run Vitest.
- **Plan 3: E2E** (spec step 7): the 6 user stories as declarative Cucumber features with Playwright Page Objects and two browser contexts.
