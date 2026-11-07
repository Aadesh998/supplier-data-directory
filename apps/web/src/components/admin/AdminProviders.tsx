"use client";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useState } from "react";
import { ApiError } from "@/lib/api-error";
import type { Session } from "@/lib/types";
import { ToastProvider } from "@/components/ui/Toast";
import { AuthProvider } from "./AuthProvider";

export function makeQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 0,
        gcTime: 5 * 60_000,
        refetchOnWindowFocus: false,
        retry: (count, err) => !(err instanceof ApiError && err.status < 500) && count < 2,
      },
      mutations: { retry: false },
    },
  });
}

export function AdminProviders({ children, initialSession, queryClient }: { children: React.ReactNode; initialSession?: Session; queryClient?: QueryClient }) {
  const [client] = useState(() => queryClient ?? makeQueryClient());
  return (
    <QueryClientProvider client={client}>
      <ToastProvider>
        <AuthProvider initialSession={initialSession}>{children}</AuthProvider>
      </ToastProvider>
    </QueryClientProvider>
  );
}
