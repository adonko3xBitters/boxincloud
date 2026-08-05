"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";

import { BrandLockup } from "@/components/brand";
import { Badge, Button, EmptyState, ErrorState, Input, Spinner, cx } from "@/components/ui";
import * as api from "@/lib/api/endpoints";
import type { Ed2kState, Ed2kStatus } from "@/lib/api/client";
import { useCurrentUser, useRequireAuth } from "@/lib/auth";
import { useT, type MessageKey } from "@/i18n";

/**
 * Module eD2k / Kad.
 *
 * Une page, comme la configuration : une adresse qu'on garde en signet, un
 * bouton Retour qui fait ce qu'on attend, et toute la largeur pour des tableaux
 * qui seront denses.
 *
 * # Pourquoi il n'y a pas encore de rail latéral
 *
 * La cible en porte un — Téléchargements, Recherche, Partagés, Serveurs, Kad,
 * Statistiques, Journal, Paramètres. À cette étape, sept de ces huit entrées
 * n'ouvriraient sur rien.
 *
 * Un rail de portes mortes est pire que pas de rail : il promet huit écrans,
 * n'en tient qu'un, et rien ne dit lesquels sont vides avant d'avoir cliqué
 * partout. Les entrées apparaîtront à mesure que leur contenu existe.
 */
export default function Page() {
  const authenticated = useRequireAuth();

  if (!authenticated) {
    return (
      <div className="grid min-h-dvh place-items-center">
        <Spinner className="size-6 text-muted" />
      </div>
    );
  }

  return <Ed2kCenter />;
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
  const { data: user } = useCurrentUser();

  const status = useQuery({
    queryKey: ["ed2k", "status"],
    queryFn: api.getEd2kStatus,
  });

  /*
    Le flux d'événements écrit dans le cache de requêtes.

    Aucun composant ne connaît EventSource : ils lisent une requête, comme
    partout ailleurs dans l'application. C'est ce qui permettra d'ajouter des
    panneaux sans que chacun gère sa propre connexion — et de n'en ouvrir
    qu'UNE, quel que soit le nombre de panneaux affichés.
  */
  useEd2kEvents(status.isSuccess);

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

  return (
    <Frame>
      <Header status={status.data} />

      {status.data.enabled ? (
        <div className="mt-4 flex flex-col gap-4">
          <DaemonForm status={status.data} />
          <NextStep />
        </div>
      ) : (
        <DisabledNotice />
      )}
    </Frame>
  );
}

/**
 * Abonnement au flux d'événements.
 *
 * Un seul `EventSource` pour toute la page. Il n'est ouvert qu'une fois l'état
 * initial chargé : sans cela, un jeton pas encore rafraîchi ferait échouer la
 * connexion, et `EventSource` réessaierait en boucle sans jamais le dire.
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

    return () => source.close();
  }, [ready, queryClient]);
}

function Header({ status }: { status: Ed2kStatus }) {
  const t = useT();

  return (
    <header className="flex flex-col gap-2">
      <div className="flex items-center gap-3">
        <h1 className="text-title font-semibold text-fg">{t("ed2k.title")}</h1>
        <Badge tone={STATE_TONES[status.state]}>{t(STATE_LABELS[status.state])}</Badge>
      </div>
      <p className="max-w-prose text-meta text-muted">{t("ed2k.intro")}</p>
      {/* Le détail vient du serveur : il dit POURQUOI l'état est celui-là,
          ce qu'un libellé d'état seul ne peut pas porter. */}
      {status.detail && <p className="text-meta text-subtle">{status.detail}</p>}
    </header>
  );
}

function DisabledNotice() {
  const t = useT();

  return (
    <section className="mt-4 rounded-lg border border-border bg-surface p-4">
      <h2 className="text-ui font-medium text-fg">{t("ed2k.disabledTitle")}</h2>
      <p className="mt-1 max-w-prose text-meta text-muted">{t("ed2k.disabledHint")}</p>
      <code className="mt-3 block w-fit rounded bg-surface-sunken px-2 py-1 text-meta text-fg">
        BOXINCLOUD_ED2K_ENABLED=true
      </code>
    </section>
  );
}

function DaemonForm({ status }: { status: Ed2kStatus }) {
  const t = useT();
  const queryClient = useQueryClient();

  const declared = status.daemon;

  const [host, setHost] = useState(declared?.host ?? "amuled");
  const [port, setPort] = useState(String(declared?.port ?? 4712));
  const [password, setPassword] = useState("");
  const [label, setLabel] = useState(declared?.label ?? "");

  const [busy, setBusy] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [fields, setFields] = useState<Record<string, string>>({});

  async function save(event: React.FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError(null);
    setFields({});

    try {
      await api.setEd2kDaemon({
        host,
        port: Number(port),
        password,
        label: label || undefined,
      });
      // Le mot de passe n'est jamais réaffiché : le champ se vide, comme il se
      // videra au prochain chargement de la page.
      setPassword("");
      await queryClient.invalidateQueries({ queryKey: ["ed2k"] });
    } catch (err) {
      setError(err instanceof Error ? err.message : t("error.generic"));
      if (err && typeof err === "object" && "fieldErrors" in err) {
        setFields((err as { fieldErrors: Record<string, string> }).fieldErrors);
      }
    } finally {
      setBusy(false);
    }
  }

  async function forget() {
    setBusy(true);
    setError(null);
    try {
      await api.forgetEd2kDaemon();
      setConfirming(false);
      setPassword("");
      await queryClient.invalidateQueries({ queryKey: ["ed2k"] });
    } catch (err) {
      setError(err instanceof Error ? err.message : t("error.generic"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="rounded-lg border border-border bg-surface p-4">
      <h2 className="text-ui font-medium text-fg">{t("ed2k.daemon.title")}</h2>
      <p className="mt-1 max-w-prose text-meta text-muted">{t("ed2k.daemon.hint")}</p>

      <form onSubmit={(event) => void save(event)} className="mt-4 flex flex-col gap-3">
        <div className="flex flex-col gap-3 sm:flex-row">
          <Input
            name="host"
            label={t("ed2k.daemon.host")}
            value={host}
            onChange={(event) => setHost(event.target.value)}
            error={fields.host}
            autoComplete="off"
          />
          <div className="sm:w-40">
            <Input
              name="port"
              type="number"
              min={1}
              max={65535}
              label={t("ed2k.daemon.port")}
              value={port}
              onChange={(event) => setPort(event.target.value)}
              hint={t("ed2k.daemon.portHint")}
              error={fields.port}
            />
          </div>
        </div>

        <Input
          name="password"
          type="password"
          label={t("ed2k.daemon.password")}
          value={password}
          onChange={(event) => setPassword(event.target.value)}
          hint={t("ed2k.daemon.passwordHint")}
          error={fields.password}
          autoComplete="new-password"
        />

        <Input
          name="label"
          label={t("ed2k.daemon.label")}
          value={label}
          onChange={(event) => setLabel(event.target.value)}
          hint={t("ed2k.daemon.labelHint")}
        />

        {error && <p className="text-meta text-danger">{error}</p>}

        <div className="flex flex-wrap items-center gap-2">
          <Button type="submit" loading={busy} size="sm">
            {t("ed2k.daemon.save")}
          </Button>

          {declared && !confirming && (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => setConfirming(true)}
              disabled={busy}
            >
              {t("ed2k.daemon.forget")}
            </Button>
          )}
        </div>

        {confirming && (
          <div className="rounded-md border border-border bg-surface-sunken p-3">
            <p className="max-w-prose text-meta text-muted">{t("ed2k.daemon.forgetConfirm")}</p>
            <div className="mt-2 flex gap-2">
              <Button
                type="button"
                variant="danger"
                size="sm"
                loading={busy}
                onClick={() => void forget()}
              >
                {t("action.confirm")}
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={() => setConfirming(false)}
              >
                {t("action.cancel")}
              </Button>
            </div>
          </div>
        )}
      </form>

      <dl className="mt-4 grid gap-2 border-t border-border pt-3 sm:grid-cols-2">
        <Fact term={t("ed2k.incoming")} hint={t("ed2k.incomingHint")}>
          <code className="text-meta text-fg">{status.incomingDir}</code>
        </Fact>

        {declared && (
          <Fact term={t("ed2k.daemon.lastSeen")}>
            <span className="text-meta text-fg">
              {declared.lastSeenAt
                ? new Date(declared.lastSeenAt).toLocaleString()
                : t("ed2k.daemon.never")}
            </span>
          </Fact>
        )}
      </dl>
    </section>
  );
}

function Fact({
  term,
  hint,
  children,
}: {
  term: string;
  hint?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="min-w-0">
      <dt className="text-micro uppercase tracking-wide text-subtle">{term}</dt>
      <dd className="mt-0.5 min-w-0 break-all">{children}</dd>
      {hint && <p className="mt-1 max-w-prose text-micro text-subtle">{hint}</p>}
    </div>
  );
}

/**
 * Ce qui n'est pas encore là.
 *
 * Annoncé plutôt que laissé deviner : une page qui déclare un démon sans jamais
 * s'y connecter ressemble à une panne, et la seule chose qui distingue les deux
 * est de le dire.
 */
function NextStep() {
  const t = useT();

  return (
    <section className="rounded-lg border border-dashed border-border p-4">
      <h2 className="text-ui font-medium text-fg">{t("ed2k.next.title")}</h2>
      <p className="mt-1 max-w-prose text-meta text-muted">{t("ed2k.next.hint")}</p>
    </section>
  );
}

/** Frame porte l'ossature commune, comme la page de configuration. */
function Frame({ children }: { children?: React.ReactNode }) {
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

        {/* Le retour est explicite : on peut arriver ici par un signet, auquel
            cas l'historique n'a nulle part où revenir. */}
        <Link
          href="/"
          className={cx(
            "pressable ml-auto rounded-md border border-border px-2.5 py-1",
            "text-ui text-muted hover:bg-surface-hover hover:text-fg",
          )}
        >
          ← boxincloud
        </Link>
      </header>

      <main className="min-h-0 flex-1 overflow-y-auto">
        <div className="mx-auto flex min-h-full w-full max-w-4xl flex-col p-4">{children}</div>
      </main>
    </div>
  );
}
