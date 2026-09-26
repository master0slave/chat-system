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
