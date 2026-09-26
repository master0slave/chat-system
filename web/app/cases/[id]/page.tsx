"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
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
      <div className="card">
        <p className="error" data-testid="room-error">
          {loadError}
        </p>
        <Link href="/cases">Back to cases</Link>
      </div>
    );
  }
  if (!kase) return <p className="muted">Loading…</p>;

  const me = session.user;
  const isAgent = me.role === "agent";
  const joined = kase.participants.some((p) => p.userId === me.id);
  const closed = kase.status === "closed";

  return (
    <div className="card">
      <p>
        <Link href="/cases">← Cases</Link>
      </p>
      <h1 data-testid="case-subject">{kase.subject}</h1>
      <p className="row">
        <span className="status" data-testid="case-status">
          {kase.status}
        </span>
        <span className="muted" data-testid="participants">
          {kase.participants.map((p) => p.name).join(", ")}
        </span>
      </p>

      {socketStatus === "reconnecting" && (
        <p className="banner" data-testid="reconnecting-banner">
          Reconnecting…
        </p>
      )}
      {kase.status === "waiting" && !isAgent && <p className="banner">Waiting for a support agent to join…</p>}
      {closed && (
        <p className="banner" data-testid="closed-banner">
          This case is closed.
        </p>
      )}

      <div className="row">
        {isAgent && !joined && !closed && (
          <button data-testid="join-button" onClick={() => run(() => api.joinCase(id))}>
            Join case
          </button>
        )}
        {isAgent && joined && !closed && (
          <button className="secondary" data-testid="close-button" onClick={() => run(() => api.closeCase(id))}>
            Close case
          </button>
        )}
      </div>
      {actionError && <p className="error">{actionError}</p>}

      {hasOlder && (
        <p>
          <button className="secondary" data-testid="load-older" onClick={loadOlder}>
            Load older messages
          </button>
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
    </div>
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
        <input
          type="text"
          data-testid="message-input"
          placeholder="Write a message"
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          style={{ flex: 1 }}
        />
        <button type="submit" data-testid="message-send" disabled={sending || draft.trim() === ""}>
          Send
        </button>
      </div>
      {error && (
        <p className="row error" data-testid="send-error">
          Not sent: {error}
          <button type="button" className="secondary" data-testid="send-retry" onClick={() => send()}>
            Retry
          </button>
        </p>
      )}
    </form>
  );
}
