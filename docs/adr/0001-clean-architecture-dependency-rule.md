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
