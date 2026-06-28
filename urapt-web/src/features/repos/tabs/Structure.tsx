import { useState } from "react";
import { toast } from "sonner";
import { Layers, Plus, Trash2, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/common/empty-state";
import { ErrorState } from "@/components/common/error-state";
import { Spinner } from "@/components/common/spinner";
import { ConfirmDialog } from "@/components/common/confirm-dialog";
import { ApiError } from "@/lib/api";
import { cn } from "@/lib/utils";
import { useRepoContext } from "../repo-context";
import {
  useArchitectures,
  useComponents,
  useCreateArchitecture,
  useCreateComponent,
  useCreateDistribution,
  useDeleteArchitecture,
  useDeleteComponent,
  useDeleteDistribution,
  useDistributions,
} from "../repo-detail-hooks";

export function Structure() {
  const { repo, canWrite } = useRepoContext();
  const { data, isLoading, isError, error } = useDistributions(repo.name);
  const createDist = useCreateDistribution(repo.name);
  const deleteDist = useDeleteDistribution(repo.name);
  const [newDist, setNewDist] = useState("");
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null);

  async function addDist(e: React.FormEvent) {
    e.preventDefault();
    const name = newDist.trim();
    if (!name) return;
    try {
      await createDist.mutateAsync({ name });
      setNewDist("");
    } catch (err) {
      toast.error(
        ApiError.isApiError(err) ? err.display() : "Failed to create distribution",
      );
    }
  }

  async function doDeleteDist() {
    if (!deleteTarget) return;
    try {
      await deleteDist.mutateAsync(deleteTarget);
      toast.success(`Deleted distribution ${deleteTarget}`);
    } catch (err) {
      toast.error(
        ApiError.isApiError(err) ? err.display() : "Failed to delete distribution",
      );
    }
  }

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

  const dists = data ?? [];

  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <p className="text-sm text-muted-foreground">
          Distributions (suites) group packages by release, with components and
          architectures.
        </p>
        {canWrite ? (
          <form onSubmit={addDist} className="flex items-center gap-2">
            <Input
              placeholder="e.g. stable"
              value={newDist}
              onChange={(e) => setNewDist(e.target.value)}
              autoCapitalize="none"
              spellCheck={false}
              className="max-w-xs"
            />
            <Button
              type="submit"
              size="sm"
              disabled={createDist.isPending || !newDist.trim()}
            >
              <Plus className="size-4" />
              Add distribution
            </Button>
          </form>
        ) : null}
      </div>

      {dists.length === 0 ? (
        <EmptyState
          icon={<Layers className="size-8" />}
          title="No distributions yet"
          description="Add a distribution like 'stable' to start organizing packages."
        />
      ) : (
        <div className="space-y-4">
          {dists.map((dist) => (
            <DistributionCard
              key={dist.id}
              name={dist.name}
              canWrite={canWrite}
              onDelete={() => setDeleteTarget(dist.name)}
            />
          ))}
        </div>
      )}

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(o) => !o && setDeleteTarget(null)}
        title="Delete distribution?"
        description={`Delete ${deleteTarget} and all of its components, architectures, and packages?`}
        confirmLabel="Delete"
        destructive
        onConfirm={doDeleteDist}
      />
    </div>
  );
}

function DistributionCard({
  name,
  canWrite,
  onDelete,
}: {
  name: string;
  canWrite: boolean;
  onDelete: () => void;
}) {
  const { repo } = useRepoContext();
  const comps = useComponents(repo.name, name);
  const arches = useArchitectures(repo.name, name);
  const createComp = useCreateComponent(repo.name, name);
  const deleteComp = useDeleteComponent(repo.name, name);
  const createArch = useCreateArchitecture(repo.name, name);
  const deleteArch = useDeleteArchitecture(repo.name, name);
  const [newComp, setNewComp] = useState("");
  const [newArch, setNewArch] = useState("");

  async function addComp(e: React.FormEvent) {
    e.preventDefault();
    const v = newComp.trim();
    if (!v) return;
    try {
      await createComp.mutateAsync({ name: v });
      setNewComp("");
    } catch (err) {
      toast.error(ApiError.isApiError(err) ? err.display() : "Failed to add component");
    }
  }

  async function addArch(e: React.FormEvent) {
    e.preventDefault();
    const v = newArch.trim();
    if (!v) return;
    try {
      await createArch.mutateAsync({ name: v });
      setNewArch("");
    } catch (err) {
      toast.error(
        ApiError.isApiError(err) ? err.display() : "Failed to add architecture",
      );
    }
  }

  async function delComp(c: string) {
    try {
      await deleteComp.mutateAsync(c);
    } catch (err) {
      toast.error(
        ApiError.isApiError(err) ? err.display() : "Failed to delete component",
      );
    }
  }

  async function delArch(a: string) {
    try {
      await deleteArch.mutateAsync(a);
    } catch (err) {
      toast.error(
        ApiError.isApiError(err) ? err.display() : "Failed to delete architecture",
      );
    }
  }

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0">
        <CardTitle className="font-mono">{name}</CardTitle>
        {canWrite ? (
          <Button
            variant="ghost"
            size="icon"
            className="text-destructive"
            title="Delete distribution"
            onClick={onDelete}
          >
            <Trash2 className="size-4" />
          </Button>
        ) : null}
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="space-y-2">
          <p className="text-xs font-medium text-muted-foreground">Components</p>
          <ChipList
            items={(comps.data ?? []).map((c) => c.name)}
            onRemove={canWrite ? delComp : undefined}
            emptyText="No components"
          />
          {canWrite ? (
            <form onSubmit={addComp} className="flex items-center gap-2">
              <Input
                placeholder="e.g. main"
                value={newComp}
                onChange={(e) => setNewComp(e.target.value)}
                autoCapitalize="none"
                spellCheck={false}
                className="h-8 max-w-[12rem]"
              />
              <Button
                type="submit"
                size="sm"
                variant="outline"
                disabled={!newComp.trim()}
              >
                <Plus className="size-3.5" /> Add
              </Button>
            </form>
          ) : null}
        </div>

        <div className="space-y-2">
          <p className="text-xs font-medium text-muted-foreground">Architectures</p>
          <ChipList
            items={(arches.data ?? []).map((a) => a.name)}
            extras={["all"]}
            onRemove={canWrite ? delArch : undefined}
            emptyText="No architectures"
          />
          {canWrite ? (
            <form onSubmit={addArch} className="flex items-center gap-2">
              <Input
                placeholder="e.g. amd64"
                value={newArch}
                onChange={(e) => setNewArch(e.target.value)}
                autoCapitalize="none"
                spellCheck={false}
                className="h-8 max-w-[12rem]"
              />
              <Button
                type="submit"
                size="sm"
                variant="outline"
                disabled={!newArch.trim()}
              >
                <Plus className="size-3.5" /> Add
              </Button>
            </form>
          ) : null}
          <p className="text-xs text-muted-foreground">
            <code>all</code> is implicit and available to every distribution.
          </p>
        </div>
      </CardContent>
    </Card>
  );
}

function ChipList({
  items,
  extras = [],
  onRemove,
  emptyText,
}: {
  items: string[];
  extras?: string[];
  onRemove?: (item: string) => void;
  emptyText: string;
}) {
  if (items.length === 0 && extras.length === 0) {
    return <p className="text-sm text-muted-foreground">{emptyText}</p>;
  }
  return (
    <div className="flex flex-wrap gap-2">
      {extras.map((e) => (
        <Badge key={e} variant="secondary" className="font-mono">
          {e}
        </Badge>
      ))}
      {items.map((item) => (
        <Badge
          key={item}
          variant="outline"
          className={cn("gap-1 font-mono", onRemove && "pr-1")}
        >
          {item}
          {onRemove ? (
            <button
              type="button"
              className="rounded-sm hover:bg-destructive/20"
              onClick={() => onRemove(item)}
              aria-label={`Remove ${item}`}
            >
              <X className="size-3" />
            </button>
          ) : null}
        </Badge>
      ))}
    </div>
  );
}
