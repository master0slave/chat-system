// Shapes from docs/openapi.yaml and docs/events.md. Keep them in sync with those files.

export type Role = "customer" | "agent";
export type CaseStatus = "waiting" | "open" | "closed";

export interface User {
  id: string;
  name: string;
  role: Role;
}

export interface Participant {
  userId: string;
  name: string;
  role: Role;
  joinedAt: string;
}

export interface Case {
  id: string;
  subject: string;
  customerId: string;
  participants: Participant[];
  status: CaseStatus;
  createdAt: string;
  updatedAt: string;
  closedAt?: string;
}

export interface Message {
  id: string;
  caseId: string;
  senderId: string;
  senderName: string;
  senderRole: Role;
  kind: "text" | "system";
  body: string;
  createdAt: string;
}

export type ServerEvent =
  | { type: "message.created"; caseId: string; data: Message }
  | { type: "participant.joined"; caseId: string; data: Participant }
  | { type: "case.closed"; caseId: string; data: Case }
  | { type: "case.created"; caseId: string; data: Case }
  | { type: "case.status_changed"; caseId: string; data: Case };
