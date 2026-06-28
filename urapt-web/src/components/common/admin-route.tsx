import type { ReactNode } from "react";
import { Navigate } from "react-router-dom";
import { useAuthStore } from "@/stores/auth";
import { useMe } from "@/hooks/use-server";

/** Wraps admin-only routes; redirects non-admins home. */
export function AdminRoute({ children }: { children: ReactNode }) {
  const token = useAuthStore((s) => s.token);
  const me = useMe(!!token);
  if (me.data && !me.data.is_admin) {
    return <Navigate to="/" replace />;
  }
  return <>{children}</>;
}
