"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useAuth } from "./AuthProvider";
import { LayoutDashboard, FileText, Map, MessagesSquare, BookOpen, LogOut, Rocket } from "lucide-react";

const links = [
  { href: "/dashboard", label: "Dashboard", icon: LayoutDashboard },
  { href: "/resume", label: "Resume", icon: FileText },
  { href: "/analyses", label: "Roadmaps", icon: Map },
  { href: "/interview", label: "Mock Interview", icon: MessagesSquare },
  { href: "/questions", label: "Question Bank", icon: BookOpen },
];

export default function Nav() {
  const { user, logout } = useAuth();
  const pathname = usePathname();
  const router = useRouter();

  if (!user) return null;

  return (
    <aside className="sticky top-0 hidden h-screen w-64 shrink-0 flex-col border-r border-white/5 bg-ink-900/60 p-5 md:flex">
      <Link href="/dashboard" className="mb-8 flex items-center gap-2 px-2">
        <span className="grid h-9 w-9 place-items-center rounded-xl bg-brand-500 shadow-glow">
          <Rocket size={18} className="text-white" />
        </span>
        <span className="text-lg font-semibold text-white">CareerForge</span>
      </Link>

      <nav className="flex flex-1 flex-col gap-1">
        {links.map(({ href, label, icon: Icon }) => {
          const active = pathname === href || pathname.startsWith(href + "/");
          return (
            <Link
              key={href}
              href={href}
              className={`flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm transition-colors ${
                active ? "bg-brand-500/15 text-white" : "text-slate-400 hover:bg-white/5 hover:text-slate-200"
              }`}
            >
              <Icon size={18} />
              {label}
            </Link>
          );
        })}
      </nav>

      <div className="mt-auto border-t border-white/5 pt-4">
        <p className="px-3 text-xs text-slate-500">Signed in as</p>
        <p className="truncate px-3 text-sm text-slate-300">{user.email}</p>
        <button
          onClick={() => { logout(); router.push("/login"); }}
          className="mt-3 flex w-full items-center gap-2 rounded-xl px-3 py-2 text-sm text-slate-400 hover:bg-white/5 hover:text-slate-200"
        >
          <LogOut size={16} /> Sign out
        </button>
      </div>
    </aside>
  );
}
