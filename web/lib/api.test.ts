import { describe, expect, it } from "vitest";
import { ApiError, createApi } from "./api";

type Call = { url: string; init: RequestInit };

function fakeFetch(status: number, body: unknown) {
  const calls: Call[] = [];
  const impl = (async (url: string, init: RequestInit) => {
    calls.push({ url, init });
    return new Response(body === undefined ? null : JSON.stringify(body), { status });
  }) as typeof fetch;
  return { calls, impl };
}

describe("createApi", () => {
  it("sends the token and a JSON body", async () => {
    const f = fakeFetch(201, { id: "m1" });
    const api = createApi("http://api", () => "tok", f.impl);

    await api.sendMessage("c1", "hello");

    expect(f.calls[0].url).toBe("http://api/v1/cases/c1/messages");
    expect(f.calls[0].init.method).toBe("POST");
    expect(f.calls[0].init.headers).toEqual({ Authorization: "Bearer tok", "Content-Type": "application/json" });
    expect(f.calls[0].init.body).toBe(JSON.stringify({ body: "hello" }));
  });

  it("sends no Authorization header when logged out", async () => {
    const f = fakeFetch(200, { token: "t", user: {} });
    const api = createApi("http://api", () => undefined, f.impl);

    await api.login("Ann", "customer");

    expect(f.calls[0].init.headers).toEqual({ "Content-Type": "application/json" });
  });

  it("turns an error response into an ApiError with the server's message", async () => {
    const api = createApi("http://api", () => "tok", fakeFetch(409, { error: "case is closed" }).impl);

    const err = await api.sendMessage("c1", "hi").catch((e: unknown) => e);

    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({ status: 409, message: "case is closed" });
  });

  it("reports a network failure as status 0", async () => {
    const failing = (async () => {
      throw new TypeError("fetch failed");
    }) as typeof fetch;
    const api = createApi("http://api", () => "tok", failing);

    await expect(api.getCase("c1")).rejects.toMatchObject({ status: 0, message: "Cannot reach the server" });
  });

  it("builds query strings for listing", async () => {
    const f = fakeFetch(200, []);
    const api = createApi("http://api", () => "tok", f.impl);

    await api.listCases("waiting");
    await api.listMessages("c1", { before: "m9", limit: 20 });
    await api.listMessages("c1");

    expect(f.calls.map((c) => c.url)).toEqual([
      "http://api/v1/cases?status=waiting",
      "http://api/v1/cases/c1/messages?before=m9&limit=20",
      "http://api/v1/cases/c1/messages",
    ]);
  });

  it("builds WebSocket URLs with the token", () => {
    const api = createApi("https://api.example", () => "a b", fakeFetch(200, null).impl);

    expect(api.caseEventsUrl("c1")).toBe("wss://api.example/v1/cases/c1/events?token=a%20b");
    expect(api.agentEventsUrl()).toBe("wss://api.example/v1/cases/events?token=a%20b");
  });
});
