"use client";

import Link from "next/link";
import { Suspense, useEffect } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useQuery, useQueryClient } from "@tanstack/react-query";

import { BrandLockup } from "@/components/brand";
import { Badge, EmptyState, ErrorState, Spinner, cx } from "@/components/ui";
import { BridgePanel } from "@/components/ed2k/bridge-panel";
import { DownloadsPanel } from "@/components/ed2k/downloads-panel";
import { LogsPanel } from "@/components/ed2k/logs-panel";
import { SearchPanel } from "@/components/ed2k/search-panel";
import { ServersPanel } from "@/components/ed2k/servers-panel";
import { SharedPanel } from "@/components/ed2k/shared-panel";
import { StatsPanel } from "@/components/ed2k/stats-panel";
import { UploadsPanel } from "@/components/ed2k/uploads-panel";
import { DaemonForm, DisabledNotice } from "@/components/ed2k/settings-panel";
import * as api from "@/lib/api/endpoints";
import type { Ed2kState, Ed2kStatus } from "@/lib/api/client";
import { useCurrentUser, useRequireAuth } from "@/lib/auth";
import { useT, type MessageKey } from "@/i18n";

/**
 * Module eD2k / Kad.
 *
 * Une page à rail latéral, sur le patron de la configuration : la section vit
 * dans l'URL, ce qui la rend partageable, rechargeable, et compatible avec le
 * bouton Retour.
 *
 * Le rail n'apparaît que maintenant. Tant que six de ses sept entrées
 * n'ouvraient sur rien, il aurait promis sept écrans pour n'en tenir qu'un, et
 * rien n'aurait dit lesquels étaient vides avant d'avoir cliqué partout.
 */

type SectionKey =
  | "telechargements"
  | "recherche"
  | "envois"
  | "partages"
  | "serveurs"
  | "statistiques"
  | "bibliotheque"
  | "journal"
  | "parametres";

type Section = {
  key: SectionKey;
  title: MessageKey;
  /** Rendu du panneau. Une fonction, pour n'en monter qu'un à la fois. */
  render: () => React.ReactNode;
};

/*
  Un tuple, pas un tableau : le premier élément est la section par défaut, et
  le typage doit garantir qu'il existe. Avec un `Section[]`, `SECTIONS[0]` est
  « peut-être indéfini » — et le repli qu'il faudrait écrire ne pourrait mener
  nulle part.
*/
const SECTIONS = [
  { key: "telechargements", title: "ed2k.section.downloads", render: () => <DownloadsPanel /> },
  { key: "recherche", title: "ed2k.section.search", render: () => <SearchPanel /> },
  { key: "envois", title: "ed2k.section.uploads", render: () => <UploadsPanel /> },
  { key: "partages", title: "ed2k.section.shared", render: () => <SharedPanel /> },
  { key: "serveurs", title: "ed2k.section.servers", render: () => <ServersPanel /> },
  { key: "statistiques", title: "ed2k.section.stats", render: () => <StatsPanel /> },
  { key: "bibliotheque", title: "ed2k.section.bridge", render: () => <BridgePanel /> },
  { key: "journal", title: "ed2k.section.logs", render: () => <LogsPanel /> },
  { key: "parametres", title: "ed2k.section.settings", render: () => null },
] as const satisfies readonly [Section, ...Section[]];


export default function Page() {
  const authenticated = useRequireAuth();

  if (!authenticated) {
    return (
      <div className="grid min-h-dvh place-items-center">
        <Spinner className="size-6 text-muted" />
      </div>
    );
  }

  /*
    `useSearchParams` suspend au premier rendu en export statique : la page est
    construite sans connaître l'URL sous laquelle elle sera servie.
  */
  return (
    <Suspense fallback={<Frame />}>
      <Ed2kCenter />
    </Suspense>
  );
}

/** Libellé d'un état. Le serveur rend un code stable, l'interface le traduit. */
const STATE_LABELS: Record<Ed2kState, MessageKey> = {
  disabled: "ed2k.state.disabled",
  unconfigured: "ed2k.state.unconfigured",
  disconnected: "ed2k.state.disconnected",
  connecting: "ed2k.state.connecting",
  connected: "ed2k.state.connected",
};

const STATE_TONES: Record<Ed2kState, "neutral" | "accent" | "success" | "warning"> = {
  disabled: "neutral",
  unconfigured: "warning",
  disconnected: "warning",
  connecting: "accent",
  connected: "success",
};

function Ed2kCenter() {
  const t = useT();
  const router = useRouter();
  const params = useSearchParams();
  const { data: user } = useCurrentUser();

  const status = useQuery({
    queryKey: ["ed2k", "status"],
    queryFn: api.getEd2kStatus,
  });

  useEd2kEvents(status.isSuccess);

  const requested = params.get("section") as SectionKey | null;
  const active = SECTIONS.find((s) => s.key === requested) ?? SECTIONS[0];

  // L'API refuserait de toute façon ; le dire ici évite un écran d'erreur pour
  // ce qui est une question de droits, pas une panne.
  if (user && user.role !== "admin") {
    return (
      <Frame>
        <EmptyState title={t("ed2k.adminOnly")} description={t("ed2k.adminOnlyHint")} />
      </Frame>
    );
  }

  if (status.isLoading) {
    return (
      <Frame>
        <div className="grid place-items-center py-16">
          <Spinner className="size-6 text-muted" />
        </div>
      </Frame>
    );
  }

  if (status.isError || !status.data) {
    return (
      <Frame>
        <ErrorState error={status.error} onRetry={() => void status.refetch()} />
      </Frame>
    );
  }

  /*
    Module éteint : ni rail, ni panneaux.

    Les six premières sections interrogeraient une API qui répond 409, et la
    septième proposerait de déclarer un démon que rien n'irait joindre. Un
    écran qui dit pourquoi, et comment l'activer, vaut mieux que sept écrans
    qui échouent chacun à leur façon.
  */
  if (!status.data.enabled) {
    return (
      <Frame status={status.data}>
        <DisabledNotice />
      </Frame>
    );
  }

  return (
    <Frame status={status.data}>
      <div className="flex min-h-0 flex-1 gap-4">
        <nav
          aria-label={t("ed2k.title")}
          className="w-44 shrink-0 border-r border-border pr-2"
        >
          {SECTIONS.map((section) => (
            <button
              key={section.key}
              onClick={() => router.push(`/ed2k?section=${section.key}`)}
              aria-current={section.key === active.key ? "page" : undefined}
              className={cx(
                "pressable mb-0.5 block w-full rounded px-2 py-1.5 text-left text-ui",
                section.key === active.key
                  ? "bg-accent-subtle font-medium text-accent-text"
                  : "text-muted hover:bg-surface-hover hover:text-fg",
              )}
            >
              {t(section.title)}
            </button>
          ))}
        </nav>

        <div className="min-w-0 flex-1">
          {active.key === "parametres" ? (
            <div className="flex flex-col gap-4">
              <DaemonForm status={status.data} />
            </div>
          ) : (
            active.render()
          )}
        </div>
      </div>
    </Frame>
  );
}

/**
 * Abonnement au flux d'événements.
 *
 * Un seul `EventSource` pour toute la page. Il n'est ouvert qu'une fois l'état
 * initial chargé : sans cela, un jeton pas encore rafraîchi ferait échouer la
 * connexion, et `EventSource` réessaierait en boucle sans jamais le dire.
 *
 * Il porte aujourd'hui l'état du module et les changements de session. Les
 * panneaux, eux, redemandent leur instantané à leur propre cadence — le jour où
 * le flux portera un événement par domaine, c'est ici que l'écriture directe
 * dans le cache remplacera ce sondage, sans qu'aucun panneau ne change.
 */
function useEd2kEvents(ready: boolean) {
  const queryClient = useQueryClient();

  useEffect(() => {
    if (!ready) return;

    const source = new EventSource(api.ed2kEventsURL());

    source.addEventListener("status", (event) => {
      try {
        const status = JSON.parse((event as MessageEvent<string>).data) as Ed2kStatus;
        queryClient.setQueryData(["ed2k", "status"], status);
      } catch {
        // Une trame illisible ne doit pas casser le flux : la suivante portera
        // l'état complet, puisque chaque message en porte un.
      }
    });

    /*
      Un changement de session invalide tout ce qui vient du démon.

      Quand la connexion tombe ou revient, les instantanés en cache décrivent un
      autre monde. Les invalider fait repartir chaque panneau visible sur des
      données fraîches, au lieu d'afficher une file figée à côté d'un bandeau
      qui annonce « déconnecté ».
    */
    for (const kind of ["daemon.connected", "daemon.disconnected"]) {
      source.addEventListener(kind, () => {
        void queryClient.invalidateQueries({ queryKey: ["ed2k"] });
      });
    }

    return () => source.close();
  }, [ready, queryClient]);
}

/** Frame porte l'ossature commune, y compris pendant la suspension. */
function Frame({
  status,
  children,
}: {
  status?: Ed2kStatus;
  children?: React.ReactNode;
}) {
  const t = useT();

  return (
    <div className="flex h-dvh flex-col overflow-hidden bg-surface-sunken">
      <header className="flex h-13 shrink-0 items-center gap-3 border-b border-border bg-surface px-4">
        <BrandLockup />

        <nav className="ml-2 flex items-center gap-2" aria-label="fil d'Ariane">
          <span aria-hidden="true" className="text-subtle">
            /
          </span>
          <span className="text-ui font-medium text-fg">{t("ed2k.title")}</span>
        </nav>

        {/* L'état du module vit dans l'en-tête, pas dans un panneau : il vaut
            pour tous, et le chercher section par section serait absurde. */}
        {status && (
          <>
            <Badge tone={STATE_TONES[status.state]}>{t(STATE_LABELS[status.state])}</Badge>
            <span className="hidden truncate text-meta text-subtle sm:block">
              {status.detail}
            </span>
          </>
        )}

        <Link
          href="/"
          className={cx(
            "pressable ml-auto shrink-0 rounded-md border border-border px-2.5 py-1",
            "text-ui text-muted hover:bg-surface-hover hover:text-fg",
          )}
        >
          ← boxincloud
        </Link>
      </header>

      <main className="min-h-0 flex-1 overflow-y-auto">
        <div className="mx-auto flex min-h-full w-full max-w-7xl flex-col p-4">{children}</div>
      </main>
    </div>
  );
}

