/**
 * Codes du serveur → libellés traduits.
 *
 * Le contrat rend des chaînes stables — `downloading`, `veryhigh`, `low` — et
 * jamais un texte affichable. C'est ce qui permet de traduire l'interface sans
 * toucher au serveur, et de changer un libellé sans casser un client.
 *
 * Les tables sont exhaustives par construction : `Record<Ed2kDownloadStatus,
 * …>` refuse de compiler si une valeur s'ajoute au contrat sans son libellé.
 * C'est exactement pourquoi le contrat déclare ses énumérations complètes dès
 * maintenant.
 */

import type { MessageKey } from "@/i18n";
import type { Ed2kDownloadStatus, Ed2kIdType, Ed2kPriority } from "@/lib/api/client";

export const STATUS_LABELS: Record<Ed2kDownloadStatus, MessageKey> = {
  waiting: "ed2k.status.waiting",
  downloading: "ed2k.status.downloading",
  paused: "ed2k.status.paused",
  stopped: "ed2k.status.stopped",
  erroneous: "ed2k.status.erroneous",
  completing: "ed2k.status.completing",
  completed: "ed2k.status.completed",
  hashing: "ed2k.status.hashing",
  allocating: "ed2k.status.allocating",
  unknown: "ed2k.status.unknown",
};

/**
 * Teinte de la barre de progression selon l'état.
 *
 * Trois teintes seulement : ce qui avance, ce qui a fini, ce qui a échoué. Le
 * reste est au repos et reste neutre — donner une couleur à chacun des dix
 * états ferait un tableau qu'on ne lit plus.
 */
export const STATUS_TONES: Record<Ed2kDownloadStatus, "accent" | "success" | "danger" | "idle"> = {
  waiting: "idle",
  downloading: "accent",
  paused: "idle",
  stopped: "idle",
  erroneous: "danger",
  completing: "success",
  completed: "success",
  hashing: "accent",
  allocating: "accent",
  unknown: "idle",
};

export const PRIORITY_LABELS: Record<Ed2kPriority, MessageKey> = {
  verylow: "ed2k.priority.verylow",
  low: "ed2k.priority.low",
  normal: "ed2k.priority.normal",
  high: "ed2k.priority.high",
  veryhigh: "ed2k.priority.veryhigh",
  auto: "ed2k.priority.auto",
};

export const ID_LABELS: Record<Ed2kIdType, MessageKey> = {
  high: "ed2k.id.high",
  low: "ed2k.id.low",
  none: "ed2k.id.none",
};

/**
 * Un LowID n'est pas un détail cosmétique : il divise les sources par cinq.
 * L'interface le signale donc en avertissement, pas en information neutre.
 */
export const ID_TONES: Record<Ed2kIdType, "success" | "warning" | "neutral"> = {
  high: "success",
  low: "warning",
  none: "neutral",
};
