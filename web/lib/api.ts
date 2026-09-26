import { clearSession, loadSession } from "./session";
import type { Case, CaseStatus, Message, Role, User } from "./types";

// The API client. Pages call these functions; they never call fetch themselves (ADR 0001).

export class ApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

type Fetch = typeof fetch;

export interface ApiOptions {
  // Called when the server answers 401: the token is missing, expired or signed with another secret.
  onUnauthorized?: () => void;
}

export function createApi(
  baseUrl: string,
  getToken: () => string | undefined,
  fetchImpl: Fetch = (...args) => fetch(...args),
  options: ApiOptions = {},
) {
  async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
    const headers: Record<string, string> = {};
    const token = getToken();
    if (token) headers.Authorization = `Bearer ${token}`;
    if (body !== undefined) headers["Content-Type"] = "application/json";

    let res: Response;
    try {
      res = await fetchImpl(baseUrl + path, {
        method,
        headers,
        body: body === undefined ? undefined : JSON.stringify(body),
      });
    } catch {
      throw new ApiError(0, "Cannot reach the server");
    }
    const data = await res.json().catch(() => null);
    if (!res.ok) {
      if (res.status === 401) options.onUnauthorized?.();
      throw new ApiError(res.status, (data as { error?: string } | null)?.error ?? `Request failed (${res.status})`);
    }
    return data as T;
  }

  const id = encodeURIComponent;

  return {
    login: (name: string, role: Role) => request<{ token: string; user: User }>("POST", "/v1/login", { name, role }),
    openCase: (question: string) => request<{ case: Case; message: Message }>("POST", "/v1/cases", { question }),
    listCases: (status?: CaseStatus) => request<Case[]>("GET", "/v1/cases" + (status ? `?status=${status}` : "")),
    getCase: (caseId: string) => request<Case>("GET", `/v1/cases/${id(caseId)}`),
    joinCase: (caseId: string) => request<Case>("POST", `/v1/cases/${id(caseId)}/join`),
    closeCase: (caseId: string) => request<Case>("POST", `/v1/cases/${id(caseId)}/close`),
    sendMessage: (caseId: string, body: string) => request<Message>("POST", `/v1/cases/${id(caseId)}/messages`, { body }),
    // listMessages returns newest first, like the API. Pass the oldest id you have as `before` to page back.
    listMessages: (caseId: string, opts: { before?: string; limit?: number } = {}) => {
      const q = new URLSearchParams();
      if (opts.before) q.set("before", opts.before);
      if (opts.limit) q.set("limit", String(opts.limit));
      const qs = q.toString();
      return request<Message[]>("GET", `/v1/cases/${id(caseId)}/messages` + (qs ? `?${qs}` : ""));
    },
    // WebSocket URLs carry the token as a query parameter, because browsers cannot set headers on a WebSocket.
    caseEventsUrl: (caseId: string) => eventsUrl(baseUrl, `/v1/cases/${id(caseId)}/events`, getToken()),
    agentEventsUrl: () => eventsUrl(baseUrl, "/v1/cases/events", getToken()),
  };
}

function eventsUrl(baseUrl: string, path: string, token: string | undefined): string {
  return baseUrl.replace(/^http/, "ws") + path + `?token=${encodeURIComponent(token ?? "")}`;
}

export type Api = ReturnType<typeof createApi>;

export const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

// A 401 means the session is no longer valid (the JWT lasts 12 hours), so log out and go to /login.
function logOut() {
  clearSession();
  if (typeof window !== "undefined" && window.location.pathname !== "/login") window.location.replace("/login");
}

export const api: Api = createApi(API_URL, () => loadSession()?.token, undefined, { onUnauthorized: logOut });
