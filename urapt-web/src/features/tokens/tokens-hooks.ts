import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createToken, listTokens, revokeToken } from "@/lib/api";
import type { CreateTokenRequest } from "@/lib/api/types";

export const TOKENS_KEY = ["me", "tokens"] as const;

export function useTokens() {
  return useQuery({ queryKey: TOKENS_KEY, queryFn: listTokens });
}

export function useCreateToken() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: CreateTokenRequest) => createToken(body),
    onSuccess: () => qc.invalidateQueries({ queryKey: TOKENS_KEY }),
  });
}

export function useRevokeToken() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => revokeToken(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: TOKENS_KEY }),
  });
}
