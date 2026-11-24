"use client";

import { DependencyList, useEffect, useState } from "react";

export const errorText = (err: unknown, fallback = "Something went wrong") =>
  err instanceof Error ? err.message : fallback;

/** Runs an async loader on mount (and when deps change), ignoring stale results. */
export function useLoad<T>(load: () => Promise<T>, deps: DependencyList = []) {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let live = true;
    setLoading(true);
    load()
      .then((d) => live && setData(d))
      .catch((e) => live && setError(errorText(e)))
      .finally(() => live && setLoading(false));
    return () => {
      live = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);

  return { data, setData, error, setError, loading };
}
