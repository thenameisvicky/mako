"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { ReactNode, useEffect, useState } from "react";
import { api } from "@/lib/api";
import { clearSession, getStoredUsername, isSignedIn } from "@/lib/auth";
import { Spinner } from "./ui";

const DOCS_URL = "https://github.com/thenameisvicky/tokenly-mako#readme";

// Feather-style paths, matching the icons on the landing page.
const ICONS = {
  sessions: <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />,
  policy: <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />,
  limits: <><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3 2" /></>,
  usage: <><path d="M3 3v18h18" /><path d="m7 15 4-4 3 3 5-6" /></>,
  lock: <><rect x="5" y="11" width="14" height="10" rx="2" /><path d="M8 11V7a4 4 0 0 1 8 0v4" /></>,
  docs: <><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" /><path d="M14 2v6h6M8 13h8M8 17h5" /></>,
  team: <><circle cx="9" cy="8" r="3.5" /><path d="M2.5 20a6.5 6.5 0 0 1 13 0M16 4.5a3.5 3.5 0 0 1 0 7M21.5 20a6.5 6.5 0 0 0-4-6" /></>,
  signout: <><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" /><path d="m16 17 5-5-5-5M21 12H9" /></>,
};

export const Icon = ({ name }: { name: keyof typeof ICONS }) => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
    {ICONS[name]}
  </svg>
);

const NAV = [{ href: "/", label: "Sessions", icon: "sessions" }] as const;
const SOON = [
  { label: "Policies", icon: "policy" },
  { label: "Token limits", icon: "limits" },
  { label: "Usage & latency", icon: "usage" },
] as const;

export function Shell({ children }: { children: ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();
  const [user, setUser] = useState<{ name: string; team: string } | null>(null);

  // Auth gate: render nothing but a spinner until we know the visitor is signed in.
  useEffect(() => {
    if (!isSignedIn()) return router.replace("/login");
    setUser({ name: getStoredUsername() ?? "", team: "" });
    api
      .me()
      .then((me) => setUser({ name: me.username, team: me.teamName }))
      .catch(() => router.replace("/login"));
  }, [router]);

  if (!user) return <Spinner />;

  const page = NAV.find((n) => n.href === pathname)?.label ?? "";
  const signOut = () => {
    clearSession();
    router.replace("/login");
  };

  return (
    <div className="console">
      <aside className="sidebar">
        <Link className="logo" href="/">
          <span className="logo-mark" />
          Mako
        </Link>

        <div className="team-card">
          <span className="team-icon"><Icon name="team" /></span>
          <span className="team-name">
            <span className="faint small">Team</span>
            {user.team || "—"}
          </span>
        </div>

        <nav className="side-nav" aria-label="Main">
          {NAV.map((n) => (
            <Link key={n.href} href={n.href} className="side-item" aria-current={pathname === n.href ? "page" : undefined}>
              <Icon name={n.icon} />
              {n.label}
            </Link>
          ))}

          <span className="side-label">Coming soon</span>
          {SOON.map((n) => (
            <span key={n.label} className="side-item" aria-disabled="true" title="Coming soon">
              <Icon name={n.icon} />
              {n.label}
              <span className="side-lock"><Icon name="lock" /></span>
            </span>
          ))}
        </nav>

        <a className="side-item side-docs" href={DOCS_URL} target="_blank" rel="noopener">
          <Icon name="docs" />
          Documentation
        </a>
      </aside>

      <div className="console-main">
        <header className="topbar">
          <h2 className="topbar-title">{page}</h2>
          <div className="topbar-actions">
            <a className="topbar-link" href={DOCS_URL} target="_blank" rel="noopener">Docs</a>
            <span className="avatar" title={user.name} aria-label={`Signed in as ${user.name}`}>
              {user.name.slice(0, 1).toUpperCase() || "?"}
            </span>
            <button className="icon-btn" aria-label="Sign out" title="Sign out" onClick={signOut}>
              <Icon name="signout" />
            </button>
          </div>
        </header>
        <main className="content">{children}</main>
      </div>
    </div>
  );
}
