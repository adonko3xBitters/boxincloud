"use client";

/**
 * Fichiers partagés.
 *
 * Trois colonnes portent tout l'intérêt de cet écran : demandes, servis,
 * envoyé. Elles répondent à « qu'est-ce que je donne, et à qui ça sert » — et
 * l'écart entre demandes et servis mesure ce que la file d'attente a refusé,
 * ce qu'aucun autre écran ne montre.
 *
 * Le chemin est celui que voit le DÉMON, pas ce serveur. La nuance compte : un
 * fichier « présent » ici peut être introuvable côté boxincloud si le montage
 * n'est pas partagé, et c'est la première cause de « le téléchargement est fini
 * mais rien n'arrive ».
 */

import { useQuery } from "@tanstack/react-query";

import { Badge } from "@/components/ui";
import { useT } from "@/i18n";
import * as api from "@/lib/api/endpoints";

import { DASH, useEd2kFormat } from "./format";
import { PRIORITY_LABELS } from "./labels";
import { Async, PanelHeader, REFRESH_MS } from "./panel";
import { Num, Row, Table, Text, type Column } from "./table";

const COLUMNS: Column[] = [
  { key: "name", label: "ed2k.col.name", width: "minmax(220px, 3fr)" },
  { key: "size", label: "ed2k.col.size", width: "92px", align: "right" },
  { key: "requests", label: "ed2k.col.requests", width: "96px", align: "right" },
  { key: "accepted", label: "ed2k.col.accepted", width: "88px", align: "right" },
  { key: "sent", label: "ed2k.col.sent", width: "96px", align: "right" },
  { key: "priority", label: "ed2k.col.priority", width: "104px" },
  { key: "complete", label: "ed2k.col.complete", width: "104px" },
];

export function SharedPanel() {
  const t = useT();
  const format = useEd2kFormat();

  const query = useQuery({
    queryKey: ["ed2k", "shared"],
    queryFn: api.listEd2kSharedFiles,
    // Le partage bouge à l'échelle de la minute, pas de la seconde : le
    // rafraîchir aussi vite que les transferts ferait relire une liste de
    // plusieurs milliers d'entrées pour rien.
    refetchInterval: REFRESH_MS * 15,
  });

  return (
    <section>
      <PanelHeader
        title={t("ed2k.section.shared")}
        hint={t("ed2k.shared.hint")}
        takenAt={query.data?.takenAt}
      />

      <Async
        query={query}
        isEmpty={(data) => data.files.length === 0}
        empty={{ title: t("ed2k.shared.empty"), description: t("ed2k.shared.emptyHint") }}
      >
        {(data) => (
          <Table columns={COLUMNS} minWidth={940} label={t("ed2k.section.shared")}>
            {data.files.map((file, index) => (
              <Row key={file.hash} striped={index % 2 === 1}>
                <span className="min-w-0">
                  <span className="block truncate text-fg" title={file.hash}>
                    {file.name || DASH}
                  </span>
                  <span className="block truncate text-micro text-subtle" title={file.path}>
                    {file.path}
                  </span>
                </span>

                <Num>{format.bytes(file.size)}</Num>
                <Num>{format.count(file.requests)}</Num>

                {/* Servis en dessous de demandés : la différence est ce que la
                    file d'attente a refusé, pas une erreur de comptage. */}
                <Num tone={file.accepted > 0 ? "fg" : "subtle"}>{format.count(file.accepted)}</Num>

                <Num>{format.bytes(file.transferred)}</Num>
                <Text>{t(PRIORITY_LABELS[file.priority])}</Text>

                <span className="min-w-0">
                  {file.complete ? (
                    <Badge tone="neutral">{t("ed2k.yes")}</Badge>
                  ) : (
                    <Badge tone="accent">{t("ed2k.shared.partial")}</Badge>
                  )}
                </span>
              </Row>
            ))}
          </Table>
        )}
      </Async>
    </section>
  );
}
