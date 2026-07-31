"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";

import { ContextMenu, useContextMenu, type MenuItem } from "./context-menu";
import { DeleteDialog, MoveDialog } from "./manage-dialogs";
import * as api from "@/lib/api/endpoints";
import { useT } from "@/i18n";
import { useWorkspace } from "@/lib/workspace";

/**
 * Menu contextuel d'un album.
 *
 * Partagé par la grille et le tableau : les deux montrent les mêmes objets et
 * doivent offrir les mêmes gestes. Deux menus séparés finiraient par diverger,
 * et l'utilisateur ne saurait plus lequel porte quoi.
 *
 * Le clic droit sélectionne ce qu'il vise, sauf si l'élément fait déjà partie
 * d'une sélection — auquel cas l'action porte sur la sélection entière. C'est le
 * comportement de tous les gestionnaires de fichiers.
 */
export function useComicMenu(titleOf: (id: string) => string) {
  const t = useT();
  const router = useRouter();
  const queryClient = useQueryClient();
  const { selection, isSelected, select, favorites, refreshMarks } = useWorkspace();

  const menu = useContextMenu<{ id: string; visible: string[] }>();
  const [dialog, setDialog] = useState<"delete" | "move" | null>(null);

  // Cible de l'action : la sélection si l'élément visé en fait partie, l'élément
  // seul sinon.
  const targets = menu.target
    ? isSelected(menu.target.id)
      ? selection
      : [menu.target.id]
    : selection;

  async function mark(action: api.BulkAction) {
    await api.bulk(action, targets);
    refreshMarks();
    await queryClient.invalidateQueries({ queryKey: ["comics"] });
    await queryClient.invalidateQueries({ queryKey: ["progress"] });
  }

  const allFavorite = targets.length > 0 && targets.every((id) => favorites.has(id));

  const items: MenuItem[] = menu.target
    ? [
        {
          label:
            targets.length > 1
              ? t("comicMenu.readNamed", { title: titleOf(menu.target.id) })
              : t("toolbar.read"),
          onSelect: () => router.push(`/read?id=${menu.target!.id}`),
        },
        { kind: "separator" },
        {
          label:
            targets.length > 1
              ? t("comicMenu.markReadCount", { count: targets.length })
              : t("comicMenu.markRead"),
          onSelect: () => void mark("read"),
        },
        {
          label: t("comicMenu.markUnread"),
          onSelect: () => void mark("unread"),
        },
        {
          label: allFavorite ? t("toolbar.unfavorite") : t("toolbar.favorite"),
          onSelect: () => void mark(allFavorite ? "unfavorite" : "favorite"),
        },
        { kind: "separator" },
        {
          label:
            targets.length > 1
              ? t("comicMenu.moveCount", { count: targets.length })
              : t("comicMenu.move"),
          onSelect: () => setDialog("move"),
        },
        {
          label:
            targets.length > 1
              ? t("comicMenu.removeCount", { count: targets.length })
              : t("comicMenu.remove"),
          destructive: true,
          onSelect: () => setDialog("delete"),
        },
      ]
    : [];

  return {
    /** À poser sur chaque élément : `onContextMenu={menu.bind(id, visibleIds)}`. */
    bind: (id: string, visible: string[]) => (event: React.MouseEvent) => {
      // Un clic droit sur un élément hors sélection le sélectionne d'abord :
      // agir sur ce qu'on ne voit pas désigné serait une surprise.
      if (!isSelected(id)) select(id, "replace", visible);
      menu.open(event, { id, visible });
    },

    node: (
      <>
        <ContextMenu position={menu.position} onClose={menu.close} items={items} />

        {dialog === "delete" && (
          <DeleteDialog
            ids={targets}
            titles={targets.map(titleOf)}
            onClose={() => setDialog(null)}
          />
        )}
        {dialog === "move" && (
          <MoveDialog
            ids={targets}
            titles={targets.map(titleOf)}
            onClose={() => setDialog(null)}
          />
        )}
      </>
    ),
  };
}
