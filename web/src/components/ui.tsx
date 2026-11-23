import type { InputHTMLAttributes, ReactNode } from "react";

export function PageHead({ eyebrow, title, children }: { eyebrow: string; title: string; children?: ReactNode }) {
  return (
    <header className="page-head">
      <span className="eyebrow">{eyebrow}</span>
      <h1>{title}</h1>
      {children && <p>{children}</p>}
    </header>
  );
}

export function Card({ title, aside, children, className = "" }: {
  title?: string;
  aside?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <section className={`card ${className}`}>
      {(title || aside) && (
        <div className="card-head">
          {title && <h3>{title}</h3>}
          {aside && <span className="faint small">{aside}</span>}
        </div>
      )}
      {children}
    </section>
  );
}

export function Field({ label, hint, ...props }: InputHTMLAttributes<HTMLInputElement> & { label: string; hint?: string }) {
  return (
    <label className="field">
      <span className="field-label">
        {label} {hint && <span className="field-hint">· {hint}</span>}
      </span>
      <input className="input" {...props} />
    </label>
  );
}

export function Notice({ ok, children }: { ok?: boolean; children: ReactNode }) {
  return (
    <div className={`notice ${ok ? "notice-ok" : ""}`} role={ok ? "status" : "alert"}>
      {children}
    </div>
  );
}

export const Spinner = () => <div className="spinner" role="status" aria-label="Loading" />;

export function ago(iso: string) {
  const s = Math.max(0, (Date.now() - new Date(iso).getTime()) / 1000);
  if (s < 60) return "just now";
  const [n, unit] = s < 3600 ? [s / 60, "minute"] : s < 86400 ? [s / 3600, "hour"] : [s / 86400, "day"];
  const v = Math.floor(n);
  return `${v} ${unit}${v === 1 ? "" : "s"} ago`;
}
