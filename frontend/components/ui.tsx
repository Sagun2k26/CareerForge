"use client";

import { ReactNode } from "react";
import { Loader2 } from "lucide-react";

export function Spinner({ label }: { label?: string }) {
  return (
    <div className="flex items-center gap-2 text-slate-400">
      <Loader2 className="animate-spin" size={18} />
      {label && <span className="text-sm">{label}</span>}
    </div>
  );
}

export function PageHeader({ title, subtitle, action }: { title: string; subtitle?: string; action?: ReactNode }) {
  return (
    <div className="mb-8 flex items-end justify-between gap-4">
      <div>
        <h1 className="text-2xl font-semibold text-white">{title}</h1>
        {subtitle && <p className="mt-1 text-sm text-slate-400">{subtitle}</p>}
      </div>
      {action}
    </div>
  );
}

export function EmptyState({ title, body, action }: { title: string; body: string; action?: ReactNode }) {
  return (
    <div className="card flex flex-col items-center py-14 text-center">
      <h3 className="text-lg font-medium text-white">{title}</h3>
      <p className="mt-2 max-w-md text-sm text-slate-400">{body}</p>
      {action && <div className="mt-5">{action}</div>}
    </div>
  );
}

// A circular score gauge (0-100).
export function GapRing({ score, size = 120 }: { score: number; size?: number }) {
  const r = size / 2 - 10;
  const c = 2 * Math.PI * r;
  const offset = c - (score / 100) * c;
  const hue = score >= 75 ? "#22d3ee" : score >= 50 ? "#8b7cff" : "#f472b6";
  return (
    <svg width={size} height={size} className="-rotate-90">
      <circle cx={size / 2} cy={size / 2} r={r} stroke="rgba(255,255,255,0.08)" strokeWidth="10" fill="none" />
      <circle
        cx={size / 2} cy={size / 2} r={r} stroke={hue} strokeWidth="10" fill="none"
        strokeDasharray={c} strokeDashoffset={offset} strokeLinecap="round"
        style={{ transition: "stroke-dashoffset 0.8s ease" }}
      />
      <text x="50%" y="50%" dy="0.35em" textAnchor="middle" className="rotate-90" fill="white" fontSize="22" fontWeight="600" transform={`rotate(90 ${size / 2} ${size / 2})`}>
        {score}
      </text>
    </svg>
  );
}

export function Bar({ value, max = 10 }: { value: number; max?: number }) {
  return (
    <div className="h-2 w-full rounded-full bg-white/10">
      <div className="h-full rounded-full bg-brand-500" style={{ width: `${(value / max) * 100}%` }} />
    </div>
  );
}
