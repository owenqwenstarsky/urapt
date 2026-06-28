import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
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
import { Spinner } from "@/components/common/spinner";
import { ConfirmDialog } from "@/components/common/confirm-dialog";
import { CopyButton } from "@/components/common/copy-button";
import { ApiError } from "@/lib/api";
import type { Visibility } from "@/lib/api/types";
import { formatDate } from "@/lib/utils";
import { useRepoContext } from "../repo-context";
import { useUpdateRepository, useDeleteRepository } from "../repos-hooks";

type DeleteHook = ReturnType<typeof useDeleteRepository>;

export function RepoOverview({ onDelete }: { onDelete: DeleteHook }) {
  const { repo, canManage } = useRepoContext();
  const navigate = useNavigate();
  const [editOpen, setEditOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);

  async function doDelete() {
    try {
      await onDelete.mutateAsync(repo.name);
      toast.success("Repository deleted");
      navigate("/");
    } catch (err) {
      toast.error(
        ApiError.isApiError(err) ? err.display() : "Failed to delete repository",
      );
    }
  }

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0">
          <CardTitle>Details</CardTitle>
          {canManage ? (
            <Button variant="outline" size="sm" onClick={() => setEditOpen(true)}>
              Edit
            </Button>
          ) : null}
        </CardHeader>
        <CardContent className="space-y-3 text-sm">
          <Row label="Name" value={repo.name} mono />
          <Row label="Owner" value={repo.owner?.username ?? "—"} />
          <Row label="Visibility" value={repo.visibility} />
          <div className="flex items-start justify-between gap-4">
            <span className="text-muted-foreground">Description</span>
            <span className="max-w-[24rem] text-right">{repo.description || "—"}</span>
          </div>
          <Row label="Repository ID" value={repo.id} mono />
          <Row label="Created" value={formatDate(repo.created_at)} />
          <Row label="Updated" value={formatDate(repo.updated_at)} />
        </CardContent>
      </Card>

      {canManage ? (
        <Card className="border-destructive/40">
          <CardHeader>
            <CardTitle className="text-destructive">Danger zone</CardTitle>
          </CardHeader>
          <CardContent className="flex items-center justify-between gap-4">
            <p className="text-sm text-muted-foreground">
              Deleting this repository removes all its distributions, packages, and
              members. This cannot be undone.
            </p>
            <Button variant="destructive" onClick={() => setDeleteOpen(true)}>
              Delete repository
            </Button>
          </CardContent>
        </Card>
      ) : null}

      <EditDialog open={editOpen} onOpenChange={setEditOpen} />
      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={`Delete ${repo.name}?`}
        description="This permanently deletes the repository and all packages within it."
        confirmLabel="Delete"
        destructive
        onConfirm={doDelete}
      />
    </div>
  );
}

function Row({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex items-center justify-between gap-4">
      <span className="text-muted-foreground">{label}</span>
      <div className="flex items-center gap-2">
        <span className={mono ? "font-mono text-xs" : ""}>{value}</span>
        {mono ? <CopyButton value={value} size="icon" /> : null}
      </div>
    </div>
  );
}

function EditDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (o: boolean) => void;
}) {
  const { repo } = useRepoContext();
  const update = useUpdateRepository(repo.name);
  const navigate = useNavigate();
  const [name, setName] = useState(repo.name);
  const [visibility, setVisibility] = useState<Visibility>(repo.visibility);
  const [description, setDescription] = useState(repo.description);

  useEffect(() => {
    if (open) {
      setName(repo.name);
      setVisibility(repo.visibility);
      setDescription(repo.description);
    }
  }, [open, repo]);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    try {
      const updated = await update.mutateAsync({
        name: name !== repo.name ? name : undefined,
        visibility: visibility !== repo.visibility ? visibility : undefined,
        description: description !== repo.description ? description : undefined,
      });
      toast.success("Repository updated");
      onOpenChange(false);
      if (updated.name && updated.name !== repo.name) {
        navigate(`/repos/${updated.name}`);
      }
    } catch (err) {
      toast.error(
        ApiError.isApiError(err) ? err.display() : "Failed to update repository",
      );
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Edit repository</DialogTitle>
          <DialogDescription>Update repository settings.</DialogDescription>
        </DialogHeader>
        <form onSubmit={onSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="edit-name">Name</Label>
            <Input
              id="edit-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              autoCapitalize="none"
              spellCheck={false}
            />
            {name !== repo.name ? (
              <p className="text-xs text-muted-foreground">
                Renaming changes the apt URL. Existing clients will need reconfiguration.
              </p>
            ) : null}
          </div>
          <div className="space-y-2">
            <Label htmlFor="edit-visibility">Visibility</Label>
            <Select
              value={visibility}
              onValueChange={(v) => setVisibility(v as Visibility)}
            >
              <SelectTrigger id="edit-visibility">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="private">Private — members only</SelectItem>
                <SelectItem value="public">Public — anonymous reads</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <Label htmlFor="edit-desc">Description</Label>
            <Input
              id="edit-desc"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              maxLength={280}
            />
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={update.isPending}>
              {update.isPending ? <Spinner className="size-4" /> : null}
              Save
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
