import type { Case, CaseStatus } from "./types";

// upsertCase puts a new or changed case into a case list, newest updatedAt first.
// With a status filter, a case that no longer matches it leaves the list.
export function upsertCase(list: Case[], changed: Case, statusFilter?: CaseStatus): Case[] {
  const others = list.filter((c) => c.id !== changed.id);
  const next = !statusFilter || changed.status === statusFilter ? [...others, changed] : others;
  return next.sort((a, b) => (a.updatedAt < b.updatedAt ? 1 : a.updatedAt > b.updatedAt ? -1 : 0));
}
