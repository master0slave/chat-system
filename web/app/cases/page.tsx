"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState, type FormEvent } from "react";
import { Activity, Inbox, Search } from "lucide-react";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
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
            <Badge variant={socketStatus === "open" ? "secondary" : "outline"} className="connection-state" aria-live="polite">
              {socketStatus === "connecting" && <Spinner data-icon="inline-start" />}
              <Activity data-icon="inline-start" />
              {socketStatus === "open" ? "Live updates on" : socketStatus === "reconnecting" ? "Reconnecting…" : "Connecting…"}
            </Badge>
          </div>

          <div className="inbox-stats" aria-label="Case totals">
            <SummaryCard label="All cases" count={summary.all} selected={!filter} onClick={() => setFilter(undefined)} />
            <SummaryCard label="Waiting" count={summary.waiting} selected={filter === "waiting"} onClick={() => setFilter("waiting")} />
            <SummaryCard label="Open" count={summary.open} selected={filter === "open"} onClick={() => setFilter("open")} />
            <SummaryCard label="Closed" count={summary.closed} selected={filter === "closed"} onClick={() => setFilter("closed")} />
          </div>

          <Card className="inbox-panel">
            <div className="inbox-toolbar">
              <div>
                <h2>Cases</h2>
                <p className="muted">{visibleCases.length} {visibleCases.length === 1 ? "conversation" : "conversations"}</p>
              </div>
              <label className="search-field">
                <Search aria-hidden="true" />
                <span className="sr-only">Search by subject or customer</span>
                <Input
                  type="search"
                  data-testid="case-search"
                  placeholder="Search cases or customers"
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                />
              </label>
            </div>

            <Tabs
              value={filter ?? "all"}
              onValueChange={(value) => setFilter(value === "all" ? undefined : (value as CaseStatus))}
            >
              <TabsList variant="line" className="inbox-filters" aria-label="Filter cases by status">
                {FILTERS.map((f) => (
                <TabsTrigger
                  key={f.label}
                  data-testid={`status-filter-${f.value ?? "all"}`}
                  value={f.value ?? "all"}
                >
                  {f.label}
                  <Badge variant="secondary">{f.value ? summary[f.value] : summary.all}</Badge>
                </TabsTrigger>
                ))}
              </TabsList>
            </Tabs>

            {error && <Alert variant="destructive" className="inbox-error"><AlertTitle>Could not load cases</AlertTitle><AlertDescription>{error}</AlertDescription></Alert>}
            {cases.length === 0 ? (
              <Empty className="inbox-empty" data-testid="case-list-empty">
                <EmptyHeader>
                  <EmptyMedia variant="icon"><Inbox /></EmptyMedia>
                  <EmptyTitle>No cases yet</EmptyTitle>
                  <EmptyDescription>New customer conversations will appear here automatically.</EmptyDescription>
                </EmptyHeader>
              </Empty>
            ) : visibleCases.length === 0 ? (
              <Empty className="inbox-empty" data-testid="case-search-empty">
                <EmptyHeader>
                  <EmptyMedia variant="icon"><Search /></EmptyMedia>
                  <EmptyTitle>No matching cases</EmptyTitle>
                  <EmptyDescription>Try another search or status filter.</EmptyDescription>
                </EmptyHeader>
              </Empty>
            ) : (
              <ul className="case-list inbox-case-list" data-testid="case-list">
                {visibleCases.map((c) => (
                  <li key={c.id} data-testid="case-item" data-case-id={c.id}>
                    <Link href={`/cases/${c.id}`} className="case-row">
                      <Avatar className={`case-avatar ${c.status}`} aria-hidden="true"><AvatarFallback>{customerName(c).slice(0, 1).toUpperCase()}</AvatarFallback></Avatar>
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
                      <Badge variant={c.status === "waiting" ? "outline" : c.status === "open" ? "secondary" : "ghost"} className={`status status-${c.status}`} data-testid="case-status">{c.status}</Badge>
                    </Link>
                  </li>
                ))}
              </ul>
            )}
          </Card>
        </section>
      ) : (
        <Card className="my-cases-card">
          <CardHeader><CardTitle>My cases</CardTitle></CardHeader>
          <CardContent>
          {error && <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}
          {cases.length === 0 ? (
            <p className="muted" data-testid="case-list-empty">No cases yet.</p>
          ) : (
            <ul className="case-list" data-testid="case-list">
              {cases.map((c) => (
                <li key={c.id} data-testid="case-item" data-case-id={c.id}>
                  <Link href={`/cases/${c.id}`} className="customer-case-row">
                    <span data-testid="case-subject">{c.subject}</span>
                    <Badge variant={c.status === "waiting" ? "outline" : c.status === "open" ? "secondary" : "ghost"} data-testid="case-status">{c.status}</Badge>
                  </Link>
                </li>
              ))}
            </ul>
          )}
          </CardContent>
        </Card>
      )}
    </>
  );
}

function SummaryCard({ label, count, selected, onClick }: { label: string; count: number; selected: boolean; onClick: () => void }) {
  return (
    <Button variant="outline" className={`summary-card ${selected ? "selected" : ""}`} onClick={onClick} aria-pressed={selected}>
      <span>{label}</span>
      <strong>{count}</strong>
    </Button>
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
    <Card>
      <CardHeader><CardTitle>Ask the support team</CardTitle></CardHeader>
      <CardContent>
      <form onSubmit={submit}>
      <Textarea
        rows={3}
        data-testid="new-case-question"
        placeholder="What do you need help with?"
        value={question}
        onChange={(e) => setQuestion(e.target.value)}
      />
      {error && <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}
      <p>
        <Button type="submit" data-testid="new-case-submit" disabled={busy || question.trim() === ""}>
          Send question
        </Button>
      </p>
      </form>
      </CardContent>
    </Card>
  );
}
