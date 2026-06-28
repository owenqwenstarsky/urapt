import { useQuery } from "@tanstack/react-query";
import { getMe, getServerInfo } from "@/lib/api";

export const SERVER_INFO_KEY = ["server-info"] as const;
export const ME_KEY = ["me"] as const;

export function useServerInfo() {
  return useQuery({ queryKey: SERVER_INFO_KEY, queryFn: getServerInfo });
}

/** Current user. Pass `enabled` based on whether a token exists. */
export function useMe(enabled: boolean) {
  return useQuery({ queryKey: ME_KEY, queryFn: getMe, enabled, retry: false });
}
