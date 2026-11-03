import { useCallback, useEffect, useState } from "react";
import { api, type User } from "./api";
import { Shell } from "./components/layout/Shell";
import { Logo } from "./components/Logo";
import { AppProvider } from "./lib/app";
import { LoginPage, SetupPage } from "./pages/Auth";

type Phase = { kind: "loading" } | { kind: "setup" } | { kind: "login" } | { kind: "ready"; user: User };

export default function App() {
  const [phase, setPhase] = useState<Phase>({ kind: "loading" });

  const refresh = useCallback(async () => {
    try {
      const st = await api.get<{ setup_required: boolean; version: string }>("/api/status");
      if (st.setup_required) return setPhase({ kind: "setup" });
      const me = await api.get<User>("/api/auth/me");
      setPhase({ kind: "ready", user: me });
    } catch {
      setPhase({ kind: "login" });
    }
  }, []);

  useEffect(() => { refresh(); }, [refresh]);
  useEffect(() => {
    const onUnauth = () => setPhase((p) => (p.kind === "ready" ? { kind: "login" } : p));
    window.addEventListener("corvus:unauthorized", onUnauth);
    return () => window.removeEventListener("corvus:unauthorized", onUnauth);
  }, []);

  if (phase.kind === "loading") {
    return <div className="grid h-dvh place-items-center bg-bg"><Logo className="h-10 w-auto animate-pulse text-fg" /></div>;
  }
  if (phase.kind === "setup") return <SetupPage onDone={(u) => setPhase({ kind: "ready", user: u })} />;
  if (phase.kind === "login") return <LoginPage onDone={() => refresh()} />;
  return (
    <AppProvider user={phase.user} setUser={(u) => setPhase({ kind: "ready", user: u })} logout={() => setPhase({ kind: "login" })}>
      <Shell />
    </AppProvider>
  );
}
