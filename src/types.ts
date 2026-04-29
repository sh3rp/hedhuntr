import type { LucideIcon } from "lucide-react";

export type ViewKey =
  | "overview"
  | "jobs"
  | "pipeline"
  | "profile"
  | "resumes"
  | "notifications"
  | "system";

export type NavItem = {
  key: ViewKey;
  label: string;
  icon: LucideIcon;
};

export type JobStatus =
  | "discovered"
  | "description_fetched"
  | "parsed"
  | "matched"
  | "ready_to_apply"
  | "applied"
  | "interview"
  | "rejected";

export type Job = {
  id: number;
  title: string;
  company: string;
  location: string;
  status: JobStatus;
  matchScore: number;
  salary: string;
  skills: string[];
  source: string;
  updatedAt: string;
};

export type PipelineStage = {
  status: JobStatus;
  label: string;
  count: number;
};

export type WorkerState = {
  name: string;
  subject: string;
  status: "idle" | "running" | "attention";
  processed: number;
  failed: number;
};

export type NotificationDelivery = {
  channel: string;
  type: "discord" | "slack";
  status: "sent" | "failed" | "disabled";
  subject: string;
  time: string;
};

export type ResumeSource = {
  id: number;
  name: string;
  format: string;
  path?: string;
  documentPath?: string;
  updatedAt?: string;
  createdAt?: string;
};

export type RealtimeEvent = {
  type: "event" | "ack" | "status";
  topic?: string;
  event_id?: string;
  event_type?: string;
  occurred_at?: string;
  payload?: unknown;
};

declare global {
  interface Window {
    hedhuntr?: {
      runtime: "electron";
      versions: {
        electron: string;
        node: string;
        chrome: string;
      };
    };
  }
}
