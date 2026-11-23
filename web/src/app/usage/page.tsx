"use client";

import { useEffect, useState } from "react";
import { Shell } from "@/components/Shell";
import { Card, Meter, Notice, PageHead, Spinner } from "@/components/ui";
import { api, PROXY_URL } from "@/lib/api";
import type { Health } from "@/lib/types";
import { useLoad } from "@/lib/use-load";

export default function UsagePage() {
  return (
    <Shell>
      <Usage />
    </Shell>
  );
}

function Usage() {
  const { data, error, loading } = useLoad(() => Promise.all([api.usage(), api.getBudget()]));
  const [health, setHealth] = useState<Health | null>(null);

  // Health is public and cheap, so poll it for a live latency readout.
  useEffect(() => {
    let live = true;
    const poll = () => api.health().then((h) => live && setHealth(h)).catch(() => live && setHealth(null));
    poll();
    const id = setInterval(poll, 5000);
    return () => {
      live = false;
      clearInterval(id);
    };
  }, []);

  if (loading) return <Spinner />;
  const [usage, budget] = data ?? [];
  const u = usage?.usage;
  const daily = u?.daily ?? [];
  const peak = Math.max(1, ...daily.map((d) => d.tokens));

  return (
    <>
      <PageHead eyebrow="Usage" title="Tokens and latency">
        Counted from response streams by the Mako proxy. Prompts are never logged.
      </PageHead>

      <div className="stack">
        {error && <Notice>{error}</Notice>}

        <div className="cols">
          <Stat label="Total tokens" value={u?.totalTokens} />
          <Stat label="Input" value={u?.inputTokens} />
          <Stat label="Output" value={u?.outputTokens} />
          <Stat label="Requests" value={u?.requests} />
        </div>

        <div className="cols">
          <Card title={`Budget · ${usage?.period ?? "monthly"}`}>
            <Meter used={u?.totalTokens ?? 0} limit={budget?.tokenLimit ?? 0} />
          </Card>

          <Card
            title="Proxy"
            aside={
              <span className="row" style={{ gap: 8 }}>
                <span className={health ? "dot dot-live" : "dot"} style={health ? undefined : { background: "var(--faint)" }} />
                {health ? "live" : "unreachable"}
              </span>
            }
          >
            {health ? (
              <>
                <div className="kv"><span>p50</span><span>{health.latency.p50}</span></div>
                <div className="kv"><span>p95</span><span>{health.latency.p95}</span></div>
                <div className="kv"><span>p99</span><span>{health.latency.p99}</span></div>
              </>
            ) : (
              <p className="muted small">
                Cannot reach <code>{PROXY_URL}</code>.
              </p>
            )}
          </Card>
        </div>

        <div className="cols">
          <Card title="Daily">
            {daily.length ? (
              <div className="bars" role="img" aria-label="Daily token usage">
                {daily.map((d) => (
                  <span
                    key={d.day}
                    title={`${d.day}: ${d.tokens.toLocaleString()} tokens`}
                    style={{ height: `${(d.tokens / peak) * 100}%` }}
                  />
                ))}
              </div>
            ) : (
              <p className="empty">No usage this period.</p>
            )}
          </Card>

          <Card title="By model">
            {u?.byModel.length ? (
              u.byModel.map((m) => (
                <div className="kv" key={m.model}>
                  <span>{m.model}</span>
                  <span>
                    {m.tokens.toLocaleString()} · {m.count} req
                  </span>
                </div>
              ))
            ) : (
              <p className="empty">
                No requests yet. Run <code>/claude</code> in mako.
              </p>
            )}
          </Card>
        </div>
      </div>
    </>
  );
}

function Stat({ label, value = 0 }: { label: string; value?: number }) {
  return (
    <div className="card">
      <div className="stat-value">{value.toLocaleString()}</div>
      <div className="small muted">{label}</div>
    </div>
  );
}
