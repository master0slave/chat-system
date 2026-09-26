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
