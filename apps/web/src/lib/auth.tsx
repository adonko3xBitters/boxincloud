"use client";

/**
 * État de session côté application.
 *
 * `useSyncExternalStore` plutôt qu'un contexte : les jetons vivent dans
 * localStorage, hors de React. S'abonner à leur changement garantit qu'une
 * déconnexion déclenchée depuis le client HTTP — par exemple après un refresh
 * token refusé — se propage immédiatement à l'interface.
 */

import { useRouter } from "next/navigation";
import { useCallback, useEffect, useSyncExternalStore } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";

import * as api from "./api/endpoints";
import { setTokens, clearTokens, getTokens, subscribeToTokens } from "./api/tokens";
import type { User } from "./api/client";

function subscribe(listener: () => void) {
  return subscribeToTokens(listener);
}

function getSnapshot(): boolean {
  return getTokens() !== null;
}

/**
 * Snapshot serveur.
 *
 * L'export statique pré-rend les pages au build : localStorage n'existe pas
 * alors. On répond « non authentifié », et le client corrige à l'hydratation.
 */
function getServerSnapshot(): boolean {
  return false;
}

export function useIsAuthenticated(): boolean {
  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
}

/** Compte courant, chargé une seule fois et partagé. */
export function useCurrentUser() {
  const authenticated = useIsAuthenticated();

  return useQuery<User>({
    queryKey: ["me"],
    queryFn: api.getCurrentUser,
    enabled: authenticated,
    staleTime: 5 * 60_000,
  });
}

export function useLogin() {
  const queryClient = useQueryClient();

  return useCallback(
    async (username: string, password: string) => {
      const tokens = await api.login({ username, password });
      setTokens(tokens);
      // Le cache appartient au compte précédent : le vider évite qu'une
      // bibliothèque reste affichée après un changement d'utilisateur.
      queryClient.clear();
      queryClient.setQueryData(["me"], tokens.user);
      return tokens.user;
    },
    [queryClient],
  );
}

export function useLogout() {
  const queryClient = useQueryClient();
  const router = useRouter();

  return useCallback(async () => {
    const stored = getTokens();

    if (stored?.refreshToken) {
      // Best effort : si le serveur est injoignable, on se déconnecte
      // localement quand même. Rester bloqué sur une session morte serait pire.
      try {
        await api.logout(stored.refreshToken);
      } catch {
        // ignoré volontairement
      }
    }

    clearTokens();
    queryClient.clear();
    router.replace("/login");
  }, [queryClient, router]);
}

/**
 * Garde de route : redirige vers la connexion si la session manque.
 *
 * À placer dans les pages authentifiées. La redirection passe par un effet
 * plutôt que par le rendu, pour ne pas déclencher une navigation pendant la
 * phase de rendu de React.
 */
export function useRequireAuth(): boolean {
  const authenticated = useIsAuthenticated();
  const router = useRouter();

  useEffect(() => {
    if (!authenticated) {
      router.replace("/login");
    }
  }, [authenticated, router]);

  return authenticated;
}

/** État de l'instance : décide entre assistant d'installation et connexion. */
export function useAuthStatus() {
  return useQuery({
    queryKey: ["auth-status"],
    queryFn: api.getAuthStatus,
    staleTime: Infinity,
    retry: 1,
  });
}
