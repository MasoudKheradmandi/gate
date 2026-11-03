import { useCallback, useEffect, useRef, useState } from "react";
import { api, isMissing } from "../api";

export interface Resource<T> {
  data: T | null;
  error: unknown;
  loading: boolean; // first load in flight
  missing: boolean; // endpoint not implemented (404/405/501)
  reload: () => void;
}

/**
 * GET a JSON resource. Optional polling (ms) pauses while the tab is hidden and
 * refreshes immediately when it becomes visible again. Stale data is kept on refresh errors.
 */
export function useResource<T>(path: string | null, opts: { poll?: number } = {}): Resource<T> {
  const { poll = 0 } = opts;
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [loading, setLoading] = useState(path !== null);
  const [tick, setTick] = useState(0);
  const pathRef = useRef(path);
  pathRef.current = path;

  useEffect(() => {
    if (path === null) { setLoading(false); return; }
    let alive = true;
    let inflight = false;
    const run = async () => {
      if (inflight) return;
      inflight = true;
      try {
        const d = await api.get<T>(path);
        if (alive && pathRef.current === path) { setData(d); setError(null); }
      } catch (e) {
        if (alive && pathRef.current === path) setError(e);
      } finally {
        inflight = false;
        if (alive) setLoading(false);
      }
    };
    run();
    let id = 0;
    if (poll > 0) id = window.setInterval(() => { if (!document.hidden) run(); }, poll);
    const onVis = () => { if (!document.hidden && poll > 0) run(); };
    document.addEventListener("visibilitychange", onVis);
    return () => { alive = false; if (id) window.clearInterval(id); document.removeEventListener("visibilitychange", onVis); };
  }, [path, poll, tick]);

  const reload = useCallback(() => setTick((x) => x + 1), []);
  return { data, error, loading, missing: isMissing(error), reload };
}
