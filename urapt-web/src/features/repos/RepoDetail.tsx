import { useParams, Link } from "react-router-dom";
import { ArrowLeft } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { PageHeader } from "@/components/common/page-header";
import { ErrorState } from "@/components/common/error-state";
import { Spinner } from "@/components/common/spinner";
import { VisibilityBadge } from "@/components/common/badges";
import { useRepository, useDeleteRepository } from "./repos-hooks";
import { useMembers } from "./repo-detail-hooks";
import { RepoProvider } from "./repo-context.tsx";
import { computeAccess, canWriteWith, canManageWith } from "./repo-context";
import { RepoOverview } from "./tabs/RepoOverview";
import { Members } from "./tabs/Members";
import { Structure } from "./tabs/Structure";
import { Packages } from "./tabs/Packages";
import { AptConfig } from "./tabs/AptConfig";
import { useMe } from "@/hooks/use-server";
import { useAuthStore } from "@/stores/auth";

export function RepoDetail() {
  const { repo, tab } = useParams<{ repo: string; tab?: string }>();
  const token = useAuthStore((s) => s.token);
  const me = useMe(!!token);
  const { data, isLoading, isError, error } = useRepository(repo ?? "");
  const members = useMembers(repo ?? "");
  const del = useDeleteRepository();

  if (isLoading) {
    return (
      <div className="flex justify-center py-16">
        <Spinner className="size-6" />
      </div>
    );
  }
  if (isError) {
    return <ErrorState error={error} />;
  }
  if (!data) return null;

  const myMember = members.data?.find((m) => m.user_id === me.data?.id);
  const access = computeAccess(data, me.data ?? null, myMember?.access ?? null);
  const canWrite = canWriteWith(access);
  const canManage = canManageWith(access);

  return (
    <RepoProvider
      value={{
        repo: data,
        me: me.data ?? null,
        access,
        canWrite,
        canManage,
      }}
    >
      <div className="space-y-6">
        <Button asChild variant="ghost" size="sm" className="-ml-2">
          <Link to="/">
            <ArrowLeft className="size-4" /> Repositories
          </Link>
        </Button>
        <PageHeader
          title={data.name}
          description={data.description || "No description"}
          actions={<VisibilityBadge visibility={data.visibility} />}
        />

        <Tabs value={tab ?? "overview"}>
          <TabsList>
            <TabsTrigger value="overview" asChild>
              <Link to={`/repos/${data.name}`}>Overview</Link>
            </TabsTrigger>
            <TabsTrigger value="members" asChild>
              <Link to={`/repos/${data.name}/members`}>Members</Link>
            </TabsTrigger>
            <TabsTrigger value="structure" asChild>
              <Link to={`/repos/${data.name}/structure`}>Distributions</Link>
            </TabsTrigger>
            <TabsTrigger value="packages" asChild>
              <Link to={`/repos/${data.name}/packages`}>Packages</Link>
            </TabsTrigger>
            <TabsTrigger value="apt-config" asChild>
              <Link to={`/repos/${data.name}/apt-config`}>Apt config</Link>
            </TabsTrigger>
          </TabsList>
        </Tabs>

        <div className="mt-2">
          {tab === "members" ? (
            <Members />
          ) : tab === "structure" ? (
            <Structure />
          ) : tab === "packages" ? (
            <Packages />
          ) : tab === "apt-config" ? (
            <AptConfig />
          ) : (
            <RepoOverview onDelete={del} />
          )}
        </div>
      </div>
    </RepoProvider>
  );
}
