import { Download } from "lucide-react";
import { toast } from "sonner";
import { PageHeader } from "@/components/common/page-header";
import { ErrorState } from "@/components/common/error-state";
import { Spinner } from "@/components/common/spinner";
import { CopyButton } from "@/components/common/copy-button";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ApiError, getServerPubkey } from "@/lib/api";
import { useAuthStore } from "@/stores/auth";
import { useMe, useServerInfo } from "@/hooks/use-server";
import { useLogout } from "@/hooks/use-logout";

export function Settings() {
  const signOut = useLogout();
  const token = useAuthStore((s) => s.token);
  const me = useMe(!!token);
  const serverInfo = useServerInfo();

  async function downloadPubkey() {
    try {
      const armored = await getServerPubkey();
      const blob = new Blob([armored], { type: "application/pgp-keys" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = "urapt-signing-key.asc";
      a.click();
      URL.revokeObjectURL(url);
    } catch (err) {
      toast.error(ApiError.isApiError(err) ? err.display() : "Failed to fetch pubkey");
    }
  }

  if (me.isLoading) {
    return (
      <div className="flex justify-center py-16">
        <Spinner className="size-6" />
      </div>
    );
  }
  if (me.isError) {
    return <ErrorState error={me.error} />;
  }

  const user = me.data;
  const info = serverInfo.data;

  return (
    <div className="space-y-6">
      <PageHeader title="Settings" description="Your account and this urapt instance" />

      <Card>
        <CardHeader>
          <CardTitle>Account</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3 text-sm">
          <Row label="Username" value={user?.username ?? "—"} />
          <Row label="Role" value={user?.is_admin ? "Server admin" : "Member"} />
          <Row label="User ID" value={user?.id ?? "—"} mono />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Instance</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3 text-sm">
          <Row label="Version" value={info ? `urapt v${info.version}` : "—"} />
          <Row
            label="Registration"
            value={info ? (info.open_registration ? "Open" : "Closed") : "—"}
          />
          <div className="flex items-start justify-between gap-4">
            <div className="space-y-1">
              <p className="text-muted-foreground">Signing key fingerprint</p>
              <p className="break-all font-mono text-xs">
                {info?.default_key_fingerprint || "—"}
              </p>
            </div>
            {info?.default_key_fingerprint ? (
              <CopyButton value={info.default_key_fingerprint} />
            ) : null}
          </div>
          <div className="pt-1">
            <Button variant="outline" onClick={downloadPubkey}>
              <Download className="size-4" />
              Download public key
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Session</CardTitle>
        </CardHeader>
        <CardContent>
          <Button variant="destructive" onClick={signOut}>
            Sign out
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}

function Row({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex items-center justify-between gap-4">
      <span className="text-muted-foreground">{label}</span>
      <span className={mono ? "font-mono text-xs" : ""}>{value}</span>
    </div>
  );
}
