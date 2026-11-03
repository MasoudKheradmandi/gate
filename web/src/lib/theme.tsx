import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from "react";

export type ThemeMode = "system" | "light" | "dark";
const KEY = "corvus.theme";

function readMode(): ThemeMode {
  try {
    const v = localStorage.getItem(KEY);
    if (v === "light" || v === "dark" || v === "system") return v;
  } catch { /* storage unavailable */ }
  return "system";
}

const mq = () => window.matchMedia("(prefers-color-scheme: dark)");
const resolve = (m: ThemeMode): "light" | "dark" => (m === "system" ? (mq().matches ? "dark" : "light") : m);
function apply(m: ThemeMode) { document.documentElement.classList.toggle("dark", resolve(m) === "dark"); }

/** Apply before first paint (called from main.tsx). */
export function initTheme() { apply(readMode()); }

const Ctx = createContext<{ mode: ThemeMode; resolved: "light" | "dark"; setMode: (m: ThemeMode) => void }>(null!);

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [mode, setModeState] = useState<ThemeMode>(readMode);
  const [resolved, setResolved] = useState<"light" | "dark">(() => resolve(readMode()));
  const setMode = useCallback((m: ThemeMode) => {
    setModeState(m);
    try { localStorage.setItem(KEY, m); } catch { /* ignore */ }
  }, []);
  useEffect(() => {
    apply(mode); setResolved(resolve(mode));
    if (mode !== "system") return;
    const q = mq();
    const on = () => { apply("system"); setResolved(resolve("system")); };
    q.addEventListener("change", on);
    return () => q.removeEventListener("change", on);
  }, [mode]);
  return <Ctx.Provider value={{ mode, resolved, setMode }}>{children}</Ctx.Provider>;
}

export const useTheme = () => useContext(Ctx);
