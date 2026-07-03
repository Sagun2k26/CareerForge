"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import Protected from "@/components/Protected";
import { PageHeader, Spinner } from "@/components/ui";
import { api, Role, Session } from "@/lib/api";
import { MessagesSquare, Play } from "lucide-react";

export default function InterviewHome() {
  const router = useRouter();
  const [roles, setRoles] = useState<Role[]>([]);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [roleId, setRoleId] = useState("");
  const [loading, setLoading] = useState(true);
  const [starting, setStarting] = useState(false);

  useEffect(() => {
    Promise.all([api.roles(), api.listInterviews()])
      .then(([r, s]) => { setRoles(r.roles || []); setSessions(s.sessions || []); })
      .finally(() => setLoading(false));
  }, []);

  async function start() {
    if (!roleId) return;
    setStarting(true);
    try {
      const res = await api.startInterview(roleId);
      router.push(`/interview/${res.session.id}`);
    } finally {
      setStarting(false);
    }
  }

  return (
    <Protected>
      <PageHeader title="Mock Interview" subtitle="Practice with an AI interviewer that scores every answer." />

      <div className="card">
        <h2 className="text-lg font-semibold text-white">Start a new interview</h2>
        <p className="mt-1 text-sm text-slate-400">Pick the role you're targeting. Questions are grounded in that role's topics.</p>
        <div className="mt-4 flex flex-col gap-3 sm:flex-row">
          <select className="input sm:flex-1" value={roleId} onChange={(e) => setRoleId(e.target.value)}>
            <option value="">Select a role…</option>
            {roles.map((r) => <option key={r.id} value={r.id}>{r.name}</option>)}
          </select>
          <button className="btn-primary" disabled={!roleId || starting} onClick={start}>
            <Play size={16} /> {starting ? "Starting…" : "Start interview"}
          </button>
        </div>
      </div>

      <h2 className="mb-3 mt-8 text-lg font-semibold text-white">Past sessions</h2>
      {loading ? (
        <Spinner label="Loading…" />
      ) : sessions.length === 0 ? (
        <p className="text-sm text-slate-400">No sessions yet — start your first interview above.</p>
      ) : (
        <div className="grid gap-3 sm:grid-cols-2">
          {sessions.map((s) => (
            <Link key={s.id} href={`/interview/${s.id}`} className="card flex items-center justify-between hover:border-brand-400/30">
              <div className="flex items-center gap-3">
                <span className="grid h-10 w-10 place-items-center rounded-xl bg-brand-500/15 text-brand-400"><MessagesSquare size={18} /></span>
                <div>
                  <p className="font-medium text-white">{s.role_name}</p>
                  <p className="text-xs text-slate-500">{new Date(s.created_at).toLocaleString()}</p>
                </div>
              </div>
              <span className={`pill ${s.status === "complete" ? "border-emerald-400/30 bg-emerald-500/10 text-emerald-300" : "border-amber-400/30 bg-amber-500/10 text-amber-300"}`}>
                {s.status}
              </span>
            </Link>
          ))}
        </div>
      )}
    </Protected>
  );
}
