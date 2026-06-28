import { useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { logout } from "@/lib/api";
import { useAuthStore } from "@/stores/auth";
import { ME_KEY, SERVER_INFO_KEY } from "@/hooks/use-server";

/** Shared sign-out handler: revokes server-side token, clears local state. */
export function useLogout() {
  const navigate = useNavigate();
  const qc = useQueryClient();
  const clear = useAuthStore((s) => s.clear);

  return async function signOut() {
    try {
      await logout().catch(() => {
        // Best-effort: clear locally even if the server call fails.
      });
    } finally {
      clear();
      qc.setQueryData(ME_KEY, null);
      qc.removeQueries({ queryKey: ["repos"] });
      qc.removeQueries({ queryKey: SERVER_INFO_KEY });
      toast.success("Signed out");
      navigate("/login");
    }
  };
}
