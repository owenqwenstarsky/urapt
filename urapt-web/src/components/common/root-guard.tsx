import { Navigate } from "react-router-dom";
import { FullPageSpinner } from "@/components/common/spinner";
import { AppShell } from "@/components/app-shell";
import { useAuthStore } from "@/stores/auth";
import { useMe, useServerInfo } from "@/hooks/use-server";

/**
 * Root guard: boots server info + current session before rendering the app
 * shell. Redirects to /setup on first run and /login when unauthenticated.
 */
export function RootGuard() {
  const token = useAuthStore((s) => s.token);
  const serverInfo = useServerInfo();
  const me = useMe(!!token);

  if (serverInfo.isLoading || serverInfo.isPending) {
    return <FullPageSpinner label="Connecting to urapt…" />;
  }

  // First-run setup takes priority: no admin exists yet.
  if (serverInfo.data?.needs_setup) {
    return <Navigate to="/setup" replace />;
  }

  if (!token) {
    return <Navigate to="/login" replace />;
  }

  if (me.isLoading || me.isPending) {
    return <FullPageSpinner label="Loading session…" />;
  }

  // Token present but invalid (401 already cleared the store; handle races).
  if (me.isError || !me.data) {
    return <Navigate to="/login" replace />;
  }

  return <AppShell />;
}
