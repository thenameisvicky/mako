"use client";

import { useMemo, useState } from "react";
import { Shell } from "@/components/Shell";
import { ago, Card, Notice, PageHead, Spinner } from "@/components/ui";
import { api } from "@/lib/api";
import type { ChatSession, ChatTranscript } from "@/lib/types";
import { errorText, useLoad } from "@/lib/use-load";

export default function SessionsPage() {
  return (
    <Shell>
      <Sessions />
    </Shell>
  );
}

interface Turn {
  role: "user" | "assistant";
  text: string;
}

/** Keeps the user and assistant text turns of a Claude Code .jsonl transcript. */
function parseTranscript(raw: string): Turn[] {
  const turns: Turn[] = [];
  for (const line of raw.split("\n")) {
    let rec: { type?: string; message?: { content?: unknown } };
    try {
      rec = JSON.parse(line);
    } catch {
      continue;
    }
    if (rec.type !== "user" && rec.type !== "assistant") continue;
    const c = rec.message?.content;
    const text = (
      typeof c === "string"
        ? c
        : Array.isArray(c)
          ? c.filter((b) => b?.type === "text").map((b) => b.text).join("\n")
          : ""
    ).trim();
    if (text) turns.push({ role: rec.type, text });
  }
  return turns;
}

const repoName = (path: string) => path.split("/").filter(Boolean).pop() ?? path;
const importCommand = (id: string) => `/local-chat-import ${id.slice(0, 8)}`;

function Sessions() {
  const { data: sessions, error, loading } = useLoad(api.chats);
  const [open, setOpen] = useState<ChatTranscript | null>(null);
  const [busyId, setBusyId] = useState("");
  const [note, setNote] = useState("");

  async function read(id: string) {
    setBusyId(id);
    setNote("");
    try {
      setOpen(await api.chat(id));
    } catch (e) {
      setNote(errorText(e));
    } finally {
      setBusyId("");
    }
  }

  async function copy(id: string) {
    try {
      await navigator.clipboard.writeText(importCommand(id));
      setNote(`Copied — run ${importCommand(id)} in mako, then claude --resume.`);
    } catch {
      setNote(`Run ${importCommand(id)} in mako, then claude --resume.`);
    }
  }

  if (loading) return <Spinner />;
  if (open) return <Reader session={open} onBack={() => setOpen(null)} onContinue={() => copy(open.sessionId)} note={note} />;

  const list = sessions ?? [];
  const mine = list.filter((s) => s.mine).length;

  return (
    <>
      <PageHead eyebrow="Shared sessions" title="Pick up where your team left off">
        Claude Code sessions synced from mako. Encrypted at rest and visible only to your team.
      </PageHead>

      <div className="stack">
        {(error || note) && <Notice ok={!error && note.startsWith("Copied")}>{error || note}</Notice>}

        <div className="stats">
          <Stat label="Shared sessions" value={list.length} />
          <Stat label="Synced by you" value={mine} />
          <Stat label="By teammates" value={list.length - mine} />
        </div>

        {list.length ? (
          <section className="card card-flush">
            <div className="table-wrap">
              <table className="table">
                <thead>
                  <tr>
                    <th>Session</th>
                    <th>Repository</th>
                    <th>Owner</th>
                    <th className="num">Messages</th>
                    <th>Last synced</th>
                    <th aria-label="Actions" />
                  </tr>
                </thead>
                <tbody>
                  {list.map((s) => (
                    <Row key={s.sessionId} s={s} busy={busyId === s.sessionId} onOpen={() => read(s.sessionId)} onContinue={() => copy(s.sessionId)} />
                  ))}
                </tbody>
              </table>
            </div>
          </section>
        ) : (
          !error && <Quickstart />
        )}
      </div>
    </>
  );
}

const Stat = ({ label, value }: { label: string; value: number }) => (
  <div className="card stat">
    <span className="small muted">{label}</span>
    <span className="stat-value">{value}</span>
  </div>
);

function Row({ s, busy, onOpen, onContinue }: {
  s: ChatSession;
  busy: boolean;
  onOpen: () => void;
  onContinue: () => void;
}) {
  return (
    <tr>
      <td>
        <div className="cell-title">
          <button className="link-title" onClick={onOpen} disabled={busy}>
            {busy ? "Opening…" : s.title || "Untitled session"}
          </button>
          <span className="mono faint">{s.sessionId.slice(0, 8)}</span>
        </div>
      </td>
      <td className="mono">{repoName(s.projectPath)}</td>
      <td>{s.mine ? <span className="tag tag-ok">you</span> : <span className="tag">teammate</span>}</td>
      <td className="num mono">{s.messageCount}</td>
      <td className="muted">{ago(s.syncedAt)}</td>
      <td className="actions">
        <button className="btn btn-sm" onClick={onContinue}>Continue</button>
      </td>
    </tr>
  );
}

/** Shown before anything is synced: the three steps to share a first session. */
function Quickstart() {
  return (
    <Card title="Share your first session">
      <ol className="steps">
        <li>
          <span>Sign in from the CLI</span>
          <code className="cmd">mako → /login</code>
        </li>
        <li>
          <span>Upload this machine&apos;s Claude Code sessions</span>
          <code className="cmd">/local-chat-sync</code>
        </li>
        <li>
          <span>On another machine, import one and resume it</span>
          <code className="cmd">/local-chat-import &lt;id&gt; → claude --resume</code>
        </li>
      </ol>
    </Card>
  );
}

function Reader({ session, onBack, onContinue, note }: {
  session: ChatTranscript;
  onBack: () => void;
  onContinue: () => void;
  note: string;
}) {
  const turns = useMemo(() => parseTranscript(session.transcript), [session]);

  return (
    <>
      <button className="link-btn small" onClick={onBack}>
        ← All sessions
      </button>
      <PageHead eyebrow={session.sessionId.slice(0, 8)} title={session.title || "Untitled session"}>
        <span className="mono">{session.projectPath}</span>
      </PageHead>

      <div className="stack">
        {note && <Notice ok={note.startsWith("Copied")}>{note}</Notice>}
        <Card
          title={`${turns.length} turns`}
          aside={
            <button className="btn btn-sm btn-primary" onClick={onContinue}>
              Continue on my machine
            </button>
          }
        >
          <div className="stack scroll">
            {turns.map((t, i) => (
              <div key={i} className={`turn turn-${t.role}`}>
                <span className="turn-role">{t.role === "user" ? "Developer" : "Claude"}</span>
                <pre>{t.text}</pre>
              </div>
            ))}
            {!turns.length && <p className="empty">This transcript has no readable turns.</p>}
          </div>
        </Card>
      </div>
    </>
  );
}
