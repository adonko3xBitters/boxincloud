"use client";

import { Suspense } from "react";

import { AppShell } from "@/components/app-shell";
import { Spinner } from "@/components/ui";
import { SeriesDetailView } from "./detail";

/** Détail d'une série : `/serie?id=…` — voir /comic/page.tsx. */
export default function Page() {
  return (
    <AppShell>
      <Suspense fallback={<Spinner className="mx-auto mt-12 size-6 text-muted" />}>
        <SeriesDetailView />
      </Suspense>
    </AppShell>
  );
}
