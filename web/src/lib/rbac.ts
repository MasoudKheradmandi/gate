import { useApp } from "./app";
import type { Key } from "../i18n";

export type Role = "admin" | "operator" | "viewer";

export const roleKey: Record<string, Key> = { admin: "roleAdmin", operator: "roleOperator", viewer: "roleViewer" };

/** Role-derived capabilities for the signed-in user. Unknown roles are treated as read-only. */
export function useCan() {
  const role = (useApp().user.role || "admin") as Role;
  const isAdmin = role === "admin";
  return { role, isAdmin, can: isAdmin || role === "operator" };
}
