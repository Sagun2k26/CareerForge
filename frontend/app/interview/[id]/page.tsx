"use client";

import { useEffect, useRef, useState } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import Protected from "@/components/Protected";
import { Bar, Spinner } from "@/components/ui";
import { api, Score, Session } from "@/lib/api";
import { ArrowLeft, Send, Bot, User as UserIcon } from "lucide-react";

function ScoreCard({ score }: { score: Score }) {
  const rows: [string, number][] = [
    ["Clarity", score.clarity],
    ["Correctness", score.correctness],
    ["Depth", score.depth],
  ];
  return (
    <div className="rounded-xl border border-white/5 bg-ink-900/50 p-4">
      <div className="grid gap-3 sm:grid-cols-3">
        {rows.map(([label, val]) => (
          <div key={label}>
            <div className="mb-1 flex justify-between text-xs text-slate-400">
              <span>{label}</span><span className="text-slate-200">{val}/10</span>
            </div>
            <Bar value={val} />
          </div>
        ))}
      </div>
      <p className="mt-3 text-sm text-slate-300">{score.feedback}</p>
    </div>
  );
}

export default function InterviewRoom() {
  const { id } = useParams<{ id: string }>();
  const [session, setSession] = useState<Session | null>(null);
  const [loading, setLoading] = useState(true);
  const [answer, setAnswer] = useState("");
  const [sending, setSending] = useState(false);
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => { api.getInterview(id).then(setSession).finally(() => setLoading(false)); }, [id]);
  useEffect(() => { bottomRef.current?.scrollIntoView({ behavior: "smooth" }); }, [session?.transcript.length]);

  async function send() {
    if (!session || !answer.trim() || sending) return;
    setSending(true);
    const myAnswer = answer;
    setAnswer("");
    try {
      const res = await api.answerInterview(session.id, myAnswer);
      setSession(res.session);
    } finally {
      setSending(false);
    }
  }

  if (loading) return <Protected><Spinner label="Loading interview…" /></Protected>;
  if (!session) return <Protected><p className="text-slate-400">Not found.</p></Protected>;

  const done = session.status === "complete";
  const avg = session.scores.length
    ? Math.round(session.scores.reduce((s, x) => s + x.clarity + x.correctness + x.depth, 0) / (session.scores.length * 3) * 10) / 10
    : 0;

  return (
    <Protected>
      <Link href="/interview" className="mb-6 inline-flex items-center gap-2 text-sm text-slate-400 hover:text-slate-200">
        <ArrowLeft size={16} /> Back to interviews
      </Link>

      <div className="mb-5 flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-white">{session.role_name}</h1>
          <p className="text-sm text-slate-400">{done ? "Completed" : "In progress"} · {session.scores.length} answered</p>
        </div>
        {session.scores.length > 0 && (
          <span className="pill border-brand-400/30 bg-brand-500/10 text-brand-400">Avg {avg}/10</span>
        )}
      </div>

      <div className="space-y-4">
        {session.transcript.map((t, i) => {
          const isBot = t.role === "interviewer";
          // The score for a candidate turn is the one recorded after that question.
          const scoreIdx = session.transcript.slice(0, i + 1).filter((x) => x.role === "candidate").length - 1;
          const score = !isBot ? session.scores[scoreIdx] : undefined;
          return (
            <div key={i}>
              <div className={`flex gap-3 ${isBot ? "" : "flex-row-reverse"}`}>
                <span className={`grid h-9 w-9 shrink-0 place-items-center rounded-full ${isBot ? "bg-brand-500/20 text-brand-400" : "bg-white/10 text-slate-300"}`}>
                  {isBot ? <Bot size={17} /> : <UserIcon size={17} />}
                </span>
                <div className={`max-w-[80%] rounded-2xl px-4 py-3 text-sm ${isBot ? "bg-ink-800 text-slate-200" : "bg-brand-500 text-white"}`}>
                  {t.content}
                </div>
              </div>
              {score && <div className="ml-12 mt-2"><ScoreCard score={score} /></div>}
            </div>
          );
        })}
        <div ref={bottomRef} />
      </div>

      {done ? (
        <div className="card mt-6 text-center">
          <p className="text-white">Interview complete — great work.</p>
          <p className="mt-1 text-sm text-slate-400">Your average score was {avg}/10 across {session.scores.length} answers.</p>
        </div>
      ) : (
        <div className="sticky bottom-0 mt-6 bg-ink-950/80 py-3 backdrop-blur">
          <div className="flex items-end gap-3">
            <textarea
              className="input min-h-[52px] flex-1 resize-none"
              placeholder="Type your answer…"
              value={answer}
              onChange={(e) => setAnswer(e.target.value)}
              onKeyDown={(e) => { if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) send(); }}
            />
            <button className="btn-primary h-[52px]" disabled={sending || !answer.trim()} onClick={send}>
              <Send size={16} /> {sending ? "Sending…" : "Send"}
            </button>
          </div>
          <p className="mt-2 text-xs text-slate-500">Tip: press ⌘/Ctrl + Enter to send.</p>
        </div>
      )}
    </Protected>
  );
}
