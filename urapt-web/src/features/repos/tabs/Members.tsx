import { useEffect, useState } from "react";
import { toast } from "sonner";
import { UserPlus } from "lucide-react";
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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
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
import { EmptyState } from "@/components/common/empty-state";
import { ErrorState } from "@/components/common/error-state";
import { Spinner } from "@/components/common/spinner";
import { ConfirmDialog } from "@/components/common/confirm-dialog";
import { AccessBadge } from "@/components/common/badges";
import { ApiError } from "@/lib/api";
import type { Access } from "@/lib/api/types";
import { formatDate } from "@/lib/utils";
import { useRepoContext } from "../repo-context";
import {
  useAddMember,
  useMembers,
  useRemoveMember,
  useUpdateMember,
} from "../repo-detail-hooks";

const ACCESS_OPTIONS: Access[] = ["read", "write", "read-write", "admin"];

export function Members() {
  const { repo, canManage } = useRepoContext();
  const { data, isLoading, isError, error } = useMembers(repo.name);
  const [addOpen, setAddOpen] = useState(false);
  const [removeTarget, setRemoveTarget] = useState<string | null>(null);
  const remove = useRemoveMember(repo.name);
  const update = useUpdateMember(repo.name);

  if (isLoading) {
    return (
      <div className="flex justify-center py-12">
        <Spinner className="size-6" />
      </div>
    );
  }
  if (isError) {
    return <ErrorState error={error} />;
  }

  const members = data ?? [];

  async function doRemove(username: string) {
    try {
      await remove.mutateAsync(username);
      toast.success(`Removed ${username}`);
    } catch (err) {
      toast.error(ApiError.isApiError(err) ? err.display() : "Failed to remove member");
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">
          Members can read or push packages based on their access level.
        </p>
        {canManage ? (
          <Button size="sm" onClick={() => setAddOpen(true)}>
            <UserPlus className="size-4" />
            Add member
          </Button>
        ) : null}
      </div>

      {members.length === 0 && !repo.owner ? (
        <EmptyState title="No members" description="Add a member to grant access." />
      ) : (
        <div className="rounded-lg border bg-card">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>User</TableHead>
                <TableHead>Access</TableHead>
                <TableHead>Added</TableHead>
                <TableHead className="w-24" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {repo.owner ? (
                <TableRow>
                  <TableCell className="font-medium">
                    {repo.owner.username}
                    <span className="ml-2 text-xs text-muted-foreground">(owner)</span>
                  </TableCell>
                  <TableCell>
                    <AccessBadge access="admin" />
                  </TableCell>
                  <TableCell className="text-muted-foreground">—</TableCell>
                  <TableCell />
                </TableRow>
              ) : null}
              {members.map((m) => (
                <TableRow key={m.user_id}>
                  <TableCell className="font-medium">
                    {m.user?.username ?? m.user_id}
                  </TableCell>
                  <TableCell>
                    {canManage ? (
                      <AccessSelect
                        value={m.access}
                        onChange={(access) =>
                          onChangeAccess(m.user?.username ?? "", access)
                        }
                      />
                    ) : (
                      <AccessBadge access={m.access} />
                    )}
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {formatDate(m.created_at)}
                  </TableCell>
                  <TableCell>
                    {canManage ? (
                      <Button
                        variant="ghost"
                        size="sm"
                        className="text-destructive"
                        onClick={() => setRemoveTarget(m.user?.username ?? "")}
                      >
                        Remove
                      </Button>
                    ) : null}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      {canManage ? <AddMemberDialog open={addOpen} onOpenChange={setAddOpen} /> : null}
      <ConfirmDialog
        open={!!removeTarget}
        onOpenChange={(o) => !o && setRemoveTarget(null)}
        title="Remove member?"
        description={`Remove ${removeTarget} from this repository? They will lose access immediately.`}
        confirmLabel="Remove"
        destructive
        onConfirm={() => {
          if (removeTarget) void doRemove(removeTarget);
        }}
      />
    </div>
  );

  async function onChangeAccess(username: string, access: Access) {
    try {
      await update.mutateAsync({ username, body: { access } });
      toast.success(`Updated ${username} to ${access}`);
    } catch (err) {
      toast.error(ApiError.isApiError(err) ? err.display() : "Failed to update member");
    }
  }
}

function AccessSelect({
  value,
  onChange,
}: {
  value: Access;
  onChange: (a: Access) => void;
}) {
  return (
    <Select value={value} onValueChange={(v) => onChange(v as Access)}>
      <SelectTrigger className="h-8 w-32">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {ACCESS_OPTIONS.map((a) => (
          <SelectItem key={a} value={a}>
            {a}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

function AddMemberDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (o: boolean) => void;
}) {
  const { repo } = useRepoContext();
  const add = useAddMember(repo.name);
  const [username, setUsername] = useState("");
  const [access, setAccess] = useState<Access>("read");

  useEffect(() => {
    if (open) {
      setUsername("");
      setAccess("read");
    }
  }, [open]);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!username.trim()) return;
    try {
      await add.mutateAsync({ username: username.trim(), access });
      toast.success(`Added ${username} as ${access}`);
      onOpenChange(false);
    } catch (err) {
      toast.error(ApiError.isApiError(err) ? err.display() : "Failed to add member");
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add member</DialogTitle>
          <DialogDescription>Grant a user access to this repository.</DialogDescription>
        </DialogHeader>
        <form onSubmit={onSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="member-username">Username</Label>
            <Input
              id="member-username"
              autoCapitalize="none"
              spellCheck={false}
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoFocus
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="member-access">Access</Label>
            <Select value={access} onValueChange={(v) => setAccess(v as Access)}>
              <SelectTrigger id="member-access">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="read">read — pull packages</SelectItem>
                <SelectItem value="write">write — push packages</SelectItem>
                <SelectItem value="read-write">read-write — pull & push</SelectItem>
                <SelectItem value="admin">admin — manage repo</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={add.isPending || !username.trim()}>
              {add.isPending ? <Spinner className="size-4" /> : null}
              Add
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
