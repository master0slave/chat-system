"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState, type FormEvent } from "react";
import { api } from "@/lib/api";
import { watchCases } from "@/lib/cases";
import type { SocketStatus } from "@/lib/socket";
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
  const [socketStatus, setSocketStatus] = useState<SocketStatus>("connecting");
  const role = session?.user.role;

  // Customers load their own cases once. They have no live feed (docs/events.md).
  useEffect(() => {
    if (role !== "customer") return;
    let active = true;
    api
      .listCases()
      .then((list) => active && setCases(list))
      .catch((err: Error) => active && setError(err.message));
    return () => {
      active = false;
    };
  }, [role]);

  // Agents see new cases and status changes live, and the list reloads after every reconnect.
  useEffect(() => {
    if (role !== "agent") return;
    setError(null);
    return watchCases({
      filter,
      load: () => api.listCases(filter),
      url: api.agentEventsUrl,
      onCases: (list) => {
        setError(null);
        setCases(list);
      },
      onStatus: setSocketStatus,
      onError: (err) => setError(err instanceof Error ? err.message : "Could not load cases"),
    });
  }, [role, filter]);

  const isAgent = role === "agent";

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
        {isAgent && socketStatus === "reconnecting" && (
          <p className="banner" data-testid="reconnecting-banner">
            Reconnecting…
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
