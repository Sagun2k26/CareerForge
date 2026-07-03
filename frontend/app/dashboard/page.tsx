"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import Protected from "@/components/Protected";
import { PageHeader, Spinner } from "@/components/ui";
import { api, Analysis, Resume, Session } from "@/lib/api";
import { FileText, Map, MessagesSquare, ArrowUpRight } from "lucide-react";

function Stat({ label, value, href, icon: Icon }: { label: string; value: number; href: string; icon: any }) {
  return (
    <Link href={href} className="card group transition-colors hover:border-brand-400/30">
      <div className="flex items-center justify-between">
        <span className="grid h-10 w-10 place-items-center rounded-xl bg-brand-500/15 text-brand-400">
          <Icon size={18} />
        </span>
        <ArrowUpRight size={18} className="text-slate-600 transition-colors group-hover:text-brand-400" />
      </div>
      <p className="mt-4 text-3xl font-semibold text-white">{value}</p>
      <p className="text-sm text-slate-400">{label}</p>
    </Link>
  );
}

export default function Dashboard() {
  const [resumes, setResumes] = useState<Resume[]>([]);
  const [analyses, setAnalyses] = useState<Analysis[]>([]);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    Promise.all([api.listResumes(), api.listAnalyses(), api.listInterviews()])
      .then(([r, a, s]) => {
        setResumes(r.resumes || []);
        setAnalyses(a.analyses || []);
        setSessions(s.sessions || []);
      })
      .finally(() => setLoading(false));
  }, []);

  return (
    <Protected>
      <PageHeader title="Dashboard" subtitle="Your career-prep at a glance." />

      {loading ? (
        <Spinner label="Loading your data…" />
      ) : (
        <>
          <div className="grid gap-5 sm:grid-cols-3">
            <Stat label="Resumes uploaded" value={resumes.length} href="/resume" icon={FileText} />
            <Stat label="Roadmaps generated" value={analyses.length} href="/analyses" icon={Map} />
            <Stat label="Mock interviews" value={sessions.length} href="/interview" icon={MessagesSquare} />
          </div>

          <div className="mt-8 grid gap-5 lg:grid-cols-2">
            <div className="card">
              <h2 className="mb-4 text-lg font-semibold text-white">Recent roadmaps</h2>
              {analyses.length === 0 ? (
                <p className="text-sm text-slate-400">
                  No roadmaps yet. <Link href="/resume" className="text-brand-400 hover:underline">Upload a resume</Link> to generate one.
                </p>
              ) : (
                <ul className="space-y-3">
                  {analyses.slice(0, 5).map((a) => (
                    <li key={a.id}>
                      <Link href={`/analyses/${a.id}`} className="flex items-center justify-between rounded-xl px-3 py-2.5 hover:bg-white/5">
                        <span className="text-sm text-slate-200">{a.role_name}</span>
                        <span className="pill border-brand-400/30 bg-brand-500/10 text-brand-400">{a.gap_score}% ready</span>
                      </Link>
                    </li>
                  ))}
                </ul>
              )}
            </div>

            <div className="card">
              <h2 className="mb-4 text-lg font-semibold text-white">Recent interviews</h2>
              {sessions.length === 0 ? (
                <p className="text-sm text-slate-400">
                  No interviews yet. <Link href="/interview" className="text-brand-400 hover:underline">Start a mock interview</Link>.
                </p>
              ) : (
                <ul className="space-y-3">
                  {sessions.slice(0, 5).map((s) => (
                    <li key={s.id}>
                      <Link href={`/interview/${s.id}`} className="flex items-center justify-between rounded-xl px-3 py-2.5 hover:bg-white/5">
                        <span className="text-sm text-slate-200">{s.role_name}</span>
                        <span className={`pill ${s.status === "complete" ? "border-emerald-400/30 bg-emerald-500/10 text-emerald-300" : "border-amber-400/30 bg-amber-500/10 text-amber-300"}`}>
                          {s.status}
                        </span>
                      </Link>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </div>
        </>
      )}
    </Protected>
  );
}
