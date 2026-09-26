import { connectEvents, type SocketLike, type SocketStatus } from "./socket";
import type { Case, CaseStatus } from "./types";

// upsertCase puts a new or changed case into a case list, newest updatedAt first.
// An older copy of a case never replaces a newer one, so a late response cannot undo an event.
// With a status filter, a case that no longer matches it leaves the list.
export function upsertCase(list: Case[], changed: Case, statusFilter?: CaseStatus): Case[] {
  const current = list.find((c) => c.id === changed.id);
  if (current && current.updatedAt > changed.updatedAt) return list;
  const others = list.filter((c) => c.id !== changed.id);
  const next = !statusFilter || changed.status === statusFilter ? [...others, changed] : others;
  return next.sort((a, b) => (a.updatedAt < b.updatedAt ? 1 : a.updatedAt > b.updatedAt ? -1 : 0));
}

export interface WatchCasesOptions {
  filter?: CaseStatus;
  load: () => Promise<Case[]>;
  url: () => string;
  onCases: (cases: Case[]) => void;
  onStatus?: (status: SocketStatus) => void;
  onError?: (err: unknown) => void;
  createSocket?: (url: string) => SocketLike;
}

// watchCases keeps an agent's case list current: it loads the list at start and after every
// (re)connect, because events sent while disconnected are lost (ADR 0003), and applies
// case.created / case.status_changed events in between. Events that arrive while a load is
// in flight are applied on top of its result, so they are not lost either.
export function watchCases(opts: WatchCasesOptions): () => void {
  let cases: Case[] = [];
  let arrivedDuringLoad: Case[] = [];
  let loading = 0;
  let stopped = false;

  function apply(changed: Case) {
    cases = upsertCase(cases, changed, opts.filter);
    opts.onCases(cases);
  }

  async function reload() {
    loading++;
    try {
      const list = await opts.load();
      if (stopped) return;
      cases = list;
      for (const c of arrivedDuringLoad) cases = upsertCase(cases, c, opts.filter);
      opts.onCases(cases);
    } catch (err) {
      if (!stopped) opts.onError?.(err);
    } finally {
      if (--loading === 0) arrivedDuringLoad = [];
    }
  }

  void reload();
  const stopSocket = connectEvents({
    url: opts.url,
    createSocket: opts.createSocket,
    onStatus: (s) => {
      opts.onStatus?.(s);
      if (s === "open") void reload();
    },
    onEvent: (e) => {
      if (e.type !== "case.created" && e.type !== "case.status_changed") return;
      if (loading > 0) arrivedDuringLoad.push(e.data);
      apply(e.data);
    },
  });

  return () => {
    stopped = true;
    stopSocket();
  };
}
