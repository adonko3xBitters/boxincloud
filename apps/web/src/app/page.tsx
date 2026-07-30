"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";

import { AppShell } from "@/components/app-shell";
import { ComicCard, ComicCardSkeleton } from "@/components/cover";
import { EmptyState, ErrorState } from "@/components/ui";
import * as api from "@/lib/api/endpoints";
import type { Comic, Progress } from "@/lib/api/client";

/**
 * Accueil.
 *
 * Trois étagères, dans l'ordre où elles servent : reprendre ce qu'on lisait,
 * poursuivre une série entamée, découvrir ce qui vient d'arriver. Un utilisateur
 * qui ouvre l'application veut presque toujours la première.
 */
export default function HomePage() {
  return (
    <AppShell>
      <HomeContent />
    </AppShell>
  );
}

function HomeContent() {
  const home = useQuery({ queryKey: ["home"], queryFn: () => api.getHome(20) });
  const reading = useQuery({
    queryKey: ["continue-reading"],
    queryFn: () => api.continueReading(20),
  });

  if (home.isError) return <ErrorState error={home.error} onRetry={() => void home.refetch()} />;

  const loading = home.isLoading || reading.isLoading;

  const inProgress = reading.data?.items ?? [];
  const recent = home.data?.recent ?? [];
  const next = home.data?.nextInSeries ?? [];

  // Bibliothèque réellement vide : on explique quoi faire plutôt que d'afficher
  // trois étagères vides les unes sous les autres.
  if (!loading && recent.length === 0 && inProgress.length === 0) {
    return <EmptyLibrary />;
  }

  return (
    <div className="flex flex-col gap-10">
      {loading ? (
        <ShelfSkeleton title="Reprendre la lecture" />
      ) : (
        inProgress.length > 0 && <ContinueShelf items={inProgress} />
      )}

      {!loading && next.length > 0 && (
        <Shelf title="Suite de vos séries" comics={next} />
      )}

      {loading ? (
        <ShelfSkeleton title="Récemment ajouté" />
      ) : (
        recent.length > 0 && (
          <Shelf title="Récemment ajouté" comics={recent} href="/library" />
        )
      )}
    </div>
  );
}

/**
 * Étagère horizontale.
 *
 * Défilement horizontal plutôt que grille : une étagère montre une sélection,
 * pas un catalogue. La grille complète vit sur /library.
 */
function Shelf({
  title,
  comics,
  href,
}: {
  title: string;
  comics: Comic[];
  href?: string;
}) {
  return (
    <section>
      <div className="mb-3 flex items-baseline justify-between gap-4">
        <h2 className="text-lg font-semibold tracking-tight">{title}</h2>
        {href && (
          <Link href={href} className="text-sm text-muted transition-colors hover:text-accent-text">
            Tout voir
          </Link>
        )}
      </div>

      <div className="no-scrollbar -mx-4 flex snap-x snap-mandatory gap-4 overflow-x-auto px-4 pb-2 sm:mx-0 sm:px-0">
        {comics.map((comic, index) => (
          <div key={comic.id} className="w-[140px] shrink-0 snap-start sm:w-[160px]">
            <ComicCard comic={comic} width={320} priority={index < 6} />
          </div>
        ))}
      </div>
    </section>
  );
}

/**
 * Étagère « Reprendre la lecture ».
 *
 * Distincte des autres : elle affiche l'avancement, qui est l'information la
 * plus utile pour décider si l'on rouvre cet album maintenant.
 */
function ContinueShelf({ items }: { items: Progress[] }) {
  const comics = useQuery({
    queryKey: ["continue-reading", "comics", items.map((i) => i.comicId)],
    queryFn: async () => {
      const results = await Promise.all(
        items.map((item) =>
          api.getComic(item.comicId).catch(() => null),
        ),
      );
      return results.filter((c): c is Comic => c !== null);
    },
    enabled: items.length > 0,
  });

  if (!comics.data || comics.data.length === 0) return null;

  const percentByComic = new Map(items.map((i) => [i.comicId, i.percent]));

  return (
    <section>
      <h2 className="mb-3 text-lg font-semibold tracking-tight">Reprendre la lecture</h2>

      <div className="no-scrollbar -mx-4 flex snap-x snap-mandatory gap-4 overflow-x-auto px-4 pb-2 sm:mx-0 sm:px-0">
        {comics.data.map((comic, index) => (
          <div key={comic.id} className="w-[140px] shrink-0 snap-start sm:w-[160px]">
            <ComicCard
              comic={comic}
              width={320}
              priority={index < 6}
              progressPercent={percentByComic.get(comic.id)}
            />
          </div>
        ))}
      </div>
    </section>
  );
}

function ShelfSkeleton({ title }: { title: string }) {
  return (
    <section>
      <h2 className="mb-3 text-lg font-semibold tracking-tight text-muted">{title}</h2>
      <div className="no-scrollbar -mx-4 flex gap-4 overflow-hidden px-4 sm:mx-0 sm:px-0">
        {Array.from({ length: 8 }, (_, i) => (
          <div key={i} className="w-[140px] shrink-0 sm:w-[160px]">
            <ComicCardSkeleton />
          </div>
        ))}
      </div>
    </section>
  );
}

function EmptyLibrary() {
  return (
    <EmptyState
      icon={
        <svg viewBox="0 0 48 48" fill="none" className="size-12" aria-hidden="true">
          <path
            d="M8 14 24 8l16 6v20l-16 6-16-6V14Z"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinejoin="round"
          />
          <path d="M8 14l16 6 16-6M24 20v20" stroke="currentColor" strokeWidth="2" strokeLinejoin="round" />
        </svg>
      }
      title="Votre bibliothèque est vide"
      description="Connectez un espace de stockage et lancez un scan pour voir vos albums apparaître ici."
      action={
        <div className="rounded-lg bg-surface-sunken p-4 text-left">
          <p className="mb-2 text-xs font-medium uppercase tracking-wide text-subtle">
            Depuis le serveur
          </p>
          <pre className="overflow-x-auto text-xs leading-relaxed">
            <code>{`boxincloudctl library add BD monminio bd/
boxincloudctl scan-now BD`}</code>
          </pre>
        </div>
      }
    />
  );
}
