"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import Protected from "@/components/Protected";
import { PageHeader, Spinner, EmptyState } from "@/components/ui";
import { api, Analysis, ApiError, Resume, Role, Tailored } from "@/lib/api";
import { Upload, FileText, Wand2, Map, CheckCircle2, Loader2, AlertCircle } from "lucide-react";

function StatusBadge({ status }: { status: string }) {
  const map: Record<string, { cls: string; icon: any; label: string }> = {
    parsed: { cls: "border-emerald-400/30 bg-emerald-500/10 text-emerald-300", icon: CheckCircle2, label: "Parsed" },
    parsing: { cls: "border-amber-400/30 bg-amber-500/10 text-amber-300", icon: Loader2, label: "Parsing…" },
    uploaded: { cls: "border-amber-400/30 bg-amber-500/10 text-amber-300", icon: Loader2, label: "Queued…" },
    failed: { cls: "border-rose-400/30 bg-rose-500/10 text-rose-300", icon: AlertCircle, label: "Failed" },
  };
  const s = map[status] || map.uploaded;
  const Icon = s.icon;
  return (
    <span className={`pill ${s.cls}`}>
      <Icon size={13} className={`mr-1.5 ${status === "parsing" || status === "uploaded" ? "animate-spin" : ""}`} />
      {s.label}
    </span>
  );
}

function ResumeCard({ resume, roles }: { resume: Resume; roles: Role[] }) {
  const router = useRouter();
  const [roleId, setRoleId] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [showTailor, setShowTailor] = useState(false);
  const [jd, setJd] = useState("");
  const [tailored, setTailored] = useState<Tailored | null>(null);

  const skills: string[] = resume.parsed?.skills || [];
  const ready = resume.status === "parsed";

  async function generate() {
    if (!roleId) { setError("Pick a target role first."); return; }
    setBusy(true); setError("");
    try {
      const a: Analysis = await api.analyze(resume.id, roleId);
      router.push(`/analyses/${a.id}`);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to analyze");
      setBusy(false);
    }
  }

  async function tailor() {
    if (jd.trim().length < 20) { setError("Paste a longer job description."); return; }
    setBusy(true); setError("");
    try {
      setTailored(await api.tailorResume(resume.id, jd));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to tailor");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="card">
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-center gap-3">
          <span className="grid h-10 w-10 place-items-center rounded-xl bg-brand-500/15 text-brand-400">
            <FileText size={18} />
          </span>
          <div>
            <p className="font-medium text-white">{resume.filename}</p>
            <p className="text-xs text-slate-500">{new Date(resume.created_at).toLocaleString()}</p>
          </div>
        </div>
        <StatusBadge status={resume.status} />
      </div>

      {ready && skills.length > 0 && (
        <div className="mt-4 flex flex-wrap gap-2">
          {skills.slice(0, 12).map((s) => (
            <span key={s} className="pill border-white/10 bg-white/5 text-slate-300">{s}</span>
          ))}
        </div>
      )}

      {ready && (
        <div className="mt-5 space-y-4 border-t border-white/5 pt-5">
          <div className="flex flex-col gap-3 sm:flex-row">
            <select className="input sm:flex-1" value={roleId} onChange={(e) => setRoleId(e.target.value)}>
              <option value="">Select a target role…</option>
              {roles.map((r) => <option key={r.id} value={r.id}>{r.name}</option>)}
            </select>
            <button className="btn-primary" disabled={busy} onClick={generate}>
              <Map size={16} /> Generate roadmap
            </button>
          </div>

          <button className="btn-ghost w-full" onClick={() => setShowTailor((v) => !v)}>
            <Wand2 size={16} /> {showTailor ? "Hide" : "Tailor"} resume to a job description
          </button>

          {showTailor && (
            <div className="space-y-3 rounded-xl border border-white/5 bg-ink-900/50 p-4">
              <textarea className="input min-h-28" placeholder="Paste the job description here…"
                value={jd} onChange={(e) => setJd(e.target.value)} />
              <button className="btn-primary" disabled={busy} onClick={tailor}>
                {busy ? "Tailoring…" : "Tailor my resume"}
              </button>

              {tailored && (
                <div className="mt-3 space-y-3 text-sm">
                  <p className="text-slate-300">{tailored.content?.summary}</p>
                  {tailored.content?.experience?.map((exp: any, i: number) => (
                    <div key={i}>
                      <p className="font-medium text-white">{exp.title} · {exp.company}</p>
                      <ul className="mt-1 list-disc space-y-1 pl-5 text-slate-400">
                        {exp.bullets?.map((b: string, j: number) => <li key={j}>{b}</li>)}
                      </ul>
                    </div>
                  ))}
                  {tailored.content?.changes && (
                    <div className="rounded-lg bg-brand-500/10 p-3">
                      <p className="text-xs font-medium text-brand-400">What changed</p>
                      <ul className="mt-1 list-disc space-y-1 pl-5 text-slate-400">
                        {tailored.content.changes.map((c: string, i: number) => <li key={i}>{c}</li>)}
                      </ul>
                    </div>
                  )}
                </div>
              )}
            </div>
          )}
        </div>
      )}

      {error && <p className="mt-3 text-sm text-rose-300">{error}</p>}
    </div>
  );
}

export default function ResumePage() {
  const [resumes, setResumes] = useState<Resume[]>([]);
  const [roles, setRoles] = useState<Role[]>([]);
  const [loading, setLoading] = useState(true);
  const [uploading, setUploading] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);

  async function load() {
    const [r, ro] = await Promise.all([api.listResumes(), api.roles()]);
    setResumes(r.resumes || []);
    setRoles(ro.roles || []);
  }

  useEffect(() => { load().finally(() => setLoading(false)); }, []);

  // Poll while anything is still processing.
  useEffect(() => {
    const pending = resumes.some((r) => r.status === "uploaded" || r.status === "parsing");
    if (!pending) return;
    const t = setInterval(() => { api.listResumes().then((r) => setResumes(r.resumes || [])); }, 3000);
    return () => clearInterval(t);
  }, [resumes]);

  async function onUpload(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    setUploading(true);
    try {
      await api.uploadResume(file);
      await load();
    } finally {
      setUploading(false);
      if (fileRef.current) fileRef.current.value = "";
    }
  }

  return (
    <Protected>
      <PageHeader
        title="Resume"
        subtitle="Upload a resume to parse it, generate a roadmap, or tailor it to a job."
        action={
          <>
            <input ref={fileRef} type="file" accept=".pdf,.docx,.txt" className="hidden" onChange={onUpload} />
            <button className="btn-primary" disabled={uploading} onClick={() => fileRef.current?.click()}>
              <Upload size={16} /> {uploading ? "Uploading…" : "Upload resume"}
            </button>
          </>
        }
      />

      {loading ? (
        <Spinner label="Loading…" />
      ) : resumes.length === 0 ? (
        <EmptyState
          title="No resumes yet"
          body="Upload a PDF, DOCX, or TXT resume. We'll parse it into a structured profile you can analyze against any role."
          action={<button className="btn-primary" onClick={() => fileRef.current?.click()}><Upload size={16} /> Upload your first resume</button>}
        />
      ) : (
        <div className="grid gap-5">
          {resumes.map((r) => <ResumeCard key={r.id} resume={r} roles={roles} />)}
        </div>
      )}
    </Protected>
  );
}
