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
