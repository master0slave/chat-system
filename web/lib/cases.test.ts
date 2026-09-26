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
