import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { Package, Plus } from "lucide-react";
import { PageHeader } from "@/components/common/page-header";
import { EmptyState } from "@/components/common/empty-state";
import { ErrorState } from "@/components/common/error-state";
import { Spinner } from "@/components/common/spinner";
import { VisibilityBadge } from "@/components/common/badges";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { formatDate } from "@/lib/utils";
import { useRepositories } from "./repos-hooks";
import { CreateRepoDialog } from "./CreateRepoDialog";

export function ReposList() {
  const { data, isLoading, isError, error, refetch } = useRepositories();
  const [createOpen, setCreateOpen] = useState(false);
  const navigate = useNavigate();

  const repos = data?.items ?? [];

  return (
    <div className="space-y-6">
      <PageHeader
        title="Repositories"
        description="Your APT repositories and those shared with you"
        actions={
          <Button onClick={() => setCreateOpen(true)}>
            <Plus className="size-4" />
            New repository
          </Button>
        }
      />

      {isLoading ? (
        <div className="flex justify-center py-16">
          <Spinner className="size-6" />
        </div>
      ) : isError ? (
        <ErrorState error={error} onRetry={() => refetch()} />
      ) : repos.length === 0 ? (
        <EmptyState
          icon={<Package className="size-8" />}
          title="No repositories yet"
          description="Create your first repository to start pushing .deb packages."
          action={
            <Button onClick={() => setCreateOpen(true)}>
              <Plus className="size-4" />
              New repository
            </Button>
          }
        />
      ) : (
        <div className="rounded-lg border bg-card">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Visibility</TableHead>
                <TableHead>Owner</TableHead>
                <TableHead>Description</TableHead>
                <TableHead>Created</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {repos.map((repo) => (
                <TableRow
                  key={repo.id}
                  className="cursor-pointer"
                  onClick={() => navigate(`/repos/${repo.name}`)}
                >
                  <TableCell className="font-mono font-medium">{repo.name}</TableCell>
                  <TableCell>
                    <VisibilityBadge visibility={repo.visibility} />
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {repo.owner?.username ?? "—"}
                  </TableCell>
                  <TableCell className="max-w-[20rem] truncate text-muted-foreground">
                    {repo.description || "—"}
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {formatDate(repo.created_at)}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      <CreateRepoDialog open={createOpen} onOpenChange={setCreateOpen} />
    </div>
  );
}
