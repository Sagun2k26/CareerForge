"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import Protected from "@/components/Protected";
import { GapRing, Spinner } from "@/components/ui";
import { api, Analysis, RoadmapItem } from "@/lib/api";
import { ArrowLeft, Check, Circle, ExternalLink, MessagesSquare } from "lucide-react";

export default function AnalysisDetail() {
  const { id } = useParams<{ id: string }>();
  const [analysis, setAnalysis] = useState<Analysis | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.getAnalysis(id).then(setAnalysis).finally(() => setLoading(false));
  }, [id]);

  async function toggle(item: RoadmapItem) {
    if (!analysis) return;
    const next = item.status === "done" ? "todo" : "done";
    // optimistic update
    setAnalysis({
      ...analysis,
      roadmap: analysis.roadmap.map((r) => (r.order === item.order ? { ...r, status: next } : r)),
    });
    await api.setProgress(analysis.id, item.order, next);
  }

  if (loading) return <Protected><Spinner label="Loading roadmap…" /></Protected>;
  if (!analysis) return <Protected><p className="text-slate-400">Not found.</p></Protected>;

  const done = analysis.roadmap.filter((r) => r.status === "done").length;
  const pct = analysis.roadmap.length ? Math.round((done / analysis.roadmap.length) * 100) : 0;

  return (
    <Protected>
      <Link href="/analyses" className="mb-6 inline-flex items-center gap-2 text-sm text-slate-400 hover:text-slate-200">
        <ArrowLeft size={16} /> Back to roadmaps
      </Link>

      <div className="card flex flex-col items-center gap-6 sm:flex-row sm:items-start">
        <GapRing score={analysis.gap_score} />
        <div className="flex-1">
          <h1 className="text-2xl font-semibold text-white">{analysis.role_name}</h1>
          <p className="mt-2 text-slate-400">{analysis.summary}</p>
          <div className="mt-4 grid gap-4 sm:grid-cols-2">
            <div>
              <p className="mb-2 text-xs font-medium uppercase tracking-wide text-emerald-400">You already have</p>
              <div className="flex flex-wrap gap-2">
                {analysis.matched_skills.map((s) => (
                  <span key={s} className="pill border-emerald-400/30 bg-emerald-500/10 text-emerald-300">{s}</span>
                ))}
              </div>
            </div>
            <div>
              <p className="mb-2 text-xs font-medium uppercase tracking-wide text-rose-400">To learn</p>
              <div className="flex flex-wrap gap-2">
                {analysis.missing_skills.map((s) => (
                  <span key={s} className="pill border-rose-400/30 bg-rose-500/10 text-rose-300">{s}</span>
                ))}
              </div>
            </div>
          </div>
          <Link href="/interview" className="btn-ghost mt-5">
            <MessagesSquare size={16} /> Practice with a mock interview
          </Link>
        </div>
      </div>

      <div className="mt-8 flex items-center justify-between">
        <h2 className="text-lg font-semibold text-white">Your learning roadmap</h2>
        <span className="text-sm text-slate-400">{done}/{analysis.roadmap.length} done · {pct}%</span>
      </div>
      <div className="mb-6 mt-2 h-2 w-full rounded-full bg-white/10">
        <div className="h-full rounded-full bg-gradient-to-r from-brand-500 to-accent-400 transition-all" style={{ width: `${pct}%` }} />
      </div>

      <ol className="space-y-4">
        {analysis.roadmap.map((item) => {
          const isDone = item.status === "done";
          return (
            <li key={item.order} className={`card transition-colors ${isDone ? "border-emerald-400/20" : ""}`}>
              <div className="flex items-start gap-4">
                <button onClick={() => toggle(item)}
                  className={`mt-0.5 grid h-7 w-7 shrink-0 place-items-center rounded-full border transition-colors ${
                    isDone ? "border-emerald-400 bg-emerald-500/20 text-emerald-300" : "border-white/20 text-slate-500 hover:border-brand-400"
                  }`}>
                  {isDone ? <Check size={15} /> : <Circle size={15} />}
                </button>
                <div className="flex-1">
                  <div className="flex items-center gap-2">
                    <span className="text-xs text-slate-500">Step {item.order}</span>
                    <h3 className={`font-medium ${isDone ? "text-slate-400 line-through" : "text-white"}`}>{item.topic}</h3>
                  </div>
                  <p className="mt-1 text-sm text-slate-400">{item.why}</p>
                  <div className="mt-3 flex flex-wrap gap-2">
                    {item.resources?.map((r) => (
                      <span key={r} className="pill border-white/10 bg-white/5 text-slate-300">
                        <ExternalLink size={12} className="mr-1.5" /> {r}
                      </span>
                    ))}
                  </div>
                  <p className="mt-3 text-sm"><span className="text-brand-400">Milestone:</span> <span className="text-slate-300">{item.milestone}</span></p>
                </div>
              </div>
            </li>
          );
        })}
      </ol>
    </Protected>
  );
}
