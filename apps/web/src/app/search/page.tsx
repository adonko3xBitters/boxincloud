"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { AppShell } from "@/components/app-shell";
import { ComicCard, ComicCardSkeleton } from "@/components/cover";
import { EmptyState, ErrorState, Input, Spinner } from "@/components/ui";
import { imageURL } from "@/lib/api/client";
import * as api from "@/lib/api/endpoints";

const MIN_QUERY = 2;
const DEBOUNCE_MS = 220;

export default function SearchPage() {
  return (
    <AppShell>
      {/* useSearchParams impose une frontière Suspense en export statique. */}
      <Suspense fallback={<Spinner className="mx-auto mt-12 size-6 text-muted" />}>
        <SearchContent />
      </Suspense>
    </AppShell>
  );
}

function SearchContent() {
  const router = useRouter();
  const params = useSearchParams();
  const initial = params.get("q") ?? "";

  const [value, setValue] = useState(initial);
  const [query, setQuery] = useState(initial);

  /*
   * Anti-rebond de la saisie.
   *
   * Sans lui, chaque frappe déclencherait une requête : sur un mot de huit
   * lettres, huit recherches plein texte dont sept sont jetées. 220 ms est le
   * seuil au-delà duquel une frappe rapide ne génère plus qu'une requête, sans
   * que l'attente se ressente.
   */
  useEffect(() => {
    const trimmed = value.trim();
    const timer = setTimeout(() => {
      setQuery(trimmed);

      // L'URL suit la recherche : un résultat se partage et se met en favori.
      // `replace` plutôt que `push` — chaque frappe ne doit pas créer une
      // entrée dans l'historique du navigateur.
      const next = trimmed ? `/search?q=${encodeURIComponent(trimmed)}` : "/search";
      router.replace(next, { scroll: false });
    }, DEBOUNCE_MS);

    return () => clearTimeout(timer);
  }, [value, router]);

  const results = useQuery({
    queryKey: ["search", query],
    queryFn: ({ signal }) => api.search({ q: query, limit: 60 }, signal),
    enabled: query.length >= MIN_QUERY,
  });

  return (
    <div className="flex flex-col gap-6">
      <div className="max-w-lg">
        <Input
          name="q"
          type="search"
          label="Rechercher"
          placeholder="Titre, série, numéro…"
          autoFocus
          value={value}
          onChange={(e) => setValue(e.target.value)}
          hint="Les accents et les petites fautes de frappe sont tolérés."
        />
      </div>

      {query.length < MIN_QUERY ? (
        <EmptyState
          title="Que cherchez-vous ?"
          description={`Saisissez au moins ${MIN_QUERY} caractères.`}
        />
      ) : results.isError ? (
        <ErrorState error={results.error} onRetry={() => void results.refetch()} />
      ) : results.isLoading ? (
        <ResultsSkeleton />
      ) : (
        <Results
          query={query}
          comics={results.data?.comics ?? []}
          series={results.data?.series ?? []}
        />
      )}
    </div>
  );
}

function Results({
  query,
  comics,
  series,
}: {
  query: string;
  comics: Awaited<ReturnType<typeof api.search>>["comics"];
  series: Awaited<ReturnType<typeof api.search>>["series"];
}) {
  if (comics.length === 0 && series.length === 0) {
    return (
      <EmptyState
        title={`Aucun résultat pour « ${query} »`}
        description="Vérifiez l'orthographe, ou essayez un terme plus court."
      />
    );
  }

  return (
    <div className="flex flex-col gap-8">
      {series.length > 0 && (
        <section>
          <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-subtle">
            Séries
          </h2>
          <div className="flex flex-wrap gap-2">
            {series.map((item) => (
              <Link
                key={item.id}
                href={`/serie?id=${item.id}`}
                className="flex items-center gap-3 rounded-lg border border-border bg-surface p-2 pr-4 transition-colors hover:bg-surface-hover"
              >
                {item.coverPath ? (
                  /* eslint-disable-next-line @next/next/no-img-element */
                  <img
                    src={imageURL(item.coverPath, { width: 160 })}
                    alt=""
                    className="h-14 w-10 rounded object-cover"
                    loading="lazy"
                  />
                ) : (
                  <div className="h-14 w-10 rounded bg-surface-sunken" />
                )}
                <div className="min-w-0">
                  <p className="truncate text-sm font-medium">{item.name}</p>
                  <p className="text-xs text-muted">
                    {item.comicCount} album{item.comicCount > 1 ? "s" : ""}
                  </p>
                </div>
              </Link>
            ))}
          </div>
        </section>
      )}

      {comics.length > 0 && (
        <section>
          <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-subtle">
            Albums
          </h2>
          <div
            className="grid gap-4"
            style={{ gridTemplateColumns: "repeat(auto-fill, minmax(140px, 1fr))" }}
          >
            {comics.map((comic, index) => (
              <ComicCard key={comic.id} comic={comic} width={320} priority={index < 12} />
            ))}
          </div>
        </section>
      )}
    </div>
  );
}

function ResultsSkeleton() {
  return (
    <div
      className="grid gap-4"
      style={{ gridTemplateColumns: "repeat(auto-fill, minmax(140px, 1fr))" }}
    >
      {Array.from({ length: 12 }, (_, i) => (
        <ComicCardSkeleton key={i} />
      ))}
    </div>
  );
}
