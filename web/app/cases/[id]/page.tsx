"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import { api, ApiError } from "@/lib/api";
import { mergeMessages } from "@/lib/messages";
import { connectEvents, type SocketStatus } from "@/lib/socket";
import type { Case, Message } from "@/lib/types";
import { useSession } from "../../use-session";

const PAGE_SIZE = 50;

export default function CaseRoomPage() {
  const { id } = useParams<{ id: string }>();
  const session = useSession();
  const [kase, setCase] = useState<Case | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [hasOlder, setHasOlder] = useState(false);
  const [socketStatus, setSocketStatus] = useState<SocketStatus>("connecting");
  const [loadError, setLoadError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const stopSocket = useRef<(() => void) | null>(null);
  const firstLoad = useRef(true);

  // refresh loads the case and the newest messages, and merges them with what is on screen.
  // It runs on open and after every reconnect, to pick up anything missed while disconnected.
  const refresh = useCallback(async () => {
    try {
      const [c, latest] = await Promise.all([api.getCase(id), api.listMessages(id, { limit: PAGE_SIZE })]);
      setCase(c);
      if (firstLoad.current) {
        firstLoad.current = false;
        setHasOlder(latest.length === PAGE_SIZE);
      }
      setMessages((current) => mergeMessages(current, latest));
    } catch (err) {
      if (err instanceof ApiError && (err.status === 403 || err.status === 404)) {
        setLoadError(err.status === 403 ? "You cannot view this case." : "This case does not exist.");
        stopSocket.current?.();
      }
    }
  }, [id]);

  useEffect(() => {
    if (!session) return;
    void refresh();
    const stop = connectEvents({
      url: () => api.caseEventsUrl(id),
      onStatus: (s) => {
        setSocketStatus(s);
        if (s === "open") void refresh();
      },
      onEvent: (e) => {
        if (e.type === "message.created") setMessages((current) => mergeMessages(current, [e.data]));
        if (e.type === "case.closed") setCase(e.data);
        if (e.type === "participant.joined") void api.getCase(id).then(setCase, () => {});
      },
    });
    stopSocket.current = stop;
    return stop;
  }, [session, id, refresh]);

  async function loadOlder() {
    const older = await api.listMessages(id, { before: messages[0]?.id, limit: PAGE_SIZE });
    setHasOlder(older.length === PAGE_SIZE);
    setMessages((current) => mergeMessages(current, older));
  }

  async function run(action: () => Promise<Case>) {
    setActionError(null);
    try {
      setCase(await action());
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Something went wrong");
    }
  }

  if (!session) return null;
  if (loadError) {
    return (
      <Alert variant="destructive" data-testid="room-error"><AlertDescription>{loadError}</AlertDescription></Alert>
    );
  }
  if (!kase) return <p className="muted loading-state"><Spinner /> Loading…</p>;

  const me = session.user;
  const isAgent = me.role === "agent";
  const joined = kase.participants.some((p) => p.userId === me.id);
  const closed = kase.status === "closed";

  return (
    <Card className="case-room">
      <CardHeader>
        <Link href="/cases" className="back-link">← Cases</Link>
        <CardTitle data-testid="case-subject">{kase.subject}</CardTitle>
      </CardHeader>
      <CardContent>
      <p className="row">
        <Badge variant={kase.status === "waiting" ? "outline" : kase.status === "open" ? "secondary" : "ghost"} className={`status status-${kase.status}`} data-testid="case-status">
          {kase.status}
        </Badge>
        <span className="muted" data-testid="participants">
          {kase.participants.map((p) => p.name).join(", ")}
        </span>
      </p>

      {socketStatus === "reconnecting" && (
        <Alert className="banner" data-testid="reconnecting-banner">
          Reconnecting…
        </Alert>
      )}
      {kase.status === "waiting" && !isAgent && <Alert className="banner">Waiting for a support agent to join…</Alert>}
      {closed && (
        <Alert className="banner" data-testid="closed-banner">
          This case is closed.
        </Alert>
      )}

      <div className="row">
        {isAgent && !joined && !closed && (
          <Button data-testid="join-button" onClick={() => run(() => api.joinCase(id))}>
            Join case
          </Button>
        )}
        {isAgent && joined && !closed && (
          <Button variant="outline" data-testid="close-button" onClick={() => run(() => api.closeCase(id))}>
            Close case
          </Button>
        )}
      </div>
      {actionError && <Alert variant="destructive"><AlertDescription>{actionError}</AlertDescription></Alert>}

      {hasOlder && (
        <p>
          <Button variant="outline" data-testid="load-older" onClick={loadOlder}>
            Load older messages
          </Button>
        </p>
      )}
      <div className="messages" data-testid="message-list">
        {messages.map((m) => (
          <div
            key={m.id}
            className={`message ${m.kind === "system" ? "system" : m.senderId === me.id ? "mine" : ""}`}
            data-testid="message"
            data-kind={m.kind}
          >
            {m.kind === "text" && <div className="muted">{m.senderName}</div>}
            <span data-testid="message-body">{m.body}</span>
          </div>
        ))}
      </div>

      {joined && !closed && (
        <SendBox caseId={id} onSent={(m) => setMessages((current) => mergeMessages(current, [m]))} />
      )}
      </CardContent>
    </Card>
  );
}

// SendBox keeps the text when sending fails and offers Retry (spec section 8).
function SendBox({ caseId, onSent }: { caseId: string; onSent: (m: Message) => void }) {
  const [draft, setDraft] = useState("");
  const [sending, setSending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function send(e?: FormEvent) {
    e?.preventDefault();
    if (draft.trim() === "") return;
    setSending(true);
    setError(null);
    try {
      onSent(await api.sendMessage(caseId, draft));
      setDraft("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not send");
    } finally {
      setSending(false);
    }
  }

  return (
    <form onSubmit={send}>
      <div className="row">
        <Input
          type="text"
          data-testid="message-input"
          placeholder="Write a message"
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          style={{ flex: 1 }}
        />
        <Button type="submit" data-testid="message-send" disabled={sending || draft.trim() === ""}>
          {sending && <Spinner data-icon="inline-start" />}
          Send
        </Button>
      </div>
      {error && (
        <Alert variant="destructive" className="row error" data-testid="send-error">
          Not sent: {error}
          <Button type="button" variant="outline" data-testid="send-retry" onClick={() => send()}>
            Retry
          </Button>
        </Alert>
      )}
    </form>
  );
}
