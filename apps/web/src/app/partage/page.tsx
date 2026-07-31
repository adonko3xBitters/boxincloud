"use client";

import { Suspense } from "react";
import { useSearchParams } from "next/navigation";
import { useQuery } from "@tanstack/react-query";

import { BrandLockup } from "@/components/brand";
import { useT } from "@/i18n";
import { EmptyState, Spinner, cx } from "@/components/ui";
import { API_BASE } from "@/lib/api/client";
import * as api from "@/lib/api/endpoints";

/**
 * Page d'un lien public.
 *
 * La seule de boxincloud qui ne demande pas de compte. Elle ne montre que ce
 * que le lien désigne — ni barre latérale, ni recherche, ni rien qui laisse
 * deviner l'existence du reste de la bibliothèque.
 *
 * Le jeton voyage en paramètre plutôt qu'en segment de chemin : l'export
 * statique de Next.js ne produit pas de routes dynamiques, et une adresse
 * `/partage?t=…` se copie aussi bien.
 */
export default function Page() {
  return (
    <Suspense
      fallback={
        <div className="grid min-h-dvh place-items-center">
          <Spinner className="size-6 text-muted" />
        </div>
      }
    >
      <SharedView />
    </Suspense>
  );
}

function SharedView() {
  const t = useT();
  const token = useSearchParams().get("t") ?? "";

  const shared = useQuery({
    queryKey: ["shared", token],
    queryFn: () => api.getSharedContent(token),
    enabled: token !== "",
    retry: false,
  });

  if (!token || shared.isError) {
    return (
      <Frame>
        <EmptyState
          title={t("shared.invalid")}
          description={t("shared.expiredOrRevokedBy")}
        />
      </Frame>
    );
  }

  if (shared.isLoading || !shared.data) {
    return (
      <Frame>
        <div className="grid place-items-center py-20">
          <Spinner className="size-6 text-muted" />
        </div>
      </Frame>
    );
  }

  const { comics, label, expiresAt } = shared.data;

  return (
    <Frame>
      <header className="mb-6">
        <h1 className="text-title font-semibold text-fg">
          {label || (comics.length > 1 ? t("shared.albums") : t("shared.album"))}
        </h1>
        <p className="mt-1 text-meta text-muted">
          {comics.length} album{comics.length > 1 ? "s" : ""} · accessible
          jusqu&apos;au {new Date(expiresAt).toLocaleDateString("fr-FR")}
        </p>
      </header>

      {comics.length === 0 ? (
        <EmptyState title={t("shared.nothingTitle")} description={t("shared.nothingDetail")} />
      ) : (
        <div
          className="grid gap-x-5 gap-y-6"
          style={{ gridTemplateColumns: "repeat(auto-fill, minmax(180px, 1fr))" }}
        >
          {comics.map((comic, index) => (
            <a
              key={comic.id}
              href={`/partage/lire?t=${encodeURIComponent(token)}&id=${comic.id}`}
              style={{ animationDelay: `${Math.min(index, 20) * 22}ms` }}
              className="rise-in group rounded-lg p-2 transition-[background-color,transform] duration-(--motion-duration-fast) ease-emphasized hover:-translate-y-0.5 hover:bg-surface-hover"
            >
              <span
                className={cx(
                  "relative block overflow-hidden rounded-[5px] bg-surface-sunken",
                  "shadow-[var(--shadow-cover)] transition-shadow duration-(--motion-duration-normal)",
                  "group-hover:shadow-[var(--shadow-cover-hover)]",
                )}
                style={{ aspectRatio: 0.7 }}
              >
                <img
                  src={`${API_BASE.replace(/\/api\/v1$/, "")}${comic.coverPath}?width=640`}
                  alt=""
                  loading="lazy"
                  decoding="async"
                  className="size-full object-cover"
                />
              </span>

              <span className="mt-2.5 block truncate text-ui font-medium text-fg" title={comic.title}>
                {comic.title}
              </span>
              <span className="mt-0.5 block truncate text-meta text-subtle">
                {comic.seriesName
                  ? `${comic.seriesName}${comic.number ? ` · ${comic.number}` : ""}`
                  : `${comic.pageCount} p.`}
              </span>
            </a>
          ))}
        </div>
      )}
    </Frame>
  );
}

function Frame({ children }: { children: React.ReactNode }) {
  return (
    <div className="min-h-dvh bg-background">
      <header className="border-b border-border bg-surface px-4 py-3">
        <BrandLockup />
      </header>

      <main className="mx-auto max-w-6xl p-6">{children}</main>

      <footer className="mx-auto max-w-6xl px-6 pb-8 text-meta leading-relaxed text-subtle">
        Ce lien donne accès à ces albums sans compte. Il expire à la date
        indiquée, et son auteur peut le fermer à tout moment.
      </footer>
    </div>
  );
}
