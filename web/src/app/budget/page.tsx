"use client";

import { FormEvent, useEffect, useState } from "react";
import { Shell } from "@/components/Shell";
import { Card, Field, Meter, Notice, PageHead, Spinner } from "@/components/ui";
import { api } from "@/lib/api";
import type { Budget } from "@/lib/types";
import { errorText, useLoad } from "@/lib/use-load";

const PERIODS: Budget["period"][] = ["daily", "weekly", "monthly"];

export default function BudgetPage() {
  return (
    <Shell>
      <BudgetEditor />
    </Shell>
  );
}

function BudgetEditor() {
  const { data, setData, error, loading } = useLoad(() => Promise.all([api.getBudget(), api.usage()]));
  const [period, setPeriod] = useState<Budget["period"]>("monthly");
  const [limit, setLimit] = useState("0");
  const [result, setResult] = useState<{ ok: boolean; msg: string } | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!data) return;
    setPeriod(data[0].period || "monthly");
    setLimit(String(data[0].tokenLimit ?? 0));
  }, [data]);

  async function save(e: FormEvent) {
    e.preventDefault();
    const tokenLimit = Number(limit);
    if (!Number.isInteger(tokenLimit) || tokenLimit < 0) return setResult({ ok: false, msg: "Token limit must be 0 or a positive whole number." });

    setSaving(true);
    try {
      setData(await Promise.all([api.saveBudget({ period, tokenLimit }), api.usage()]));
      setResult({ ok: true, msg: "Budget saved." });
    } catch (err) {
      setResult({ ok: false, msg: errorText(err) });
    } finally {
      setSaving(false);
    }
  }

  if (loading) return <Spinner />;
  const [budget, usage] = data ?? [];

  return (
    <>
      <PageHead eyebrow="Budget" title="Token limits">
        Checked atomically in memory by the Mako proxy. Over the limit, requests stop before they reach the provider.
      </PageHead>

      <form className="stack" onSubmit={save}>
        {error && <Notice>{error}</Notice>}
        {result && <Notice ok={result.ok}>{result.msg}</Notice>}
        {budget && !budget.configured && <Notice>No budget yet — usage is unlimited.</Notice>}

        <Card title="This period" aside={usage?.period}>
          <Meter used={usage?.usage.totalTokens ?? 0} limit={Number(limit) || 0} />
        </Card>

        <Card>
          <div className="stack">
            <div className="field">
              <span className="field-label">Reset period</span>
              <div className="segmented">
                {PERIODS.map((p) => (
                  <button key={p} type="button" aria-pressed={period === p} onClick={() => setPeriod(p)}>
                    {p[0].toUpperCase() + p.slice(1)}
                  </button>
                ))}
              </div>
            </div>
            <Field label="Token limit" hint="0 is unlimited" type="number" min={0} value={limit} onChange={(e) => setLimit(e.target.value)} />
          </div>
        </Card>

        <div>
          <button className="btn btn-primary" disabled={saving}>
            {saving ? "Saving…" : "Save budget"}
          </button>
        </div>
      </form>
    </>
  );
}
