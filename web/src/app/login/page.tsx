"use client";

import { useRouter } from "next/navigation";
import { FormEvent, useEffect, useState } from "react";
import { Field, Notice } from "@/components/ui";
import { api } from "@/lib/api";
import { isSignedIn, setSession } from "@/lib/auth";
import { errorText } from "@/lib/use-load";

export default function LoginPage() {
  const router = useRouter();
  const [isCreate, setIsCreate] = useState(false);
  const [createTeam, setCreateTeam] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (isSignedIn()) router.replace("/");
  }, [router]);

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = Object.fromEntries(new FormData(e.currentTarget)) as Record<string, string>;
    setError("");
    setBusy(true);
    try {
      const { token } = await api.login(f.username, f.password, {
        isCreate,
        createTeam,
        teamId: f.teamId,
        teamName: f.teamName,
      });
      setSession(token, f.username);
      router.replace("/");
    } catch (err) {
      setError(errorText(err, "Sign in failed"));
      setBusy(false);
    }
  }

  return (
    <div className="auth">
      <div className="card">
        <div className="logo" style={{ marginBottom: 24 }}>
          <span className="logo-mark" />
          Mako
        </div>
        <h1 style={{ fontSize: 22 }}>{isCreate ? "Create your account" : "Sign in"}</h1>
        <p className="muted small" style={{ margin: "6px 0 24px" }}>
          Same credentials as <code>/login</code> in the mako CLI.
        </p>

        <form className="stack" onSubmit={submit}>
          <Field label="Username" name="username" autoComplete="username" autoFocus required />
          <Field label="Password" name="password" type="password" autoComplete={isCreate ? "new-password" : "current-password"} required />

          {isCreate && (
            <>
              <label className="check">
                <input type="checkbox" checked={createTeam} onChange={(e) => setCreateTeam(e.target.checked)} />
                <span>
                  Create a new team
                  <span className="faint small" style={{ display: "block" }}>You&apos;ll be its admin.</span>
                </span>
              </label>
              {createTeam ? (
                <Field key="name" label="Team name" name="teamName" required />
              ) : (
                <Field key="id" label="Team ID" name="teamId" placeholder="00000000-0000-0000-0000-000000000000" required />
              )}
            </>
          )}

          {error && <Notice>{error}</Notice>}

          <button className="btn btn-primary" disabled={busy} style={{ justifyContent: "center" }}>
            {busy ? "Signing in…" : isCreate ? "Create account" : "Sign in"}
          </button>
          <button
            type="button"
            className="link-btn small"
            onClick={() => {
              setIsCreate(!isCreate);
              setCreateTeam(false);
              setError("");
            }}
          >
            {isCreate ? "Have an account? Sign in" : "New here? Create an account"}
          </button>
        </form>
      </div>
    </div>
  );
}
