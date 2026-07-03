"use client";

import { useEffect, useState } from "react";
import Protected from "@/components/Protected";
import { PageHeader, Spinner, EmptyState } from "@/components/ui";
import { api, ModelAnswer, Question, Role } from "@/lib/api";
import { Search, Sparkles, Check, BookOpen, Lightbulb } from "lucide-react";

function QuestionRow({ q, roleName }: { q: Question; roleName: string }) {
  const [practiced, setPracticed] = useState(q.practiced);
  const [answer, setAnswer] = useState<ModelAnswer | null>(null);
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);

  async function togglePracticed() {
    const next = !practiced;
    setPracticed(next);
    await api.markPracticed(q.id, next);
  }

  async function showAnswer() {
    setOpen((v) => !v);
    if (!answer) {
      setLoading(true);
      try { setAnswer(await api.modelAnswer(q.id, roleName)); }
      finally { setLoading(false); }
    }
  }

  return (
    <div className="card">
      <div className="flex items-start justify-between gap-4">
        <div>
          <span className="pill border-white/10 bg-white/5 text-slate-400">{q.topic}</span>
          <p className="mt-2 text-slate-200">{q.prompt}</p>
        </div>
        <button onClick={togglePracticed}
          className={`shrink-0 rounded-full border px-3 py-1.5 text-xs transition-colors ${
            practiced ? "border-emerald-400/40 bg-emerald-500/15 text-emerald-300" : "border-white/15 text-slate-400 hover:border-brand-400"
          }`}>
          {practiced ? <span className="flex items-center gap-1"><Check size={13} /> Practiced</span> : "Mark practiced"}
        </button>
      </div>

      <button className="btn-ghost mt-4" onClick={showAnswer}>
        <Lightbulb size={15} /> {open ? "Hide" : "Show"} model answer
      </button>

      {open && (
        <div className="mt-3 rounded-xl border border-white/5 bg-ink-900/50 p-4">
          {loading ? <Spinner label="Generating model answer…" /> : answer && (
            <>
              <p className="text-sm text-slate-300">{answer.model_answer}</p>
              {answer.key_points?.length > 0 && (
                <>
                  <p className="mt-3 text-xs font-medium text-brand-400">Key points to hit</p>
                  <ul className="mt-1 list-disc space-y-1 pl-5 text-sm text-slate-400">
                    {answer.key_points.map((k, i) => <li key={i}>{k}</li>)}
                  </ul>
                </>
              )}
            </>
          )}
        </div>
      )}
    </div>
  );
}

export default function QuestionsPage() {
  const [roles, setRoles] = useState<Role[]>([]);
  const [roleId, setRoleId] = useState("");
  const [questions, setQuestions] = useState<Question[]>([]);
  const [query, setQuery] = useState("");
  const [loading, setLoading] = useState(false);
  const [searching, setSearching] = useState(false);

  const roleName = roles.find((r) => r.id === roleId)?.name || "";

  useEffect(() => {
    api.roles().then((r) => {
      setRoles(r.roles || []);
      if (r.roles?.length) setRoleId(r.roles[0].id);
    });
  }, []);

  useEffect(() => {
    if (!roleId) return;
    setLoading(true);
    api.listQuestions(roleId).then((q) => setQuestions(q.questions || [])).finally(() => setLoading(false));
  }, [roleId]);

  async function runSearch(e: React.FormEvent) {
    e.preventDefault();
    if (!query.trim()) return;
    setSearching(true);
    try {
      const res = await api.searchQuestions(query, roleId || undefined);
      setQuestions(res.questions || []);
    } finally {
      setSearching(false);
    }
  }

  return (
    <Protected>
      <PageHeader title="Question Bank" subtitle="Browse role-specific prep, search semantically, and study model answers." />

      <div className="card mb-6 space-y-4">
        <div className="flex flex-col gap-3 sm:flex-row">
          <select className="input sm:w-72" value={roleId} onChange={(e) => { setQuery(""); setRoleId(e.target.value); }}>
            {roles.map((r) => <option key={r.id} value={r.id}>{r.name}</option>)}
          </select>
          <form onSubmit={runSearch} className="flex flex-1 gap-2">
            <div className="relative flex-1">
              <Search size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-500" />
              <input className="input pl-9" placeholder="Semantic search, e.g. 'how to scale a database'"
                value={query} onChange={(e) => setQuery(e.target.value)} />
            </div>
            <button className="btn-primary" disabled={searching}><Sparkles size={16} /> {searching ? "…" : "Search"}</button>
          </form>
        </div>
      </div>

      {loading ? (
        <Spinner label="Loading questions…" />
      ) : questions.length === 0 ? (
        <EmptyState title="No questions found" body="Try a different role or search term." />
      ) : (
        <div className="grid gap-4">
          {questions.map((q) => <QuestionRow key={q.id} q={q} roleName={roleName} />)}
        </div>
      )}
    </Protected>
  );
}
