import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  addMember,
  createArchitecture,
  createComponent,
  createDistribution,
  deleteArchitecture,
  deleteComponent,
  deleteDistribution,
  deletePackage,
  getPackage,
  listArchitectures,
  listComponents,
  listDistributions,
  listMembers,
  listPackages,
  pushPackage,
  removeMember,
  updateMember,
  updateRepository,
} from "@/lib/api";
import type {
  AddMemberRequest,
  CreateNamedRequest,
  UpdateMemberRequest,
} from "@/lib/api/types";
import { REPO_KEY } from "./repos-hooks";

const DIST_KEY = (repo: string) => ["repo", repo, "distributions"] as const;
const COMP_KEY = (repo: string, dist: string) =>
  ["repo", repo, "distributions", dist, "components"] as const;
const ARCH_KEY = (repo: string, dist: string) =>
  ["repo", repo, "distributions", dist, "architectures"] as const;
const PKGS_KEY = (repo: string, dist: string) =>
  ["repo", repo, "distributions", dist, "packages"] as const;
const MEMBERS_KEY = (repo: string) => ["repo", repo, "members"] as const;

// --- members ---

export function useMembers(repo: string) {
  return useQuery({ queryKey: MEMBERS_KEY(repo), queryFn: () => listMembers(repo) });
}

export function useAddMember(repo: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: AddMemberRequest) => addMember(repo, body),
    onSuccess: () => qc.invalidateQueries({ queryKey: MEMBERS_KEY(repo) }),
  });
}

export function useUpdateMember(repo: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ username, body }: { username: string; body: UpdateMemberRequest }) =>
      updateMember(repo, username, body),
    onSuccess: () => qc.invalidateQueries({ queryKey: MEMBERS_KEY(repo) }),
  });
}

export function useRemoveMember(repo: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (username: string) => removeMember(repo, username),
    onSuccess: () => qc.invalidateQueries({ queryKey: MEMBERS_KEY(repo) }),
  });
}

// --- distributions ---

export function useDistributions(repo: string) {
  return useQuery({ queryKey: DIST_KEY(repo), queryFn: () => listDistributions(repo) });
}

export function useCreateDistribution(repo: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: CreateNamedRequest) => createDistribution(repo, body),
    onSuccess: () => qc.invalidateQueries({ queryKey: DIST_KEY(repo) }),
  });
}

export function useDeleteDistribution(repo: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (dist: string) => deleteDistribution(repo, dist),
    onSuccess: () => qc.invalidateQueries({ queryKey: DIST_KEY(repo) }),
  });
}

// --- components ---

export function useComponents(repo: string, dist: string) {
  return useQuery({
    queryKey: COMP_KEY(repo, dist),
    queryFn: () => listComponents(repo, dist),
    enabled: !!dist,
  });
}

export function useCreateComponent(repo: string, dist: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: CreateNamedRequest) => createComponent(repo, dist, body),
    onSuccess: () => qc.invalidateQueries({ queryKey: COMP_KEY(repo, dist) }),
  });
}

export function useDeleteComponent(repo: string, dist: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (comp: string) => deleteComponent(repo, dist, comp),
    onSuccess: () => qc.invalidateQueries({ queryKey: COMP_KEY(repo, dist) }),
  });
}

// --- architectures ---

export function useArchitectures(repo: string, dist: string) {
  return useQuery({
    queryKey: ARCH_KEY(repo, dist),
    queryFn: () => listArchitectures(repo, dist),
    enabled: !!dist,
  });
}

export function useCreateArchitecture(repo: string, dist: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: CreateNamedRequest) => createArchitecture(repo, dist, body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ARCH_KEY(repo, dist) }),
  });
}

export function useDeleteArchitecture(repo: string, dist: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (arch: string) => deleteArchitecture(repo, dist, arch),
    onSuccess: () => qc.invalidateQueries({ queryKey: ARCH_KEY(repo, dist) }),
  });
}

// --- packages ---

export function usePackages(repo: string, dist: string) {
  return useQuery({
    queryKey: PKGS_KEY(repo, dist),
    queryFn: () => listPackages(repo, dist, { per_page: 500 }),
    enabled: !!dist,
  });
}

export function usePushPackage(repo: string, dist: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ file, component }: { file: File; component: string }) =>
      pushPackage(repo, dist, file, component),
    onSuccess: () => qc.invalidateQueries({ queryKey: PKGS_KEY(repo, dist) }),
  });
}

export function usePackage(repo: string, id: string) {
  return useQuery({
    queryKey: ["repo", repo, "package", id],
    queryFn: () => getPackage(repo, id),
    enabled: !!id,
  });
}

export function useDeletePackage(repo: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id }: { id: string; dist: string }) => deletePackage(repo, id),
    onSuccess: (_data, { dist }) =>
      qc.invalidateQueries({ queryKey: PKGS_KEY(repo, dist) }),
  });
}

// re-export repo mutations used by detail
export { updateRepository };
export { REPO_KEY };
