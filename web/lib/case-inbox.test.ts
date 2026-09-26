import { describe, expect, it } from "vitest";
import { filterCases, summarizeCases } from "./case-inbox";
import type { Case } from "./types";

function makeCase(overrides: Partial<Case> & Pick<Case, "id" | "subject">): Case {
  const { id, subject, ...changes } = overrides;
  return {
    id,
    subject,
    customerId: "customer-1",
    participants: [
      { userId: "customer-1", name: "Ann Customer", role: "customer", joinedAt: "2026-09-26T09:00:00.000Z" },
    ],
    status: "waiting",
    createdAt: "2026-09-26T09:00:00.000Z",
    updatedAt: "2026-09-26T09:00:00.000Z",
    ...changes,
  };
}

describe("case inbox", () => {
  const cases = [
    makeCase({ id: "1", subject: "Cannot reset password" }),
    makeCase({ id: "2", subject: "Billing question", status: "open", participants: [
      { userId: "customer-2", name: "Bee Customer", role: "customer", joinedAt: "2026-09-26T09:00:00.000Z" },
      { userId: "agent-1", name: "Agent One", role: "agent", joinedAt: "2026-09-26T09:10:00.000Z" },
    ] }),
    makeCase({ id: "3", subject: "Export data", status: "closed" }),
  ];

  it("counts cases by status across the whole inbox", () => {
    expect(summarizeCases(cases)).toEqual({ all: 3, waiting: 1, open: 1, closed: 1 });
  });

  it("filters by status and searches subject or customer name", () => {
    expect(filterCases(cases, "waiting", "PASSWORD").map((item) => item.id)).toEqual(["1"]);
    expect(filterCases(cases, undefined, "bee customer").map((item) => item.id)).toEqual(["2"]);
  });
});
