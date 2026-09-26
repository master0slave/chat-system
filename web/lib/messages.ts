import type { Message } from "./types";

// mergeMessages adds incoming messages to the current list and returns a new list, oldest first.
// A message can arrive twice (the REST response and the message.created event, or a refetch
// after reconnecting), so messages are keyed by id and the later copy wins.
export function mergeMessages(current: Message[], incoming: Message[]): Message[] {
  const byId = new Map<string, Message>();
  for (const m of current) byId.set(m.id, m);
  for (const m of incoming) byId.set(m.id, m);
  return [...byId.values()].sort(compareMessages);
}

// Oldest first by createdAt. Ids sort in creation order (UUIDv7), so they break ties
// between messages created in the same millisecond.
function compareMessages(a: Message, b: Message): number {
  if (a.createdAt !== b.createdAt) return a.createdAt < b.createdAt ? -1 : 1;
  return a.id < b.id ? -1 : a.id > b.id ? 1 : 0;
}
