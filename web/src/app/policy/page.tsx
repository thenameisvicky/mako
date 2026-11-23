"use client";

import { FormEvent, useState } from "react";
import { Shell } from "@/components/Shell";
import { Card, Field, Notice, PageHead, Spinner } from "@/components/ui";
import { api } from "@/lib/api";
import type { Policy } from "@/lib/types";
import { errorText, useLoad } from "@/lib/use-load";

export default function PolicyPage() {
  return (
    <Shell>
      <PolicyEditor />
    </Shell>
  );
}

const toList = (s: string) => s.split(/[\n,]/).map((v) => v.trim()).filter(Boolean);

function PolicyEditor() {
  const { data: policy, setData, error, loading } = useLoad(api.getPolicy);
  const [result, setResult] = useState<{ ok: boolean; msg: string } | null>(null);
  const [saving, setSaving] = useState(false);

  async function save(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = new FormData(e.currentTarget);
    const max = Number(f.get("max"));
    if (!Number.isInteger(max) || max < 0) return setResult({ ok: false, msg: "Max tokens must be 0 or a positive whole number." });

    setSaving(true);
    try {
      setData(
        await api.savePolicy({
          allowedModels: toList(String(f.get("allowed"))),
          blockedModels: toList(String(f.get("blocked"))),
          maxTokensPerRequest: max,
        }),
      );
      setResult({ ok: true, msg: "Saved. Applies to the next proxied request." });
    } catch (err) {
      setResult({ ok: false, msg: errorText(err) });
    } finally {
      setSaving(false);
    }
  }

  if (loading) return <Spinner />;
  const p: Partial<Policy> = policy ?? {};

  return (
    <>
      <PageHead eyebrow="Policy" title="Model rules">
        Evaluated by the Mako proxy on every request, from memory.
      </PageHead>

      <form className="stack" onSubmit={save}>
        {error && <Notice>{error}</Notice>}
        {result && <Notice ok={result.ok}>{result.msg}</Notice>}
        {policy && !policy.configured && <Notice>No policy yet — every model is allowed.</Notice>}

        <Card>
          <div className="stack">
            <ListArea name="allowed" label="Allowed models" hint="one per line, empty allows all" value={p.allowedModels} placeholder={"claude-sonnet-4-5\nclaude-haiku-4-5"} />
            <ListArea name="blocked" label="Blocked models" hint="overrides the allow list" value={p.blockedModels} placeholder="claude-opus-4" />
            <Field name="max" label="Max tokens per request" hint="0 is unlimited" type="number" min={0} defaultValue={p.maxTokensPerRequest ?? 0} />
          </div>
        </Card>

        <div>
          <button className="btn btn-primary" disabled={saving}>
            {saving ? "Saving…" : "Save policy"}
          </button>
        </div>
      </form>
    </>
  );
}

function ListArea({ name, label, hint, value, placeholder }: {
  name: string;
  label: string;
  hint: string;
  value?: string[];
  placeholder: string;
}) {
  return (
    <label className="field">
      <span className="field-label">
        {label} <span className="field-hint">· {hint}</span>
      </span>
      {/* Keyed on the saved value so a save re-seeds the uncontrolled textarea. */}
      <textarea key={value?.join()} className="input" name={name} rows={3} defaultValue={value?.join("\n")} placeholder={placeholder} />
    </label>
  );
}
