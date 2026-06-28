import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { KeyRound, ShieldAlert } from "lucide-react";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { CopyButton } from "@/components/common/copy-button";
import { EmptyState } from "@/components/common/empty-state";
import { Spinner } from "@/components/common/spinner";
import { useAuthStore } from "@/stores/auth";
import { useRepoContext } from "../repo-context";
import { useArchitectures, useComponents, useDistributions } from "../repo-detail-hooks";

export function AptConfig() {
  const { repo } = useRepoContext();
  const dists = useDistributions(repo.name);
  const [dist, setDist] = useState("");
  const [useMyToken, setUseMyToken] = useState(false);
  const sessionToken = useAuthStore((s) => s.token);

  useEffect(() => {
    if (!dist && dists.data && dists.data.length > 0) setDist(dists.data[0].name);
  }, [dists.data, dist]);

  const comps = useComponents(repo.name, dist);
  const arches = useArchitectures(repo.name, dist);

  const isPrivate = repo.visibility === "private";
  const origin = window.location.origin;
  const signedBy = `/usr/share/keyrings/urapt-${repo.name}.gpg`;

  const block = useMemo(() => {
    if (!dist) return "";
    const compList = (comps.data ?? []).map((c) => c.name).join(" ") || "main";
    const archStr = (arches.data ?? []).map((a) => a.name).join(",");

    const lines: string[] = [];
    lines.push("# 1. Install the repository signing key:");
    lines.push(
      `curl -fsSL ${origin}/api/v1/server/pubkey | sudo gpg --dearmor -o ${signedBy}`,
    );
    lines.push("");
    lines.push("# 2. Add the repository to apt:");
    const archPart = archStr ? `arch=${archStr} ` : "";
    const debLine = `deb [${archPart}signed-by=${signedBy}] ${origin}/apt/${repo.name}/ ${dist} ${compList}`;
    lines.push(`echo '${debLine}' | sudo tee /etc/apt/sources.list.d/${repo.name}.list`);
    lines.push("");
    lines.push("# 3. Update apt:");
    lines.push("sudo apt update");

    if (isPrivate) {
      const host = hostOf(origin);
      const password = useMyToken && sessionToken ? sessionToken : "<your-api-token>";
      lines.push("");
      lines.push("# 4. This repository is private. Configure apt credentials:");
      lines.push(`sudo tee /etc/apt/auth.conf.d/${repo.name}.conf <<EOF`);
      lines.push(`machine ${host}`);
      lines.push(`login ${"<your-username>"}`);
      lines.push(`password ${password}`);
      lines.push("EOF");
    }
    return lines.join("\n");
  }, [
    dist,
    comps.data,
    arches.data,
    origin,
    signedBy,
    repo.name,
    isPrivate,
    useMyToken,
    sessionToken,
  ]);

  if (dists.isLoading) {
    return (
      <div className="flex justify-center py-12">
        <Spinner className="size-6" />
      </div>
    );
  }
  if ((dists.data ?? []).length === 0) {
    return (
      <EmptyState
        icon={<KeyRound className="size-8" />}
        title="No distributions yet"
        description="Create a distribution on the Distributions tab to generate apt config."
      />
    );
  }

  return (
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground">
        Run these commands on a Debian/Ubuntu client to install packages from this
        repository.
      </p>

      <div className="flex items-center gap-2">
        <Label className="text-muted-foreground">Distribution</Label>
        <Select value={dist} onValueChange={setDist}>
          <SelectTrigger className="w-40">
            <SelectValue placeholder="Select…" />
          </SelectTrigger>
          <SelectContent>
            {dists.data!.map((d) => (
              <SelectItem key={d.id} value={d.name}>
                {d.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {isPrivate ? (
        <Alert>
          <ShieldAlert className="size-4" />
          <AlertTitle>Private repository</AlertTitle>
          <AlertDescription className="space-y-2">
            <p>
              Clients must authenticate with an API token. Create one on the{" "}
              <Link to="/tokens" className="underline">
                Tokens
              </Link>{" "}
              page and use it as the password.
            </p>
            <label className="flex items-center gap-2 text-xs">
              <input
                type="checkbox"
                checked={useMyToken}
                onChange={(e) => setUseMyToken(e.target.checked)}
              />
              Insert my current session token as the password
            </label>
            {useMyToken ? (
              <p className="text-xs text-destructive">
                Warning: this is your login session token. Anyone with it has full account
                access. Prefer a dedicated token.
              </p>
            ) : null}
          </AlertDescription>
        </Alert>
      ) : null}

      <div className="relative rounded-lg border bg-card p-4">
        <div className="absolute right-3 top-3">
          <CopyButton value={block} />
        </div>
        <pre className="overflow-x-auto whitespace-pre pr-20 font-mono text-xs leading-relaxed">
          {block}
        </pre>
      </div>

      <Button asChild variant="outline" size="sm">
        <Link to="/tokens">
          <KeyRound className="size-4" />
          Manage tokens
        </Link>
      </Button>
    </div>
  );
}

function hostOf(url: string): string {
  try {
    return new URL(url).host;
  } catch {
    return url;
  }
}
