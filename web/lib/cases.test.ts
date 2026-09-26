import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { upsertCase, watchCases } from "./cases";
import type { SocketLike, SocketStatus } from "./socket";
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

  it("never replaces a case with an older copy of it", () => {
    const list = [kase("a", "open", "2026-09-26T09:05:00.000Z")];

    const next = upsertCase(list, kase("a", "waiting", "2026-09-26T09:00:00.000Z"));

    expect(next.map((c) => c.status)).toEqual(["open"]);
  });

  it("ignores a new case that does not match the status filter", () => {
    expect(upsertCase([], kase("a", "closed", "2026-09-26T09:00:00.000Z"), "waiting")).toEqual([]);
  });
});

class FakeSocket implements SocketLike {
  onopen: ((ev: Event) => void) | null = null;
  onmessage: ((ev: MessageEvent) => void) | null = null;
  onclose: ((ev: CloseEvent) => void) | null = null;
  close() {}
  open() {
    this.onopen?.(new Event("open"));
  }
  send(event: unknown) {
    this.onmessage?.({ data: JSON.stringify(event) } as MessageEvent);
  }
  drop() {
    this.onclose?.({} as CloseEvent);
  }
}

// deferred lets a test decide when a list request finishes.
function deferred<T>() {
  let resolve!: (v: T) => void;
  const promise = new Promise<T>((r) => (resolve = r));
  return { promise, resolve };
}

function setupWatch(filter?: CaseStatus) {
  const sockets: FakeSocket[] = [];
  const loads: ReturnType<typeof deferred<Case[]>>[] = [];
  const seen: Case[][] = [];
  const statuses: SocketStatus[] = [];
  const stop = watchCases({
    filter,
    load: () => {
      const d = deferred<Case[]>();
      loads.push(d);
      return d.promise;
    },
    url: () => "ws://api/v1/cases/events?token=t",
    onCases: (cases) => seen.push(cases),
    onStatus: (s) => statuses.push(s),
    createSocket: () => {
      const s = new FakeSocket();
      sockets.push(s);
      return s;
    },
  });
  const latest = () => (seen.at(-1) ?? []).map((c) => c.id);
  return { sockets, loads, statuses, latest, stop };
}

const flush = () => new Promise((r) => setTimeout(r, 0));

describe("watchCases", () => {
  beforeEach(() => vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"], shouldAdvanceTime: true }));
  afterEach(() => vi.useRealTimers());

  it("loads the list at start and again after every reconnect", async () => {
    const w = setupWatch();
    w.loads[0].resolve([kase("a", "waiting", "2026-09-26T09:00:00.000Z")]);
    await flush();
    expect(w.latest()).toEqual(["a"]);

    w.sockets[0].open();
    expect(w.loads).toHaveLength(2);
    w.loads[1].resolve([kase("a", "waiting", "2026-09-26T09:00:00.000Z")]);
    await flush();

    // A case is opened while the agent is disconnected: its event is lost...
    w.sockets[0].drop();
    vi.advanceTimersByTime(1000);
    w.sockets[1].open();
    // ...but the reload after reconnecting finds it.
    expect(w.loads).toHaveLength(3);
    w.loads[2].resolve([kase("b", "waiting", "2026-09-26T09:03:00.000Z"), kase("a", "waiting", "2026-09-26T09:00:00.000Z")]);
    await flush();

    expect(w.latest()).toEqual(["b", "a"]);
    expect(w.statuses).toContain("reconnecting");
    w.stop();
  });

  it("keeps an event that arrives while the list is loading", async () => {
    const w = setupWatch();
    w.sockets[0].open();

    w.sockets[0].send({ type: "case.created", caseId: "new", data: kase("new", "waiting", "2026-09-26T09:05:00.000Z") });
    w.loads[0].resolve([kase("old", "waiting", "2026-09-26T09:00:00.000Z")]);
    w.loads[1].resolve([kase("old", "waiting", "2026-09-26T09:00:00.000Z")]);
    await flush();

    expect(w.latest()).toEqual(["new", "old"]);
    w.stop();
  });

  it("applies the status filter to live events", async () => {
    const w = setupWatch("waiting");
    w.loads[0].resolve([kase("a", "waiting", "2026-09-26T09:00:00.000Z")]);
    await flush();

    w.sockets[0].send({ type: "case.status_changed", caseId: "a", data: kase("a", "open", "2026-09-26T09:01:00.000Z") });

    expect(w.latest()).toEqual([]);
    w.stop();
  });

  it("ignores a load that finishes after stop", async () => {
    const w = setupWatch();
    const before = w.latest();
    w.stop();

    w.loads[0].resolve([kase("a", "waiting", "2026-09-26T09:00:00.000Z")]);
    await flush();

    expect(w.latest()).toEqual(before);
  });
});
