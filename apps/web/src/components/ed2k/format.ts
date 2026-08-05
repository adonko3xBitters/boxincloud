"use client";

/**
 * Mise en forme des chiffres du module.
 *
 * Un gestionnaire de téléchargements est fait de nombres : octets, débits,
 * durées, compteurs. Les formater au même endroit évite que « 1.5 Go » et
 * « 1,5 Go » cohabitent dans deux colonnes voisines — ce qui se remarque
 * immédiatement et donne l'impression que les deux ne viennent pas de la même
 * mesure.
 *
 * Les unités passent par le catalogue : « ko » et « kB » ne s'écrivent pas
 * pareil, et une unité en dur est une unité qui ne se traduira jamais.
 */

import { useCallback, useMemo } from "react";

import { useLocale, useT } from "@/i18n";

/** Ce que les tableaux affichent à la place d'une valeur absente. */
export const DASH = "—";

export type Ed2kFormatters = {
  /** Octets, en unité binaire. */
  bytes: (value: number) => string;
  /** Octets par seconde. Zéro devient un tiret : « 0 o/s » n'apprend rien. */
  speed: (value: number) => string;
  /** Durée en secondes, réduite à sa plus grande unité. */
  duration: (seconds: number) => string;
  /** Temps écoulé depuis une date ISO. */
  since: (iso: string) => string;
  /** Entier, avec les séparateurs de milliers de la langue courante. */
  count: (value: number) => string;
  /** Heure seule : la date complète n'apprend rien sur un chiffre d'il y a dix secondes. */
  time: (iso: string) => string;
  /** Part, entre 0 et 1. `Intl` place le symbole et l'espace selon la langue. */
  percent: (value: number) => string;
};

export function useEd2kFormat(): Ed2kFormatters {
  const t = useT();
  const { locale } = useLocale();

  const count = useCallback(
    (value: number) => value.toLocaleString(locale === "fr" ? "fr-FR" : "en-US"),
    [locale],
  );

  const bytes = useCallback(
    (value: number) => {
      const units = [
        t("ed2k.unit.byte"),
        t("ed2k.unit.kilo"),
        t("ed2k.unit.mega"),
        t("ed2k.unit.giga"),
        t("ed2k.unit.tera"),
      ];

      if (value < 1024) return `${count(value)} ${units[0]}`;

      let scaled = value / 1024;
      let unit = 1;
      while (scaled >= 1024 && unit < units.length - 1) {
        scaled /= 1024;
        unit += 1;
      }

      // Une décimale au-dessus du kilo-octet, aucune en dessous : « 3,7 Go »
      // se lit, « 3 968,4 Mo » occupe une colonne pour une précision dont
      // personne ne fait rien.
      return `${scaled.toFixed(scaled >= 100 ? 0 : 1)} ${units[unit]}`;
    },
    [t, count],
  );

  const duration = useCallback(
    (seconds: number) => {
      if (seconds < 60) return t("ed2k.unit.second", { value: Math.round(seconds) });
      if (seconds < 3600) return t("ed2k.unit.minute", { value: Math.round(seconds / 60) });
      if (seconds < 86400) return t("ed2k.unit.hour", { value: Math.round(seconds / 3600) });
      return t("ed2k.unit.day", { value: Math.round(seconds / 86400) });
    },
    [t],
  );

  return useMemo(
    () => ({
      bytes,
      count,
      duration,
      speed: (value) => (value <= 0 ? DASH : t("ed2k.unit.perSecond", { value: bytes(value) })),
      since: (iso) => {
        const elapsed = (Date.now() - new Date(iso).getTime()) / 1000;
        return elapsed < 0 ? DASH : duration(elapsed);
      },
      time: (iso) =>
        new Date(iso).toLocaleTimeString(locale === "fr" ? "fr-FR" : "en-US", {
          hour: "2-digit",
          minute: "2-digit",
          second: "2-digit",
        }),
      percent: (value) =>
        new Intl.NumberFormat(locale === "fr" ? "fr-FR" : "en-US", {
          style: "percent",
          maximumFractionDigits: 1,
        }).format(value),
    }),
    [bytes, count, duration, locale, t],
  );
}

/**
 * Part reçue d'un fichier, entre 0 et 1.
 *
 * Bornée volontairement : `sizeDone` peut dépasser `size` d'un bloc pendant
 * l'assemblage, et une barre à 103 % dépasserait son cadre.
 */
export function receivedShare(done: number, size: number): number {
  if (size <= 0) return 0;
  return Math.min(1, Math.max(0, done / size));
}
