import type { ReactNode } from "react";
import { RepoCtx, type RepoContextValue } from "./repo-context";

export function RepoProvider({
  value,
  children,
}: {
  value: RepoContextValue;
  children: ReactNode;
}) {
  return <RepoCtx.Provider value={value}>{children}</RepoCtx.Provider>;
}
