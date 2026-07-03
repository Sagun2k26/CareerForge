"use client";

import Link from "next/link";
import { useAuth } from "@/components/AuthProvider";
import {
  Rocket, FileText, Map, MessagesSquare, BookOpen, Wand2, ArrowRight, Sparkles,
} from "lucide-react";

const features = [
  { icon: FileText, title: "Resume parsing", body: "Upload a PDF or DOCX. We extract a structured profile — skills, experience, education — with an LLM." },
  { icon: Map, title: "Skill-gap + roadmap", body: "Compare your profile to a target role, get a readiness score and an ordered learning plan you can track." },
  { icon: MessagesSquare, title: "Scored mock interviews", body: "A stateful interviewer asks one question at a time and scores clarity, correctness, and depth." },
  { icon: BookOpen, title: "Role-specific question bank", body: "Browse and semantically search curated prep questions, mark them practiced, and get model answers." },
  { icon: Wand2, title: "JD-tailored resume", body: "Paste a job description and get your bullets rewritten and reordered to match — before/after view." },
  { icon: Sparkles, title: "Grounded with RAG", body: "Retrieval over role and question embeddings keeps answers relevant and reduces hallucination." },
];

export default function Landing() {
  const { user } = useAuth();
  const cta = user ? "/dashboard" : "/register";

  return (
    <div className="mx-auto max-w-6xl px-6">
      <header className="flex items-center justify-between py-6">
        <div className="flex items-center gap-2">
          <span className="grid h-9 w-9 place-items-center rounded-xl bg-brand-500 shadow-glow">
            <Rocket size={18} className="text-white" />
          </span>
          <span className="text-lg font-semibold text-white">CareerForge</span>
        </div>
        <div className="flex items-center gap-3">
          {user ? (
            <Link href="/dashboard" className="btn-primary">Dashboard</Link>
          ) : (
            <>
              <Link href="/login" className="btn-ghost">Sign in</Link>
              <Link href="/register" className="btn-primary">Get started</Link>
            </>
          )}
        </div>
      </header>

      <section className="py-20 text-center">
        <div className="animate-fade-up">
          <span className="pill border-brand-400/30 bg-brand-500/10 text-brand-400">
            <Sparkles size={13} className="mr-1.5" /> AI-powered career prep
          </span>
          <h1 className="mx-auto mt-6 max-w-3xl text-balance text-5xl font-bold leading-tight text-white sm:text-6xl">
            Land the role with an{" "}
            <span className="bg-gradient-to-r from-brand-400 to-accent-400 bg-clip-text text-transparent">
              AI growth coach
            </span>
          </h1>
          <p className="mx-auto mt-6 max-w-2xl text-lg text-slate-400">
            Upload your resume, choose a target role, and get a scored skill gap, a personalized
            roadmap, scored mock interviews, and a question bank to prepare from — all in one place.
          </p>
          <div className="mt-8 flex items-center justify-center gap-3">
            <Link href={cta} className="btn-primary px-6 py-3 text-base">
              Start free <ArrowRight size={18} />
            </Link>
            <Link href="/login" className="btn-ghost px-6 py-3 text-base">I have an account</Link>
          </div>
        </div>
      </section>

      <section className="grid gap-5 pb-24 sm:grid-cols-2 lg:grid-cols-3">
        {features.map(({ icon: Icon, title, body }, i) => (
          <div key={title} className="card animate-fade-up" style={{ animationDelay: `${i * 60}ms` }}>
            <span className="grid h-11 w-11 place-items-center rounded-xl bg-brand-500/15 text-brand-400">
              <Icon size={20} />
            </span>
            <h3 className="mt-4 text-lg font-semibold text-white">{title}</h3>
            <p className="mt-2 text-sm text-slate-400">{body}</p>
          </div>
        ))}
      </section>
    </div>
  );
}
