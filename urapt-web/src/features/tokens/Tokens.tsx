import { useEffect, useState } from "react";
import { toast } from "sonner";
import { KeyRound, Plus } from "lucide-react";
import { PageHeader } from "@/components/common/page-header";
import { EmptyState } from "@/components/common/empty-state";
import { ErrorState } from "@/components/common/error-state";
import { Spinner } from "@/components/common/spinner";
import { CopyButton } from "@/components/common/copy-button";
import { ConfirmDialog } from "@/components/common/confirm-dialog";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { ApiError } from "@/lib/api";
import { formatDate } from "@/lib/utils";
import { useCreateToken, useRevokeToken, useTokens } from "./tokens-hooks";

export function Tokens() {
  const { data, isLoading, isError, error } = useTokens();
  const revoke = useRevokeToken();
  const [createOpen, setCreateOpen] = useState(false);
  const [revokeTarget, setRevokeTarget] = useState<{ id: string; name: string } | null>(
    null,
  );

  async function doRevoke() {
    if (!revokeTarget) return;
    try {
      await revoke.mutateAsync(revokeTarget.id);
      toast.success(`Revoked ${revokeTarget.name}`);
      setRevokeTarget(null);
    } catch (err) {
      toast.error(ApiError.isApiError(err) ? err.display() : "Failed to revoke token");
    }
  }

  const tokens = data?.items ?? [];

  return (
    <div className="space-y-6">
      <PageHeader
        title="API tokens"
        description="Tokens authenticate the urapt CLI and private-repo apt clients"
        actions={
          <Button onClick={() => setCreateOpen(true)}>
            <Plus className="size-4" />
            New token
          </Button>
        }
      />

      <Alert>
        <KeyRound className="size-4" />
        <AlertTitle>Treat tokens like passwords</AlertTitle>
        <AlertDescription>
          A token grants full access to your account. Use it as the password in apt's
          <code> auth.conf</code> for private repositories, or with{" "}
          <code>urapt login</code>.
        </AlertDescription>
      </Alert>

      {isLoading ? (
        <div className="flex justify-center py-12">
          <Spinner className="size-6" />
        </div>
      ) : isError ? (
        <ErrorState error={error} />
      ) : tokens.length === 0 ? (
        <EmptyState
          icon={<KeyRound className="size-8" />}
          title="No tokens"
          description="Create a token to use the urapt CLI or authenticate apt clients."
          action={
            <Button onClick={() => setCreateOpen(true)}>
              <Plus className="size-4" />
              New token
            </Button>
          }
        />
      ) : (
        <div className="rounded-lg border bg-card">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Prefix</TableHead>
                <TableHead>Last used</TableHead>
                <TableHead>Created</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="w-24" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {tokens.map((t) => (
                <TableRow key={t.id}>
                  <TableCell className="font-medium">{t.name}</TableCell>
                  <TableCell className="font-mono text-xs text-muted-foreground">
                    {t.prefix}…
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {formatDate(t.last_used_at)}
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {formatDate(t.created_at)}
                  </TableCell>
                  <TableCell>
                    {t.revoked_at ? (
                      <Badge variant="outline" className="text-muted-foreground">
                        revoked
                      </Badge>
                    ) : (
                      <Badge variant="secondary">active</Badge>
                    )}
                  </TableCell>
                  <TableCell>
                    {!t.revoked_at ? (
                      <Button
                        variant="ghost"
                        size="sm"
                        className="text-destructive"
                        onClick={() => setRevokeTarget({ id: t.id, name: t.name })}
                      >
                        Revoke
                      </Button>
                    ) : null}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      <CreateTokenDialog open={createOpen} onOpenChange={setCreateOpen} />
      <ConfirmDialog
        open={!!revokeTarget}
        onOpenChange={(o) => !o && setRevokeTarget(null)}
        title="Revoke token?"
        description={`Revoke "${revokeTarget?.name}"? Any client using it will lose access immediately.`}
        confirmLabel="Revoke"
        destructive
        onConfirm={doRevoke}
      />
    </div>
  );
}

function CreateTokenDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (o: boolean) => void;
}) {
  const create = useCreateToken();
  const [name, setName] = useState("");
  const [createdToken, setCreatedToken] = useState<string | null>(null);

  useEffect(() => {
    if (open) {
      setName("");
      setCreatedToken(null);
    }
  }, [open]);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim()) return;
    try {
      const token = await create.mutateAsync({ name: name.trim() });
      setCreatedToken(token.token ?? null);
    } catch (err) {
      toast.error(ApiError.isApiError(err) ? err.display() : "Failed to create token");
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        {createdToken ? (
          <div className="space-y-4">
            <DialogHeader>
              <DialogTitle>Token created</DialogTitle>
              <DialogDescription>
                Copy this token now. It won't be shown again.
              </DialogDescription>
            </DialogHeader>
            <Alert variant="destructive">
              <AlertTitle>Store it safely</AlertTitle>
              <AlertDescription>
                The server stores only a hash of the token. You can't recover it later.
              </AlertDescription>
            </Alert>
            <div className="flex items-center gap-2 rounded-md border bg-muted p-3">
              <code className="flex-1 break-all font-mono text-xs">{createdToken}</code>
              <CopyButton value={createdToken} />
            </div>
            <DialogFooter>
              <Button onClick={() => onOpenChange(false)}>Done</Button>
            </DialogFooter>
          </div>
        ) : (
          <form onSubmit={onSubmit} className="space-y-4">
            <DialogHeader>
              <DialogTitle>New API token</DialogTitle>
              <DialogDescription>
                Name it so you can recognize it later.
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-2">
              <Label htmlFor="token-name">Name</Label>
              <Input
                id="token-name"
                placeholder="e.g. ci-builder"
                value={name}
                onChange={(e) => setName(e.target.value)}
                autoFocus
                maxLength={64}
              />
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={create.isPending || !name.trim()}>
                {create.isPending ? <Spinner className="size-4" /> : null}
                Create
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
}
