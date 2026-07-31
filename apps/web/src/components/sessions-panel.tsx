"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";

import { Button, Spinner, cx } from "./ui";
import * as api from "@/lib/api/endpoints";
import { useLocale, type Locale, type MessageKey } from "@/i18n";

/**
 * Appareils connectés au compte.
 *
 * Un écran qui n'existe que pour un moment précis : celui où l'on s'aperçoit
 * qu'un téléphone a disparu. Tout y est donc orienté vers un geste unique et
 * sans ambiguïté — couper cet appareil-là, tout de suite.
 */
export function SessionsPanel({ onClose }: { onClose: () => void }) {
  const { locale, t } = useLocale();
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
    onError: (e: unknown) => setError(e instanceof Error ? e.message : t("error.generic")),
  });

  const logoutAll = useMutation({
    mutationFn: api.logoutAllDevices,
    onSuccess: () => {
      // Toutes les sessions sont coupées, y compris celle-ci : recharger
      // renvoie vers la connexion, ce qui est exactement ce qu'on a demandé.
      window.location.reload();
    },
    onError: (e: unknown) => setError(e instanceof Error ? e.message : t("error.generic")),
  });

  return (
    <div className="fixed inset-0 z-[60] grid place-items-center bg-[var(--overlay)] p-4">
      <div
        role="dialog"
        aria-modal="true"
        aria-label={t("devices.title")}
        className="rise-in flex max-h-[80vh] w-full max-w-lg flex-col gap-4 rounded-xl border border-border bg-surface p-4 shadow-2xl"
      >
        <div className="flex items-center justify-between">
          <h2 className="text-title font-semibold text-fg">{t("devices.title")}</h2>
          <button
            onClick={onClose}
            aria-label={t("action.close")}
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
            <p className="text-ui text-muted">{t("devices.empty")}</p>
          )}

          <ul className="flex flex-col gap-1">
            {devices.data?.devices.map((device) => (
              <li
                key={device.id}
                className="flex items-center gap-3 rounded-lg border border-border bg-surface-sunken px-3 py-2"
              >
                <div className="min-w-0 flex-1">
                  <p className="truncate text-ui text-fg">
                    {device.name || t("devices.unnamed")}
                    {device.current && (
                      <span className="ml-2 rounded bg-accent-subtle px-1.5 py-0.5 text-micro font-semibold text-accent-text">
                        {t("devices.current")}
                      </span>
                    )}
                  </p>
                  <p className="truncate text-meta text-subtle">
                    {platformLabel(device.platform, t)}
                    {device.appVersion ? ` · ${device.appVersion}` : ""} ·{" "}
                    {lastSeen(device.lastSeenAt, locale, t)}
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
                  {t("devices.revoke")}
                </button>
              </li>
            ))}
          </ul>
        </div>

        <div className="flex items-center justify-between gap-3 border-t border-border pt-3">
          <p className="text-meta leading-relaxed text-subtle">
{t("devices.hint")}
          </p>
          <Button
            variant="danger"
            disabled={logoutAll.isPending}
            onClick={() => logoutAll.mutate()}
          >
            {t("devices.revokeAll")}
          </Button>
        </div>
      </div>
    </div>
  );
}

function platformLabel(platform: string, t: (k: MessageKey) => string): string {
  switch (platform) {
    case "web":
      return t("devices.platform.web");
    case "android":
      return t("devices.platform.android");
    case "ios":
      return t("devices.platform.ios");
    case "desktop":
      return t("devices.platform.desktop");
    default:
      // Une plateforme inconnue s'affiche telle quelle : inventer un libellé
      // pour une valeur qu'on ne connaît pas serait pire que la montrer.
      return platform;
  }
}

/**
 * Dernière activité, en langage courant.
 *
 * Une date absolue obligerait à faire le calcul soi-même — et c'est justement
 * l'écart au présent qui compte : un appareil vu « il y a 3 mois » se révoque
 * sans hésiter, un appareil vu « à l'instant » mérite qu'on réfléchisse.
 *
 * `Intl.RelativeTimeFormat` plutôt qu'une chaîne par unité et par langue : le
 * navigateur connaît déjà les formes, y compris celles qu'on n'aurait pas
 * prévues, et le catalogue n'a pas à porter douze entrées pour une information
 * que la plateforme sait produire.
 */
function lastSeen(iso: string, locale: Locale, t: (k: MessageKey) => string): string {
  const seen = new Date(iso).getTime();
  if (Number.isNaN(seen)) return t("devices.seen.unknown");

  const minutes = Math.round((Date.now() - seen) / 60000);
  if (minutes < 2) return t("devices.seen.now");

  const format = new Intl.RelativeTimeFormat(locale, { numeric: "always" });

  if (minutes < 60) return format.format(-minutes, "minute");

  const hours = Math.round(minutes / 60);
  if (hours < 24) return format.format(-hours, "hour");

  const days = Math.round(hours / 24);
  if (days < 30) return format.format(-days, "day");

  return format.format(-Math.round(days / 30), "month");
}
