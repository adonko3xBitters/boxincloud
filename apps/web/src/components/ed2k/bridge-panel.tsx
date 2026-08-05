"use client";

/**
 * Le pont vers la bibliothèque.
 *
 * C'est l'écran qui décide de ce que boxincloud FAIT du contenu, et non de la
 * façon dont il l'affiche : un fichier terminé reste sur disque, ou devient un
 * album indexé, lisible depuis le navigateur et depuis le téléphone.
 *
 * La règle est portée par la catégorie du démon, parce que c'est le seul
 * découpage qui ait du sens — un client eD2k sert à récupérer toutes sortes de
 * choses, et seule une partie a sa place dans un catalogue d'albums.
 *
 * L'écran montre les deux moitiés ensemble : les règles, et ce qu'elles ont
 * produit. Les séparer obligerait à naviguer pour vérifier qu'une règle
 * fonctionne, ce qui est précisément la question qu'on se pose en la posant.
 */

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { Badge, Button, Input } from "@/components/ui";
import { useT, type MessageKey } from "@/i18n";
import * as api from "@/lib/api/endpoints";
import type { Ed2kPublication } from "@/lib/api/client";

import { CommandError, useCommand } from "./commands";
import { DASH, useEd2kFormat } from "./format";
import { Async, PanelHeader, REFRESH_MS } from "./panel";
import { Num, Row, Table, Text, type Column } from "./table";

const STATUS_LABELS: Record<Ed2kPublication["status"], MessageKey> = {
  pending: "ed2k.pub.pending",
  published: "ed2k.pub.published",
  skipped: "ed2k.pub.skipped",
  error: "ed2k.pub.error",
};

const STATUS_TONES: Record<Ed2kPublication["status"], "neutral" | "success" | "danger" | "accent"> =
  {
    pending: "accent",
    published: "success",
    skipped: "neutral",
    error: "danger",
  };

const COLUMNS: Column[] = [
  { key: "name", label: "ed2k.col.name", width: "minmax(240px, 3fr)" },
  { key: "size", label: "ed2k.col.size", width: "96px", align: "right" },
  { key: "category", label: "ed2k.col.category", width: "88px", align: "right" },
  { key: "status", label: "ed2k.col.status", width: "128px" },
  { key: "detail", label: "ed2k.col.detail", width: "minmax(200px, 2fr)" },
];

export function BridgePanel() {
  const t = useT();
  const command = useCommand();

  const destinations = useQuery({
    queryKey: ["ed2k", "destinations"],
    queryFn: api.listEd2kDestinations,
  });

  const libraries = useQuery({ queryKey: ["libraries"], queryFn: api.listLibraries });

  const publications = useQuery({
    queryKey: ["ed2k", "publications"],
    queryFn: () => api.listEd2kPublications(100),
    // Une publication est un événement rare : la scruter à la seconde
    // n'apprendrait rien et relirait cent lignes pour rien.
    refetchInterval: REFRESH_MS * 5,
  });

  return (
    <section>
      <PanelHeader title={t("ed2k.section.bridge")} hint={t("ed2k.bridge.hint")} />

      <CommandError error={command.error} onDismiss={command.clearError} />

      <DestinationForm
        command={command}
        libraries={libraries.data?.libraries ?? []}
        existing={destinations.data?.destinations ?? []}
      />

      <h3 className="mb-2 mt-4 text-ui font-medium text-fg">{t("ed2k.bridge.history")}</h3>

      <Async
        query={publications}
        isEmpty={(data) => data.publications.length === 0}
        empty={{
          title: t("ed2k.bridge.empty"),
          description: t("ed2k.bridge.emptyHint"),
        }}
      >
        {(data) => (
          <Table columns={COLUMNS} minWidth={900} label={t("ed2k.bridge.history")}>
            {data.publications.map((publication, index) => (
              <PublicationRow
                key={publication.hash}
                publication={publication}
                striped={index % 2 === 1}
              />
            ))}
          </Table>
        )}
      </Async>
    </section>
  );
}

function PublicationRow({
  publication,
  striped,
}: {
  publication: Ed2kPublication;
  striped: boolean;
}) {
  const t = useT();
  const format = useEd2kFormat();

  return (
    <Row striped={striped}>
      <span className="min-w-0">
        <span className="block truncate text-fg" title={publication.hash}>
          {publication.name || DASH}
        </span>
      </span>

      <Num>{format.bytes(publication.size)}</Num>
      <Num tone="muted">{publication.category}</Num>

      <span className="min-w-0">
        <Badge tone={STATUS_TONES[publication.status]}>
          {t(STATUS_LABELS[publication.status])}
        </Badge>
      </span>

      {/*
        Le détail n'est utile que sur un échec, mais il est affiché tel quel :
        c'est le message du serveur, qui nomme la variable à vérifier quand le
        volume du démon n'est pas monté.
      */}
      <Text tone={publication.status === "error" ? "danger" : "subtle"}>
        {publication.detail || DASH}
      </Text>
    </Row>
  );
}

/**
 * Déclaration d'une règle.
 *
 * Un formulaire à quatre champs plutôt qu'un tableau éditable : les règles sont
 * peu nombreuses — autant que de catégories dans le démon — et se posent une
 * fois. Un tableau à édition en place coûterait plus à écrire qu'il ne ferait
 * gagner.
 */
function DestinationForm({
  command,
  libraries,
  existing,
}: {
  command: ReturnType<typeof useCommand>;
  libraries: { id: string; name: string }[];
  existing: { category: number; label: string; libraryId?: string; folder: string }[];
}) {
  const t = useT();

  const [category, setCategory] = useState("0");
  const [label, setLabel] = useState("");
  const [libraryId, setLibraryId] = useState("");
  const [folder, setFolder] = useState("");

  return (
    <div className="rounded-lg border border-border bg-surface p-3">
      <h3 className="text-ui font-medium text-fg">{t("ed2k.bridge.rules")}</h3>

      {existing.length > 0 && (
        <ul className="mt-2 flex flex-col gap-1">
          {existing.map((rule) => {
            const library = libraries.find((l) => l.id === rule.libraryId);
            return (
              <li
                key={rule.category}
                className="flex flex-wrap items-baseline gap-2 text-meta text-muted"
              >
                <span className="tabular-nums text-subtle">#{rule.category}</span>
                <span className="text-fg">{rule.label}</span>
                <span aria-hidden="true">→</span>
                {library ? (
                  <span className="text-accent-text">
                    {library.name}
                    {rule.folder ? ` / ${rule.folder}` : ""}
                  </span>
                ) : (
                  <span>{t("ed2k.bridge.onDisk")}</span>
                )}
              </li>
            );
          })}
        </ul>
      )}

      <form
        onSubmit={(event) => {
          event.preventDefault();
          void command
            .run(() =>
              api.setEd2kDestination({
                category: Number(category),
                label,
                // La chaîne vide signifie « laisser sur disque » : on l'envoie
                // en null pour que le serveur ne la prenne pas pour un
                // identifiant illisible.
                libraryId: libraryId || null,
                folder,
              }),
            )
            .then(() => {
              setLabel("");
              setFolder("");
            });
        }}
        className="mt-3 flex flex-wrap items-end gap-2"
      >
        <div className="w-24">
          <Input
            name="category"
            type="number"
            min={0}
            label={t("ed2k.col.category")}
            value={category}
            onChange={(event) => setCategory(event.target.value)}
          />
        </div>

        <div className="min-w-[140px] flex-1">
          <Input
            name="label"
            label={t("ed2k.bridge.label")}
            value={label}
            onChange={(event) => setLabel(event.target.value)}
          />
        </div>

        <label className="flex min-w-[180px] flex-1 flex-col gap-1.5">
          <span className="text-sm font-medium text-fg">{t("ed2k.bridge.library")}</span>
          <select
            value={libraryId}
            onChange={(event) => setLibraryId(event.target.value)}
            className="h-10 w-full rounded-md border border-border-strong bg-surface px-2 text-sm text-fg"
          >
            <option value="">{t("ed2k.bridge.onDisk")}</option>
            {libraries.map((library) => (
              <option key={library.id} value={library.id}>
                {library.name}
              </option>
            ))}
          </select>
        </label>

        <div className="min-w-[140px] flex-1">
          <Input
            name="folder"
            label={t("ed2k.bridge.folder")}
            value={folder}
            onChange={(event) => setFolder(event.target.value)}
            disabled={libraryId === ""}
          />
        </div>

        <Button type="submit" size="md" loading={command.busy} disabled={label.trim() === ""}>
          {t("action.save")}
        </Button>
      </form>

      <p className="mt-2 max-w-prose text-micro text-subtle">{t("ed2k.bridge.ruleHint")}</p>
    </div>
  );
}
