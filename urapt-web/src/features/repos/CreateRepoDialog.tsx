import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
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
import { Spinner } from "@/components/common/spinner";
import { ApiError } from "@/lib/api";
import type { Visibility } from "@/lib/api/types";
import { useCreateRepository } from "./repos-hooks";

interface CreateRepoDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

const NAME_RE = /^[a-z0-9_-]{3,64}$/;

export function CreateRepoDialog({ open, onOpenChange }: CreateRepoDialogProps) {
  const navigate = useNavigate();
  const create = useCreateRepository();
  const [name, setName] = useState("");
  const [visibility, setVisibility] = useState<Visibility>("private");
  const [description, setDescription] = useState("");

  useEffect(() => {
    if (open) {
      setName("");
      setVisibility("private");
      setDescription("");
    }
  }, [open]);

  const nameInvalid = name.length > 0 && !NAME_RE.test(name);
  const canSubmit = NAME_RE.test(name) && !create.isPending;

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!canSubmit) return;
    try {
      const repo = await create.mutateAsync({
        name,
        visibility,
        description: description.trim() || undefined,
      });
      toast.success(`Repository "${repo.name}" created`);
      onOpenChange(false);
      navigate(`/repos/${repo.name}`);
    } catch (err) {
      toast.error(
        ApiError.isApiError(err) ? err.display() : "Failed to create repository",
      );
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>New repository</DialogTitle>
          <DialogDescription>
            Create a new APT repository to push packages into.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={onSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="repo-name">Name</Label>
            <Input
              id="repo-name"
              autoCapitalize="none"
              spellCheck={false}
              placeholder="myrepo"
              value={name}
              onChange={(e) => setName(e.target.value)}
              autoFocus
            />
            {nameInvalid ? (
              <p className="text-xs text-destructive">
                Lowercase letters, digits, <code>-</code> and <code>_</code>. 3–64 chars.
              </p>
            ) : (
              <p className="text-xs text-muted-foreground">
                Used in the apt URL: <code>/apt/&lt;name&gt;/</code>
              </p>
            )}
          </div>
          <div className="space-y-2">
            <Label htmlFor="repo-visibility">Visibility</Label>
            <Select
              value={visibility}
              onValueChange={(v) => setVisibility(v as Visibility)}
            >
              <SelectTrigger id="repo-visibility">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="private">Private — members only</SelectItem>
                <SelectItem value="public">Public — anonymous reads</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <Label htmlFor="repo-desc">Description (optional)</Label>
            <Input
              id="repo-desc"
              placeholder="What is this repo for?"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              maxLength={280}
            />
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={!canSubmit}>
              {create.isPending ? <Spinner className="size-4" /> : null}
              Create
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
