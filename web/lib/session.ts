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
