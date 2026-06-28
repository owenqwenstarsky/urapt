import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  createRepository,
  deleteRepository,
  getRepository,
  listRepositories,
  updateRepository,
} from "@/lib/api";
import type { CreateRepoRequest, UpdateRepoRequest } from "@/lib/api/types";

export const REPO_KEY = (name: string) => ["repo", name] as const;
export const REPO_LIST_KEY = ["repos", "list"] as const;

export function useRepositories() {
  return useQuery({
    queryKey: REPO_LIST_KEY,
    queryFn: () => listRepositories({ per_page: 200 }),
  });
}

export function useRepository(name: string) {
  return useQuery({
    queryKey: REPO_KEY(name),
    queryFn: () => getRepository(name),
  });
}

export function useCreateRepository() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: CreateRepoRequest) => createRepository(body),
    onSuccess: () => qc.invalidateQueries({ queryKey: REPO_LIST_KEY }),
  });
}

export function useUpdateRepository(name: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: UpdateRepoRequest) => updateRepository(name, body),
    onSuccess: (updated) => {
      qc.setQueryData(REPO_KEY(name), updated);
      if (updated.name && updated.name !== name) {
        qc.invalidateQueries({ queryKey: REPO_LIST_KEY });
      }
    },
  });
}

export function useDeleteRepository() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => deleteRepository(name),
    onSuccess: () => qc.invalidateQueries({ queryKey: REPO_LIST_KEY }),
  });
}
