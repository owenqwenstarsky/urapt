import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { AuthLayout } from "@/features/auth/auth-layout";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/common/spinner";
import { ApiError, register } from "@/lib/api";
import { useAuthStore } from "@/stores/auth";

export function FirstRun() {
  const navigate = useNavigate();
  const qc = useQueryClient();
  const setSession = useAuthStore((s) => s.setSession);

  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [busy, setBusy] = useState(false);

  const tooShort = password.length > 0 && password.length < 8;
  const mismatch = confirm.length > 0 && password !== confirm;
  const canSubmit =
    username.trim().length >= 3 && password.length >= 8 && password === confirm && !busy;

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!canSubmit) return;
    setBusy(true);
    try {
      const res = await register({ username: username.trim(), password });
      setSession(res.token, res.user);
      qc.setQueryData(["me"], res.user);
      qc.invalidateQueries({ queryKey: ["server-info"] });
      toast.success("Admin account created. Welcome to urapt.");
      navigate("/");
    } catch (err) {
      toast.error(ApiError.isApiError(err) ? err.display() : "Registration failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <AuthLayout
      title="Set up urapt"
      description="Create the first admin account for this instance"
    >
      <form onSubmit={onSubmit} className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="username">Admin username</Label>
          <Input
            id="username"
            autoCapitalize="none"
            spellCheck={false}
            autoComplete="username"
            placeholder="admin"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            required
            minLength={3}
            maxLength={32}
          />
          <p className="text-xs text-muted-foreground">
            Lowercase letters, digits, <code>-</code> and <code>_</code>. 3–32 chars.
          </p>
        </div>
        <div className="space-y-2">
          <Label htmlFor="password">Password</Label>
          <Input
            id="password"
            type="password"
            autoComplete="new-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            minLength={8}
          />
          {tooShort ? (
            <p className="text-xs text-destructive">At least 8 characters.</p>
          ) : null}
        </div>
        <div className="space-y-2">
          <Label htmlFor="confirm">Confirm password</Label>
          <Input
            id="confirm"
            type="password"
            autoComplete="new-password"
            value={confirm}
            onChange={(e) => setConfirm(e.target.value)}
            required
          />
          {mismatch ? (
            <p className="text-xs text-destructive">Passwords don't match.</p>
          ) : null}
        </div>
        <Button type="submit" className="w-full" disabled={!canSubmit}>
          {busy ? <Spinner className="size-4" /> : null}
          Create admin account
        </Button>
      </form>
    </AuthLayout>
  );
}
