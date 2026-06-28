import { create } from "zustand";
import { persist } from "zustand/middleware";
import type { User } from "@/lib/api/types";

const STORAGE_KEY = "urapt.auth";

interface AuthState {
  token: string | null;
  user: User | null;
  setSession: (token: string, user: User) => void;
  setUser: (user: User | null) => void;
  clear: () => void;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      token: null,
      user: null,
      setSession: (token, user) => set({ token, user }),
      setUser: (user) => set({ user }),
      clear: () => set({ token: null, user: null }),
    }),
    {
      name: STORAGE_KEY,
      // Only persist the token; user is re-fetched on boot via GET /me.
      partialize: (s) => ({ token: s.token }) as AuthState,
    },
  ),
);

/** Synchronous token getter for the API client (avoids React render cycles). */
export function getToken(): string | null {
  return useAuthStore.getState().token;
}
