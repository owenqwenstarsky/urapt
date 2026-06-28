import { useState, type ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ThemeProvider } from "@/components/theme-provider";
import { Toaster } from "@/components/ui/sonner";
import { configureAuth } from "@/lib/api";
import { getToken, useAuthStore } from "@/stores/auth";

/** Wire the auth store's token + 401 handler into the API client once. */
configureAuth(getToken, () => {
  // A 401 means the token is invalid/expired. Clear local session; the root
  // guard will redirect to /login on the next render cycle.
  useAuthStore.getState().clear();
});

export function Providers({ children }: { children: ReactNode }) {
  const [client] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            retry: (failureCount, error) => {
              // Don't retry on auth errors; the user must re-authenticate.
              const status = (error as { status?: number })?.status;
              if (status === 401 || status === 403 || status === 404) return false;
              return failureCount < 2;
            },
            staleTime: 30_000,
            refetchOnWindowFocus: false,
          },
        },
      }),
  );

  return (
    <ThemeProvider
      attribute="class"
      defaultTheme="system"
      enableSystem
      disableTransitionOnChange
    >
      <QueryClientProvider client={client}>
        {children}
        <Toaster richColors position="top-right" />
      </QueryClientProvider>
    </ThemeProvider>
  );
}
