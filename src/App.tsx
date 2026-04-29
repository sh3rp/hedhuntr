import {
  Activity,
  AlertCircle,
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
  RotateCcw,
  Search,
  Settings,
  UserRound,
  Wifi,
  WifiOff
} from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { ReactNode } from "react";
import { approveApplicationForAutomation, failAutomationRun, loadDashboardData, markAutomationSubmitted, retryAutomationRun, saveCandidateProfile, updateReviewMaterialStatus, type DashboardData } from "./api/client";
import { jobs as mockJobs, notifications as mockNotifications, pipeline as mockPipeline, resumeSources as mockResumeSources, workers as mockWorkers } from "./data/mockData";
import { useRealtime } from "./hooks/useRealtime";
import type { AutomationRunView, CandidateProfile, Certification, Education, Job, JobStatus, NavItem, ProfileLink, ProfileQualityReport, Project, RealtimeEvent, ReviewApplication, ReviewMaterial, ReviewMaterialStatus, ViewKey, WorkHistory } from "./types";

const navItems: NavItem[] = [
  { key: "overview", label: "Overview", icon: LayoutDashboard },
  { key: "jobs", label: "Jobs", icon: Search },
  { key: "pipeline", label: "Pipeline", icon: ClipboardList },
  { key: "review", label: "Review", icon: FileCheck2 },
  { key: "automation", label: "Automation", icon: PlayCircle },
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
        {view === "automation" && <AutomationView runs={data.automationRuns} onChanged={refreshDashboard} />}
        {view === "profile" && <ProfileView onChanged={refreshDashboard} profile={data.profile} quality={data.profileQuality} />}
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

function ProfileView({ profile, quality, onChanged }: { profile: CandidateProfile | null; quality: ProfileQualityReport | null; onChanged: () => void }) {
  const initial = profile ?? emptyProfile();
  const [draft, setDraft] = useState<CandidateProfile>(initial);
  const [skillsText, setSkillsText] = useState(initial.skills.join(", "));
  const [titlesText, setTitlesText] = useState(initial.preferred_titles.join(", "));
  const [locationsText, setLocationsText] = useState(initial.preferred_locations.join(", "));
  const [saving, setSaving] = useState(false);
  const [validationErrors, setValidationErrors] = useState<string[]>([]);

  useEffect(() => {
    const next = profile ?? emptyProfile();
    setDraft(next);
    setSkillsText(next.skills.join(", "));
    setTitlesText(next.preferred_titles.join(", "));
    setLocationsText(next.preferred_locations.join(", "));
    setValidationErrors([]);
  }, [profile]);

  const save = async () => {
    const payload = {
      ...draft,
      skills: csv(skillsText),
      preferred_titles: csv(titlesText),
      preferred_locations: csv(locationsText),
      min_salary: draft.min_salary === null || draft.min_salary === undefined ? null : Number(draft.min_salary)
    };
    const errors = validateProfileDraft(payload);
    setValidationErrors(errors);
    if (errors.length > 0) {
      return;
    }
    setSaving(true);
    try {
      await saveCandidateProfile(payload);
      onChanged();
    } finally {
      setSaving(false);
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
