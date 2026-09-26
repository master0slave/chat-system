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
