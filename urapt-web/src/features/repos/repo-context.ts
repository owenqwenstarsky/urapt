import { createContext, useContext } from "react";
import type { Repository, User } from "@/lib/api/types";

export type EffectiveAccess = "read" | "write" | "read-write" | "admin" | "owner" | null;

export interface RepoContextValue {
  repo: Repository;
  me: User | null;
  access: EffectiveAccess;
  canWrite: boolean;
  canManage: boolean;
}

export const RepoCtx = createContext<RepoContextValue | null>(null);

export function useRepoContext(): RepoContextValue {
  const v = useContext(RepoCtx);
  if (!v) throw new Error("useRepoContext must be used within a RepoProvider");
  return v;
}

/** Compute the current user's effective access on a repo. */
export function computeAccess(
  repo: Repository,
  me: User | null,
  myMemberAccess: string | null,
): EffectiveAccess {
  if (!me) return null;
  if (me.is_admin || repo.owner_user_id === me.id) return "owner";
  return (myMemberAccess as EffectiveAccess) ?? null;
}

export function canWriteWith(access: EffectiveAccess): boolean {
  return (
    access === "owner" ||
    access === "admin" ||
    access === "write" ||
    access === "read-write"
  );
}

export function canManageWith(access: EffectiveAccess): boolean {
  return access === "owner" || access === "admin";
}
