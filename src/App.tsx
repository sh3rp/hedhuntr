import {
  Activity,
  Bell,
  BriefcaseBusiness,
  CheckCircle2,
  ClipboardList,
  Database,
  FileText,
  Gauge,
  Laptop,
  LayoutDashboard,
  Radio,
  Search,
  Settings,
  UserRound,
  Wifi,
  WifiOff
} from "lucide-react";
import { useMemo, useState } from "react";
import { jobs, notifications, pipeline, resumeSources, workers } from "./data/mockData";
import { useRealtime } from "./hooks/useRealtime";
import type { Job, JobStatus, NavItem, ViewKey } from "./types";

const navItems: NavItem[] = [
  { key: "overview", label: "Overview", icon: LayoutDashboard },
  { key: "jobs", label: "Jobs", icon: Search },
  { key: "pipeline", label: "Pipeline", icon: ClipboardList },
  { key: "profile", label: "Profile", icon: UserRound },
  { key: "resumes", label: "Resumes", icon: FileText },
  { key: "notifications", label: "Notifications", icon: Bell },
  { key: "system", label: "System", icon: Settings }
];

const statusLabels: Record<JobStatus, string> = {
  discovered: "Discovered",
  parsed: "Parsed",
  matched: "Matched",
  ready_to_apply: "Ready",
  applied: "Applied",
  interview: "Interview",
  rejected: "Rejected"
};

export function App() {
  const [view, setView] = useState<ViewKey>("overview");
  const realtime = useRealtime();
  const isElectron = window.hedhuntr?.runtime === "electron";

  const averageScore = useMemo(() => {
    const scored = jobs.filter((job) => job.matchScore > 0);
    return Math.round(scored.reduce((sum, job) => sum + job.matchScore, 0) / scored.length);
  }, []);

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
            <button className="primary-action" type="button">
              <CheckCircle2 size={18} />
              Review Queue
            </button>
          </div>
        </header>

        {view === "overview" && <Overview averageScore={averageScore} />}
        {view === "jobs" && <JobsView />}
        {view === "pipeline" && <PipelineView />}
        {view === "profile" && <ProfileView />}
        {view === "resumes" && <ResumesView />}
        {view === "notifications" && <NotificationsView />}
        {view === "system" && <SystemView realtimeEvents={realtime.events} />}
      </main>
    </div>
  );
}

function Overview({ averageScore }: { averageScore: number }) {
  return (
    <div className="view-stack">
      <section className="metric-grid" aria-label="Pipeline summary">
        <Metric icon={BriefcaseBusiness} label="Tracked Jobs" value="42" note="+8 today" />
        <Metric icon={Gauge} label="Avg Match" value={`${averageScore}%`} note="parsed jobs" />
        <Metric icon={CheckCircle2} label="Ready" value="4" note="needs review" />
        <Metric icon={Bell} label="Notifications" value="14" note="sent this week" />
      </section>

      <section className="split-layout">
        <div className="panel">
          <PanelHeader title="Highest Fit Jobs" icon={Search} />
          <JobTable rows={jobs.slice(0, 3)} compact />
        </div>
        <div className="panel">
          <PanelHeader title="Pipeline" icon={Activity} />
          <PipelineBars />
        </div>
      </section>
    </div>
  );
}

function JobsView() {
  const [query, setQuery] = useState("");
  const filtered = jobs.filter((job) =>
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

function PipelineView() {
  return (
    <div className="kanban-grid">
      {pipeline.map((stage) => (
        <section className="stage-column" key={stage.status}>
          <div className="stage-header">
            <strong>{stage.label}</strong>
            <span>{stage.count}</span>
          </div>
          {jobs
            .filter((job) => job.status === stage.status)
            .map((job) => (
              <JobTile job={job} key={job.id} />
            ))}
        </section>
      ))}
    </div>
  );
}

function ProfileView() {
  return (
    <div className="view-stack">
      <section className="panel profile-panel">
        <PanelHeader title="Candidate Profile" icon={UserRound} />
        <div className="profile-grid">
          <div>
            <span className="field-label">Name</span>
            <strong>Example Candidate</strong>
          </div>
          <div>
            <span className="field-label">Preference</span>
            <strong>Remote, $140k minimum</strong>
          </div>
          <div>
            <span className="field-label">Titles</span>
            <strong>Backend, Platform, Software Engineer</strong>
          </div>
        </div>
        <div className="skill-row">
          {["Go", "TypeScript", "React", "NATS", "SQLite", "Docker", "AWS"].map((skill) => (
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

function ResumesView() {
  return (
    <div className="view-stack">
      <section className="panel">
        <PanelHeader title="Resume Sources" icon={FileText} />
        <div className="list-table">
          {resumeSources.map((source) => (
            <div className="list-row" key={source.id}>
              <span>{source.name}</span>
              <span>{source.format}</span>
              <span>{source.path}</span>
              <span>{source.updatedAt}</span>
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

function NotificationsView() {
  return (
    <section className="panel">
      <PanelHeader title="Deliveries" icon={Bell} />
      <div className="list-table">
        {notifications.map((delivery) => (
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

function SystemView({ realtimeEvents }: { realtimeEvents: ReturnType<typeof useRealtime>["events"] }) {
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

function PipelineBars() {
  const max = Math.max(...pipeline.map((stage) => stage.count));
  return (
    <div className="pipeline-bars">
      {pipeline.map((stage) => (
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
