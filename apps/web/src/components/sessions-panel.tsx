"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";

import { Button, Spinner, cx } from "./ui";
import * as api from "@/lib/api/endpoints";

/**
 * Appareils connectés au compte.
 *
 * Un écran qui n'existe que pour un moment précis : celui où l'on s'aperçoit
 * qu'un téléphone a disparu. Tout y est donc orienté vers un geste unique et
 * sans ambiguïté — couper cet appareil-là, tout de suite.
 */
export function SessionsPanel({ onClose }: { onClose: () => void }) {
  const queryClient = useQueryClient();
  const [error, setError] = useState<string | null>(null);

  const devices = useQuery({ queryKey: ["devices"], queryFn: api.listDevices });

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") onClose();
    }
    document.addEventListener("keydown", onKeyDown, true);
    return () => document.removeEventListener("keydown", onKeyDown, true);
  }, [onClose]);

  const revoke = useMutation({
    mutationFn: api.revokeDevice,
    onSuccess: () => {
      setError(null);
      void queryClient.invalidateQueries({ queryKey: ["devices"] });
    },
    onError: (e: unknown) => setError(describe(e)),
  });

  const logoutAll = useMutation({
    mutationFn: api.logoutAllDevices,
    onSuccess: () => {
      // Toutes les sessions sont coupées, y compris celle-ci : recharger
      // renvoie vers la connexion, ce qui est exactement ce qu'on a demandé.
      window.location.reload();
    },
    onError: (e: unknown) => setError(describe(e)),
  });

  return (
    <div className="fixed inset-0 z-[60] grid place-items-center bg-[var(--overlay)] p-4">
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Appareils connectés"
        className="rise-in flex max-h-[80vh] w-full max-w-lg flex-col gap-4 rounded-xl border border-border bg-surface p-4 shadow-2xl"
      >
        <div className="flex items-center justify-between">
          <h2 className="text-title font-semibold text-fg">Appareils connectés</h2>
          <button
            onClick={onClose}
            aria-label="Fermer"
            className="pressable grid size-8 place-items-center rounded text-subtle hover:bg-surface-hover hover:text-fg"
          >
            <svg viewBox="0 0 16 16" fill="none" className="size-4" aria-hidden="true">
              <path d="m4 4 8 8M12 4l-8 8" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
            </svg>
          </button>
        </div>

        {error && (
          <p className="rounded-md border border-danger/40 bg-danger/10 px-3 py-2 text-meta leading-relaxed text-danger">
            {error}
          </p>
        )}

        <div className="min-h-0 flex-1 overflow-y-auto">
          {devices.isLoading && <Spinner className="size-5 text-muted" />}

          {devices.data?.devices.length === 0 && (
            <p className="text-ui text-muted">Aucun appareil enregistré.</p>
          )}

          <ul className="flex flex-col gap-1">
            {devices.data?.devices.map((device) => (
              <li
                key={device.id}
                className="flex items-center gap-3 rounded-lg border border-border bg-surface-sunken px-3 py-2"
              >
                <div className="min-w-0 flex-1">
                  <p className="truncate text-ui text-fg">
                    {device.name || "Appareil sans nom"}
                    {device.current && (
                      <span className="ml-2 rounded bg-accent-subtle px-1.5 py-0.5 text-micro font-semibold text-accent-text">
                        celui-ci
                      </span>
                    )}
                  </p>
                  <p className="truncate text-meta text-subtle">
                    {platformLabel(device.platform)}
                    {device.appVersion ? ` · ${device.appVersion}` : ""} ·{" "}
                    {lastSeen(device.lastSeenAt)}
                  </p>
                </div>

                <button
                  disabled={revoke.isPending}
                  onClick={() => revoke.mutate(device.id)}
                  className={cx(
                    "pressable shrink-0 rounded-md border border-border px-2.5 py-1 text-meta",
                    "text-danger hover:bg-danger/10 disabled:opacity-40",
                  )}
                >
                  Révoquer
                </button>
              </li>
            ))}
          </ul>
        </div>

        <div className="flex items-center justify-between gap-3 border-t border-border pt-3">
          <p className="text-meta leading-relaxed text-subtle">
            Révoquer un appareil coupe son accès immédiatement, sans toucher aux
            autres.
          </p>
          <Button
            variant="danger"
            disabled={logoutAll.isPending}
            onClick={() => logoutAll.mutate()}
          >
            Tout déconnecter
          </Button>
        </div>
      </div>
    </div>
  );
}

function platformLabel(platform: string): string {
  switch (platform) {
    case "web":
      return "Navigateur";
    case "android":
      return "Android";
    case "ios":
      return "iOS";
    case "desktop":
      return "Ordinateur";
    default:
      return platform;
  }
}

/**
 * Dernière activité, en langage courant.
 *
 * Une date absolue obligerait à faire le calcul soi-même — et c'est justement
 * l'écart au présent qui compte : un appareil vu « il y a 3 mois » se révoque
 * sans hésiter, un appareil vu « à l'instant » mérite qu'on réfléchisse.
 */
function lastSeen(iso: string): string {
  const seen = new Date(iso).getTime();
  if (Number.isNaN(seen)) return "date inconnue";

  const minutes = Math.round((Date.now() - seen) / 60000);
  if (minutes < 2) return "à l'instant";
  if (minutes < 60) return `il y a ${minutes} min`;

  const hours = Math.round(minutes / 60);
  if (hours < 24) return `il y a ${hours} h`;

  const days = Math.round(hours / 24);
  if (days < 30) return `il y a ${days} j`;

  const months = Math.round(days / 30);
  return `il y a ${months} mois`;
}

function describe(error: unknown): string {
  if (error instanceof Error) return error.message;
  return "Une erreur est survenue.";
}
