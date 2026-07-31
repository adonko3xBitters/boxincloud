"use client";

import Link from "next/link";
import { Suspense, useMemo } from "react";
import { useRouter, useSearchParams } from "next/navigation";

import { AccountsPanel } from "@/components/accounts-panel";
import { BrandLockup } from "@/components/brand";
import { DiscoverySources } from "@/components/discovery-panel";
import { MobileAppDialog } from "@/components/mobile-app-dialog";
import { SessionsPanel } from "@/components/sessions-panel";
import { StoragePanel } from "@/components/storage-panel";
import { cx } from "@/components/ui";
import { useCurrentUser } from "@/lib/auth";
import { useT, type MessageKey } from "@/i18n";

/**
 * Configuration — une vraie page, pas une boîte de dialogue.
 *
 * La différence n'est pas cosmétique. Une boîte de dialogue est faite pour une
 * décision courte prise sans quitter ce qu'on faisait ; régler ses espaces de
 * stockage ou ses comptes n'est ni court, ni accessoire à la lecture en cours.
 *
 * Une page apporte trois choses qu'une modale ne peut pas donner : une adresse
 * qu'on peut garder en signet ou envoyer à quelqu'un, un bouton Retour du
 * navigateur qui fait ce qu'on attend, et toute la largeur — les tableaux de
 * bibliothèques et de comptes y étaient à l'étroit.
 *
 * La section ouverte vit dans l'URL plutôt que dans un état local, pour la même
 * raison : `/configuration?section=stockage` se partage, se recharge, et revient
 * au bon endroit après un retour arrière.
 */

type SectionKey = "stockage" | "comptes" | "catalogues" | "appareils" | "application";

type Section = {
  key: SectionKey;
  title: MessageKey;
  description: MessageKey;
  /** Réservée aux administrateurs : l'API refuserait de toute façon. */
  adminOnly: boolean;
  icon: React.ReactNode;
};

const SECTIONS: Section[] = [
  {
    key: "stockage",
    title: "storage.title",
    description: "settings.storage.hint",
    adminOnly: true,
    icon: (
      <path
        d="M3 5.5C3 4.4 5.7 3.5 9 3.5s6 .9 6 2v7c0 1.1-2.7 2-6 2s-6-.9-6-2v-7Zm0 3.5c0 1.1 2.7 2 6 2s6-.9 6-2"
        stroke="currentColor"
        strokeWidth="1.4"
        strokeLinecap="round"
      />
    ),
  },
  {
    key: "comptes",
    title: "accounts.title",
    description: "settings.accounts.hint",
    adminOnly: true,
    icon: (
      <path
        d="M9 9a2.75 2.75 0 1 0 0-5.5A2.75 2.75 0 0 0 9 9Zm-5.5 5.5c0-2.5 2.5-4 5.5-4s5.5 1.5 5.5 4"
        stroke="currentColor"
        strokeWidth="1.4"
        strokeLinecap="round"
      />
    ),
  },
  {
    key: "catalogues",
    title: "discovery.sources.title",
    description: "settings.sources.hint",
    adminOnly: true,
    icon: (
      <path
        d="M9 15.5a6.5 6.5 0 1 0 0-13 6.5 6.5 0 0 0 0 13Zm-6.5-6.5h13M9 2.5c1.8 2 2.7 4.2 2.7 6.5S10.8 13.5 9 15.5C7.2 13.5 6.3 11.3 6.3 9S7.2 4.5 9 2.5Z"
        stroke="currentColor"
        strokeWidth="1.4"
        strokeLinecap="round"
      />
    ),
  },
  {
    key: "appareils",
    title: "account.devices",
    description: "settings.devices.hint",
    adminOnly: false,
    icon: (
      <path
        d="M6.5 2.5h5a1.5 1.5 0 0 1 1.5 1.5v10a1.5 1.5 0 0 1-1.5 1.5h-5A1.5 1.5 0 0 1 5 14V4a1.5 1.5 0 0 1 1.5-1.5Zm2 10.5h1"
        stroke="currentColor"
        strokeWidth="1.4"
        strokeLinecap="round"
      />
    ),
  },
  {
    key: "application",
    title: "download.title",
    description: "settings.mobile.hint",
    adminOnly: false,
    icon: (
      <path
        d="M9 3v8m0 0 3-3m-3 3-3-3m-3 6h12"
        stroke="currentColor"
        strokeWidth="1.4"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    ),
  },
];

export default function Page() {
  /*
    `useSearchParams` suspend au premier rendu en export statique : la page est
    construite sans connaître l'URL sous laquelle elle sera servie. La frontière
    est donc obligatoire, et son repli tient la place plutôt que de laisser un
    écran blanc.
  */
  return (
    <Suspense fallback={<Frame />}>
      <Configuration />
    </Suspense>
  );
}

function Configuration() {
  const t = useT();
  const router = useRouter();
  const params = useSearchParams();
  const { data: user } = useCurrentUser();

  const isAdmin = user?.role === "admin";

  const sections = useMemo(
    () => SECTIONS.filter((entry) => !entry.adminOnly || isAdmin),
    [isAdmin],
  );

  const requested = params.get("section") as SectionKey | null;
  // Une section demandée mais interdite retombe sur le sommaire : afficher un
  // refus pour une adresse recopiée serait plus déroutant qu'utile.
  const active = sections.find((entry) => entry.key === requested) ?? null;

  const open = (key: SectionKey) => router.push(`/configuration?section=${key}`);

  return (
    <Frame
      breadcrumb={
        active && (
          <>
            <span aria-hidden="true" className="text-subtle">
              /
            </span>
            <Link
              href="/configuration"
              className="text-ui text-muted hover:text-fg hover:underline"
            >
              {t("settings.title")}
            </Link>
            <span aria-hidden="true" className="text-subtle">
              /
            </span>
            <span className="text-ui font-medium text-fg">{t(active.title)}</span>
          </>
        )
      }
    >
      {active ? (
        <SectionBody section={active.key} onDone={() => router.push("/configuration")} />
      ) : (
        <ul className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {sections.map((entry) => (
            <li key={entry.key}>
              <button
                onClick={() => open(entry.key)}
                className={cx(
                  "pressable flex h-full w-full items-start gap-3 rounded-lg border border-border bg-surface p-4 text-left",
                  "hover:border-border-strong hover:bg-surface-hover",
                )}
              >
                <span className="grid size-9 shrink-0 place-items-center rounded-md bg-accent-subtle text-accent-text">
                  <svg viewBox="0 0 18 18" fill="none" className="size-4.5" aria-hidden="true">
                    {entry.icon}
                  </svg>
                </span>
                <span className="min-w-0">
                  <span className="block text-ui font-medium text-fg">{t(entry.title)}</span>
                  <span className="mt-0.5 block text-meta text-muted">
                    {t(entry.description)}
                  </span>
                </span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </Frame>
  );
}

/**
 * SectionBody rend le panneau correspondant, sans son habillage de dialogue.
 *
 * Les panneaux sont réutilisés tels quels : ils étaient déjà écrits, testés et
 * traduits. Seule leur enveloppe change — voir `Shell`.
 */
function SectionBody({ section, onDone }: { section: SectionKey; onDone: () => void }) {
  switch (section) {
    case "stockage":
      return <StoragePanel embedded onClose={onDone} />;
    case "comptes":
      return <AccountsPanel embedded onClose={onDone} />;
    case "appareils":
      return <SessionsPanel embedded onClose={onDone} />;
    case "application":
      return <MobileAppDialog embedded onClose={onDone} />;
    case "catalogues":
      return <DiscoverySources />;
  }
}

/** Frame porte l'ossature commune, y compris pendant la suspension. */
function Frame({
  breadcrumb,
  children,
}: {
  breadcrumb?: React.ReactNode;
  children?: React.ReactNode;
}) {
  return (
    <div className="flex h-dvh flex-col overflow-hidden bg-surface-sunken">
      <header className="flex h-13 shrink-0 items-center gap-3 border-b border-border bg-surface px-4">
        <BrandLockup />

        <nav className="ml-2 flex items-center gap-2" aria-label="fil d'Ariane">
          {breadcrumb}
        </nav>

        {/*
          Le retour vers la bibliothèque est explicite, et pas seulement laissé
          au bouton du navigateur : on peut arriver ici par un signet, auquel cas
          l'historique n'a nulle part où revenir.
        */}
        <Link
          href="/"
          className="pressable ml-auto rounded-md border border-border px-2.5 py-1 text-ui text-muted hover:bg-surface-hover hover:text-fg"
        >
          ← boxincloud
        </Link>
      </header>

      <main className="min-h-0 flex-1 overflow-y-auto">
        <div className="mx-auto flex min-h-full w-full max-w-6xl flex-col p-4">{children}</div>
      </main>
    </div>
  );
}
