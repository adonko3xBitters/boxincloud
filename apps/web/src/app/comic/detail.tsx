"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useQuery } from "@tanstack/react-query";

import { Cover } from "@/components/cover";
import { Badge, Button, ErrorState, Skeleton } from "@/components/ui";
import * as api from "@/lib/api/endpoints";
import type { Comic, Progress } from "@/lib/api/client";

/**
 * Page de détail d'un album.
 *
 * L'écran depuis lequel on décide d'ouvrir un album. Il met donc en avant la
 * couverture en grand, l'action de lecture, et la progression s'il y en a une —
 * le reste des métadonnées vient après.
 */
export function ComicDetailView() {
  const comicId = useSearchParams().get("id") ?? "";

  const comic = useQuery({
    queryKey: ["comic", comicId],
    queryFn: () => api.getComic(comicId),
    enabled: Boolean(comicId),
  });

  const progress = useQuery({
    queryKey: ["progress", comicId],
    queryFn: () => api.getProgress(comicId),
    enabled: Boolean(comicId),
  });

  if (comic.isError) {
    return <ErrorState error={comic.error} onRetry={() => void comic.refetch()} />;
  }
  if (comic.isLoading || !comic.data) {
    return <DetailSkeleton />;
  }

  return <Detail comic={comic.data} progress={progress.data} />;
}

function Detail({ comic, progress }: { comic: Comic; progress?: Progress }) {
  const started = progress && progress.status !== "unread" && progress.page > 0;
  const finished = progress?.status === "read";

  return (
    <article className="flex flex-col gap-8 lg:flex-row lg:gap-10">
      <div className="mx-auto w-full max-w-[260px] shrink-0 lg:mx-0">
        <Cover comic={comic} width={640} priority />
      </div>

      <div className="min-w-0 flex-1">
        {comic.seriesName && comic.seriesId && (
          <Link
            href={`/serie?id=${comic.seriesId}`}
            className="text-sm font-medium text-accent-text hover:underline"
          >
            {comic.seriesName}
          </Link>
        )}

        <h1 className="mt-1 text-2xl font-semibold tracking-tight">{comic.title}</h1>

        <div className="mt-3 flex flex-wrap items-center gap-2">
          {comic.number && <Badge tone="accent">N° {comic.number}</Badge>}
          <Badge>{comic.pageCount} pages</Badge>
          <Badge>{comic.format.toUpperCase()}</Badge>
          {comic.language && <Badge>{comic.language.toUpperCase()}</Badge>}
          {finished && <Badge tone="success">Lu</Badge>}
          {comic.state === "error" && <Badge tone="danger">Erreur d&apos;indexation</Badge>}
        </div>

        {/*
          Le lecteur arrive en M4. Le bouton est donc désactivé plutôt
          qu'absent : l'utilisateur voit où se trouvera l'action, et l'infobulle
          dit pourquoi elle ne fonctionne pas encore.
        */}
        <div className="mt-6 flex flex-wrap items-center gap-3">
          <Button
            size="lg"
            disabled
            title="Le lecteur arrive dans une prochaine version"
          >
            {started ? `Reprendre p. ${progress!.page + 1}` : "Lire"}
          </Button>

          {started && (
            <div className="min-w-[160px] flex-1 sm:max-w-xs">
              <div className="mb-1 flex justify-between text-xs text-muted">
                <span>
                  Page {progress!.page + 1} sur {progress!.pageCount || comic.pageCount}
                </span>
                <span>{Math.round(progress!.percent)} %</span>
              </div>
              <div className="h-1.5 overflow-hidden rounded-full bg-surface-sunken">
                <div
                  className="h-full rounded-full bg-accent transition-[width] duration-[--motion-duration-normal]"
                  style={{ width: `${Math.min(100, progress!.percent)}%` }}
                />
              </div>
            </div>
          )}
        </div>

        {comic.summary && (
          <p className="mt-6 max-w-prose text-sm leading-relaxed text-muted">{comic.summary}</p>
        )}

        <dl className="mt-8 grid grid-cols-2 gap-x-8 gap-y-4 text-sm sm:max-w-md">
          {comic.releasedAt && <Field label="Parution" value={formatDate(comic.releasedAt)} />}
          {comic.volume !== undefined && <Field label="Volume" value={String(comic.volume)} />}
          <Field label="Taille" value={formatBytes(comic.fileSize)} />
          <Field label="Ajouté" value={formatDate(comic.createdAt)} />
        </dl>
      </div>
    </article>
  );
}

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt className="text-xs uppercase tracking-wide text-subtle">{label}</dt>
      <dd className="mt-0.5 text-fg">{value}</dd>
    </div>
  );
}

function DetailSkeleton() {
  return (
    <div className="flex flex-col gap-8 lg:flex-row lg:gap-10">
      <div className="mx-auto w-full max-w-[260px] shrink-0 lg:mx-0">
        <Skeleton className="w-full" />
        <div className="skeleton rounded-cover" style={{ aspectRatio: 0.7 }} />
      </div>
      <div className="flex-1 space-y-3">
        <Skeleton className="h-4 w-32" />
        <Skeleton className="h-7 w-2/3" />
        <Skeleton className="h-5 w-48" />
        <Skeleton className="h-12 w-40" />
        <Skeleton className="h-20 w-full max-w-prose" />
      </div>
    </div>
  );
}

// ─── Formatage ───────────────────────────────────────────────────────────────

function formatDate(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  return new Intl.DateTimeFormat("fr-FR", { dateStyle: "long" }).format(date);
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} o`;
  const units = ["ko", "Mo", "Go", "To"];
  let value = bytes / 1024;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit++;
  }
  return `${value.toFixed(1)} ${units[unit]}`;
}
