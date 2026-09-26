"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState, type FormEvent } from "react";
import { api } from "@/lib/api";
import { upsertCase } from "@/lib/cases";
import { connectEvents } from "@/lib/socket";
import type { Case, CaseStatus } from "@/lib/types";
import { useSession } from "../use-session";

const FILTERS: { label: string; value?: CaseStatus }[] = [
  { label: "All" },
  { label: "Waiting", value: "waiting" },
  { label: "Open", value: "open" },
  { label: "Closed", value: "closed" },
];

export default function CasesPage() {
  const session = useSession();
  const [cases, setCases] = useState<Case[]>([]);
  const [filter, setFilter] = useState<CaseStatus | undefined>(undefined);
  const [error, setError] = useState<string | null>(null);
  const isAgent = session?.user.role === "agent";

  useEffect(() => {
    if (!session) return;
    let active = true;
    api
      .listCases(filter)
      .then((list) => active && setCases(list))
      .catch((err: Error) => active && setError(err.message));
    return () => {
      active = false;
    };
  }, [session, filter]);

  // Agents see new cases and status changes live.
  useEffect(() => {
    if (!isAgent) return;
    return connectEvents({
      url: api.agentEventsUrl,
      onEvent: (e) => {
        if (e.type === "case.created" || e.type === "case.status_changed") {
          setCases((list) => upsertCase(list, e.data, filter));
        }
      },
    });
  }, [isAgent, filter]);

  if (!session) return null;

  return (
    <>
      {!isAgent && <NewCaseForm />}
      <div className="card">
        <h1>{isAgent ? "All cases" : "My cases"}</h1>
        {isAgent && (
          <p className="row">
            {FILTERS.map((f) => (
              <button
                key={f.label}
                className={filter === f.value ? "" : "secondary"}
                data-testid={`status-filter-${f.value ?? "all"}`}
                onClick={() => setFilter(f.value)}
              >
                {f.label}
              </button>
            ))}
          </p>
        )}
        {error && <p className="error">{error}</p>}
        {cases.length === 0 ? (
          <p className="muted" data-testid="case-list-empty">
            No cases yet.
          </p>
        ) : (
          <ul className="case-list" data-testid="case-list">
            {cases.map((c) => (
              <li key={c.id} data-testid="case-item" data-case-id={c.id}>
                <Link href={`/cases/${c.id}`}>
                  <span data-testid="case-subject">{c.subject}</span>
                  <span className="status" data-testid="case-status">
                    {c.status}
                  </span>
                </Link>
              </li>
            ))}
          </ul>
        )}
      </div>
    </>
  );
}

function NewCaseForm() {
  const router = useRouter();
  const [question, setQuestion] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const { case: opened } = await api.openCase(question);
      router.push(`/cases/${opened.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not open the case");
      setBusy(false);
    }
  }

  return (
    <form className="card" onSubmit={submit}>
      <h1>Ask the support team</h1>
      <textarea
        rows={3}
        data-testid="new-case-question"
        placeholder="What do you need help with?"
        value={question}
        onChange={(e) => setQuestion(e.target.value)}
      />
      {error && <p className="error">{error}</p>}
      <p>
        <button type="submit" data-testid="new-case-submit" disabled={busy || question.trim() === ""}>
          Send question
        </button>
      </p>
    </form>
  );
}
