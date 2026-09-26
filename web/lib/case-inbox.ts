import type { Case, CaseStatus } from "./types";

export interface CaseSummary {
  all: number;
  waiting: number;
  open: number;
  closed: number;
}

export function summarizeCases(cases: Case[]): CaseSummary {
  return cases.reduce<CaseSummary>(
    (summary, kase) => {
      summary.all += 1;
      summary[kase.status] += 1;
      return summary;
    },
    { all: 0, waiting: 0, open: 0, closed: 0 },
  );
}

export function filterCases(cases: Case[], status: CaseStatus | undefined, query: string): Case[] {
  const normalizedQuery = query.trim().toLocaleLowerCase();
  return cases.filter((kase) => {
    if (status && kase.status !== status) return false;
    if (!normalizedQuery) return true;

    const customerNames = kase.participants.filter((person) => person.role === "customer").map((person) => person.name);
    return [kase.subject, ...customerNames].some((value) => value.toLocaleLowerCase().includes(normalizedQuery));
  });
}

export function customerName(kase: Case): string {
  return kase.participants.find((person) => person.role === "customer")?.name ?? "Customer";
}
