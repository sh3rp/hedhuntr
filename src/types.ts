import type { LucideIcon } from "lucide-react";

export type ViewKey =
  | "overview"
  | "jobs"
  | "pipeline"
  | "review"
  | "automation"
  | "interviews"
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
  type: "discord" | "slack" | string;
  status: "sent" | "failed" | "disabled";
  subject: string;
  time: string;
};

export type NotificationChannel = {
  id?: number;
  name: string;
  type: "discord" | "slack" | string;
  enabled: boolean;
  webhookUrl?: string;
  createdAt?: string;
  updatedAt?: string;
};

export type NotificationRule = {
  id?: number;
  name: string;
  eventSubject: string;
  enabled: boolean;
  minScore?: number | null;
  createdAt?: string;
  updatedAt?: string;
};

export type NotificationSettings = {
  channels: NotificationChannel[];
  rules: NotificationRule[];
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

export type ReviewMaterialStatus = "draft" | "approved" | "rejected" | "needs_changes" | "regeneration_requested";

export type ReviewMaterial = {
  id: number;
  kind: "resume" | "cover_letter" | string;
  status: ReviewMaterialStatus;
  notes: string;
  documentId: number;
  path: string;
  content: string;
  updatedAt: string;
};

export type ReviewApplication = {
  applicationId: number;
  jobId: number;
  candidateProfileId: number;
  jobTitle: string;
  company: string;
  location: string;
  matchScore: number;
  applicationStatus: string;
  updatedAt: string;
  materials: ReviewMaterial[];
};

export type WorkHistory = {
  company: string;
  title: string;
  location?: string;
  start_date?: string;
  end_date?: string;
  current?: boolean;
  summary?: string;
  highlights?: string[];
  technologies?: string[];
};

export type Project = {
  name: string;
  role?: string;
  url?: string;
  summary?: string;
  highlights?: string[];
  technologies?: string[];
};

export type Education = {
  institution: string;
  degree?: string;
  field?: string;
  start_date?: string;
  end_date?: string;
  summary?: string;
};

export type Certification = {
  name: string;
  issuer?: string;
  issued_at?: string;
  expires_at?: string;
  url?: string;
};

export type ProfileLink = {
  label: string;
  url: string;
};

export type CandidateProfile = {
  id?: number;
  name: string;
  headline?: string;
  skills: string[];
  preferred_titles: string[];
  preferred_locations: string[];
  remote_preference?: "remote" | "hybrid" | "onsite" | "";
  min_salary?: number | null;
  work_history: WorkHistory[];
  projects: Project[];
  education: Education[];
  certifications: Certification[];
  links: ProfileLink[];
};

export type ProfileQualityCheck = {
  id: string;
  label: string;
  status: "complete" | "missing" | string;
  message: string;
  weight: number;
};

export type ProfileQualityReport = {
  score: number;
  status: "ready" | "usable" | "incomplete" | string;
  checks: ProfileQualityCheck[];
};

export type AutomationRun = {
  id: number;
  applicationId: number;
  jobId: number;
  candidateProfileId: number;
  status: "requested" | "started" | "review_required" | "failed" | "submitted";
  resumeMaterialId: number;
  coverLetterMaterialId?: number;
  finalUrl?: string;
  error?: string;
  requestedAt: string;
  startedAt?: string;
  reviewRequiredAt?: string;
  finishedAt?: string;
  updatedAt: string;
};

export type AutomationLog = {
  id: number;
  runId: number;
  level: "info" | "warn" | "error" | string;
  message: string;
  details: Record<string, unknown>;
  createdAt: string;
};

export type AutomationRunView = AutomationRun & {
  jobTitle: string;
  company: string;
  location: string;
  logs: AutomationLog[];
};

export type AutomationHandoff = {
  applicationId: number;
  automationRun: AutomationRun;
  packet: unknown;
};

export type InterviewTask = {
  id: number;
  interviewId: number;
  title: string;
  status: "open" | "done" | string;
  dueAt?: string;
  notes?: string;
  createdAt: string;
  updatedAt: string;
};

export type Interview = {
  id: number;
  applicationId: number;
  jobId: number;
  candidateProfileId: number;
  jobTitle: string;
  company: string;
  stage: string;
  status: "scheduled" | "completed" | "cancelled" | "no_show" | "offer" | "rejected" | string;
  scheduledAt?: string;
  durationMinutes?: number;
  location?: string;
  contacts: string[];
  notes?: string;
  outcome?: string;
  tasks: InterviewTask[];
  createdAt: string;
  updatedAt: string;
};

export type CreateInterviewRequest = {
  applicationId: number;
  stage: string;
  status?: string;
  scheduledAt?: string;
  durationMinutes?: number;
  location?: string;
  contacts: string[];
  notes?: string;
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
