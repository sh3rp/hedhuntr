import {
  Activity,
  AlertCircle,
  Bell,
  BriefcaseBusiness,
  CalendarClock,
  CheckCircle2,
  ClipboardList,
  Code2,
  Database,
  Eye,
  FileCheck2,
  FileText,
  Gauge,
  Laptop,
  LayoutDashboard,
  ListChecks,
  PlayCircle,
  Radio,
  RotateCcw,
  Search,
  Settings,
  UserRound,
  Wifi,
  WifiOff
} from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { ReactNode } from "react";
import { approveApplicationForAutomation, createInterview, createInterviewTask, failAutomationRun, loadDashboardData, markAutomationSubmitted, retryAutomationRun, saveCandidateProfile, saveNotificationChannel, saveNotificationRule, updateInterviewStatus, updateInterviewTaskStatus, updateReviewMaterialStatus, type DashboardData } from "./api/client";
import { jobs as mockJobs, notifications as mockNotifications, pipeline as mockPipeline, resumeSources as mockResumeSources, workers as mockWorkers } from "./data/mockData";
import { useRealtime } from "./hooks/useRealtime";
import type { AutomationRunView, CandidateProfile, Certification, CreateInterviewRequest, Education, Interview, Job, JobStatus, NavItem, NotificationChannel, NotificationRule, NotificationSettings, ProfileLink, ProfileQualityReport, Project, RealtimeEvent, ReviewApplication, ReviewMaterial, ReviewMaterialStatus, ViewKey, WorkHistory } from "./types";

const navItems: NavItem[] = [
  { key: "overview", label: "Overview", icon: LayoutDashboard },
  { key: "jobs", label: "Jobs", icon: Search },
  { key: "pipeline", label: "Pipeline", icon: ClipboardList },
  { key: "review", label: "Review", icon: FileCheck2 },
  { key: "automation", label: "Automation", icon: PlayCircle },
  { key: "interviews", label: "Interviews", icon: CalendarClock },
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
    profileQuality: null,
    resumeSources: mockResumeSources,
    reviewApplications: [],
    automationRuns: [],
    interviews: [],
    notifications: mockNotifications,
    notificationSettings: { channels: [], rules: [] },
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
        {view === "automation" && <AutomationView runs={data.automationRuns} onChanged={refreshDashboard} />}
        {view === "interviews" && <InterviewsView applications={data.reviewApplications} interviews={data.interviews} onChanged={refreshDashboard} />}
        {view === "profile" && <ProfileView onChanged={refreshDashboard} profile={data.profile} quality={data.profileQuality} />}
        {view === "resumes" && <ResumesView sources={data.resumeSources} />}
        {view === "notifications" && <NotificationsView onChanged={refreshDashboard} rows={data.notifications} settings={data.notificationSettings} />}
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
  const [tab, setTab] = useState<"preview" | "markdown" | "checks">("preview");
  const review = useMemo(() => reviewMaterial(material), [material]);

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
          <strong>{materialLabel(material.kind)}</strong>
          <span>{material.path}</span>
        </div>
        <StatusPill status={material.status.replaceAll("_", " ")} />
      </div>
      <MaterialReviewSummary review={review} />
      <div className="material-tabs" role="tablist" aria-label={`${material.kind} review modes`}>
        <button className={tab === "preview" ? "active" : ""} onClick={() => setTab("preview")} type="button">
          <Eye size={16} />
          Preview
        </button>
        <button className={tab === "markdown" ? "active" : ""} onClick={() => setTab("markdown")} type="button">
          <Code2 size={16} />
          Markdown
        </button>
        <button className={tab === "checks" ? "active" : ""} onClick={() => setTab("checks")} type="button">
          <ListChecks size={16} />
          Checks
        </button>
      </div>
      {tab === "preview" && <MarkdownPreview content={material.content} />}
      {tab === "markdown" && <pre className="material-preview">{material.content || "Document content is unavailable."}</pre>}
      {tab === "checks" && <MaterialChecks review={review} />}
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

type MaterialReview = {
  sections: string[];
  bullets: string[];
  links: string[];
  reviewNotes: string[];
  skills: string[];
  warnings: string[];
  wordCount: number;
};

function reviewMaterial(material: ReviewMaterial): MaterialReview {
  const lines = material.content.split(/\r?\n/);
  const sections = lines
    .filter((line) => line.startsWith("## "))
    .map((line) => line.replace(/^##\s+/, "").trim())
    .filter(Boolean);
  const bullets = lines
    .filter((line) => line.trim().startsWith("- "))
    .map((line) => line.trim().replace(/^-\s+/, ""))
    .filter(Boolean);
  const reviewIndex = lines.findIndex((line) => line.trim().toLowerCase() === "## review notes");
  const reviewNotes = reviewIndex >= 0
    ? lines.slice(reviewIndex + 1).filter((line) => line.trim().startsWith("- ")).map((line) => line.trim().replace(/^-\s+/, ""))
    : [];
  const links = [...material.content.matchAll(/\[[^\]]+\]\(([^)]+)\)/g)].map((match) => match[1]);
  const skills = extractSectionBullets(lines, "Relevant Skills");
  const warnings = reviewWarnings(material, { sections, bullets, links, reviewNotes, skills, wordCount: wordCount(material.content), warnings: [] });
  return {
    sections,
    bullets,
    links,
    reviewNotes,
    skills,
    warnings,
    wordCount: wordCount(material.content)
  };
}

function MaterialReviewSummary({ review }: { review: MaterialReview }) {
  return (
    <div className="material-review-summary">
      <div>
        <span className="field-label">Words</span>
        <strong>{review.wordCount}</strong>
      </div>
      <div>
        <span className="field-label">Sections</span>
        <strong>{review.sections.length}</strong>
      </div>
      <div>
        <span className="field-label">Bullets</span>
        <strong>{review.bullets.length}</strong>
      </div>
      <div>
        <span className="field-label">Links</span>
        <strong>{review.links.length}</strong>
      </div>
    </div>
  );
}

function MaterialChecks({ review }: { review: MaterialReview }) {
  return (
    <div className="material-checks">
      {review.warnings.length > 0 ? (
        <div className="validation-summary" role="alert">
          <strong>Review before approving</strong>
          <ul>
            {review.warnings.map((warning) => (
              <li key={warning}>{warning}</li>
            ))}
          </ul>
        </div>
      ) : (
        <div className="quality-check complete">
          <CheckCircle2 size={18} />
          <div>
            <strong>No structural warnings</strong>
            <span>Still verify every claim against the source profile before approving.</span>
          </div>
        </div>
      )}
      <div className="review-columns">
        <div>
          <span className="field-label">Sections</span>
          <div className="review-chip-list">
            {review.sections.map((section) => <span className="skill-chip" key={section}>{section}</span>)}
            {review.sections.length === 0 ? <span className="empty-state">No sections found.</span> : null}
          </div>
        </div>
        <div>
          <span className="field-label">Target Skills</span>
          <div className="review-chip-list">
            {review.skills.map((skill) => <span className="skill-chip" key={skill}>{skill}</span>)}
            {review.skills.length === 0 ? <span className="empty-state">No relevant skills section found.</span> : null}
          </div>
        </div>
      </div>
      {review.reviewNotes.length > 0 ? (
        <div>
          <span className="field-label">Generator Review Notes</span>
          <ul className="review-note-list">
            {review.reviewNotes.map((note) => <li key={note}>{note}</li>)}
          </ul>
        </div>
      ) : null}
    </div>
  );
}

function MarkdownPreview({ content }: { content: string }) {
  if (!content) {
    return <div className="markdown-preview empty-state">Document content is unavailable.</div>;
  }
  const blocks = content.split(/\n{2,}/).map((block) => block.trim()).filter(Boolean);
  return (
    <div className="markdown-preview">
      {blocks.map((block, index) => renderMarkdownBlock(block, index))}
    </div>
  );
}

function renderMarkdownBlock(block: string, index: number) {
  if (block.startsWith("# ")) {
    return <h1 key={index}>{inlineMarkdown(block.replace(/^#\s+/, ""))}</h1>;
  }
  if (block.startsWith("## ")) {
    return <h2 key={index}>{inlineMarkdown(block.replace(/^##\s+/, ""))}</h2>;
  }
  if (block.startsWith("### ")) {
    return <h3 key={index}>{inlineMarkdown(block.replace(/^###\s+/, ""))}</h3>;
  }
  const lines = block.split(/\r?\n/);
  if (lines.every((line) => line.trim().startsWith("- "))) {
    return (
      <ul key={index}>
        {lines.map((line) => <li key={line}>{inlineMarkdown(line.trim().replace(/^-\s+/, ""))}</li>)}
      </ul>
    );
  }
  return <p key={index}>{inlineMarkdown(block.replace(/\n/g, " "))}</p>;
}

function inlineMarkdown(value: string) {
  const parts: ReactNode[] = [];
  const pattern = /(\[[^\]]+\]\([^)]+\)|_[^_]+_)/g;
  let last = 0;
  for (const match of value.matchAll(pattern)) {
    const index = match.index ?? 0;
    if (index > last) parts.push(value.slice(last, index));
    const token = match[0];
    const link = token.match(/^\[([^\]]+)\]\(([^)]+)\)$/);
    if (link) {
      parts.push(<a href={link[2]} key={`${index}-${token}`} rel="noreferrer" target="_blank">{link[1]}</a>);
    } else {
      parts.push(<em key={`${index}-${token}`}>{token.slice(1, -1)}</em>);
    }
    last = index + token.length;
  }
  if (last < value.length) parts.push(value.slice(last));
  return parts;
}

function extractSectionBullets(lines: string[], section: string) {
  const start = lines.findIndex((line) => line.trim().toLowerCase() === `## ${section}`.toLowerCase());
  if (start < 0) return [];
  const values: string[] = [];
  for (const line of lines.slice(start + 1)) {
    if (line.startsWith("## ")) break;
    if (line.trim().startsWith("- ")) values.push(line.trim().replace(/^-\s+/, ""));
  }
  return values;
}

function reviewWarnings(material: ReviewMaterial, review: MaterialReview) {
  const warnings: string[] = [];
  if (!material.content.trim()) warnings.push("Generated document content is missing.");
  if (material.kind === "resume" && !review.sections.includes("Review Notes")) warnings.push("Resume is missing generator review notes.");
  if (material.kind === "resume" && !review.sections.includes("Selected Highlights")) warnings.push("Resume is missing selected highlights.");
  if (material.kind === "cover_letter" && review.wordCount > 500) warnings.push("Cover letter is longer than 500 words.");
  if (material.kind === "application_answers" && !review.sections.includes("Work authorization")) warnings.push("Application answers are missing the work authorization review prompt.");
  if (material.kind === "application_answers" && !review.sections.includes("Review Notes")) warnings.push("Application answers are missing review notes.");
  if (review.reviewNotes.some((note) => note.toLowerCase().includes("verify"))) warnings.push("Generator explicitly requested verification before use.");
  return warnings;
}

function materialLabel(kind: string) {
  switch (kind) {
    case "cover_letter":
      return "Cover Letter";
    case "application_answers":
      return "Application Answers";
    case "resume":
      return "Resume";
    default:
      return kind.replaceAll("_", " ");
  }
}

function wordCount(value: string) {
  return value.trim() ? value.trim().split(/\s+/).length : 0;
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

function AutomationView({ runs, onChanged }: { runs: AutomationRunView[]; onChanged: () => void }) {
  const [busyID, setBusyID] = useState<number | null>(null);

  const runAction = async (run: AutomationRunView, action: "submitted" | "failed" | "retry") => {
    setBusyID(run.id);
    try {
      if (action === "submitted") {
        await markAutomationSubmitted(run.id, run.finalUrl ?? "");
      } else if (action === "failed") {
        await failAutomationRun(run.id, "Marked failed by user.");
      } else {
        await retryAutomationRun(run.id);
      }
      onChanged();
    } finally {
      setBusyID(null);
    }
  };

  if (runs.length === 0) {
    return (
      <section className="panel">
        <PanelHeader title="Automation Runs" icon={PlayCircle} />
        <span className="empty-state">No automation runs have been requested.</span>
      </section>
    );
  }

  return (
    <div className="view-stack">
      {runs.map((run) => (
        <section className="panel automation-card" key={run.id}>
          <div className="automation-header">
            <div>
              <strong>{run.jobTitle}</strong>
              <span>{run.company} · {run.location || "Location not listed"}</span>
            </div>
            <StatusPill status={run.status.replaceAll("_", " ")} />
          </div>
          <div className="review-summary">
            <div>
              <span className="field-label">Run</span>
              <strong>#{run.id}</strong>
            </div>
            <div>
              <span className="field-label">Requested</span>
              <strong>{run.requestedAt}</strong>
            </div>
            <div>
              <span className="field-label">Final URL</span>
              <strong>{run.finalUrl || "Pending"}</strong>
            </div>
          </div>
          {run.error && <div className="automation-error">{run.error}</div>}
          <div className="material-actions">
            <button className="primary-action" disabled={busyID === run.id || run.status === "submitted"} onClick={() => runAction(run, "submitted")} type="button">
              <CheckCircle2 size={18} />
              Mark Submitted
            </button>
            <button className="secondary-action danger" disabled={busyID === run.id || run.status === "submitted"} onClick={() => runAction(run, "failed")} type="button">
              Fail
            </button>
            <button className="secondary-action" disabled={busyID === run.id} onClick={() => runAction(run, "retry")} type="button">
              <RotateCcw size={18} />
              Retry
            </button>
          </div>
          <div className="event-feed automation-logs">
            {run.logs.length === 0 ? (
              <span className="empty-state">No logs recorded for this run.</span>
            ) : (
              run.logs.map((log) => (
                <div className="event-row" key={log.id}>
                  <strong>{log.message}</strong>
                  <span>{log.createdAt}</span>
                </div>
              ))
            )}
          </div>
        </section>
      ))}
    </div>
  );
}

function InterviewsView({ applications, interviews, onChanged }: { applications: ReviewApplication[]; interviews: Interview[]; onChanged: () => void }) {
  const initialApplicationID = applications[0]?.applicationId ?? 0;
  const [draft, setDraft] = useState<CreateInterviewRequest>({
    applicationId: initialApplicationID,
    stage: "recruiter_screen",
    status: "scheduled",
    scheduledAt: "",
    durationMinutes: 30,
    location: "",
    contacts: [],
    notes: ""
  });
  const [contactsText, setContactsText] = useState("");
  const [taskDrafts, setTaskDrafts] = useState<Record<number, string>>({});
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (draft.applicationId === 0 && initialApplicationID > 0) {
      setDraft((current) => ({ ...current, applicationId: initialApplicationID }));
    }
  }, [draft.applicationId, initialApplicationID]);

  const saveInterview = async () => {
    setError("");
    if (draft.applicationId <= 0) {
      setError("Select an application before creating an interview.");
      return;
    }
    if (!draft.stage.trim()) {
      setError("Stage is required.");
      return;
    }
    setSaving(true);
    try {
      await createInterview({
        ...draft,
        contacts: csv(contactsText)
      });
      setDraft({
        applicationId: draft.applicationId,
        stage: "recruiter_screen",
        status: "scheduled",
        scheduledAt: "",
        durationMinutes: 30,
        location: "",
        contacts: [],
        notes: ""
      });
      setContactsText("");
      onChanged();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to create interview.");
    } finally {
      setSaving(false);
    }
  };

  const changeStatus = async (interview: Interview, status: string) => {
    await updateInterviewStatus(interview.id, status, interview.outcome ?? "", interview.notes ?? "");
    onChanged();
  };

  const addTask = async (interviewID: number) => {
    const title = taskDrafts[interviewID]?.trim() ?? "";
    if (!title) return;
    await createInterviewTask(interviewID, title);
    setTaskDrafts({ ...taskDrafts, [interviewID]: "" });
    onChanged();
  };

  const toggleTask = async (taskID: number, status: string) => {
    await updateInterviewTaskStatus(taskID, status === "done" ? "open" : "done");
    onChanged();
  };

  return (
    <div className="view-stack">
      <section className="panel">
        <PanelHeader title="Schedule Interview" icon={CalendarClock} />
        <div className="profile-form">
          <label>
            <span className="field-label">Application</span>
            <select value={draft.applicationId} onChange={(event) => setDraft({ ...draft, applicationId: Number(event.target.value) })}>
              <option value={0}>Select application</option>
              {applications.map((application) => (
                <option key={application.applicationId} value={application.applicationId}>
                  {application.jobTitle} at {application.company}
                </option>
              ))}
            </select>
          </label>
          <label>
            <span className="field-label">Stage</span>
            <select value={draft.stage} onChange={(event) => setDraft({ ...draft, stage: event.target.value })}>
              <option value="recruiter_screen">Recruiter Screen</option>
              <option value="hiring_manager">Hiring Manager</option>
              <option value="technical">Technical</option>
              <option value="onsite">Onsite</option>
              <option value="final">Final</option>
              <option value="offer">Offer</option>
            </select>
          </label>
          <label>
            <span className="field-label">Scheduled At</span>
            <input value={draft.scheduledAt ?? ""} onChange={(event) => setDraft({ ...draft, scheduledAt: event.target.value })} placeholder="2026-05-05T15:00:00Z" />
          </label>
          <label>
            <span className="field-label">Duration Minutes</span>
            <input type="number" value={draft.durationMinutes ?? ""} onChange={(event) => setDraft({ ...draft, durationMinutes: event.target.value === "" ? 0 : Number(event.target.value) })} />
          </label>
          <label>
            <span className="field-label">Location</span>
            <input value={draft.location ?? ""} onChange={(event) => setDraft({ ...draft, location: event.target.value })} />
          </label>
          <label>
            <span className="field-label">Contacts</span>
            <input value={contactsText} onChange={(event) => setContactsText(event.target.value)} placeholder="Hiring Manager, Engineering Lead" />
          </label>
          <label className="wide-field">
            <span className="field-label">Notes</span>
            <textarea value={draft.notes ?? ""} onChange={(event) => setDraft({ ...draft, notes: event.target.value })} />
          </label>
        </div>
        {error ? <div className="validation-summary" role="alert">{error}</div> : null}
        <div className="material-actions profile-actions">
          <button className="primary-action" disabled={saving || applications.length === 0} onClick={saveInterview} type="button">
            <CheckCircle2 size={18} />
            Create Interview
          </button>
        </div>
      </section>

      <section className="panel">
        <PanelHeader title="Interview Pipeline" icon={CalendarClock} />
        <div className="interview-list">
          {interviews.length === 0 ? <span className="empty-state">No interviews tracked yet.</span> : null}
          {interviews.map((interview) => (
            <article className="interview-card" key={interview.id}>
              <div className="interview-card-header">
                <div>
                  <strong>{interview.jobTitle} at {interview.company}</strong>
                  <span>{interview.stage.replaceAll("_", " ")} · {interview.status}</span>
                </div>
                <select value={interview.status} onChange={(event) => changeStatus(interview, event.target.value)}>
                  <option value="scheduled">Scheduled</option>
                  <option value="completed">Completed</option>
                  <option value="cancelled">Cancelled</option>
                  <option value="no_show">No Show</option>
                  <option value="offer">Offer</option>
                  <option value="rejected">Rejected</option>
                </select>
              </div>
              <div className="interview-meta">
                <span>{interview.scheduledAt || "Unscheduled"}</span>
                <span>{interview.durationMinutes ? `${interview.durationMinutes} min` : "Duration unset"}</span>
                <span>{interview.location || "Location unset"}</span>
              </div>
              {interview.contacts.length > 0 ? (
                <div className="skill-row">
                  {interview.contacts.map((contact) => (
                    <span className="skill-chip" key={contact}>{contact}</span>
                  ))}
                </div>
              ) : null}
              {interview.notes ? <p>{interview.notes}</p> : null}
              <div className="interview-tasks">
                {interview.tasks.map((task) => (
                  <div className={`interview-task-row ${task.status === "done" ? "done" : ""}`} key={task.id}>
                    <button className="icon-button" onClick={() => toggleTask(task.id, task.status)} title={task.status === "done" ? "Reopen task" : "Mark task done"} type="button">
                      <CheckCircle2 size={18} />
                    </button>
                    <div>
                      <strong>{task.title}</strong>
                      <span>{task.dueAt || task.status}</span>
                    </div>
                  </div>
                ))}
                <div className="task-entry">
                  <input value={taskDrafts[interview.id] ?? ""} onChange={(event) => setTaskDrafts({ ...taskDrafts, [interview.id]: event.target.value })} placeholder="Add follow-up task" />
                  <button className="secondary-action" onClick={() => addTask(interview.id)} type="button">Add Task</button>
                </div>
              </div>
            </article>
          ))}
        </div>
      </section>
    </div>
  );
}

function ProfileView({ profile, quality, onChanged }: { profile: CandidateProfile | null; quality: ProfileQualityReport | null; onChanged: () => void }) {
  const initial = profile ?? emptyProfile();
  const [draft, setDraft] = useState<CandidateProfile>(initial);
  const [skillsText, setSkillsText] = useState(initial.skills.join(", "));
  const [titlesText, setTitlesText] = useState(initial.preferred_titles.join(", "));
  const [locationsText, setLocationsText] = useState(initial.preferred_locations.join(", "));
  const [saving, setSaving] = useState(false);
  const [validationErrors, setValidationErrors] = useState<string[]>([]);
  const [profileMessage, setProfileMessage] = useState("");
  const importInputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    const next = profile ?? emptyProfile();
    setDraft(next);
    setSkillsText(next.skills.join(", "));
    setTitlesText(next.preferred_titles.join(", "));
    setLocationsText(next.preferred_locations.join(", "));
    setValidationErrors([]);
    setProfileMessage("");
  }, [profile]);

  const profilePayload = (): CandidateProfile => ({
    ...draft,
    skills: csv(skillsText),
    preferred_titles: csv(titlesText),
    preferred_locations: csv(locationsText),
    min_salary: draft.min_salary === null || draft.min_salary === undefined ? null : Number(draft.min_salary)
  });

  const save = async () => {
    const payload = profilePayload();
    const errors = validateProfileDraft(payload);
    setValidationErrors(errors);
    if (errors.length > 0) {
      return;
    }
    setSaving(true);
    try {
      await saveCandidateProfile(payload);
      setProfileMessage("Profile saved.");
      onChanged();
    } finally {
      setSaving(false);
    }
  };

  const exportProfile = () => {
    const payload = profilePayload();
    const data = JSON.stringify(payload, null, 2);
    const blob = new Blob([data], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    const name = payload.name.trim().toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "") || "candidate-profile";
    link.href = url;
    link.download = `${name}.json`;
    link.click();
    URL.revokeObjectURL(url);
  };

  const importProfile = async (file: File | null) => {
    if (!file) return;
    try {
      const parsed = JSON.parse(await file.text()) as unknown;
      const imported = normalizeImportedProfile(parsed);
      const errors = validateProfileDraft(imported);
      setDraft(imported);
      setSkillsText(imported.skills.join(", "));
      setTitlesText(imported.preferred_titles.join(", "));
      setLocationsText(imported.preferred_locations.join(", "));
      setValidationErrors(errors);
      setProfileMessage(errors.length > 0 ? "Profile imported with validation issues. Review the errors before saving." : "Profile imported. Review and save when ready.");
    } catch (err) {
      setProfileMessage("");
      setValidationErrors([err instanceof Error ? err.message : "Unable to import profile JSON."]);
    } finally {
      if (importInputRef.current) {
        importInputRef.current.value = "";
      }
    }
  };

  const updateWorkHistory = (index: number, item: WorkHistory) => {
    setDraft({ ...draft, work_history: replaceAt(draft.work_history, index, item) });
  };
  const updateProject = (index: number, item: Project) => {
    setDraft({ ...draft, projects: replaceAt(draft.projects, index, item) });
  };
  const updateEducation = (index: number, item: Education) => {
    setDraft({ ...draft, education: replaceAt(draft.education, index, item) });
  };
  const updateCertification = (index: number, item: Certification) => {
    setDraft({ ...draft, certifications: replaceAt(draft.certifications, index, item) });
  };
  const updateLink = (index: number, item: ProfileLink) => {
    setDraft({ ...draft, links: replaceAt(draft.links, index, item) });
  };

  return (
    <div className="view-stack">
      <ProfileQualityPanel quality={quality} />
      <section className="panel profile-panel">
        <PanelHeader title="Candidate Profile" icon={UserRound} />
        <div className="profile-import-export">
          <input
            accept="application/json,.json"
            className="hidden-file-input"
            onChange={(event) => importProfile(event.target.files?.[0] ?? null)}
            ref={importInputRef}
            type="file"
          />
          <button className="secondary-action" onClick={() => importInputRef.current?.click()} type="button">
            Import JSON
          </button>
          <button className="secondary-action" onClick={exportProfile} type="button">
            Export JSON
          </button>
        </div>
        <div className="profile-form">
          <label>
            <span className="field-label">Name</span>
            <input value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} />
          </label>
          <label>
            <span className="field-label">Headline</span>
            <input value={draft.headline ?? ""} onChange={(event) => setDraft({ ...draft, headline: event.target.value })} />
          </label>
          <label>
            <span className="field-label">Remote Preference</span>
            <select value={draft.remote_preference ?? ""} onChange={(event) => setDraft({ ...draft, remote_preference: event.target.value as CandidateProfile["remote_preference"] })}>
              <option value="">Unset</option>
              <option value="remote">Remote</option>
              <option value="hybrid">Hybrid</option>
              <option value="onsite">Onsite</option>
            </select>
          </label>
          <label>
            <span className="field-label">Minimum Salary</span>
            <input
              type="number"
              value={draft.min_salary ?? ""}
              onChange={(event) => setDraft({ ...draft, min_salary: event.target.value === "" ? null : Number(event.target.value) })}
            />
          </label>
          <label className="wide-field">
            <span className="field-label">Skills</span>
            <input value={skillsText} onChange={(event) => setSkillsText(event.target.value)} />
          </label>
          <label className="wide-field">
            <span className="field-label">Preferred Titles</span>
            <input value={titlesText} onChange={(event) => setTitlesText(event.target.value)} />
          </label>
          <label className="wide-field">
            <span className="field-label">Preferred Locations</span>
            <input value={locationsText} onChange={(event) => setLocationsText(event.target.value)} />
          </label>
        </div>
        <div className="skill-row">
          {csv(skillsText).map((skill) => (
            <span className="skill-chip" key={skill}>
              {skill}
            </span>
          ))}
        </div>
        {profileMessage ? <div className="profile-message">{profileMessage}</div> : null}
        {validationErrors.length > 0 ? <ValidationSummary errors={validationErrors} /> : null}
        <div className="profile-sections">
          <ProfileSection title="Work History" onAdd={() => setDraft({ ...draft, work_history: [...draft.work_history, emptyWorkHistory()] })}>
            {draft.work_history.length === 0 ? <span className="empty-state">No work history added.</span> : null}
            {draft.work_history.map((item, index) => (
              <div className="profile-section-card" key={index}>
                <div className="profile-section-header">
                  <strong>{item.title || item.company || `Role ${index + 1}`}</strong>
                  <button className="secondary-action danger" onClick={() => setDraft({ ...draft, work_history: removeAt(draft.work_history, index) })} type="button">
                    Remove
                  </button>
                </div>
                <div className="profile-form compact-profile-form">
                  <TextField label="Company" value={item.company} onChange={(value) => updateWorkHistory(index, { ...item, company: value })} />
                  <TextField label="Title" value={item.title} onChange={(value) => updateWorkHistory(index, { ...item, title: value })} />
                  <TextField label="Location" value={item.location} onChange={(value) => updateWorkHistory(index, { ...item, location: value })} />
                  <TextField label="Start Date" value={item.start_date} onChange={(value) => updateWorkHistory(index, { ...item, start_date: value })} />
                  <TextField label="End Date" value={item.end_date} onChange={(value) => updateWorkHistory(index, { ...item, end_date: value })} />
                  <label className="checkbox-field">
                    <input checked={Boolean(item.current)} type="checkbox" onChange={(event) => updateWorkHistory(index, { ...item, current: event.target.checked })} />
                    <span>Current Role</span>
                  </label>
                  <TextAreaField label="Summary" value={item.summary} onChange={(value) => updateWorkHistory(index, { ...item, summary: value })} />
                  <TextField label="Highlights" value={(item.highlights ?? []).join(", ")} onChange={(value) => updateWorkHistory(index, { ...item, highlights: csv(value) })} wide />
                  <TextField label="Technologies" value={(item.technologies ?? []).join(", ")} onChange={(value) => updateWorkHistory(index, { ...item, technologies: csv(value) })} wide />
                </div>
              </div>
            ))}
          </ProfileSection>

          <ProfileSection title="Projects" onAdd={() => setDraft({ ...draft, projects: [...draft.projects, emptyProject()] })}>
            {draft.projects.length === 0 ? <span className="empty-state">No projects added.</span> : null}
            {draft.projects.map((item, index) => (
              <div className="profile-section-card" key={index}>
                <div className="profile-section-header">
                  <strong>{item.name || `Project ${index + 1}`}</strong>
                  <button className="secondary-action danger" onClick={() => setDraft({ ...draft, projects: removeAt(draft.projects, index) })} type="button">
                    Remove
                  </button>
                </div>
                <div className="profile-form compact-profile-form">
                  <TextField label="Name" value={item.name} onChange={(value) => updateProject(index, { ...item, name: value })} />
                  <TextField label="Role" value={item.role} onChange={(value) => updateProject(index, { ...item, role: value })} />
                  <TextField label="URL" value={item.url} onChange={(value) => updateProject(index, { ...item, url: value })} />
                  <TextAreaField label="Summary" value={item.summary} onChange={(value) => updateProject(index, { ...item, summary: value })} />
                  <TextField label="Highlights" value={(item.highlights ?? []).join(", ")} onChange={(value) => updateProject(index, { ...item, highlights: csv(value) })} wide />
                  <TextField label="Technologies" value={(item.technologies ?? []).join(", ")} onChange={(value) => updateProject(index, { ...item, technologies: csv(value) })} wide />
                </div>
              </div>
            ))}
          </ProfileSection>

          <ProfileSection title="Education" onAdd={() => setDraft({ ...draft, education: [...draft.education, emptyEducation()] })}>
            {draft.education.length === 0 ? <span className="empty-state">No education added.</span> : null}
            {draft.education.map((item, index) => (
              <div className="profile-section-card" key={index}>
                <div className="profile-section-header">
                  <strong>{item.institution || `Education ${index + 1}`}</strong>
                  <button className="secondary-action danger" onClick={() => setDraft({ ...draft, education: removeAt(draft.education, index) })} type="button">
                    Remove
                  </button>
                </div>
                <div className="profile-form compact-profile-form">
                  <TextField label="Institution" value={item.institution} onChange={(value) => updateEducation(index, { ...item, institution: value })} />
                  <TextField label="Degree" value={item.degree} onChange={(value) => updateEducation(index, { ...item, degree: value })} />
                  <TextField label="Field" value={item.field} onChange={(value) => updateEducation(index, { ...item, field: value })} />
                  <TextField label="Start Date" value={item.start_date} onChange={(value) => updateEducation(index, { ...item, start_date: value })} />
                  <TextField label="End Date" value={item.end_date} onChange={(value) => updateEducation(index, { ...item, end_date: value })} />
                  <TextAreaField label="Summary" value={item.summary} onChange={(value) => updateEducation(index, { ...item, summary: value })} />
                </div>
              </div>
            ))}
          </ProfileSection>

          <ProfileSection title="Certifications" onAdd={() => setDraft({ ...draft, certifications: [...draft.certifications, emptyCertification()] })}>
            {draft.certifications.length === 0 ? <span className="empty-state">No certifications added.</span> : null}
            {draft.certifications.map((item, index) => (
              <div className="profile-section-card" key={index}>
                <div className="profile-section-header">
                  <strong>{item.name || `Certification ${index + 1}`}</strong>
                  <button className="secondary-action danger" onClick={() => setDraft({ ...draft, certifications: removeAt(draft.certifications, index) })} type="button">
                    Remove
                  </button>
                </div>
                <div className="profile-form compact-profile-form">
                  <TextField label="Name" value={item.name} onChange={(value) => updateCertification(index, { ...item, name: value })} />
                  <TextField label="Issuer" value={item.issuer} onChange={(value) => updateCertification(index, { ...item, issuer: value })} />
                  <TextField label="Issued At" value={item.issued_at} onChange={(value) => updateCertification(index, { ...item, issued_at: value })} />
                  <TextField label="Expires At" value={item.expires_at} onChange={(value) => updateCertification(index, { ...item, expires_at: value })} />
                  <TextField label="URL" value={item.url} onChange={(value) => updateCertification(index, { ...item, url: value })} wide />
                </div>
              </div>
            ))}
          </ProfileSection>

          <ProfileSection title="Links" onAdd={() => setDraft({ ...draft, links: [...draft.links, emptyLink()] })}>
            {draft.links.length === 0 ? <span className="empty-state">No links added.</span> : null}
            {draft.links.map((item, index) => (
              <div className="profile-section-card" key={index}>
                <div className="profile-section-header">
                  <strong>{item.label || `Link ${index + 1}`}</strong>
                  <button className="secondary-action danger" onClick={() => setDraft({ ...draft, links: removeAt(draft.links, index) })} type="button">
                    Remove
                  </button>
                </div>
                <div className="profile-form compact-profile-form">
                  <TextField label="Label" value={item.label} onChange={(value) => updateLink(index, { ...item, label: value })} />
                  <TextField label="URL" value={item.url} onChange={(value) => updateLink(index, { ...item, url: value })} />
                </div>
              </div>
            ))}
          </ProfileSection>
        </div>
        <div className="material-actions profile-actions">
          <button className="primary-action" disabled={saving} onClick={save} type="button">
            <CheckCircle2 size={18} />
            Save Profile
          </button>
        </div>
      </section>
      <section className="panel">
        <PanelHeader title="Source Commands" icon={Database} />
        <code className="command-line">go run ./cmd/profile -db hedhuntr.db -profile configs/candidate-profile.example.json -print</code>
      </section>
    </div>
  );
}

function ValidationSummary({ errors }: { errors: string[] }) {
  return (
    <div className="validation-summary" role="alert">
      <strong>Profile cannot be saved yet</strong>
      <ul>
        {errors.map((error) => (
          <li key={error}>{error}</li>
        ))}
      </ul>
    </div>
  );
}

function ProfileQualityPanel({ quality }: { quality: ProfileQualityReport | null }) {
  const report = quality ?? { score: 0, status: "incomplete", checks: [] };
  const missing = report.checks.filter((item) => item.status !== "complete");
  const complete = report.checks.length - missing.length;
  return (
    <section className="panel profile-quality-panel">
      <div className="profile-quality-summary">
        <div>
          <PanelHeader title="Profile Quality" icon={Gauge} />
          <p>{complete} of {report.checks.length} checks complete</p>
        </div>
        <div className={`quality-score ${report.status}`}>
          <strong>{report.score}</strong>
          <span>{report.status}</span>
        </div>
      </div>
      <div className="quality-meter" aria-label={`Profile quality ${report.score}%`}>
        <span style={{ width: `${Math.max(0, Math.min(100, report.score))}%` }} />
      </div>
      <div className="quality-check-grid">
        {report.checks.map((item) => {
          const completeCheck = item.status === "complete";
          const Icon = completeCheck ? CheckCircle2 : AlertCircle;
          return (
            <div className={`quality-check ${completeCheck ? "complete" : "missing"}`} key={item.id}>
              <Icon size={18} />
              <div>
                <strong>{item.label}</strong>
                <span>{item.message}</span>
              </div>
            </div>
          );
        })}
      </div>
    </section>
  );
}

function ProfileSection({ children, onAdd, title }: { children: ReactNode; onAdd: () => void; title: string }) {
  return (
    <section className="profile-section">
      <div className="profile-section-title">
        <h3>{title}</h3>
        <button className="secondary-action" onClick={onAdd} type="button">
          Add
        </button>
      </div>
      <div className="profile-section-list">{children}</div>
    </section>
  );
}

function TextField({ label, onChange, value, wide }: { label: string; onChange: (value: string) => void; value?: string; wide?: boolean }) {
  return (
    <label className={wide ? "wide-field" : undefined}>
      <span className="field-label">{label}</span>
      <input value={value ?? ""} onChange={(event) => onChange(event.target.value)} />
    </label>
  );
}

function TextAreaField({ label, onChange, value }: { label: string; onChange: (value: string) => void; value?: string }) {
  return (
    <label className="wide-field">
      <span className="field-label">{label}</span>
      <textarea value={value ?? ""} onChange={(event) => onChange(event.target.value)} />
    </label>
  );
}

function emptyProfile(): CandidateProfile {
  return {
    name: "",
    headline: "",
    skills: [],
    preferred_titles: [],
    preferred_locations: [],
    remote_preference: "",
    min_salary: null,
    work_history: [],
    projects: [],
    education: [],
    certifications: [],
    links: []
  };
}

function emptyWorkHistory(): WorkHistory {
  return {
    company: "",
    title: "",
    location: "",
    start_date: "",
    end_date: "",
    current: false,
    summary: "",
    highlights: [],
    technologies: []
  };
}

function emptyProject(): Project {
  return {
    name: "",
    role: "",
    url: "",
    summary: "",
    highlights: [],
    technologies: []
  };
}

function emptyEducation(): Education {
  return {
    institution: "",
    degree: "",
    field: "",
    start_date: "",
    end_date: "",
    summary: ""
  };
}

function emptyCertification(): Certification {
  return {
    name: "",
    issuer: "",
    issued_at: "",
    expires_at: "",
    url: ""
  };
}

function emptyLink(): ProfileLink {
  return {
    label: "",
    url: ""
  };
}

function validateProfileDraft(profile: CandidateProfile) {
  const errors: string[] = [];
  if (!isPresent(profile.name)) {
    errors.push("Name is required.");
  }
  if (profile.skills.length === 0) {
    errors.push("At least one skill is required.");
  }
  if (profile.min_salary !== null && profile.min_salary !== undefined && profile.min_salary < 0) {
    errors.push("Minimum salary cannot be negative.");
  }
  if (profile.remote_preference && !["remote", "hybrid", "onsite"].includes(profile.remote_preference)) {
    errors.push("Remote preference must be remote, hybrid, or onsite.");
  }
  profile.work_history.forEach((item, index) => {
    if (!isPresent(item.company)) {
      errors.push(`Work history ${index + 1}: company is required.`);
    }
    if (!isPresent(item.title)) {
      errors.push(`Work history ${index + 1}: title is required.`);
    }
  });
  profile.projects.forEach((item, index) => {
    if (!isPresent(item.name)) {
      errors.push(`Project ${index + 1}: name is required.`);
    }
  });
  profile.education.forEach((item, index) => {
    if (!isPresent(item.institution)) {
      errors.push(`Education ${index + 1}: institution is required.`);
    }
  });
  profile.certifications.forEach((item, index) => {
    if (!isPresent(item.name)) {
      errors.push(`Certification ${index + 1}: name is required.`);
    }
  });
  profile.links.forEach((item, index) => {
    if (!isPresent(item.label)) {
      errors.push(`Link ${index + 1}: label is required.`);
    }
    if (!isPresent(item.url)) {
      errors.push(`Link ${index + 1}: URL is required.`);
    }
  });
  return errors;
}

function normalizeImportedProfile(value: unknown): CandidateProfile {
  if (!isRecord(value)) {
    throw new Error("Imported profile must be a JSON object.");
  }
  return {
    id: typeof value.id === "number" ? value.id : undefined,
    name: stringValue(value.name),
    headline: stringValue(value.headline),
    skills: stringArray(value.skills),
    preferred_titles: stringArray(value.preferred_titles),
    preferred_locations: stringArray(value.preferred_locations),
    remote_preference: remotePreferenceValue(value.remote_preference),
    min_salary: numberOrNull(value.min_salary),
    work_history: arrayOfRecords(value.work_history).map((item) => ({
      company: stringValue(item.company),
      title: stringValue(item.title),
      location: stringValue(item.location),
      start_date: stringValue(item.start_date),
      end_date: stringValue(item.end_date),
      current: Boolean(item.current),
      summary: stringValue(item.summary),
      highlights: stringArray(item.highlights),
      technologies: stringArray(item.technologies)
    })),
    projects: arrayOfRecords(value.projects).map((item) => ({
      name: stringValue(item.name),
      role: stringValue(item.role),
      url: stringValue(item.url),
      summary: stringValue(item.summary),
      highlights: stringArray(item.highlights),
      technologies: stringArray(item.technologies)
    })),
    education: arrayOfRecords(value.education).map((item) => ({
      institution: stringValue(item.institution),
      degree: stringValue(item.degree),
      field: stringValue(item.field),
      start_date: stringValue(item.start_date),
      end_date: stringValue(item.end_date),
      summary: stringValue(item.summary)
    })),
    certifications: arrayOfRecords(value.certifications).map((item) => ({
      name: stringValue(item.name),
      issuer: stringValue(item.issuer),
      issued_at: stringValue(item.issued_at),
      expires_at: stringValue(item.expires_at),
      url: stringValue(item.url)
    })),
    links: arrayOfRecords(value.links).map((item) => ({
      label: stringValue(item.label),
      url: stringValue(item.url)
    }))
  };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function arrayOfRecords(value: unknown) {
  return Array.isArray(value) ? value.filter(isRecord) : [];
}

function stringValue(value: unknown) {
  return typeof value === "string" ? value : "";
}

function stringArray(value: unknown) {
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === "string").map((item) => item.trim()).filter(Boolean) : [];
}

function numberOrNull(value: unknown) {
  if (typeof value === "number" && Number.isFinite(value)) return value;
  return null;
}

function remotePreferenceValue(value: unknown): CandidateProfile["remote_preference"] {
  return value === "remote" || value === "hybrid" || value === "onsite" ? value : "";
}

function isPresent(value: string | undefined) {
  return Boolean(value?.trim());
}

function replaceAt<T>(items: T[], index: number, value: T) {
  return items.map((item, itemIndex) => (itemIndex === index ? value : item));
}

function removeAt<T>(items: T[], index: number) {
  return items.filter((_, itemIndex) => itemIndex !== index);
}

function csv(value: string) {
  return value
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
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

function NotificationsView({ onChanged, rows, settings }: { onChanged: () => void; rows: typeof mockNotifications; settings: NotificationSettings }) {
  const [channelDraft, setChannelDraft] = useState<NotificationChannel>({
    name: "",
    type: "discord",
    enabled: true,
    webhookUrl: ""
  });
  const [ruleDraft, setRuleDraft] = useState<NotificationRule>({
    name: "jobs-matched",
    eventSubject: "jobs.matched",
    enabled: true,
    minScore: 70
  });
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);

  const saveChannel = async (channel: NotificationChannel) => {
    setBusy(true);
    setMessage("");
    try {
      await saveNotificationChannel(channel);
      setChannelDraft({ name: "", type: "discord", enabled: true, webhookUrl: "" });
      setMessage("Notification channel saved.");
      onChanged();
    } catch (err) {
      setMessage(err instanceof Error ? err.message : "Unable to save notification channel.");
    } finally {
      setBusy(false);
    }
  };

  const saveRule = async (rule: NotificationRule) => {
    setBusy(true);
    setMessage("");
    try {
      await saveNotificationRule({ ...rule, minScore: rule.minScore === null || rule.minScore === undefined ? null : Number(rule.minScore) });
      setRuleDraft({ name: "jobs-matched", eventSubject: "jobs.matched", enabled: true, minScore: 70 });
      setMessage("Notification rule saved.");
      onChanged();
    } catch (err) {
      setMessage(err instanceof Error ? err.message : "Unable to save notification rule.");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="view-stack">
      <section className="split-layout">
        <div className="panel">
          <PanelHeader title="Channels" icon={Bell} />
          <div className="settings-list">
            {settings.channels.length === 0 ? <span className="empty-state">No notification channels configured.</span> : null}
            {settings.channels.map((channel) => (
              <div className="settings-row" key={channel.name}>
                <div>
                  <strong>{channel.name}</strong>
                  <span>{channel.type} · {channel.webhookUrl ? "webhook configured" : "missing webhook"}</span>
                </div>
                <StatusPill status={channel.enabled ? "enabled" : "disabled"} />
                <button className="secondary-action" disabled={busy} onClick={() => saveChannel({ ...channel, enabled: !channel.enabled })} type="button">
                  {channel.enabled ? "Disable" : "Enable"}
                </button>
              </div>
            ))}
          </div>
          <div className="profile-form settings-form">
            <TextField label="Name" value={channelDraft.name} onChange={(value) => setChannelDraft({ ...channelDraft, name: value })} />
            <label>
              <span className="field-label">Type</span>
              <select value={channelDraft.type} onChange={(event) => setChannelDraft({ ...channelDraft, type: event.target.value })}>
                <option value="discord">Discord</option>
                <option value="slack">Slack</option>
              </select>
            </label>
            <TextField label="Webhook URL" value={channelDraft.webhookUrl} onChange={(value) => setChannelDraft({ ...channelDraft, webhookUrl: value })} wide />
          </div>
          <button className="primary-action" disabled={busy} onClick={() => saveChannel(channelDraft)} type="button">
            <CheckCircle2 size={18} />
            Save Channel
          </button>
        </div>

        <div className="panel">
          <PanelHeader title="Rules" icon={Settings} />
          <div className="settings-list">
            {settings.rules.length === 0 ? <span className="empty-state">No notification rules configured.</span> : null}
            {settings.rules.map((rule) => (
              <div className="settings-row" key={rule.name}>
                <div>
                  <strong>{rule.name}</strong>
                  <span>{rule.eventSubject}{rule.minScore !== null && rule.minScore !== undefined ? ` · min score ${rule.minScore}` : ""}</span>
                </div>
                <StatusPill status={rule.enabled ? "enabled" : "disabled"} />
                <button className="secondary-action" disabled={busy} onClick={() => saveRule({ ...rule, enabled: !rule.enabled })} type="button">
                  {rule.enabled ? "Disable" : "Enable"}
                </button>
              </div>
            ))}
          </div>
          <div className="profile-form settings-form">
            <TextField label="Name" value={ruleDraft.name} onChange={(value) => setRuleDraft({ ...ruleDraft, name: value })} />
            <label>
              <span className="field-label">Event Subject</span>
              <select value={ruleDraft.eventSubject} onChange={(event) => setRuleDraft({ ...ruleDraft, eventSubject: event.target.value })}>
                <option value="jobs.matched">Jobs Matched</option>
                <option value="applications.ready">Applications Ready</option>
              </select>
            </label>
            <label>
              <span className="field-label">Minimum Score</span>
              <input
                type="number"
                value={ruleDraft.minScore ?? ""}
                onChange={(event) => setRuleDraft({ ...ruleDraft, minScore: event.target.value === "" ? null : Number(event.target.value) })}
              />
            </label>
          </div>
          <button className="primary-action" disabled={busy} onClick={() => saveRule(ruleDraft)} type="button">
            <CheckCircle2 size={18} />
            Save Rule
          </button>
        </div>
      </section>

      {message ? <div className="profile-message">{message}</div> : null}

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
    </div>
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
