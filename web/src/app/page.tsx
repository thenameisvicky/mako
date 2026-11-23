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

  return (
    <>
      <PageHead eyebrow="Shared sessions" title="Pick up where your team left off">
        Claude Code sessions synced from mako. Encrypted at rest and visible only to your team.
      </PageHead>

      <div className="stack">
        {(error || note) && <Notice ok={!error && note.startsWith("Copied")}>{error || note}</Notice>}

        {sessions?.length ? (
          <div className="cols">
            {sessions.map((s) => (
              <SessionCard
                key={s.sessionId}
                s={s}
                busy={busyId === s.sessionId}
                onOpen={() => read(s.sessionId)}
                onContinue={() => copy(s.sessionId)}
              />
            ))}
          </div>
        ) : (
          !error && (
            <Card>
              <p className="empty">
                Nothing shared yet. Run <code>/local-chat-sync</code> in mako to share this machine&apos;s sessions.
              </p>
            </Card>
          )
        )}
      </div>
    </>
  );
}

function SessionCard({ s, busy, onOpen, onContinue }: {
  s: ChatSession;
  busy: boolean;
  onOpen: () => void;
  onContinue: () => void;
}) {
  return (
    <article className="card session">
      <div className="row" style={{ justifyContent: "space-between" }}>
        <span className="mono faint">{s.sessionId.slice(0, 8)}</span>
        {s.mine ? <span className="tag tag-ok">yours</span> : <span className="tag">teammate</span>}
      </div>
      <h3>{s.title || "Untitled session"}</h3>
      <div className="meta">
        <span>Repository <b>{repoName(s.projectPath)}</b></span>
        <span>Last synced <b>{ago(s.syncedAt)}</b></span>
        <span><b>{s.messageCount}</b> messages</span>
      </div>
      <div className="row" style={{ marginTop: 6 }}>
        <button className="btn btn-sm" onClick={onOpen} disabled={busy}>
          {busy ? "Opening…" : "Open"}
        </button>
        <button className="btn btn-sm btn-primary" onClick={onContinue}>
          Continue
        </button>
      </div>
    </article>
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
