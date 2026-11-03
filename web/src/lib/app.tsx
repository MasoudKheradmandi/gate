import { createContext, useCallback, useContext, useEffect, type ReactNode } from "react";
import type { Overview, User } from "../api";
import { useResource } from "../hooks/useResource";

interface AppState {
  user: User;
  setUser: (u: User) => void;
  overview: Overview | null;
  overviewError: unknown;
  refreshOverview: () => void;
  logout: () => void;
}

const Ctx = createContext<AppState>(null!);

/** Holds the signed-in user and the shared, polled /api/overview (visibility-aware). */
export function AppProvider({ user, setUser, logout, children }: { user: User; setUser: (u: User) => void; logout: () => void; children: ReactNode }) {
  const ov = useResource<Overview>("/api/overview", { poll: 4000 });
  const reload = ov.reload;
  const refreshOverview = useCallback(() => reload(), [reload]);
  useEffect(() => {
    window.addEventListener("corvus:refresh", refreshOverview);
    return () => window.removeEventListener("corvus:refresh", refreshOverview);
  }, [refreshOverview]);
  return (
    <Ctx.Provider value={{ user, setUser, overview: ov.data, overviewError: ov.error, refreshOverview, logout }}>
      {children}
    </Ctx.Provider>
  );
}

export const useApp = () => useContext(Ctx);
