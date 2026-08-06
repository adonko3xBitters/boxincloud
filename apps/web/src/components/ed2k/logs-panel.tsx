"use client";

/**
 * Journal du démon.
 *
 * Volontairement brut : le texte tel qu'amuled l'écrit, sans découpage en
 * colonnes ni coloration par niveau. Le format n'est pas stable d'une version à
 * l'autre, et une analyse qui se tromperait rendrait des lignes tronquées là où
 * le texte intégral est parfaitement lisible.
 *
 * L'ordre est celui du démon — le plus ancien en haut — et la vue se cale en
 * bas au chargement, comme un terminal : ce qu'on vient lire est ce qui vient
 * de se produire.
 */

import { useEffect, useRef } from "react";
import { useQuery } from "@tanstack/react-query";

import { useT } from "@/i18n";
import * as api from "@/lib/api/endpoints";

import { CommandError, PanelAction, useCommand } from "./commands";
import { Async, PanelHeader, REFRESH_MS } from "./panel";

export function LogsPanel() {
  const t = useT();
  const command = useCommand();
  const box = useRef<HTMLDivElement>(null);

  const logs = useQuery({
    queryKey: ["ed2k", "logs"],
    queryFn: api.getEd2kLogs,
    refetchInterval: REFRESH_MS * 3,
  });

  /*
    Recalage en bas à chaque arrivée.

    On pousse le conteneur, on ne demande pas à un repère de se rendre visible.
    `scrollIntoView` remonte jusqu'au premier ancêtre qui défile et l'ajuste
    LUI AUSSI : la fenêtre entière sautait à chaque rafraîchissement, alors
    qu'on ne voulait bouger que le journal.

    Pas d'animation : un défilement doux toutes les six secondes sur un journal
    qui bouge donne une page qui glisse en permanence.
  */
  const count = logs.data?.lines.length ?? 0;
  useEffect(() => {
    const node = box.current;
    if (node) node.scrollTop = node.scrollHeight;
  }, [count]);

  return (
    /*
      `h-full` et non `flex-1` : le parent défile, donc sa hauteur est définie,
      et c'est cette hauteur-là qu'il faut prendre. Avec `flex-1` dans un
      conteneur qui défile, la section grandit avec le journal — et c'est la
      colonne entière qui glisse, en emportant l'en-tête et le bouton Vider.
    */
    <section className="flex h-full min-h-0 flex-col">
      <PanelHeader title={t("ed2k.section.logs")} hint={t("ed2k.logs.hint")}>
        <PanelAction
          label={t("ed2k.logs.clear")}
          variant="ghost"
          busy={command.busy}
          onClick={() => void command.run(api.clearEd2kLogs)}
        />
      </PanelHeader>

      <CommandError error={command.error} onDismiss={command.clearError} />

      <Async
        query={logs}
        isEmpty={(data) => data.lines.length === 0}
        empty={{ title: t("ed2k.logs.empty"), description: t("ed2k.logs.emptyHint") }}
      >
        {(data) => (
          <div
            ref={box}
            className="min-h-0 flex-1 overflow-auto overscroll-contain rounded-lg border border-border bg-surface-sunken p-3"
          >
            {/*
              `pre` et non une liste : les lignes du démon sont alignées à la
              colonne près par ses propres horodatages, et une police
              proportionnelle détruirait cet alignement.
            */}
            <pre className="whitespace-pre-wrap break-words text-micro leading-relaxed text-muted">
              {data.lines.join("\n")}
            </pre>
          </div>
        )}
      </Async>
    </section>
  );
}
