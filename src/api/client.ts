import type { Job, NotificationDelivery, PipelineStage, ResumeSource, WorkerState } from "../types";

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

export type DashboardData = {
  jobs: Job[];
  pipeline: PipelineStage[];
  profile: unknown;
  resumeSources: ResumeSource[];
  notifications: NotificationDelivery[];
  workers: WorkerState[];
};

export async function loadDashboardData(fallback: DashboardData): Promise<DashboardData> {
  const [jobs, pipeline, profile, resumeSources, notifications, workers] = await Promise.all([
    getJSON<Job[]>("/api/jobs", fallback.jobs),
    getJSON<PipelineStage[]>("/api/pipeline", fallback.pipeline),
    getJSON<unknown>("/api/profile", fallback.profile),
    getJSON<ResumeSource[]>("/api/resume-sources", fallback.resumeSources),
    getJSON<NotificationDelivery[]>("/api/notifications", fallback.notifications),
    getJSON<WorkerState[]>("/api/workers", fallback.workers)
  ]);

  return { jobs, pipeline, profile, resumeSources, notifications, workers };
}
