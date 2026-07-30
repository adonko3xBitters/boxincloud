"use client";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useState } from "react";
import { ApiError } from "@/lib/api/client";

export function Providers({ children }: { children: React.ReactNode }) {
  const [client] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            // Une bibliothèque bouge peu entre deux scans : garder les données
            // fraîches une minute évite de recharger la grille à chaque
            // navigation, ce qui se voit immédiatement à l'usage.
            staleTime: 60_000,
            gcTime: 5 * 60_000,

            // Le rechargement au retour d'onglet est agaçant sur une grille de
            // couvertures : l'image se recharge sans que rien n'ait changé.
            refetchOnWindowFocus: false,

            retry: (failureCount, error) => {
              // Inutile de réessayer ce que le serveur a explicitement refusé.
              if (error instanceof ApiError && error.status >= 400 && error.status < 500) {
                return false;
              }
              return failureCount < 2;
            },
          },
        },
      }),
  );

  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}
