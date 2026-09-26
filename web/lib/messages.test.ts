import { describe, expect, it } from "vitest";
import { mergeMessages } from "./messages";
import type { Message } from "./types";

function msg(id: string, createdAt: string, body = id): Message {
  return {
    id,
    caseId: "c1",
    senderId: "customer:ann",
    senderName: "Ann",
    senderRole: "customer",
    kind: "text",
    body,
    createdAt,
  };
}

describe("mergeMessages", () => {
  it("shows a message once when it arrives from both the response and the event", () => {
    const sent = msg("m2", "2026-09-26T09:01:00.000Z");

    const afterResponse = mergeMessages([msg("m1", "2026-09-26T09:00:00.000Z")], [sent]);
    const afterEvent = mergeMessages(afterResponse, [sent]);

    expect(afterEvent.map((m) => m.id)).toEqual(["m1", "m2"]);
  });

  it("sorts oldest first, whatever order messages arrive in", () => {
    const newestFirstFromApi = [
      msg("m3", "2026-09-26T09:02:00.000Z"),
      msg("m2", "2026-09-26T09:01:00.000Z"),
      msg("m1", "2026-09-26T09:00:00.000Z"),
    ];

    expect(mergeMessages([], newestFirstFromApi).map((m) => m.id)).toEqual(["m1", "m2", "m3"]);
  });

  it("breaks ties in the same millisecond by id", () => {
    const sameTime = "2026-09-26T09:00:00.000Z";

    const merged = mergeMessages([msg("0192-b", sameTime)], [msg("0192-a", sameTime)]);

    expect(merged.map((m) => m.id)).toEqual(["0192-a", "0192-b"]);
  });

  it("keeps the later copy of a message", () => {
    const merged = mergeMessages([msg("m1", "2026-09-26T09:00:00.000Z", "old")], [msg("m1", "2026-09-26T09:00:00.000Z", "new")]);

    expect(merged.map((m) => m.body)).toEqual(["new"]);
  });

  it("does not change the lists it was given", () => {
    const current = [msg("m1", "2026-09-26T09:00:00.000Z")];

    mergeMessages(current, [msg("m2", "2026-09-26T09:01:00.000Z")]);

    expect(current).toHaveLength(1);
  });
});
