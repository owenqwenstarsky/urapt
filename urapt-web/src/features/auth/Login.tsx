import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { AuthLayout } from "./auth-layout";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/common/spinner";
import { ApiError, login } from "@/lib/api";
import { useAuthStore } from "@/stores/auth";

export function Login() {
  const navigate = useNavigate();
  const qc = useQueryClient();
  const setSession = useAuthStore((s) => s.setSession);

  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (busy) return;
    setBusy(true);
    try {
      const res = await login({ username: username.trim(), password });
      setSession(res.token, res.user);
      await qc.invalidateQueries({ queryKey: ["me"] });
      qc.setQueryData(["me"], res.user);
      navigate("/");
    } catch (err) {
      toast.error(ApiError.isApiError(err) ? err.display() : "Login failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <AuthLayout title="urapt" description="Sign in to manage your repositories">
      <form onSubmit={onSubmit} className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="username">Username</Label>
          <Input
            id="username"
            autoComplete="username"
            autoCapitalize="none"
            spellCheck={false}
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            required
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="password">Password</Label>
          <Input
            id="password"
            type="password"
            autoComplete="current-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
          />
        </div>
        <Button type="submit" className="w-full" disabled={busy}>
          {busy ? <Spinner className="size-4" /> : null}
          Sign in
        </Button>
      </form>
    </AuthLayout>
  );
}
