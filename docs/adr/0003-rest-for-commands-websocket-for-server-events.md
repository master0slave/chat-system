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
