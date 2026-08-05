/**
 * Met en mots les échecs du module eD2k.
 *
 * # Pourquoi ce module a besoin de son propre traducteur
 *
 * `describeError` suffit partout ailleurs parce que partout ailleurs, l'erreur
 * vient de boxincloud. Ici, trois échecs ont une cause que le catalogue commun
 * ne sait pas nommer, et dont la formulation générique envoie chercher au
 * mauvais endroit :
 *
 * - **le module est désactivé** — ce n'est pas un conflit, c'est une variable
 *   d'environnement, et le geste correctif n'est pas dans l'interface ;
 * - **aucun démon n'est déclaré** — « cet élément n'existe plus » ferait
 *   conclure à une suppression, alors qu'il n'y a jamais rien eu à supprimer.
 *   L'état neuf d'une instance doit dire où aller : Paramètres ;
 * - **le démon a refusé** — et lui seul sait pourquoi.
 *
 * # Le cas du refus, qui mérite d'être expliqué
 *
 * Le projet tient une règle : le `detail` du serveur n'est jamais affiché, il
 * est en anglais. Ce fichier fait UNE exception, pour la seule réponse dont le
 * detail ne vient pas de nous.
 *
 * « Kad is disabled in preferences » dit exactement quoi faire. Remplacé par
 * « le démon a refusé la commande », il ne reste rien. La citation est donc
 * conservée telle quelle, encadrée d'une phrase française qui l'ATTRIBUE : le
 * lecteur voit une langue étrangère et sait immédiatement que ce n'est pas
 * boxincloud qui parle.
 */

import { useCallback } from "react";

import { useT, type MessageKey } from "@/i18n";
import { ApiError } from "@/lib/api/client";
import { describeError } from "@/lib/api/problem";

const PREFIX = "https://boxincloud.dev/problems/";

/** Rend une fonction qui met une erreur du module en français. */
export function useEd2kError() {
  const t = useT();

  return useCallback(
    (error: unknown): string => {
      if (!(error instanceof ApiError)) return describeError(error, t);

      const type = error.problem?.type?.startsWith(PREFIX)
        ? error.problem.type.slice(PREFIX.length)
        : "";

      if (type === "daemon-refused") {
        const detail = error.problem?.detail?.trim();
        // Sans explication, la citation n'a rien à citer.
        if (!detail) return t("ed2k.error.refused");
        return `${t("ed2k.error.refusedWith")} « ${detail} »`;
      }

      const key = SPECIFIC[type];
      if (key) return t(key);

      return describeError(error, t);
    },
    [t],
  );
}

/**
 * Les types dont le message générique induirait en erreur.
 *
 * Volontairement court : tout ce qui n'y figure pas retombe sur le catalogue
 * commun, qui est juste. Un doublon ici serait une traduction de plus à garder
 * alignée pour rien.
 */
const SPECIFIC: Record<string, MessageKey> = {
  conflict: "ed2k.error.disabled",
  "not-found": "ed2k.error.noDaemon",
};
