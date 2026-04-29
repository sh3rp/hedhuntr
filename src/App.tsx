import {
  Activity,
  Bell,
  BriefcaseBusiness,
  CheckCircle2,
  ClipboardList,
  Database,
  FileCheck2,
  FileText,
  Gauge,
  Laptop,
  LayoutDashboard,
  PlayCircle,
  Radio,
  Search,
  Settings,
  UserRound,
  Wifi,
  WifiOff
} from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { approveApplicationForAutomation, loadDashboardData, updateReviewMaterialStatus, type DashboardData } from "./api/client";
import { jobs as mockJobs, notifications as mockNotifications, pipeline as mockPipeline, resumeSources as mockResumeSources, workers as mockWorkers } from "./data/mockData";
import { useRealtime } from "./hooks/useRealtime";
import type { Job, JobStatus, NavItem, RealtimeEvent, ReviewApplication, ReviewMaterial, ReviewMaterialStatus, ViewKey } from "./types";

const navItems: NavItem[] = [
  { key: "overview", label: "Overview", icon: LayoutDashboard },
  { key: "jobs", label: "Jobs", icon: Search },
  { key: "pipeline", label: "Pipeline", icon: ClipboardList },
  { key: "review", label: "Review", icon: FileCheck2 },
  { key: "profile", label: "Profile", icon: UserRound },
  { key: "resumes", label: "Resumes", icon: FileText },
  { key: "notifications", label: "Notifications", icon: Bell },
  { key: "system", label: "System", icon: Settings }
];

const statusLabels: Record<JobStatus, string> = {
  discovered: "Discovered",
  description_fetched: "Fetched",
  parsed: "Parsed",
  matched: "Matched",
  ready_to_apply: "Ready",
  applied: "Applied",
  interview: "Interview",
  rejected: "Rejected"
};

export function App() {
  const [view, setView] = useState<ViewKey>("overview");
  const [data, setData] = useState<DashboardData>({
    jobs: mockJobs,
    pipeline: mockPipeline,
    profile: null,
    resumeSources: mockResumeSources,
    reviewApplications: [],
    notifications: mockNotifications,
    workers: mockWorkers
  });
  const dataRef = useRef(data);
  const refreshDashboard = useCallback((event?: RealtimeEvent) => {
    if (event && event.type !== "event") return;
    loadDashboardData(dataRef.current).then((loaded) => {
      dataRef.current = loaded;
      setData(loaded);
    });
  }, []);
  const realtime = useRealtime(refreshDashboard);
  const isElectron = window.hedhuntr?.runtime === "electron";

  useEffect(() => {
    let cancelled = false;
    loadDashboardData(data).then((loaded) => {
      if (!cancelled) {
        dataRef.current = loaded;
        setData(loaded);
      }
    });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    dataRef.current = data;
  }, [data]);

  const averageScore = useMemo(() => {
    const scored = data.jobs.filter((job) => job.matchScore > 0);
    if (scored.length === 0) return 0;
    return Math.round(scored.reduce((sum, job) => sum + job.matchScore, 0) / scored.length);
  }, [data.jobs]);

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand-block">
          <div className="brand-mark">
            <BriefcaseBusiness size={22} />
          </div>
          <div>
            <h1>Hedhuntr</h1>
            <span>{isElectron ? "Desktop Console" : "Web Console"}</span>
          </div>
        </div>

        <nav className="nav-list" aria-label="Primary">
          {navItems.map((item) => {
            const Icon = item.icon;
            return (
              <button
                className={view === item.key ? "nav-item active" : "nav-item"}
                key={item.key}
                onClick={() => setView(item.key)}
                type="button"
              >
                <Icon size={18} />
                <span>{item.label}</span>
              </button>
            );
          })}
        </nav>

        <div className="connection-panel">
          <div className={`connection-dot ${realtime.state}`} />
          <div>
            <strong>{realtime.state}</strong>
            <span>{realtime.wsUrl}</span>
          </div>
        </div>
      </aside>

      <main className="workspace">
        <header className="topbar">
          <div>
            <p className="eyebrow">Job Application Operations</p>
            <h2>{navItems.find((item) => item.key === view)?.label}</h2>
          </div>
          <div className="topbar-actions">
            <button className="icon-button" title="WebSocket status" type="button">
              {realtime.state === "connected" ? <Wifi size={18} /> : <WifiOff size={18} />}
            </button>
            <button className="primary-action" onClick={() => setView("review")} type="button">
              <CheckCircle2 size={18} />
              Review Queue
            </button>
          </div>
        </header>

        {view === "overview" && <Overview averageScore={averageScore} data={data} />}
        {view === "jobs" && <JobsView rows={data.jobs} />}
        {view === "pipeline" && <PipelineView rows={data.jobs} stages={data.pipeline} />}
        {view === "review" && <ReviewView applications={data.reviewApplications} onChanged={refreshDashboard} />}
        {view === "profile" && <ProfileView profile={data.profile} />}
        {view === "resumes" && <ResumesView sources={data.resumeSources} />}
        {view === "notifications" && <NotificationsView rows={data.notifications} />}
        {view === "system" && <SystemView realtimeEvents={realtime.events} workers={data.workers} />}
      </main>
    </div>
  );
}

function Overview({ averageScore, data }: { averageScore: number; data: DashboardData }) {
  return (
    <div className="view-stack">
      <section className="metric-grid" aria-label="Pipeline summary">
        <Metric icon={BriefcaseBusiness} label="Tracked Jobs" value={`${data.jobs.length}`} note="loaded jobs" />
        <Metric icon={Gauge} label="Avg Match" value={`${averageScore}%`} note="parsed jobs" />
        <Metric icon={CheckCircle2} label="Ready" value={`${data.reviewApplications.length}`} note="needs review" />
        <Metric icon={Bell} label="Notifications" value={`${data.notifications.length}`} note="recent deliveries" />
      </section>

      <section className="split-layout">
        <div className="panel">
          <PanelHeader title="Highest Fit Jobs" icon={Search} />
          <JobTable rows={data.jobs.slice(0, 3)} compact />
        </div>
        <div className="panel">
          <PanelHeader title="Pipeline" icon={Activity} />
          <PipelineBars stages={data.pipeline} />
        </div>
      </section>
    </div>
  );
}

function JobsView({ rows }: { rows: Job[] }) {
  const [query, setQuery] = useState("");
  const filtered = rows.filter((job) =>
    `${job.title} ${job.company} ${job.skills.join(" ")}`.toLowerCase().includes(query.toLowerCase())
  );

  return (
    <div className="view-stack">
      <section className="toolbar-row">
        <div className="search-box">
          <Search size={18} />
          <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Filter jobs" />
        </div>
        <button className="secondary-action" type="button">
          Import URL
        </button>
      </section>
      <section className="panel">
        <PanelHeader title="Job Queue" icon={BriefcaseBusiness} />
        <JobTable rows={filtered} />
      </section>
    </div>
  );
}

function PipelineView({ rows, stages }: { rows: Job[]; stages: typeof mockPipeline }) {
  return (
    <div className="kanban-grid">
      {stages.map((stage) => (
        <section className="stage-column" key={stage.status}>
          <div className="stage-header">
            <strong>{stage.label}</strong>
            <span>{stage.count}</span>
          </div>
          {rows
            .filter((job) => job.status === stage.status)
            .map((job) => (
              <JobTile job={job} key={job.id} />
            ))}
        </section>
      ))}
    </div>
  );
}

function ReviewView({ applications, onChanged }: { applications: ReviewApplication[]; onChanged: () => void }) {
  const [selectedID, setSelectedID] = useState<number | null>(applications[0]?.applicationId ?? null);
  const [automationBusy, setAutomationBusy] = useState(false);
  const selected = applications.find((application) => application.applicationId === selectedID) ?? applications[0];

  useEffect(() => {
    if (!selected && applications[0]) {
      setSelectedID(applications[0].applicationId);
    }
  }, [applications, selected]);

  if (applications.length === 0) {
    return (
      <section className="panel">
        <PanelHeader title="Review Queue" icon={FileCheck2} />
        <span className="empty-state">No generated application materials are waiting for review.</span>
      </section>
    );
  }

  const automationReady = selected ? canApproveAutomation(selected) : false;
  const approveAutomation = async () => {
    if (!selected) return;
    setAutomationBusy(true);
    try {
      await approveApplicationForAutomation(selected.applicationId);
      onChanged();
    } finally {
      setAutomationBusy(false);
    }
  };

  return (
    <div className="review-layout">
      <section className="panel review-list">
        <PanelHeader title="Review Queue" icon={FileCheck2} />
        {applications.map((application) => (
          <button
            className={application.applicationId === selected?.applicationId ? "review-item active" : "review-item"}
            key={application.applicationId}
            onClick={() => setSelectedID(application.applicationId)}
            type="button"
          >
            <strong>{application.jobTitle}</strong>
            <span>{application.company} · {application.matchScore}%</span>
            <StatusPill status={application.applicationStatus} />
          </button>
        ))}
      </section>

      {selected && (
        <section className="panel review-detail">
          <PanelHeader title={`${selected.jobTitle} at ${selected.company}`} icon={ClipboardList} />
          <div className="review-summary">
            <div>
              <span className="field-label">Location</span>
              <strong>{selected.location || "Not listed"}</strong>
            </div>
            <div>
              <span className="field-label">Match</span>
              <strong>{selected.matchScore}%</strong>
            </div>
            <div>
              <span className="field-label">Updated</span>
              <strong>{selected.updatedAt}</strong>
            </div>
          </div>
          <div className="handoff-panel">
            <div>
              <strong>Automation Handoff</strong>
              <span>{automationReady ? "Approved materials are ready for assisted filling." : automationBlockReason(selected)}</span>
            </div>
            <button className="primary-action" disabled={!automationReady || automationBusy} onClick={approveAutomation} type="button">
              <PlayCircle size={18} />
              Approve for Automation
            </button>
          </div>
          <div className="material-stack">
            {selected.materials.map((material) => (
              <MaterialReviewCard key={material.id} material={material} onChanged={onChanged} />
            ))}
          </div>
        </section>
      )}
    </div>
  );
}

function canApproveAutomation(application: ReviewApplication) {
  const resume = application.materials.find((material) => material.kind === "resume");
  if (!resume || resume.status !== "approved") return false;
  return application.materials.every((material) => material.status === "approved");
}

function automationBlockReason(application: ReviewApplication) {
  const resume = application.materials.find((material) => material.kind === "resume");
  if (!resume) return "A generated resume is required before automation can start.";
  if (resume.status !== "approved") return "Approve the generated resume before automation can start.";
  const pending = application.materials.find((material) => material.status !== "approved");
  if (pending) return `${pending.kind.replaceAll("_", " ")} is ${pending.status.replaceAll("_", " ")}.`;
  return "Approved materials are ready for assisted filling.";
}

function MaterialReviewCard({ material, onChanged }: { material: ReviewMaterial; onChanged: () => void }) {
  const [busy, setBusy] = useState(false);

  const updateStatus = async (status: ReviewMaterialStatus) => {
    setBusy(true);
    try {
      await updateReviewMaterialStatus(material.id, status, statusNote(status));
      onChanged();
    } finally {
      setBusy(false);
    }
  };

  return (
    <article className="material-card">
      <div className="material-header">
        <div>
          <strong>{material.kind === "cover_letter" ? "Cover Letter" : "Resume"}</strong>
          <span>{material.path}</span>
        </div>
        <StatusPill status={material.status.replaceAll("_", " ")} />
      </div>
      <pre className="material-preview">{material.content || "Document content is unavailable."}</pre>
      <div className="material-actions">
        <button className="primary-action" disabled={busy} onClick={() => updateStatus("approved")} type="button">
          <CheckCircle2 size={18} />
          Approve
        </button>
        <button className="secondary-action" disabled={busy} onClick={() => updateStatus("needs_changes")} type="button">
          Needs Changes
        </button>
        <button className="secondary-action" disabled={busy} onClick={() => updateStatus("regeneration_requested")} type="button">
          Regenerate
        </button>
        <button className="secondary-action danger" disabled={busy} onClick={() => updateStatus("rejected")} type="button">
          Reject
        </button>
      </div>
    </article>
  );
}

function statusNote(status: ReviewMaterialStatus) {
  switch (status) {
    case "approved":
      return "Approved for application use.";
    case "needs_changes":
      return "Reviewer requested changes.";
    case "regeneration_requested":
      return "Reviewer requested regeneration.";
    case "rejected":
      return "Rejected by reviewer.";
    default:
      return "";
  }
}

function ProfileView({ profile }: { profile: unknown }) {
  const profileRecord = profile as { name?: string; headline?: string; skills?: string[]; preferred_titles?: string[]; remote_preference?: string; min_salary?: number } | null;
  return (
    <div className="view-stack">
      <section className="panel profile-panel">
        <PanelHeader title="Candidate Profile" icon={UserRound} />
        <div className="profile-grid">
          <div>
            <span className="field-label">Name</span>
            <strong>{profileRecord?.name ?? "Example Candidate"}</strong>
          </div>
          <div>
            <span className="field-label">Preference</span>
            <strong>{profileRecord?.remote_preference ?? "Remote"}{profileRecord?.min_salary ? `, $${Math.round(profileRecord.min_salary / 1000)}k minimum` : ""}</strong>
          </div>
          <div>
            <span className="field-label">Titles</span>
            <strong>{profileRecord?.preferred_titles?.join(", ") ?? "Backend, Platform, Software Engineer"}</strong>
          </div>
        </div>
        <div className="skill-row">
          {(profileRecord?.skills ?? ["Go", "TypeScript", "React", "NATS", "SQLite", "Docker", "AWS"]).map((skill) => (
            <span className="skill-chip" key={skill}>
              {skill}
            </span>
          ))}
        </div>
      </section>
      <section className="panel">
        <PanelHeader title="Source Commands" icon={Database} />
        <code className="command-line">go run ./cmd/profile -db hedhuntr.db -profile configs/candidate-profile.example.json -print</code>
      </section>
    </div>
  );
}

function ResumesView({ sources }: { sources: typeof mockResumeSources }) {
  return (
    <div className="view-stack">
      <section className="panel">
        <PanelHeader title="Resume Sources" icon={FileText} />
        <div className="list-table">
          {sources.map((source) => (
            <div className="list-row" key={source.id}>
              <span>{source.name}</span>
              <span>{source.format}</span>
              <span>{source.path ?? source.documentPath ?? "stored document"}</span>
              <span>{source.updatedAt ?? source.createdAt ?? ""}</span>
            </div>
          ))}
        </div>
      </section>
      <section className="panel">
        <PanelHeader title="Import Command" icon={FileText} />
        <code className="command-line">go run ./cmd/resume import -db hedhuntr.db -file examples/resume.example.md -name "Base Resume"</code>
      </section>
    </div>
  );
}

function NotificationsView({ rows }: { rows: typeof mockNotifications }) {
  return (
    <section className="panel">
      <PanelHeader title="Deliveries" icon={Bell} />
      <div className="list-table">
        {rows.map((delivery) => (
          <div className="list-row" key={`${delivery.channel}-${delivery.time}`}>
            <span>{delivery.channel}</span>
            <span>{delivery.type}</span>
            <StatusPill status={delivery.status} />
            <span>{delivery.subject}</span>
            <span>{delivery.time}</span>
          </div>
        ))}
      </div>
    </section>
  );
}

function SystemView({ realtimeEvents, workers }: { realtimeEvents: ReturnType<typeof useRealtime>["events"]; workers: typeof mockWorkers }) {
  return (
    <div className="split-layout">
      <section className="panel">
        <PanelHeader title="Workers" icon={Radio} />
        <div className="worker-list">
          {workers.map((worker) => (
            <div className="worker-row" key={worker.name}>
              <div>
                <strong>{worker.name}</strong>
                <span>{worker.subject}</span>
              </div>
              <StatusPill status={worker.status} />
              <span>{worker.processed}</span>
              <span>{worker.failed}</span>
            </div>
          ))}
        </div>
      </section>
      <section className="panel">
        <PanelHeader title="WebSocket Events" icon={Laptop} />
        <div className="event-feed">
          {realtimeEvents.length === 0 ? (
            <span className="empty-state">Waiting for backend WebSocket events.</span>
          ) : (
            realtimeEvents.map((event, index) => (
              <div className="event-row" key={`${event.event_id ?? "event"}-${index}`}>
                <strong>{event.event_type ?? event.type}</strong>
                <span>{event.topic ?? "system"}</span>
              </div>
            ))
          )}
        </div>
      </section>
    </div>
  );
}

function Metric({ icon: Icon, label, value, note }: { icon: typeof Activity; label: string; value: string; note: string }) {
  return (
    <div className="metric">
      <Icon size={20} />
      <span>{label}</span>
      <strong>{value}</strong>
      <small>{note}</small>
    </div>
  );
}

function PanelHeader({ title, icon: Icon }: { title: string; icon: typeof Activity }) {
  return (
    <div className="panel-header">
      <div>
        <Icon size={18} />
        <h3>{title}</h3>
      </div>
    </div>
  );
}

function PipelineBars({ stages }: { stages: typeof mockPipeline }) {
  const max = Math.max(1, ...stages.map((stage) => stage.count));
  return (
    <div className="pipeline-bars">
      {stages.map((stage) => (
        <div className="bar-row" key={stage.status}>
          <span>{stage.label}</span>
          <div className="bar-track">
            <div style={{ width: `${(stage.count / max) * 100}%` }} />
          </div>
          <strong>{stage.count}</strong>
        </div>
      ))}
    </div>
  );
}

function JobTable({ rows, compact = false }: { rows: Job[]; compact?: boolean }) {
  return (
    <div className={compact ? "job-table compact" : "job-table"}>
      <div className="job-table-head">
        <span>Role</span>
        <span>Status</span>
        <span>Score</span>
        <span>Salary</span>
      </div>
      {rows.map((job) => (
        <div className="job-table-row" key={job.id}>
          <div>
            <strong>{job.title}</strong>
            <span>{job.company} · {job.location}</span>
          </div>
          <StatusPill status={statusLabels[job.status]} />
          <strong>{job.matchScore > 0 ? `${job.matchScore}%` : "Pending"}</strong>
          <span>{job.salary}</span>
        </div>
      ))}
    </div>
  );
}

function JobTile({ job }: { job: Job }) {
  return (
    <article className="job-tile">
      <strong>{job.title}</strong>
      <span>{job.company}</span>
      <div className="job-tile-footer">
        <span>{job.matchScore > 0 ? `${job.matchScore}%` : "New"}</span>
        <span>{job.updatedAt}</span>
      </div>
    </article>
  );
}

function StatusPill({ status }: { status: string }) {
  const key = status.toLowerCase().replaceAll(" ", "-");
  return <span className={`status-pill ${key}`}>{status}</span>;
}
