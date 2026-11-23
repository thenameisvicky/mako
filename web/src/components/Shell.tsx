"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { ReactNode, useEffect, useState } from "react";
import { api } from "@/lib/api";
import { clearSession, getStoredUsername, isSignedIn } from "@/lib/auth";
import { Spinner } from "./ui";

const NAV = [
  { href: "/", label: "Sessions" },
  { href: "/usage", label: "Usage" },
  { href: "/policy", label: "Policy" },
  { href: "/budget", label: "Budget" },
];

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

  return (
    <>
      <header className="nav">
        <div className="nav-inner">
          <Link className="logo" href="/">
            <span className="logo-mark" />
            Mako <span className="logo-by">by Tokenly</span>
          </Link>
          <nav className="nav-links">
            {NAV.map((n) => (
              <Link key={n.href} href={n.href} aria-current={pathname === n.href ? "page" : undefined}>
                {n.label}
              </Link>
            ))}
          </nav>
          <div className="nav-user">
            <span>
              {user.name}
              {user.team && <span className="faint"> · {user.team}</span>}
            </span>
            <button
              className="btn btn-sm"
              onClick={() => {
                clearSession();
                router.replace("/login");
              }}
            >
              Sign out
            </button>
          </div>
        </div>
      </header>
      <main>{children}</main>
    </>
  );
}
