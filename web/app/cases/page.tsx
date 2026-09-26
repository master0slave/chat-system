"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState, type FormEvent } from "react";
import { api } from "@/lib/api";
import { customerName, filterCases, summarizeCases } from "@/lib/case-inbox";
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
  const [query, setQuery] = useState("");
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

  // Agents keep the full live feed so the status summaries remain accurate while filtering.
  useEffect(() => {
    if (role !== "agent") return;
    setError(null);
    return watchCases({
      load: () => api.listCases(),
      url: api.agentEventsUrl,
      onCases: (list) => {
        setError(null);
        setCases(list);
      },
      onStatus: setSocketStatus,
      onError: (err) => setError(err instanceof Error ? err.message : "Could not load cases"),
    });
  }, [role]);

  const isAgent = role === "agent";
  const summary = summarizeCases(cases);
  const visibleCases = filterCases(cases, filter, query);

  if (!session) return null;

  return (
    <>
      {!isAgent && <NewCaseForm />}
      {isAgent ? (
        <section className="agent-inbox" aria-labelledby="inbox-title">
          <div className="inbox-heading">
            <div>
              <p className="eyebrow">SUPPORT WORKSPACE</p>
              <h1 id="inbox-title">Agent inbox</h1>
              <p className="muted">Triage customer cases and continue conversations.</p>
            </div>
            <span className={`connection-state ${socketStatus}`} aria-live="polite">
              <span className="connection-dot" />
              {socketStatus === "open" ? "Live updates on" : socketStatus === "reconnecting" ? "Reconnecting…" : "Connecting…"}
            </span>
          </div>

          <div className="inbox-stats" aria-label="Case totals">
            <SummaryCard label="All cases" count={summary.all} selected={!filter} onClick={() => setFilter(undefined)} />
            <SummaryCard label="Waiting" count={summary.waiting} selected={filter === "waiting"} onClick={() => setFilter("waiting")} />
            <SummaryCard label="Open" count={summary.open} selected={filter === "open"} onClick={() => setFilter("open")} />
            <SummaryCard label="Closed" count={summary.closed} selected={filter === "closed"} onClick={() => setFilter("closed")} />
          </div>

          <div className="inbox-panel">
            <div className="inbox-toolbar">
              <div>
                <h2>Cases</h2>
                <p className="muted">{visibleCases.length} {visibleCases.length === 1 ? "conversation" : "conversations"}</p>
              </div>
              <label className="search-field">
                <span aria-hidden="true" className="search-icon">⌕</span>
                <span className="sr-only">Search by subject or customer</span>
                <input
                  type="search"
                  data-testid="case-search"
                  placeholder="Search cases or customers"
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                />
              </label>
            </div>

            <div className="inbox-filters" aria-label="Filter cases by status">
              {FILTERS.map((f) => (
                <button
                  key={f.label}
                  className={`filter-tab ${filter === f.value ? "active" : ""}`}
                  data-testid={`status-filter-${f.value ?? "all"}`}
                  aria-pressed={filter === f.value}
                  onClick={() => setFilter(f.value)}
                >
                  {f.label}
                  <span>{f.value ? summary[f.value] : summary.all}</span>
                </button>
              ))}
            </div>

            {error && <p className="error inbox-error" role="alert">{error}</p>}
            {cases.length === 0 ? (
              <div className="inbox-empty" data-testid="case-list-empty">
                <span className="empty-icon" aria-hidden="true">✉</span>
                <h3>No cases yet</h3>
                <p>New customer conversations will appear here automatically.</p>
              </div>
            ) : visibleCases.length === 0 ? (
              <div className="inbox-empty" data-testid="case-search-empty">
                <span className="empty-icon" aria-hidden="true">⌕</span>
                <h3>No matching cases</h3>
                <p>Try another search or status filter.</p>
              </div>
            ) : (
              <ul className="case-list inbox-case-list" data-testid="case-list">
                {visibleCases.map((c) => (
                  <li key={c.id} data-testid="case-item" data-case-id={c.id}>
                    <Link href={`/cases/${c.id}`} className="case-row">
                      <span className={`case-avatar ${c.status}`} aria-hidden="true">{customerName(c).slice(0, 1).toUpperCase()}</span>
                      <span className="case-copy">
                        <span className="case-row-heading">
                          <strong data-testid="case-subject">{c.subject}</strong>
                          <time dateTime={c.updatedAt}>{formatDate(c.updatedAt)}</time>
                        </span>
                        <span className="case-row-meta">
                          <span data-testid="case-customer">{customerName(c)}</span>
                          <span className="meta-separator" aria-hidden="true">·</span>
                          <span>{c.participants.filter((person) => person.role === "agent").length} agent{c.participants.filter((person) => person.role === "agent").length === 1 ? "" : "s"}</span>
                        </span>
                      </span>
                      <span className={`status status-${c.status}`} data-testid="case-status">{c.status}</span>
                    </Link>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </section>
      ) : (
        <div className="card">
          <h1>My cases</h1>
          {error && <p className="error">{error}</p>}
          {cases.length === 0 ? (
            <p className="muted" data-testid="case-list-empty">No cases yet.</p>
          ) : (
            <ul className="case-list" data-testid="case-list">
              {cases.map((c) => (
                <li key={c.id} data-testid="case-item" data-case-id={c.id}>
                  <Link href={`/cases/${c.id}`}>
                    <span data-testid="case-subject">{c.subject}</span>
                    <span className="status" data-testid="case-status">{c.status}</span>
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </>
  );
}

function SummaryCard({ label, count, selected, onClick }: { label: string; count: number; selected: boolean; onClick: () => void }) {
  return (
    <button className={`summary-card ${selected ? "selected" : ""}`} onClick={onClick} aria-pressed={selected}>
      <span>{label}</span>
      <strong>{count}</strong>
    </button>
  );
}

function formatDate(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  return new Intl.DateTimeFormat("en", { month: "short", day: "numeric", hour: "numeric", minute: "2-digit" }).format(date);
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
