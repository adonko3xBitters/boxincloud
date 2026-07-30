"use client";

import { Suspense } from "react";

import { Spinner } from "@/components/ui";
import { ReaderView } from "./reader";

/**
 * Lecteur : `/read?id=…&page=…`
 *
 * Hors de la coquille applicative — le lecteur occupe tout l'écran, sans
 * en-tête ni navigation. C'est la seule page du projet dans ce cas, et c'est
 * délibéré : on lit une planche, pas une application.
 */
export default function Page() {
  return (
    <Suspense
      fallback={
        <div className="grid min-h-dvh place-items-center bg-black">
          <Spinner className="size-7 text-white/60" />
        </div>
      }
    >
      <ReaderView />
    </Suspense>
  );
}
