"use client";

import { Suspense, useEffect, useMemo, useState } from "react";
import { useSearchParams } from "next/navigation";
import { useQuery } from "@tanstack/react-query";

import { useT } from "@/i18n";
import { EmptyState, Spinner, cx } from "@/components/ui";
import { API_BASE, request } from "@/lib/api/client";
import type { Manifest } from "@/lib/api/client";

/**
 * Lecture d'un album partagé.
 *
 * Volontairement plus dépouillée que le lecteur complet : pas de progression —
 * il n'y a pas de compte à qui l'attacher — pas de réglages persistés, pas de
 * bibliothèque autour. Ce qu'on prête, c'est un album, pas une application.
 */
export default function Page() {
  return (
    <Suspense
      fallback={
        <div className="grid min-h-dvh place-items-center bg-black">
          <Spinner className="size-6 text-white/60" />
        </div>
      }
    >
      <SharedReader />
    </Suspense>
  );
}

function SharedReader() {
  const t = useT();
  const params = useSearchParams();
  const token = params.get("t") ?? "";
  const comicID = params.get("id") ?? "";

  const [page, setPage] = useState(0);

  const manifest = useQuery({
    queryKey: ["shared-manifest", token, comicID],
    queryFn: () =>
      request<Manifest>(`/share/${token}/comics/${comicID}/manifest`, { anonymous: true }),
    enabled: token !== "" && comicID !== "",
    retry: false,
  });

  const pageCount = manifest.data?.pageCount ?? 0;

  const pageURL = useMemo(
    () => (index: number) =>
      `${API_BASE}/share/${encodeURIComponent(token)}/comics/${comicID}/pages/${index}?width=1600`,
    [token, comicID],
  );

  // Préchargement de la suivante : tourner une page doit être instantané, ici
  // comme dans le lecteur complet.
  useEffect(() => {
    if (page + 1 >= pageCount) return;
    const img = new Image();
    img.src = pageURL(page + 1);
  }, [page, pageCount, pageURL]);

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "ArrowRight" || event.key === " " || event.key === "ArrowDown") {
        event.preventDefault();
        setPage((p) => Math.min(pageCount - 1, p + 1));
      } else if (event.key === "ArrowLeft" || event.key === "ArrowUp") {
        event.preventDefault();
        setPage((p) => Math.max(0, p - 1));
      }
    }
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [pageCount]);

  if (manifest.isError) {
    return (
      <div className="grid min-h-dvh place-items-center bg-background">
        <EmptyState
          title={t("shared.invalid")}
          description={t("shared.expiredOrRevoked")}
        />
      </div>
    );
  }

  if (manifest.isLoading || !manifest.data) {
    return (
      <div className="grid min-h-dvh place-items-center bg-black">
        <Spinner className="size-6 text-white/60" />
      </div>
    );
  }

  return (
    <div className="relative h-dvh w-full overflow-hidden bg-black">
      <div className="flex h-full items-center justify-center">
        <img
          src={pageURL(page)}
          alt={`Page ${page + 1}`}
          decoding="async"
          draggable={false}
          className="max-h-full max-w-full select-none object-contain"
        />
      </div>

      <button
        onClick={() => setPage((p) => Math.max(0, p - 1))}
        aria-label={t("reader.previousPage")}
        className="absolute inset-y-0 left-0 w-[35%] cursor-w-resize"
      />
      <button
        onClick={() => setPage((p) => Math.min(pageCount - 1, p + 1))}
        aria-label={t("reader.nextPage")}
        className="absolute inset-y-0 right-0 w-[35%] cursor-e-resize"
      />

      <div
        className={cx(
          "pointer-events-none absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/85 to-transparent",
          "px-4 pb-3 pt-10 text-center",
        )}
      >
        <span className="font-mono text-meta tabular-nums text-white/70">
          {page + 1} / {pageCount}
        </span>
      </div>

      <a
        href={`/partage?t=${encodeURIComponent(token)}`}
        className="absolute left-3 top-3 rounded-md bg-black/60 px-3 py-1.5 text-meta text-white/80 backdrop-blur hover:text-white"
      >
        ← Retour
      </a>
    </div>
  );
}
