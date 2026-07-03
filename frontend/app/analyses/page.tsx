"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import Protected from "@/components/Protected";
import { PageHeader, Spinner, EmptyState } from "@/components/ui";
import { api, Analysis } from "@/lib/api";
import { Map, ArrowUpRight } from "lucide-react";

export default function AnalysesPage() {
  const [analyses, setAnalyses] = useState<Analysis[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.listAnalyses().then((a) => setAnalyses(a.analyses || [])).finally(() => setLoading(false));
  }, []);

  return (
    <Protected>
      <PageHeader title="Roadmaps" subtitle="Your skill-gap analyses and learning plans." />

      {loading ? (
        <Spinner label="Loading…" />
      ) : analyses.length === 0 ? (
        <EmptyState
          title="No roadmaps yet"
          body="Generate a roadmap from a parsed resume and a target role to see your skill gap and a step-by-step learning plan."
          action={<Link href="/resume" className="btn-primary"><Map size={16} /> Go to resumes</Link>}
        />
      ) : (
        <div className="grid gap-4 sm:grid-cols-2">
          {analyses.map((a) => (
            <Link key={a.id} href={`/analyses/${a.id}`} className="card group transition-colors hover:border-brand-400/30">
              <div className="flex items-start justify-between">
                <div>
                  <p className="font-medium text-white">{a.role_name}</p>
                  <p className="mt-1 text-xs text-slate-500">{new Date(a.created_at).toLocaleDateString()}</p>
                </div>
                <ArrowUpRight size={18} className="text-slate-600 transition-colors group-hover:text-brand-400" />
              </div>
              <div className="mt-5 flex items-center gap-3">
                <div className="h-2 flex-1 rounded-full bg-white/10">
                  <div className="h-full rounded-full bg-gradient-to-r from-brand-500 to-accent-400" style={{ width: `${a.gap_score}%` }} />
                </div>
                <span className="text-sm font-semibold text-white">{a.gap_score}%</span>
              </div>
            </Link>
          ))}
        </div>
      )}
    </Protected>
  );
}
