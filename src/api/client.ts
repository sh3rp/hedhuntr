import type { AutomationHandoff, Job, NotificationDelivery, PipelineStage, ResumeSource, ReviewApplication, ReviewMaterial, ReviewMaterialStatus, WorkerState } from "../types";

const apiBase = (import.meta.env.VITE_HEDHUNTR_API_URL as string | undefined) ?? "";

async function getJSON<T>(path: string, fallback: T): Promise<T> {
  try {
    const response = await fetch(`${apiBase}${path}`);
    if (!response.ok) return fallback;
    return (await response.json()) as T;
  } catch {
    return fallback;
  }
}

async function postJSON<T>(path: string, body: unknown): Promise<T> {
  const response = await fetch(`${apiBase}${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body)
  });
  if (!response.ok) {
    const message = await response.text();
    throw new Error(message || `Request failed with ${response.status}`);
  }
  return (await response.json()) as T;
}

export type DashboardData = {
  jobs: Job[];
  pipeline: PipelineStage[];
  profile: unknown;
  resumeSources: ResumeSource[];
  reviewApplications: ReviewApplication[];
  notifications: NotificationDelivery[];
  workers: WorkerState[];
};

export async function loadDashboardData(fallback: DashboardData): Promise<DashboardData> {
  const [jobs, pipeline, profile, resumeSources, reviewApplications, notifications, workers] = await Promise.all([
    getJSON<Job[]>("/api/jobs", fallback.jobs),
    getJSON<PipelineStage[]>("/api/pipeline", fallback.pipeline),
    getJSON<unknown>("/api/profile", fallback.profile),
    getJSON<ResumeSource[]>("/api/resume-sources", fallback.resumeSources),
    getJSON<ReviewApplication[]>("/api/review/applications", fallback.reviewApplications),
    getJSON<NotificationDelivery[]>("/api/notifications", fallback.notifications),
    getJSON<WorkerState[]>("/api/workers", fallback.workers)
  ]);

  return { jobs, pipeline, profile, resumeSources, reviewApplications, notifications, workers };
}

export function updateReviewMaterialStatus(id: number, status: ReviewMaterialStatus, notes = ""): Promise<ReviewMaterial> {
  return postJSON<ReviewMaterial>(`/api/review/materials/${id}/status`, { status, notes });
}

export function approveApplicationForAutomation(applicationId: number): Promise<AutomationHandoff> {
  return postJSON<AutomationHandoff>(`/api/applications/${applicationId}/approve-automation`, {});
}
