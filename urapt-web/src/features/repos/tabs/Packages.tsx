import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { Download, Package as PackageIcon, Trash2, Upload } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
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
import { EmptyState } from "@/components/common/empty-state";
import { ErrorState } from "@/components/common/error-state";
import { Spinner } from "@/components/common/spinner";
import { ConfirmDialog } from "@/components/common/confirm-dialog";
import { ApiError, getPackageFile } from "@/lib/api";
import type { Package } from "@/lib/api/types";
import { formatBytes, formatDate } from "@/lib/utils";
import { useRepoContext } from "../repo-context";
import {
  useComponents,
  useDeletePackage,
  useDistributions,
  usePackages,
  usePushPackage,
} from "../repo-detail-hooks";

export function Packages() {
  const { repo, canWrite } = useRepoContext();
  const dists = useDistributions(repo.name);
  const [dist, setDist] = useState<string>("");
  const [uploadOpen, setUploadOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Package | null>(null);
  const [detail, setDetail] = useState<Package | null>(null);
  const delPkg = useDeletePackage(repo.name);

  // Auto-select the first distribution once loaded.
  useEffect(() => {
    if (!dist && dists.data && dists.data.length > 0) {
      setDist(dists.data[0].name);
    }
  }, [dists.data, dist]);

  const packages = usePackages(repo.name, dist);

  async function doDelete() {
    if (!deleteTarget) return;
    try {
      await delPkg.mutateAsync({ id: deleteTarget.id, dist });
      toast.success(`Deleted ${deleteTarget.name} ${deleteTarget.version}`);
      setDeleteTarget(null);
    } catch (err) {
      toast.error(ApiError.isApiError(err) ? err.display() : "Failed to delete package");
    }
  }

  async function download(pkg: Package) {
    try {
      const blob = await getPackageFile(repo.name, pkg.id);
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = pkg.filename;
      a.click();
      URL.revokeObjectURL(url);
    } catch (err) {
      toast.error(ApiError.isApiError(err) ? err.display() : "Download failed");
    }
  }

  if (dists.isLoading) {
    return (
      <div className="flex justify-center py-12">
        <Spinner className="size-6" />
      </div>
    );
  }
  if (dists.isError) {
    return <ErrorState error={dists.error} />;
  }

  if ((dists.data ?? []).length === 0) {
    return (
      <EmptyState
        icon={<PackageIcon className="size-8" />}
        title="No distributions yet"
        description="Create a distribution on the Distributions tab before pushing packages."
      />
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
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
        {canWrite && dist ? (
          <Button size="sm" onClick={() => setUploadOpen(true)}>
            <Upload className="size-4" />
            Upload package
          </Button>
        ) : null}
      </div>

      {!dist ? null : packages.isLoading ? (
        <div className="flex justify-center py-12">
          <Spinner className="size-6" />
        </div>
      ) : packages.isError ? (
        <ErrorState error={packages.error} />
      ) : (packages.data?.items ?? []).length === 0 ? (
        <EmptyState
          icon={<PackageIcon className="size-8" />}
          title="No packages"
          description={`No .deb packages in ${dist} yet.`}
          action={
            canWrite ? (
              <Button size="sm" onClick={() => setUploadOpen(true)}>
                <Upload className="size-4" />
                Upload package
              </Button>
            ) : undefined
          }
        />
      ) : (
        <div className="rounded-lg border bg-card">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Version</TableHead>
                <TableHead>Arch</TableHead>
                <TableHead>Size</TableHead>
                <TableHead>Uploaded</TableHead>
                <TableHead className="w-24" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {packages.data!.items.map((pkg) => (
                <TableRow
                  key={pkg.id}
                  className="cursor-pointer"
                  onClick={() => setDetail(pkg)}
                >
                  <TableCell className="font-mono font-medium">{pkg.name}</TableCell>
                  <TableCell className="font-mono text-xs">{pkg.version}</TableCell>
                  <TableCell className="font-mono text-xs">{pkg.architecture}</TableCell>
                  <TableCell className="text-muted-foreground">
                    {formatBytes(pkg.size)}
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {formatDate(pkg.created_at)}
                  </TableCell>
                  <TableCell>
                    <div
                      className="flex items-center gap-1"
                      onClick={(e) => e.stopPropagation()}
                    >
                      <Button
                        variant="ghost"
                        size="icon"
                        className="size-8"
                        title="Download .deb"
                        onClick={() => download(pkg)}
                      >
                        <Download className="size-4" />
                      </Button>
                      {canWrite ? (
                        <Button
                          variant="ghost"
                          size="icon"
                          className="size-8 text-destructive"
                          title="Delete"
                          onClick={() => setDeleteTarget(pkg)}
                        >
                          <Trash2 className="size-4" />
                        </Button>
                      ) : null}
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      {canWrite && dist ? (
        <UploadDialog dist={dist} open={uploadOpen} onOpenChange={setUploadOpen} />
      ) : null}

      <PackageDetailDialog pkg={detail} onClose={() => setDetail(null)} />

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(o) => !o && setDeleteTarget(null)}
        title="Delete package?"
        description={
          deleteTarget
            ? `Delete ${deleteTarget.name} ${deleteTarget.version} ${deleteTarget.architecture}?`
            : ""
        }
        confirmLabel="Delete"
        destructive
        onConfirm={doDelete}
      />
    </div>
  );
}

function UploadDialog({
  dist,
  open,
  onOpenChange,
}: {
  dist: string;
  open: boolean;
  onOpenChange: (o: boolean) => void;
}) {
  const { repo } = useRepoContext();
  const comps = useComponents(repo.name, dist);
  const push = usePushPackage(repo.name, dist);
  const [file, setFile] = useState<File | null>(null);
  const [component, setComponent] = useState("");

  useEffect(() => {
    if (open) {
      setFile(null);
      setComponent("");
    }
  }, [open]);

  // Auto-pick the first component if only one exists.
  useEffect(() => {
    if (open && !component && comps.data && comps.data.length === 1) {
      setComponent(comps.data[0].name);
    }
  }, [open, comps.data, component]);

  const noComponents = (comps.data ?? []).length === 0;
  const canSubmit = !!file && !!component && !push.isPending;

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!file || !component) return;
    try {
      const pkg = await push.mutateAsync({ file, component });
      toast.success(`Uploaded ${pkg.name} ${pkg.version}`);
      onOpenChange(false);
    } catch (err) {
      toast.error(ApiError.isApiError(err) ? err.display() : "Upload failed");
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Upload package to {dist}</DialogTitle>
          <DialogDescription>
            Select a <code>.deb</code> file and the target component.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={onSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="pkg-file">Package file</Label>
            <Input
              id="pkg-file"
              type="file"
              accept=".deb,application/vnd.debian.binary-package"
              onChange={(e) => setFile(e.target.files?.[0] ?? null)}
              required
            />
            {file ? (
              <p className="text-xs text-muted-foreground">
                {file.name} ({formatBytes(file.size)})
              </p>
            ) : null}
          </div>
          <div className="space-y-2">
            <Label htmlFor="pkg-component">Component</Label>
            {noComponents ? (
              <p className="text-sm text-muted-foreground">
                No components in {dist}. Add one on the Distributions tab first.
              </p>
            ) : (
              <Select value={component} onValueChange={setComponent}>
                <SelectTrigger id="pkg-component">
                  <SelectValue placeholder="Select component…" />
                </SelectTrigger>
                <SelectContent>
                  {comps.data!.map((c) => (
                    <SelectItem key={c.id} value={c.name}>
                      {c.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={!canSubmit}>
              {push.isPending ? <Spinner className="size-4" /> : null}
              Upload
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function PackageDetailDialog({
  pkg,
  onClose,
}: {
  pkg: Package | null;
  onClose: () => void;
}) {
  const fields = useMemo(() => {
    if (!pkg) return [];
    const entries: [string, string][] = [
      ["Name", pkg.name],
      ["Version", pkg.version],
      ["Architecture", pkg.architecture],
      ["Component", pkg.component_id],
      ["Source", pkg.source ?? ""],
      ["Maintainer", pkg.maintainer ?? ""],
      ["Priority", pkg.priority ?? ""],
      ["Section", pkg.section ?? ""],
      ["Origin", pkg.origin ?? ""],
      ["Homepage", pkg.homepage ?? ""],
      ["Installed-Size", pkg.installed_size ? String(pkg.installed_size) : ""],
      ["Depends", pkg.depends ?? ""],
      ["Pre-Depends", pkg.pre_depends ?? ""],
      ["Recommends", pkg.recommends ?? ""],
      ["Suggests", pkg.suggests ?? ""],
      ["Conflicts", pkg.conflicts ?? ""],
      ["Breaks", pkg.breaks ?? ""],
      ["Provides", pkg.provides ?? ""],
      ["Replaces", pkg.replaces ?? ""],
      ["Enhances", pkg.enhances ?? ""],
    ];
    return entries.filter(([, v]) => v);
  }, [pkg]);

  return (
    <Dialog open={!!pkg} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle className="font-mono">
            {pkg?.name} {pkg?.version}
          </DialogTitle>
          <DialogDescription>Package control metadata</DialogDescription>
        </DialogHeader>
        {pkg ? (
          <div className="max-h-[60vh] space-y-2 overflow-y-auto">
            <div className="space-y-1 rounded-md border p-3 text-sm">
              {fields.map(([k, v]) => (
                <div key={k} className="flex gap-3">
                  <span className="w-28 shrink-0 text-muted-foreground">{k}</span>
                  <span className="break-all font-mono text-xs">{v}</span>
                </div>
              ))}
            </div>
            <div className="space-y-1 rounded-md border p-3 text-sm">
              <div className="flex gap-3">
                <span className="w-28 shrink-0 text-muted-foreground">SHA256</span>
                <span className="break-all font-mono text-xs">{pkg.sha256}</span>
              </div>
              <div className="flex gap-3">
                <span className="w-28 shrink-0 text-muted-foreground">Size</span>
                <span className="font-mono text-xs">{formatBytes(pkg.size)}</span>
              </div>
              <div className="flex gap-3">
                <span className="w-28 shrink-0 text-muted-foreground">Filename</span>
                <span className="break-all font-mono text-xs">{pkg.filename}</span>
              </div>
            </div>
          </div>
        ) : null}
      </DialogContent>
    </Dialog>
  );
}
