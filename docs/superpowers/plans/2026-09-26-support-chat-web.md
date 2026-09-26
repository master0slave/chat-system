# Support Chat Web Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Next.js client: `/login`, `/cases` and `/cases/[id]`. Customers and agents chat in real time against the Go API from plan 1.

**Architecture:** Next.js App Router, used only as a client UI: every page is a client component. All backend access lives in `web/lib`: `api.ts` (REST), `socket.ts` (a reconnecting WebSocket), `messages.ts` and `cases.ts` (merging live data) and `session.ts` (who is logged in). Pages never call `fetch` or open sockets themselves (ADR 0001 rule 6), and `web/lib/architecture.test.ts` enforces that.

**Tech Stack:** Next.js 16.3.6, React 19.3.0, TypeScript 7.0.2, Vitest 5.0.2, Node 26 / npm 11.

**Spec:** `docs/superpowers/specs/2026-09-26-support-chat-design.md` (sections 4, 7, 8, 9; build-order step 6)

**This is plan 2 of 3.** Plan 1 (`docs/superpowers/plans/2026-09-26-support-chat-api.md`) is merged: the API runs on `:8080` and its contracts are `docs/openapi.yaml` and `docs/events.md`. Plan 3 (Cucumber E2E) will drive these pages through the `data-testid` attributes listed below, so keep them stable.

## Global Constraints

- Only `web/lib/**` may call `fetch`, open a `WebSocket`, or touch `sessionStorage`/`localStorage` (ADR 0001 rule 6).
- `web/lib/types.ts` mirrors `docs/openapi.yaml` and `docs/events.md`. Field names are exactly those in the contracts.
- API base URL: `process.env.NEXT_PUBLIC_API_URL`, default `http://localhost:8080`. The web runs on **`:3100`** (port 3000 is taken on the developer's machine). Task 1 changes the API's default `WEB_ORIGINS` to `http://localhost:3100` to match.
- Reconnect backoff: 1s, 2s, 4s … capped at 30s, with a "Reconnecting…" banner. After every (re)connect, refetch over REST and merge by `id` (spec section 8).
- Failed send: the text stays in the input and a Retry button appears (spec section 8).
- Messages on screen: keyed by `id`, sorted oldest first by `createdAt`, with ties broken by `id` (spec section 7, step 7).
- The session lives in `sessionStorage`, so each browser tab can be a different user (a customer in one tab and an agent in another).
- `make test` must pass at the end of every task. It now runs Go tests plus the web typecheck and Vitest.
- Commit messages: Conventional Commits, ending with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Stable `data-testid`s for plan 3: `login-name`, `login-role-customer`, `login-role-agent`, `login-submit`, `login-error`, `current-user`, `logout`, `new-case-question`, `new-case-submit`, `status-filter-{all,waiting,open,closed}`, `case-list`, `case-list-empty`, `case-item` (with `data-case-id`), `case-subject`, `case-status`, `participants`, `reconnecting-banner`, `closed-banner`, `join-button`, `close-button`, `load-older`, `message-list`, `message` (with `data-kind`), `message-body`, `message-input`, `message-send`, `send-error`, `send-retry`, `room-error`.

### Testing split (spec section 9)

Vitest covers `web/lib`: the merge/dedupe logic, reconnect backoff, the API client and the dependency rule. Pages have no unit tests. Each page task ends with a scripted manual check in a browser with exact expected results, and plan 3 turns those checks into Cucumber scenarios. A page task's "failing test" is therefore the typecheck and build plus the manual check. The real TDD cycle in this plan runs in Tasks 1–5.

## Review Focus

1. **A sent message arrives twice: once in the POST response and once as `message.created`.** It must show once. Covered by `mergeMessages` tests (Task 1) and the two-tab check (Task 7).
2. **The API restarts while a case room is open.** The banner shows, the socket reconnects on its own with backoff, missed messages appear, and nothing is duplicated. Covered by `connectEvents` tests (Task 4) and the reconnect check (Task 7 Step 5).
3. **A send fails (the case was closed meanwhile, or the network is down).** The text stays and Retry is offered. Covered by the manual check in Task 7 Step 6.
4. **A customer and an agent log in in two tabs of the same browser.** Each tab keeps its own user. Covered by `session` tests (Task 2) and the two-tab check (Task 7).
5. **A customer pastes another customer's case URL.** They see "You cannot view this case." and the page stops retrying the socket. Covered by Task 7 Step 7.

## File Map

```
Makefile (test-web, run-web), .gitignore                  Task 1
web/package.json, package-lock.json, tsconfig.json, next.config.ts   Task 1
web/app/layout.tsx (no header yet), page.tsx, globals.css, icon.svg  Task 1; layout replaced in Task 5
web/lib/types.ts, messages.ts (+ test)                    Task 1
web/lib/cases.ts, session.ts (+ tests)                    Task 2
web/lib/api.ts (+ test)                                   Task 3
web/lib/socket.ts (+ test)                                Task 4
web/lib/architecture.test.ts, app/header.tsx, app/use-session.ts, app/login/page.tsx   Task 5
web/AGENTS.md, web/CLAUDE.md (written by `next dev`)      Task 5
web/app/cases/page.tsx                                    Task 6
web/app/cases/[id]/page.tsx                               Task 7
```

---

### Task 1: Web skeleton, Makefile targets and message merging

**Files:**
- Create: `web/package.json`, `web/tsconfig.json`, `web/next.config.ts`
- Create: `web/app/layout.tsx`, `web/app/page.tsx`, `web/app/globals.css`, `web/app/icon.svg`
- Create: `web/lib/types.ts`, `web/lib/messages.ts`
- Test: `web/lib/messages.test.ts`
- Modify: `Makefile`, `.gitignore`
- Modify: `api/app.go` (default `WEB_ORIGINS`)
- Test: `api/config_test.go`

**Interfaces:**
- Consumes: the JSON shapes in `docs/openapi.yaml` and `docs/events.md`
- Produces: the types `Role`, `CaseStatus`, `User`, `Participant`, `Case`, `Message` and `ServerEvent` (a union over `type`) from `@/lib/types`; `mergeMessages(current: Message[], incoming: Message[]): Message[]` from `@/lib/messages`; Make targets `test-web` and `run-web` (the web runs on :3100), with `test` now running both suites; the API allows `http://localhost:3100` by default

- [ ] **Step 1: Make the API allow the web app's port by default**

The web app runs on :3100, but the API from plan 1 allows only `http://localhost:3000` by default, so the browser would be refused CORS and WebSocket access.

`api/config_test.go`:

```go
package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The web app runs on :3100 (web/package.json), so that is the origin the API allows by default.
func TestDefaultWebOriginIsTheWebApp(t *testing.T) {
	t.Setenv("WEB_ORIGINS", "")

	assert.Equal(t, []string{"http://localhost:3100"}, configFromEnv().AllowedOrigins)
}

func TestWebOriginsFromEnvironment(t *testing.T) {
	t.Setenv("WEB_ORIGINS", "http://a.example,http://b.example")

	assert.Equal(t, []string{"http://a.example", "http://b.example"}, configFromEnv().AllowedOrigins)
}
```

Run: `cd api && go test -count=1 -run WebOrigin .`
Expected: FAIL with `expected: []string{"http://localhost:3100"}` and `actual: []string{"http://localhost:3000"}`.

In `api/app.go`, change the default in `configFromEnv`:

```go
		AllowedOrigins: strings.Split(env("WEB_ORIGINS", "http://localhost:3100"), ","),
```

Run the same command again. Expected: `ok  supportchat`.

- [ ] **Step 2: Create the package and config**

`web/package.json`:

```json
{
  "name": "supportchat-web",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "next dev --port 3100",
    "build": "next build",
    "start": "next start --port 3100",
    "typecheck": "next typegen && tsc --noEmit",
    "test": "vitest run"
  },
  "dependencies": {
    "next": "16.3.6",
    "react": "19.3.0",
    "react-dom": "19.3.0"
  },
  "devDependencies": {
    "@types/node": "26.6.3",
    "@types/react": "19.3.0",
    "@types/react-dom": "19.3.0",
    "typescript": "7.0.2",
    "vitest": "5.0.2"
  }
}
```

`web/tsconfig.json`:

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "lib": [
      "dom",
      "dom.iterable",
      "es2022"
    ],
    "allowJs": false,
    "skipLibCheck": true,
    "strict": true,
    "noEmit": true,
    "esModuleInterop": true,
    "module": "esnext",
    "moduleResolution": "bundler",
    "resolveJsonModule": true,
    "isolatedModules": true,
    "jsx": "react-jsx",
    "incremental": true,
    "plugins": [
      {
        "name": "next"
      }
    ],
    "paths": {
      "@/*": [
        "./*"
      ]
    }
  },
  "include": [
    "next-env.d.ts",
    "**/*.ts",
    "**/*.tsx",
    ".next/types/**/*.ts",
    ".next/dev/types/**/*.ts"
  ],
  "exclude": [
    "node_modules"
  ]
}
```

`web/next.config.ts`:

```ts
import type { NextConfig } from "next";

const nextConfig: NextConfig = {};

export default nextConfig;
```

Run: `cd web && npm install --no-audit --no-fund`
Expected: `added 60 packages` (or similar) and a new `web/package-lock.json`. A notice about `npm approve-scripts` is fine: nothing in this plan needs install scripts.

- [ ] **Step 3: Create the app shell**

Next needs an `app/` directory before `next typegen` or `next build` will run. The header is added in Task 5.

`web/app/layout.tsx`:

```tsx
import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Support Chat",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>
        <main>{children}</main>
      </body>
    </html>
  );
}
```

`web/app/page.tsx`:

```tsx
import { redirect } from "next/navigation";

export default function Home() {
  redirect("/cases");
}
```

`web/app/icon.svg`:

```svg
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><rect width="32" height="32" rx="8" fill="#2563eb"/><path d="M8 10h16v10H14l-4 4v-4H8z" fill="#fff"/></svg>
```

`web/app/globals.css`:

```css
:root {
  --bg: #f6f7f9;
  --surface: #ffffff;
  --text: #1c2430;
  --muted: #667085;
  --border: #d8dde5;
  --accent: #2563eb;
  --accent-text: #ffffff;
  --danger: #b42318;
  --mine: #e8f0fe;
  --system: #f2f4f7;
  color-scheme: light;
}

@media (prefers-color-scheme: dark) {
  :root {
    --bg: #111418;
    --surface: #1a1f26;
    --text: #e6e9ee;
    --muted: #98a2b3;
    --border: #2d3540;
    --accent: #5b8def;
    --accent-text: #0b1220;
    --danger: #f97066;
    --mine: #1d2b45;
    --system: #222932;
    color-scheme: dark;
  }
}

* {
  box-sizing: border-box;
}

body {
  margin: 0;
  background: var(--bg);
  color: var(--text);
  font: 15px/1.5 system-ui, -apple-system, "Segoe UI", "Noto Sans Thai", sans-serif;
}

main {
  max-width: 760px;
  margin: 0 auto;
  padding: 16px;
}

a {
  color: var(--accent);
}

h1 {
  font-size: 1.4rem;
  margin: 8px 0 16px;
}

.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 16px;
  background: var(--surface);
  border-bottom: 1px solid var(--border);
}

.header a {
  color: var(--text);
  font-weight: 600;
  text-decoration: none;
}

.card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 16px;
  margin-bottom: 16px;
}

.row {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}

input[type="text"],
textarea {
  width: 100%;
  padding: 8px 10px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--bg);
  color: var(--text);
  font: inherit;
}

button {
  padding: 8px 14px;
  border: 1px solid var(--accent);
  border-radius: 8px;
  background: var(--accent);
  color: var(--accent-text);
  font: inherit;
  cursor: pointer;
}

button.secondary {
  background: transparent;
  color: var(--accent);
}

button:disabled {
  opacity: 0.5;
  cursor: default;
}

.error {
  color: var(--danger);
}

.muted {
  color: var(--muted);
  font-size: 0.9em;
}

.banner {
  padding: 8px 12px;
  border-radius: 8px;
  background: var(--system);
  margin-bottom: 12px;
}

.status {
  display: inline-block;
  padding: 1px 8px;
  border-radius: 999px;
  border: 1px solid var(--border);
  font-size: 0.8em;
}

.case-list {
  list-style: none;
  margin: 0;
  padding: 0;
}

.case-list li {
  border-top: 1px solid var(--border);
}

.case-list a {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 4px;
  color: var(--text);
  text-decoration: none;
}

.messages {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: 60vh;
  overflow-y: auto;
  margin-bottom: 12px;
}

.message {
  padding: 8px 12px;
  border-radius: 10px;
  background: var(--system);
  max-width: 85%;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.message.mine {
  align-self: flex-end;
  background: var(--mine);
}

.message.system {
  align-self: center;
  background: transparent;
  color: var(--muted);
  font-size: 0.9em;
}
```

- [ ] **Step 4: Add the Makefile targets and ignore rules**

Replace `Makefile` with (recipe lines start with a TAB):

```makefile
COMPOSE = docker compose -f api/deployment/local/docker-compose.yaml

.PHONY: up down test test-api test-web run-api run-web

up: ## Start MongoDB and wait until it is healthy
	$(COMPOSE) up -d --wait

down: ## Stop MongoDB (data is kept in a volume)
	$(COMPOSE) down

test: up test-api test-web ## Run every test. A change is done only when this passes.

test-api:
	cd api && go vet ./... && go test -race -count=1 ./...

test-web: web/node_modules
	cd web && npm run typecheck && npm test

web/node_modules: web/package-lock.json
	cd web && npm ci --no-audit --no-fund
	touch web/node_modules

run-api: up ## Run the API on :8080
	cd api && go run .

run-web: web/node_modules ## Run the web app on :3100 (needs run-api in another terminal)
	cd web && npm run dev
```

Replace `.gitignore` with:

```text
.DS_Store
.env
node_modules/
.next/
api/bin/
.superpowers/
web/next-env.d.ts
web/tsconfig.tsbuildinfo
```

- [ ] **Step 5: Write the types and the failing merge tests**

`web/lib/types.ts`:

```ts
// Shapes from docs/openapi.yaml and docs/events.md. Keep them in sync with those files.

export type Role = "customer" | "agent";
export type CaseStatus = "waiting" | "open" | "closed";

export interface User {
  id: string;
  name: string;
  role: Role;
}

export interface Participant {
  userId: string;
  name: string;
  role: Role;
  joinedAt: string;
}

export interface Case {
  id: string;
  subject: string;
  customerId: string;
  participants: Participant[];
  status: CaseStatus;
  createdAt: string;
  updatedAt: string;
  closedAt?: string;
}

export interface Message {
  id: string;
  caseId: string;
  senderId: string;
  senderName: string;
  senderRole: Role;
  kind: "text" | "system";
  body: string;
  createdAt: string;
}

export type ServerEvent =
  | { type: "message.created"; caseId: string; data: Message }
  | { type: "participant.joined"; caseId: string; data: Participant }
  | { type: "case.closed"; caseId: string; data: Case }
  | { type: "case.created"; caseId: string; data: Case }
  | { type: "case.status_changed"; caseId: string; data: Case };
```

`web/lib/messages.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { mergeMessages } from "./messages";
import type { Message } from "./types";

function msg(id: string, createdAt: string, body = id): Message {
  return {
    id,
    caseId: "c1",
    senderId: "customer:ann",
    senderName: "Ann",
    senderRole: "customer",
    kind: "text",
    body,
    createdAt,
  };
}

describe("mergeMessages", () => {
  it("shows a message once when it arrives from both the response and the event", () => {
    const sent = msg("m2", "2026-09-26T09:01:00.000Z");

    const afterResponse = mergeMessages([msg("m1", "2026-09-26T09:00:00.000Z")], [sent]);
    const afterEvent = mergeMessages(afterResponse, [sent]);

    expect(afterEvent.map((m) => m.id)).toEqual(["m1", "m2"]);
  });

  it("sorts oldest first, whatever order messages arrive in", () => {
    const newestFirstFromApi = [
      msg("m3", "2026-09-26T09:02:00.000Z"),
      msg("m2", "2026-09-26T09:01:00.000Z"),
      msg("m1", "2026-09-26T09:00:00.000Z"),
    ];

    expect(mergeMessages([], newestFirstFromApi).map((m) => m.id)).toEqual(["m1", "m2", "m3"]);
  });

  it("breaks ties in the same millisecond by id", () => {
    const sameTime = "2026-09-26T09:00:00.000Z";

    const merged = mergeMessages([msg("0192-b", sameTime)], [msg("0192-a", sameTime)]);

    expect(merged.map((m) => m.id)).toEqual(["0192-a", "0192-b"]);
  });

  it("keeps the later copy of a message", () => {
    const merged = mergeMessages([msg("m1", "2026-09-26T09:00:00.000Z", "old")], [msg("m1", "2026-09-26T09:00:00.000Z", "new")]);

    expect(merged.map((m) => m.body)).toEqual(["new"]);
  });

  it("does not change the lists it was given", () => {
    const current = [msg("m1", "2026-09-26T09:00:00.000Z")];

    mergeMessages(current, [msg("m2", "2026-09-26T09:01:00.000Z")]);

    expect(current).toHaveLength(1);
  });
});
```

Run: `cd web && npx vitest run lib/messages.test.ts`
Expected: FAIL with `Failed to resolve import "./messages"` (or `Cannot find module`).

- [ ] **Step 6: Implement**

`web/lib/messages.ts`:

```ts
import type { Message } from "./types";

// mergeMessages adds incoming messages to the current list and returns a new list, oldest first.
// A message can arrive twice (the REST response and the message.created event, or a refetch
// after reconnecting), so messages are keyed by id and the later copy wins.
export function mergeMessages(current: Message[], incoming: Message[]): Message[] {
  const byId = new Map<string, Message>();
  for (const m of current) byId.set(m.id, m);
  for (const m of incoming) byId.set(m.id, m);
  return [...byId.values()].sort(compareMessages);
}

// Oldest first by createdAt. Ids sort in creation order (UUIDv7), so they break ties
// between messages created in the same millisecond.
function compareMessages(a: Message, b: Message): number {
  if (a.createdAt !== b.createdAt) return a.createdAt < b.createdAt ? -1 : 1;
  return a.id < b.id ? -1 : a.id > b.id ? 1 : 0;
}
```

- [ ] **Step 7: Run everything**

Run: `make test`
Expected: all Go packages `ok`, then `✓ Types generated successfully` and `Tests  5 passed (5)`.

Run: `cd web && npx next build`
Expected: `✓ Compiled successfully` and the routes `/`, `/_not-found` and `/icon.svg`.

- [ ] **Step 8: Commit**

```bash
git add Makefile .gitignore web/ api/app.go api/config_test.go
git commit -m "feat(web): add Next.js skeleton on :3100 and message merging

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Case list merging and the session store

**Files:**
- Create: `web/lib/cases.ts`, `web/lib/session.ts`
- Test: `web/lib/cases.test.ts`, `web/lib/session.test.ts`

**Interfaces:**
- Consumes: `Case`, `CaseStatus`, `User` from Task 1
- Produces: `upsertCase(list: Case[], changed: Case, statusFilter?: CaseStatus): Case[]`; `Session { token: string; user: User }`; `saveSession(session, store?)`, `loadSession(store?): Session | null`, `clearSession(store?)`. `store` defaults to `window.sessionStorage` and is `undefined` on the server.

- [ ] **Step 1: Write the failing tests**

`web/lib/cases.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { upsertCase } from "./cases";
import type { Case, CaseStatus } from "./types";

function kase(id: string, status: CaseStatus, updatedAt: string): Case {
  return {
    id,
    subject: id,
    customerId: "customer:ann",
    participants: [],
    status,
    createdAt: "2026-09-26T09:00:00.000Z",
    updatedAt,
  };
}

describe("upsertCase", () => {
  it("adds a new case at the top", () => {
    const list = [kase("old", "waiting", "2026-09-26T09:00:00.000Z")];

    const next = upsertCase(list, kase("new", "waiting", "2026-09-26T09:05:00.000Z"));

    expect(next.map((c) => c.id)).toEqual(["new", "old"]);
  });

  it("replaces a changed case and moves it by updatedAt", () => {
    const list = [kase("a", "waiting", "2026-09-26T09:02:00.000Z"), kase("b", "waiting", "2026-09-26T09:01:00.000Z")];

    const next = upsertCase(list, kase("b", "open", "2026-09-26T09:03:00.000Z"));

    expect(next.map((c) => `${c.id}:${c.status}`)).toEqual(["b:open", "a:waiting"]);
  });

  it("drops a case that no longer matches the status filter", () => {
    const list = [kase("a", "waiting", "2026-09-26T09:00:00.000Z")];

    const next = upsertCase(list, kase("a", "open", "2026-09-26T09:01:00.000Z"), "waiting");

    expect(next).toEqual([]);
  });

  it("ignores a new case that does not match the status filter", () => {
    expect(upsertCase([], kase("a", "closed", "2026-09-26T09:00:00.000Z"), "waiting")).toEqual([]);
  });
});
```

`web/lib/session.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { clearSession, loadSession, saveSession } from "./session";

class MemoryStorage implements Storage {
  private items = new Map<string, string>();
  get length() {
    return this.items.size;
  }
  clear() {
    this.items.clear();
  }
  getItem(key: string) {
    return this.items.get(key) ?? null;
  }
  key(index: number) {
    return [...this.items.keys()][index] ?? null;
  }
  removeItem(key: string) {
    this.items.delete(key);
  }
  setItem(key: string, value: string) {
    this.items.set(key, value);
  }
}

const session = { token: "t1", user: { id: "customer:ann", name: "Ann", role: "customer" as const } };

describe("session", () => {
  it("saves, loads and clears", () => {
    const store = new MemoryStorage();

    saveSession(session, store);
    expect(loadSession(store)).toEqual(session);

    clearSession(store);
    expect(loadSession(store)).toBeNull();
  });

  it("treats damaged data as logged out", () => {
    const store = new MemoryStorage();
    store.setItem("supportchat.session", "{not json");

    expect(loadSession(store)).toBeNull();
  });

  it("is logged out when there is no storage (server rendering)", () => {
    expect(loadSession(undefined)).toBeNull();
  });
});
```

Run: `cd web && npx vitest run lib/cases.test.ts lib/session.test.ts`
Expected: FAIL. Both files fail to resolve `./cases` and `./session`.

- [ ] **Step 2: Implement**

`web/lib/cases.ts`:

```ts
import type { Case, CaseStatus } from "./types";

// upsertCase puts a new or changed case into a case list, newest updatedAt first.
// With a status filter, a case that no longer matches it leaves the list.
export function upsertCase(list: Case[], changed: Case, statusFilter?: CaseStatus): Case[] {
  const others = list.filter((c) => c.id !== changed.id);
  const next = !statusFilter || changed.status === statusFilter ? [...others, changed] : others;
  return next.sort((a, b) => (a.updatedAt < b.updatedAt ? 1 : a.updatedAt > b.updatedAt ? -1 : 0));
}
```

`web/lib/session.ts`:

```ts
import type { User } from "./types";

export interface Session {
  token: string;
  user: User;
}

const KEY = "supportchat.session";

// The session lives in sessionStorage, so each browser tab can be a different user
// (for example a customer in one tab and an agent in another).
function storage(): Storage | undefined {
  return typeof window === "undefined" ? undefined : window.sessionStorage;
}

export function saveSession(session: Session, store: Storage | undefined = storage()): void {
  store?.setItem(KEY, JSON.stringify(session));
}

export function loadSession(store: Storage | undefined = storage()): Session | null {
  const raw = store?.getItem(KEY);
  if (!raw) return null;
  try {
    return JSON.parse(raw) as Session;
  } catch {
    return null;
  }
}

export function clearSession(store: Storage | undefined = storage()): void {
  store?.removeItem(KEY);
}
```

- [ ] **Step 3: Run everything**

Run: `make test`
Expected: Go `ok` lines, then `Tests  12 passed (12)`.

- [ ] **Step 4: Commit**

```bash
git add web/lib
git commit -m "feat(web): add case list upsert and per-tab session store

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: API client

**Files:**
- Create: `web/lib/api.ts`
- Test: `web/lib/api.test.ts`

**Interfaces:**
- Consumes: `loadSession` (Task 2), the types (Task 1)
- Produces: `createApi(baseUrl, getToken, fetchImpl?)`, `ApiError { status: number; message: string }` (status 0 means the server could not be reached), `API_URL`, and the shared instance `api`. It has these methods: `login(name, role)`, `openCase(question)`, `listCases(status?)`, `getCase(id)`, `joinCase(id)`, `closeCase(id)`, `sendMessage(id, body)`, `listMessages(id, {before?, limit?})` (newest first), `caseEventsUrl(id)` and `agentEventsUrl()`.

- [ ] **Step 1: Write the failing tests**

`web/lib/api.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { ApiError, createApi } from "./api";

type Call = { url: string; init: RequestInit };

function fakeFetch(status: number, body: unknown) {
  const calls: Call[] = [];
  const impl = (async (url: string, init: RequestInit) => {
    calls.push({ url, init });
    return new Response(body === undefined ? null : JSON.stringify(body), { status });
  }) as typeof fetch;
  return { calls, impl };
}

describe("createApi", () => {
  it("sends the token and a JSON body", async () => {
    const f = fakeFetch(201, { id: "m1" });
    const api = createApi("http://api", () => "tok", f.impl);

    await api.sendMessage("c1", "hello");

    expect(f.calls[0].url).toBe("http://api/v1/cases/c1/messages");
    expect(f.calls[0].init.method).toBe("POST");
    expect(f.calls[0].init.headers).toEqual({ Authorization: "Bearer tok", "Content-Type": "application/json" });
    expect(f.calls[0].init.body).toBe(JSON.stringify({ body: "hello" }));
  });

  it("sends no Authorization header when logged out", async () => {
    const f = fakeFetch(200, { token: "t", user: {} });
    const api = createApi("http://api", () => undefined, f.impl);

    await api.login("Ann", "customer");

    expect(f.calls[0].init.headers).toEqual({ "Content-Type": "application/json" });
  });

  it("turns an error response into an ApiError with the server's message", async () => {
    const api = createApi("http://api", () => "tok", fakeFetch(409, { error: "case is closed" }).impl);

    const err = await api.sendMessage("c1", "hi").catch((e: unknown) => e);

    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({ status: 409, message: "case is closed" });
  });

  it("reports a network failure as status 0", async () => {
    const failing = (async () => {
      throw new TypeError("fetch failed");
    }) as typeof fetch;
    const api = createApi("http://api", () => "tok", failing);

    await expect(api.getCase("c1")).rejects.toMatchObject({ status: 0, message: "Cannot reach the server" });
  });

  it("builds query strings for listing", async () => {
    const f = fakeFetch(200, []);
    const api = createApi("http://api", () => "tok", f.impl);

    await api.listCases("waiting");
    await api.listMessages("c1", { before: "m9", limit: 20 });
    await api.listMessages("c1");

    expect(f.calls.map((c) => c.url)).toEqual([
      "http://api/v1/cases?status=waiting",
      "http://api/v1/cases/c1/messages?before=m9&limit=20",
      "http://api/v1/cases/c1/messages",
    ]);
  });

  it("builds WebSocket URLs with the token", () => {
    const api = createApi("https://api.example", () => "a b", fakeFetch(200, null).impl);

    expect(api.caseEventsUrl("c1")).toBe("wss://api.example/v1/cases/c1/events?token=a%20b");
    expect(api.agentEventsUrl()).toBe("wss://api.example/v1/cases/events?token=a%20b");
  });
});
```

Run: `cd web && npx vitest run lib/api.test.ts`
Expected: FAIL, because `./api` cannot be resolved.

- [ ] **Step 2: Implement**

`web/lib/api.ts`:

```ts
import { loadSession } from "./session";
import type { Case, CaseStatus, Message, Role, User } from "./types";

// The API client. Pages call these functions; they never call fetch themselves (ADR 0001).

export class ApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

type Fetch = typeof fetch;

export function createApi(baseUrl: string, getToken: () => string | undefined, fetchImpl: Fetch = (...args) => fetch(...args)) {
  async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
    const headers: Record<string, string> = {};
    const token = getToken();
    if (token) headers.Authorization = `Bearer ${token}`;
    if (body !== undefined) headers["Content-Type"] = "application/json";

    let res: Response;
    try {
      res = await fetchImpl(baseUrl + path, {
        method,
        headers,
        body: body === undefined ? undefined : JSON.stringify(body),
      });
    } catch {
      throw new ApiError(0, "Cannot reach the server");
    }
    const data = await res.json().catch(() => null);
    if (!res.ok) {
      throw new ApiError(res.status, (data as { error?: string } | null)?.error ?? `Request failed (${res.status})`);
    }
    return data as T;
  }

  const id = encodeURIComponent;

  return {
    login: (name: string, role: Role) => request<{ token: string; user: User }>("POST", "/v1/login", { name, role }),
    openCase: (question: string) => request<{ case: Case; message: Message }>("POST", "/v1/cases", { question }),
    listCases: (status?: CaseStatus) => request<Case[]>("GET", "/v1/cases" + (status ? `?status=${status}` : "")),
    getCase: (caseId: string) => request<Case>("GET", `/v1/cases/${id(caseId)}`),
    joinCase: (caseId: string) => request<Case>("POST", `/v1/cases/${id(caseId)}/join`),
    closeCase: (caseId: string) => request<Case>("POST", `/v1/cases/${id(caseId)}/close`),
    sendMessage: (caseId: string, body: string) => request<Message>("POST", `/v1/cases/${id(caseId)}/messages`, { body }),
    // listMessages returns newest first, like the API. Pass the oldest id you have as `before` to page back.
    listMessages: (caseId: string, opts: { before?: string; limit?: number } = {}) => {
      const q = new URLSearchParams();
      if (opts.before) q.set("before", opts.before);
      if (opts.limit) q.set("limit", String(opts.limit));
      const qs = q.toString();
      return request<Message[]>("GET", `/v1/cases/${id(caseId)}/messages` + (qs ? `?${qs}` : ""));
    },
    // WebSocket URLs carry the token as a query parameter, because browsers cannot set headers on a WebSocket.
    caseEventsUrl: (caseId: string) => eventsUrl(baseUrl, `/v1/cases/${id(caseId)}/events`, getToken()),
    agentEventsUrl: () => eventsUrl(baseUrl, "/v1/cases/events", getToken()),
  };
}

function eventsUrl(baseUrl: string, path: string, token: string | undefined): string {
  return baseUrl.replace(/^http/, "ws") + path + `?token=${encodeURIComponent(token ?? "")}`;
}

export type Api = ReturnType<typeof createApi>;

export const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export const api: Api = createApi(API_URL, () => loadSession()?.token);
```

- [ ] **Step 3: Run everything**

Run: `make test`
Expected: `Tests  18 passed (18)`.

- [ ] **Step 4: Commit**

```bash
git add web/lib
git commit -m "feat(web): add typed API client

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: Reconnecting event socket

**Files:**
- Create: `web/lib/socket.ts`
- Test: `web/lib/socket.test.ts`

**Interfaces:**
- Consumes: `ServerEvent` (Task 1)
- Produces:
  - `backoffDelay(attempt: number): number` and `MAX_BACKOFF_MS = 30000`
  - `SocketStatus = "connecting" | "open" | "reconnecting"`
  - `connectEvents({ url: () => string, onEvent, onStatus?, createSocket? }): () => void`. The returned function stops the socket for good. `onStatus("open")` fires after every successful (re)connect, which is when callers should refetch.

- [ ] **Step 1: Write the failing tests**

`web/lib/socket.test.ts`:

```ts
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { backoffDelay, connectEvents, type SocketLike, type SocketStatus } from "./socket";

class FakeSocket implements SocketLike {
  onopen: ((ev: Event) => void) | null = null;
  onmessage: ((ev: MessageEvent) => void) | null = null;
  onclose: ((ev: CloseEvent) => void) | null = null;
  closed = false;
  constructor(readonly url: string) {}
  close() {
    this.closed = true;
  }
  open() {
    this.onopen?.(new Event("open"));
  }
  receive(data: string) {
    this.onmessage?.({ data } as MessageEvent);
  }
  drop() {
    this.onclose?.({} as CloseEvent);
  }
}

function setup() {
  const sockets: FakeSocket[] = [];
  const statuses: SocketStatus[] = [];
  const events: unknown[] = [];
  let n = 0;
  const stop = connectEvents({
    url: () => `ws://api/events?token=t${++n}`,
    onEvent: (e) => events.push(e),
    onStatus: (s) => statuses.push(s),
    createSocket: (url) => {
      const s = new FakeSocket(url);
      sockets.push(s);
      return s;
    },
  });
  return { sockets, statuses, events, stop };
}

describe("backoffDelay", () => {
  it("doubles from 1s and stops at 30s", () => {
    expect([0, 1, 2, 3, 4, 5, 6, 10].map(backoffDelay)).toEqual([1000, 2000, 4000, 8000, 16000, 30000, 30000, 30000]);
  });
});

describe("connectEvents", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("delivers parsed events and ignores garbage", () => {
    const { sockets, events } = setup();
    sockets[0].open();

    sockets[0].receive('{"type":"message.created","caseId":"c1","data":{}}');
    sockets[0].receive("not json");

    expect(events).toEqual([{ type: "message.created", caseId: "c1", data: {} }]);
  });

  it("reconnects with growing delays and reports each state", () => {
    const { sockets, statuses } = setup();
    sockets[0].open();

    sockets[0].drop();
    vi.advanceTimersByTime(999);
    expect(sockets).toHaveLength(1);
    vi.advanceTimersByTime(1);
    expect(sockets).toHaveLength(2);

    sockets[1].drop(); // the retry fails too
    vi.advanceTimersByTime(1999);
    expect(sockets).toHaveLength(2);
    vi.advanceTimersByTime(1);
    expect(sockets).toHaveLength(3);

    sockets[2].open();
    expect(statuses).toEqual(["connecting", "open", "reconnecting", "reconnecting", "reconnecting", "reconnecting", "open"]);
  });

  it("starts again from 1s after a successful reconnect", () => {
    const { sockets } = setup();
    sockets[0].drop();
    vi.advanceTimersByTime(1000);
    sockets[1].drop();
    vi.advanceTimersByTime(2000);
    sockets[2].open();

    sockets[2].drop();
    vi.advanceTimersByTime(1000);

    expect(sockets).toHaveLength(4);
  });

  it("asks for the URL again on every attempt", () => {
    const { sockets } = setup();
    sockets[0].drop();
    vi.advanceTimersByTime(1000);

    expect(sockets.map((s) => s.url)).toEqual(["ws://api/events?token=t1", "ws://api/events?token=t2"]);
  });

  it("stops for good when closed by the caller", () => {
    const { sockets, stop } = setup();
    sockets[0].open();

    stop();
    sockets[0].drop();
    vi.advanceTimersByTime(60_000);

    expect(sockets[0].closed).toBe(true);
    expect(sockets).toHaveLength(1);
  });
});
```

Run: `cd web && npx vitest run lib/socket.test.ts`
Expected: FAIL, because `./socket` cannot be resolved.

- [ ] **Step 2: Implement**

`web/lib/socket.ts`:

```ts
import type { ServerEvent } from "./types";

// A WebSocket that reconnects with exponential backoff (1s, 2s, 4s, … up to 30s).
// Pages open sockets only through connectEvents (ADR 0001, ADR 0003).

export type SocketStatus = "connecting" | "open" | "reconnecting";

export const MAX_BACKOFF_MS = 30_000;

// backoffDelay returns how long to wait before reconnect attempt number `attempt` (0-based).
export function backoffDelay(attempt: number): number {
  return Math.min(1000 * 2 ** attempt, MAX_BACKOFF_MS);
}

// The part of the browser WebSocket that connectEvents uses, so tests can pass a fake.
export interface SocketLike {
  onopen: ((ev: Event) => void) | null;
  onmessage: ((ev: MessageEvent) => void) | null;
  onclose: ((ev: CloseEvent) => void) | null;
  close(): void;
}

export interface EventSocketOptions {
  url: () => string; // called on every attempt, so a fresh token is used
  onEvent: (event: ServerEvent) => void;
  onStatus?: (status: SocketStatus) => void;
  createSocket?: (url: string) => SocketLike;
}

// connectEvents keeps a socket open until the returned function is called.
// After every successful (re)connect it reports "open": that is when callers should
// refetch over REST, because events sent while disconnected are lost (ADR 0003).
export function connectEvents(opts: EventSocketOptions): () => void {
  const createSocket = opts.createSocket ?? ((url: string) => new WebSocket(url));
  let socket: SocketLike | null = null;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let attempt = 0;
  let stopped = false;

  function connect() {
    opts.onStatus?.(attempt === 0 ? "connecting" : "reconnecting");
    const s = createSocket(opts.url());
    socket = s;
    s.onopen = () => {
      attempt = 0;
      opts.onStatus?.("open");
    };
    s.onmessage = (ev) => {
      try {
        opts.onEvent(JSON.parse(String(ev.data)) as ServerEvent);
      } catch {
        // Ignore frames that are not valid JSON.
      }
    };
    s.onclose = () => {
      if (stopped || socket !== s) return;
      opts.onStatus?.("reconnecting");
      timer = setTimeout(connect, backoffDelay(attempt));
      attempt++;
    };
  }

  connect();
  return () => {
    stopped = true;
    clearTimeout(timer);
    socket?.close();
  };
}
```

- [ ] **Step 3: Run everything**

Run: `make test`
Expected: `Tests  24 passed (24)`.

- [ ] **Step 4: Commit**

```bash
git add web/lib
git commit -m "feat(web): add reconnecting event socket with backoff

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: Dependency guard, app shell and login page

**Files:**
- Test: `web/lib/architecture.test.ts`
- Create: `web/app/header.tsx`, `web/app/use-session.ts`, `web/app/login/page.tsx`
- Modify: `web/app/layout.tsx` (add the header)
- Create (generated): `web/AGENTS.md`, `web/CLAUDE.md`

**Interfaces:**
- Consumes: `api.login` (Task 3), `saveSession/loadSession/clearSession` (Task 2)
- Produces: `useSession(): Session | null` from `app/use-session.ts`, which redirects to `/login` when there is no session and is used by Tasks 6–7; and the header, which shows `current-user` and `logout`

- [ ] **Step 1: Write the guard and watch it catch a violation**

`web/lib/architecture.test.ts`:

```ts
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import { describe, expect, it } from "vitest";

// ADR 0001 rule 6: only web/lib talks to the backend. Pages and components go through it.
const forbidden = [
  { pattern: /\bfetch\s*\(/, why: "use api from @/lib/api" },
  { pattern: /new\s+WebSocket\s*\(/, why: "use connectEvents from @/lib/socket" },
  { pattern: /\b(sessionStorage|localStorage)\b/, why: "use @/lib/session" },
];

const root = join(__dirname, "..");

function sourceFiles(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) return sourceFiles(path);
    return /\.(ts|tsx)$/.test(name) ? [path] : [];
  });
}

describe("dependency rule", () => {
  it("keeps backend calls inside web/lib", () => {
    const violations = sourceFiles(join(root, "app")).flatMap((file) => {
      const text = readFileSync(file, "utf8");
      return forbidden.filter((f) => f.pattern.test(text)).map((f) => `${relative(root, file)}: ${f.why}`);
    });

    expect(violations).toEqual([]);
  });
});
```

Create a temporary file `web/app/zz-violation.ts`:

```ts
export async function bad() {
  return fetch("/x");
}
```

Run: `cd web && npx vitest run lib/architecture.test.ts`
Expected: FAIL, with `"app/zz-violation.ts: use api from @/lib/api"` in the diff.

Delete `web/app/zz-violation.ts` and run the command again. Expected: `Tests  1 passed (1)`.

- [ ] **Step 2: Write the shell and the login page**

`web/app/use-session.ts`:

```ts
"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { loadSession, type Session } from "@/lib/session";

// useSession returns the logged-in session, or null while loading.
// Without a session it sends the browser to /login.
export function useSession(): Session | null {
  const router = useRouter();
  const [session, setSession] = useState<Session | null>(null);

  useEffect(() => {
    const s = loadSession();
    if (s) setSession(s);
    else router.replace("/login");
  }, [router]);

  return session;
}
```

`web/app/header.tsx`:

```tsx
"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { clearSession, loadSession, type Session } from "@/lib/session";

export function Header() {
  const pathname = usePathname();
  const router = useRouter();
  const [session, setSession] = useState<Session | null>(null);

  // Re-read on every navigation, because logging in happens on another page.
  useEffect(() => setSession(loadSession()), [pathname]);

  function logout() {
    clearSession();
    setSession(null);
    router.push("/login");
  }

  return (
    <header className="header">
      <Link href="/cases">Support Chat</Link>
      {session && (
        <div className="row">
          <span data-testid="current-user">
            {session.user.name} <span className="muted">({session.user.role})</span>
          </span>
          <button className="secondary" data-testid="logout" onClick={logout}>
            Log out
          </button>
        </div>
      )}
    </header>
  );
}
```

Replace `web/app/layout.tsx` with:

```tsx
import type { Metadata } from "next";
import { Header } from "./header";
import "./globals.css";

export const metadata: Metadata = {
  title: "Support Chat",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>
        <Header />
        <main>{children}</main>
      </body>
    </html>
  );
}
```

`web/app/login/page.tsx`:

```tsx
"use client";

import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";
import { api } from "@/lib/api";
import { saveSession } from "@/lib/session";
import type { Role } from "@/lib/types";

export default function LoginPage() {
  const router = useRouter();
  const [name, setName] = useState("");
  const [role, setRole] = useState<Role>("customer");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      saveSession(await api.login(name, role));
      router.push("/cases");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Login failed");
      setBusy(false);
    }
  }

  return (
    <div className="card">
      <h1>Log in</h1>
      <p className="muted">No password in this version: pick a name and a role (ADR 0005).</p>
      <form onSubmit={submit}>
        <p>
          <label>
            Name
            <input type="text" data-testid="login-name" value={name} onChange={(e) => setName(e.target.value)} autoFocus />
          </label>
        </p>
        <p className="row">
          <label>
            <input
              type="radio"
              name="role"
              data-testid="login-role-customer"
              checked={role === "customer"}
              onChange={() => setRole("customer")}
            />{" "}
            Customer
          </label>
          <label>
            <input type="radio" name="role" data-testid="login-role-agent" checked={role === "agent"} onChange={() => setRole("agent")} />{" "}
            Support agent
          </label>
        </p>
        {error && (
          <p className="error" data-testid="login-error">
            {error}
          </p>
        )}
        <button type="submit" data-testid="login-submit" disabled={busy || name.trim() === ""}>
          Log in
        </button>
      </form>
    </div>
  );
}
```

- [ ] **Step 3: Typecheck, test and build**

Run: `make test && (cd web && npx next build)`
Expected: `Tests  25 passed (25)`. The build lists `/login`.

- [ ] **Step 4: Check it in a browser**

Terminal 1: `make run-api`. Terminal 2: `make run-web`.

`next dev` writes `web/AGENTS.md` and `web/CLAUDE.md` the first time it runs. They are Next's instructions for coding agents, and the file itself asks to be committed, so commit them in Step 5.

Open `http://localhost:3100/login` and check each of these:
1. Leave the name empty. The Log in button is disabled.
2. Type `Ann`, keep "Customer" selected, and click Log in. The browser goes to `/cases`, which shows a 404 until Task 6. The header shows `Ann (customer)` and a Log out button.
3. Click Log out. You are back on `/login` and the header shows no user.
4. Stop the API (Ctrl+C in terminal 1), then try to log in. The page shows `Cannot reach the server`.

- [ ] **Step 5: Commit**

```bash
git add web/
git commit -m "feat(web): add dependency guard, header and login page

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: Case list page

**Files:**
- Create: `web/app/cases/page.tsx`

**Interfaces:**
- Consumes: `useSession` (Task 5); `api.listCases`, `api.openCase`, `api.agentEventsUrl` (Task 3); `connectEvents` (Task 4); `upsertCase` (Task 2)
- Produces: route `/cases`. Customers see a "new case" form and their own cases. Agents see every case, status filter buttons, and live updates from the agent feed.

- [ ] **Step 1: Write the page**

`web/app/cases/page.tsx`:

```tsx
"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState, type FormEvent } from "react";
import { api } from "@/lib/api";
import { upsertCase } from "@/lib/cases";
import { connectEvents } from "@/lib/socket";
import type { Case, CaseStatus } from "@/lib/types";
import { useSession } from "../use-session";

const FILTERS: { label: string; value?: CaseStatus }[] = [
  { label: "All" },
  { label: "Waiting", value: "waiting" },
  { label: "Open", value: "open" },
  { label: "Closed", value: "closed" },
];

export default function CasesPage() {
  const session = useSession();
  const [cases, setCases] = useState<Case[]>([]);
  const [filter, setFilter] = useState<CaseStatus | undefined>(undefined);
  const [error, setError] = useState<string | null>(null);
  const isAgent = session?.user.role === "agent";

  useEffect(() => {
    if (!session) return;
    let active = true;
    api
      .listCases(filter)
      .then((list) => active && setCases(list))
      .catch((err: Error) => active && setError(err.message));
    return () => {
      active = false;
    };
  }, [session, filter]);

  // Agents see new cases and status changes live.
  useEffect(() => {
    if (!isAgent) return;
    return connectEvents({
      url: api.agentEventsUrl,
      onEvent: (e) => {
        if (e.type === "case.created" || e.type === "case.status_changed") {
          setCases((list) => upsertCase(list, e.data, filter));
        }
      },
    });
  }, [isAgent, filter]);

  if (!session) return null;

  return (
    <>
      {!isAgent && <NewCaseForm />}
      <div className="card">
        <h1>{isAgent ? "All cases" : "My cases"}</h1>
        {isAgent && (
          <p className="row">
            {FILTERS.map((f) => (
              <button
                key={f.label}
                className={filter === f.value ? "" : "secondary"}
                data-testid={`status-filter-${f.value ?? "all"}`}
                onClick={() => setFilter(f.value)}
              >
                {f.label}
              </button>
            ))}
          </p>
        )}
        {error && <p className="error">{error}</p>}
        {cases.length === 0 ? (
          <p className="muted" data-testid="case-list-empty">
            No cases yet.
          </p>
        ) : (
          <ul className="case-list" data-testid="case-list">
            {cases.map((c) => (
              <li key={c.id} data-testid="case-item" data-case-id={c.id}>
                <Link href={`/cases/${c.id}`}>
                  <span data-testid="case-subject">{c.subject}</span>
                  <span className="status" data-testid="case-status">
                    {c.status}
                  </span>
                </Link>
              </li>
            ))}
          </ul>
        )}
      </div>
    </>
  );
}

function NewCaseForm() {
  const router = useRouter();
  const [question, setQuestion] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const { case: opened } = await api.openCase(question);
      router.push(`/cases/${opened.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not open the case");
      setBusy(false);
    }
  }

  return (
    <form className="card" onSubmit={submit}>
      <h1>Ask the support team</h1>
      <textarea
        rows={3}
        data-testid="new-case-question"
        placeholder="What do you need help with?"
        value={question}
        onChange={(e) => setQuestion(e.target.value)}
      />
      {error && <p className="error">{error}</p>}
      <p>
        <button type="submit" data-testid="new-case-submit" disabled={busy || question.trim() === ""}>
          Send question
        </button>
      </p>
    </form>
  );
}
```

- [ ] **Step 2: Typecheck, test and build**

Run: `make test && (cd web && npx next build)`
Expected: `Tests  25 passed (25)`. The build lists `/cases`.

- [ ] **Step 3: Check it in a browser (two tabs)**

With `make run-api` and `make run-web` running:
1. Tab A: log in as customer `Ann`. `/cases` shows "Ask the support team" and "No cases yet."
2. Tab B (a new tab, which has its own session): log in as agent `Bob`. It shows "All cases", the filter buttons All/Waiting/Open/Closed, and "No cases yet." (or older cases, if the database has some).
3. Tab A: type `How do I reset my password?` and click "Send question". Tab A moves to `/cases/<id>`, which is a 404 until Task 7.
4. Tab B, **without refreshing**: the new case appears at the top with status `waiting`. That is the `case.created` event.
5. Tab B: click "Closed". The list no longer shows the waiting case. Click "All" and it is back.
6. Tab A: go to `/cases`. It lists only Ann's case. Log in as customer `Cat` in a third tab and `/cases` shows "No cases yet."

- [ ] **Step 4: Commit**

```bash
git add web/app/cases/page.tsx
git commit -m "feat(web): add case list with new-case form and live agent feed

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: Case room page

**Files:**
- Create: `web/app/cases/[id]/page.tsx`

**Interfaces:**
- Consumes: `useSession`; `api.getCase/listMessages/joinCase/closeCase/sendMessage/caseEventsUrl`, `ApiError`; `connectEvents`, `SocketStatus`; `mergeMessages`
- Produces: route `/cases/[id]`, with the `data-testid`s listed in Global Constraints

- [ ] **Step 1: Write the page**

`web/app/cases/[id]/page.tsx`:

```tsx
"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import { api, ApiError } from "@/lib/api";
import { mergeMessages } from "@/lib/messages";
import { connectEvents, type SocketStatus } from "@/lib/socket";
import type { Case, Message } from "@/lib/types";
import { useSession } from "../../use-session";

const PAGE_SIZE = 50;

export default function CaseRoomPage() {
  const { id } = useParams<{ id: string }>();
  const session = useSession();
  const [kase, setCase] = useState<Case | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [hasOlder, setHasOlder] = useState(false);
  const [socketStatus, setSocketStatus] = useState<SocketStatus>("connecting");
  const [loadError, setLoadError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const stopSocket = useRef<(() => void) | null>(null);
  const firstLoad = useRef(true);

  // refresh loads the case and the newest messages, and merges them with what is on screen.
  // It runs on open and after every reconnect, to pick up anything missed while disconnected.
  const refresh = useCallback(async () => {
    try {
      const [c, latest] = await Promise.all([api.getCase(id), api.listMessages(id, { limit: PAGE_SIZE })]);
      setCase(c);
      if (firstLoad.current) {
        firstLoad.current = false;
        setHasOlder(latest.length === PAGE_SIZE);
      }
      setMessages((current) => mergeMessages(current, latest));
    } catch (err) {
      if (err instanceof ApiError && (err.status === 403 || err.status === 404)) {
        setLoadError(err.status === 403 ? "You cannot view this case." : "This case does not exist.");
        stopSocket.current?.();
      }
    }
  }, [id]);

  useEffect(() => {
    if (!session) return;
    void refresh();
    const stop = connectEvents({
      url: () => api.caseEventsUrl(id),
      onStatus: (s) => {
        setSocketStatus(s);
        if (s === "open") void refresh();
      },
      onEvent: (e) => {
        if (e.type === "message.created") setMessages((current) => mergeMessages(current, [e.data]));
        if (e.type === "case.closed") setCase(e.data);
        if (e.type === "participant.joined") void api.getCase(id).then(setCase, () => {});
      },
    });
    stopSocket.current = stop;
    return stop;
  }, [session, id, refresh]);

  async function loadOlder() {
    const older = await api.listMessages(id, { before: messages[0]?.id, limit: PAGE_SIZE });
    setHasOlder(older.length === PAGE_SIZE);
    setMessages((current) => mergeMessages(current, older));
  }

  async function run(action: () => Promise<Case>) {
    setActionError(null);
    try {
      setCase(await action());
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Something went wrong");
    }
  }

  if (!session) return null;
  if (loadError) {
    return (
      <div className="card">
        <p className="error" data-testid="room-error">
          {loadError}
        </p>
        <Link href="/cases">Back to cases</Link>
      </div>
    );
  }
  if (!kase) return <p className="muted">Loading…</p>;

  const me = session.user;
  const isAgent = me.role === "agent";
  const joined = kase.participants.some((p) => p.userId === me.id);
  const closed = kase.status === "closed";

  return (
    <div className="card">
      <p>
        <Link href="/cases">← Cases</Link>
      </p>
      <h1 data-testid="case-subject">{kase.subject}</h1>
      <p className="row">
        <span className="status" data-testid="case-status">
          {kase.status}
        </span>
        <span className="muted" data-testid="participants">
          {kase.participants.map((p) => p.name).join(", ")}
        </span>
      </p>

      {socketStatus === "reconnecting" && (
        <p className="banner" data-testid="reconnecting-banner">
          Reconnecting…
        </p>
      )}
      {kase.status === "waiting" && !isAgent && <p className="banner">Waiting for a support agent to join…</p>}
      {closed && (
        <p className="banner" data-testid="closed-banner">
          This case is closed.
        </p>
      )}

      <div className="row">
        {isAgent && !joined && !closed && (
          <button data-testid="join-button" onClick={() => run(() => api.joinCase(id))}>
            Join case
          </button>
        )}
        {isAgent && joined && !closed && (
          <button className="secondary" data-testid="close-button" onClick={() => run(() => api.closeCase(id))}>
            Close case
          </button>
        )}
      </div>
      {actionError && <p className="error">{actionError}</p>}

      {hasOlder && (
        <p>
          <button className="secondary" data-testid="load-older" onClick={loadOlder}>
            Load older messages
          </button>
        </p>
      )}
      <div className="messages" data-testid="message-list">
        {messages.map((m) => (
          <div
            key={m.id}
            className={`message ${m.kind === "system" ? "system" : m.senderId === me.id ? "mine" : ""}`}
            data-testid="message"
            data-kind={m.kind}
          >
            {m.kind === "text" && <div className="muted">{m.senderName}</div>}
            <span data-testid="message-body">{m.body}</span>
          </div>
        ))}
      </div>

      {joined && !closed && (
        <SendBox caseId={id} onSent={(m) => setMessages((current) => mergeMessages(current, [m]))} />
      )}
    </div>
  );
}

// SendBox keeps the text when sending fails and offers Retry (spec section 8).
function SendBox({ caseId, onSent }: { caseId: string; onSent: (m: Message) => void }) {
  const [draft, setDraft] = useState("");
  const [sending, setSending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function send(e?: FormEvent) {
    e?.preventDefault();
    if (draft.trim() === "") return;
    setSending(true);
    setError(null);
    try {
      onSent(await api.sendMessage(caseId, draft));
      setDraft("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not send");
    } finally {
      setSending(false);
    }
  }

  return (
    <form onSubmit={send}>
      <div className="row">
        <input
          type="text"
          data-testid="message-input"
          placeholder="Write a message"
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          style={{ flex: 1 }}
        />
        <button type="submit" data-testid="message-send" disabled={sending || draft.trim() === ""}>
          Send
        </button>
      </div>
      {error && (
        <p className="row error" data-testid="send-error">
          Not sent: {error}
          <button type="button" className="secondary" data-testid="send-retry" onClick={() => send()}>
            Retry
          </button>
        </p>
      )}
    </form>
  );
}
```

- [ ] **Step 2: Typecheck, test and build**

Run: `make test && (cd web && npx next build)`
Expected: `Tests  25 passed (25)`. The build lists `ƒ /cases/[id]`.

- [ ] **Step 3: Real-time conversation (user stories 1–3)**

With `make run-api` and `make run-web` running, tab A is customer `Ann` and tab B is agent `Bob`:
1. Tab A: open a case with `ลืมรหัสผ่าน ทำยังไงดีครับ`. The room shows the Thai subject, status `waiting`, "Waiting for a support agent to join…", Ann's message on the right, and a message box.
2. Tab B: open the case from the list. It shows a "Join case" button and no message box. Click Join case.
3. Tab A, **without refreshing**: the status becomes `open`, the participants line reads `Ann, Bob`, and "Bob joined the case" appears as a centred grey line.
4. Tab B: type a reply and press Enter. It appears once in tab B and once in tab A, without refreshing. There must be no duplicate in tab B, even though it gets both the POST response and the event.
5. Tab A: reply `ได้แล้ว ขอบคุณครับ`. It appears in tab B.

- [ ] **Step 4: History and closing (user stories 4–5)**

1. Tab A: reload the page. The whole conversation is still there, in the same order.
2. Tab B: click "Close case". Both tabs, without refreshing, show `closed`, the banner "This case is closed.", and "Case closed by Bob". The message box disappears from both.

- [ ] **Step 5: Reconnect (Review Focus 2)**

1. Open a new case as Ann in tab A, and join it as Bob in tab B.
2. Stop the API (Ctrl+C). Within a few seconds both tabs show "Reconnecting…".
3. Start the API again with `make run-api`. Within about 30 seconds the banner disappears by itself, with no reload.
4. Send a message from tab B. It appears in tab A once, and no earlier message is duplicated.

- [ ] **Step 6: Failed send and Retry (Review Focus 3)**

1. In tab A (Ann, joined case, open), stop the API, type `still there?` and press Enter. "Not sent: Cannot reach the server" and a Retry button appear, and the text stays in the box.
2. Start the API and click Retry once the banner is gone. The message is sent, the error disappears and the box empties.

- [ ] **Step 7: Someone else's case (user story 6, Review Focus 5)**

1. Copy tab A's room URL. In a new tab, log in as customer `Cat` and paste the URL. The page shows "You cannot view this case." and a "Back to cases" link.
2. In the browser devtools Network tab, filtered to WS, there is at most one failed attempt for `/events` and no new ones over the next 10 seconds. The socket was stopped.
3. `http://localhost:3100/cases/does-not-exist` shows "This case does not exist."

- [ ] **Step 8: Commit**

```bash
git add "web/app/cases/[id]/page.tsx"
git commit -m "feat(web): add real-time case room with reconnect and retry

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

## After this plan

- **Plan 3: E2E** (spec step 7). Cucumber + Playwright + Page Objects (`LoginPage`, `CaseListPage`, `CaseRoomPage`) over the `data-testid`s above, with one declarative feature per user story and two browser contexts for the real-time scenarios. The manual checks in Tasks 5–7 are the scenarios to automate.
