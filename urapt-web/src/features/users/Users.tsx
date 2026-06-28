import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { ShieldCheck, Trash2 } from "lucide-react";
import { PageHeader } from "@/components/common/page-header";
import { EmptyState } from "@/components/common/empty-state";
import { ErrorState } from "@/components/common/error-state";
import { Spinner } from "@/components/common/spinner";
import { ConfirmDialog } from "@/components/common/confirm-dialog";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Switch } from "@/components/ui/switch";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { ApiError, deleteUser, listUsers, updateUser } from "@/lib/api";
import { useAuthStore } from "@/stores/auth";
import { useMe } from "@/hooks/use-server";
import { formatDate } from "@/lib/utils";

const USERS_KEY = ["users"] as const;

export function Users() {
  const { data, isLoading, isError, error } = useQuery({
    queryKey: USERS_KEY,
    queryFn: () => listUsers({ per_page: 500 }),
  });
  const qc = useQueryClient();
  const me = useMe(!!useAuthStore.getState().token);
  const myId = me.data?.id;

  const toggleAdmin = useMutation({
    mutationFn: ({ id, isAdmin }: { id: string; isAdmin: boolean }) =>
      updateUser(id, { is_admin: isAdmin }),
    onSuccess: () => qc.invalidateQueries({ queryKey: USERS_KEY }),
  });
  const remove = useMutation({
    mutationFn: (id: string) => deleteUser(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: USERS_KEY });
      qc.invalidateQueries({ queryKey: ["me"] });
    },
  });

  const [deleteTarget, setDeleteTarget] = useState<{
    id: string;
    username: string;
  } | null>(null);
  const users = data?.items ?? [];

  async function doDelete() {
    if (!deleteTarget) return;
    try {
      await remove.mutateAsync(deleteTarget.id);
      toast.success(`Deleted user ${deleteTarget.username}`);
      setDeleteTarget(null);
    } catch (err) {
      toast.error(ApiError.isApiError(err) ? err.display() : "Failed to delete user");
    }
  }

  async function onToggle(id: string, isAdmin: boolean, username: string) {
    try {
      await toggleAdmin.mutateAsync({ id, isAdmin });
      toast.success(`${username} is ${isAdmin ? "now an admin" : "no longer an admin"}`);
    } catch (err) {
      toast.error(ApiError.isApiError(err) ? err.display() : "Failed to update user");
    }
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Users"
        description="All accounts on this urapt instance (server admin only)"
      />

      {isLoading ? (
        <div className="flex justify-center py-12">
          <Spinner className="size-6" />
        </div>
      ) : isError ? (
        <ErrorState error={error} />
      ) : users.length === 0 ? (
        <EmptyState title="No users" />
      ) : (
        <div className="rounded-lg border bg-card">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Username</TableHead>
                <TableHead>Role</TableHead>
                <TableHead>Admin</TableHead>
                <TableHead>Created</TableHead>
                <TableHead className="w-24" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {users.map((u) => {
                const isSelf = u.id === myId;
                return (
                  <TableRow key={u.id}>
                    <TableCell className="font-medium">
                      {u.username}
                      {isSelf ? (
                        <span className="ml-2 text-xs text-muted-foreground">(you)</span>
                      ) : null}
                    </TableCell>
                    <TableCell>
                      {u.is_admin ? (
                        <Badge variant="secondary">
                          <ShieldCheck className="mr-1 size-3" />
                          admin
                        </Badge>
                      ) : (
                        <Badge variant="outline" className="text-muted-foreground">
                          member
                        </Badge>
                      )}
                    </TableCell>
                    <TableCell>
                      <Switch
                        checked={u.is_admin}
                        disabled={isSelf || toggleAdmin.isPending}
                        onCheckedChange={(v) => onToggle(u.id, v, u.username)}
                      />
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {formatDate(u.created_at)}
                    </TableCell>
                    <TableCell>
                      {!isSelf ? (
                        <Button
                          variant="ghost"
                          size="icon"
                          className="text-destructive"
                          title="Delete user"
                          onClick={() =>
                            setDeleteTarget({ id: u.id, username: u.username })
                          }
                        >
                          <Trash2 className="size-4" />
                        </Button>
                      ) : null}
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </div>
      )}

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(o) => !o && setDeleteTarget(null)}
        title="Delete user?"
        description={`Delete ${deleteTarget?.username}? Their API tokens are revoked and they lose access to all repositories.`}
        confirmLabel="Delete"
        destructive
        onConfirm={doDelete}
      />
    </div>
  );
}
