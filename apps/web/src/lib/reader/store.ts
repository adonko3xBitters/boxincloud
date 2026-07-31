"use client";
import type { MessageKey } from "@/i18n";


/**
 * État du lecteur.
 *
 * Réglages persistés localement plutôt que côté serveur : le mode de lecture
 * dépend de l'écran, pas du compte. Un même utilisateur veut le défilement
 * continu sur téléphone et la double page sur un écran large — synchroniser ce
 * choix entre appareils serait une nuisance, pas un service.
 */

import { useSyncExternalStore } from "react";

export type ReadingMode = "single" | "double" | "scroll";
export type FitMode = "width" | "height" | "page";
export type Direction = "ltr" | "rtl";

export type ReaderSettings = {
  mode: ReadingMode;
  fit: FitMode;
  /** Sens de lecture. `rtl` pour les mangas. */
  direction: Direction;
};

const DEFAULTS: ReaderSettings = {
  mode: "single",
  fit: "height",
  direction: "ltr",
};

const KEY = "boxincloud.reader";

let cache: ReaderSettings = DEFAULTS;
let loaded = false;

const listeners = new Set<() => void>();

function read(): ReaderSettings {
  if (typeof window === "undefined") return DEFAULTS;
  if (loaded) return cache;

  loaded = true;
  try {
    const raw = window.localStorage.getItem(KEY);
    if (raw) {
      const parsed = JSON.parse(raw) as Partial<ReaderSettings>;
      cache = { ...DEFAULTS, ...parsed };
    }
  } catch {
    // Réglages corrompus : on repart des défauts plutôt que de bloquer
    // l'ouverture d'un album.
    cache = DEFAULTS;
  }
  return cache;
}

export function getSettings(): ReaderSettings {
  return read();
}

export function setSettings(patch: Partial<ReaderSettings>): void {
  cache = { ...read(), ...patch };
  loaded = true;

  try {
    window.localStorage.setItem(KEY, JSON.stringify(cache));
  } catch {
    // Stockage plein ou navigation privée : les réglages ne survivront pas à
    // la session, mais la lecture en cours n'a aucune raison d'échouer.
  }

  for (const listener of listeners) listener();
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function useReaderSettings(): ReaderSettings {
  return useSyncExternalStore(subscribe, getSettings, () => DEFAULTS);
}

// ─── Libellés ────────────────────────────────────────────────────────────────

export const MODE_KEYS: Record<ReadingMode, MessageKey> = {
  single: "reader.modeSingle",
  double: "reader.modeDouble",
  scroll: "reader.modeScroll",
};

export const FIT_KEYS: Record<FitMode, MessageKey> = {
  width: "reader.fitWidth",
  height: "reader.fitHeight",
  page: "reader.fitPage",
};
