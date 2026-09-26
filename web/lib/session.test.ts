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
