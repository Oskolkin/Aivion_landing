import type { ChatSession } from "@/lib/chat-api";

export type ChatSessionKind = "action" | "analytics" | "export" | "general";
export type ChatSessionDisplayStatus = "active" | "archived" | "completed";

export type ChatSessionDisplay = {
  id: number;
  title: string;
  kind: ChatSessionKind;
  status: ChatSessionDisplayStatus;
  badgeLabel: string;
  subtitle: string;
  lastMessageAt?: string | null;
  actionViewId?: string;
  categoryLabel?: string;
  raw: ChatSession;
};

export type ChatAttachmentDisplay = {
  id?: string;
  name: string;
  kind?: "xlsx" | "csv" | "pdf" | "json" | "txt" | "docx" | "md" | "file";
  visualLabel?: string;
  sizeLabel?: string;
  downloadUrl?: string;
  warningCode?: string | null;
};
