"use client";

import { createContext, useCallback, useContext, useEffect, useState } from "react";

import { fr, type MessageKey } from "./fr";
import { en } from "./en";

/**
 * Traduction de l'interface.
 *
 * Un dictionnaire et une fonction, pas une bibliothèque. Le besoin tient en
 * cinquante lignes — deux langues, pas de pluriels complexes, pas de formats de
 * date localisés au-delà de ce que `Intl` donne déjà — et les bibliothèques
 * d'internationalisation apportent surtout un système de chargement paresseux
 * dont un catalogue de trois cents chaînes n'a que faire.
 *
 * Le français est le défaut. Ce n'est pas un choix de commodité : c'est la
 * langue dans laquelle le projet a été écrit, et une traduction faite après
 * coup est toujours moins juste que l'original. L'anglais existe pour ne pas
 * fermer la porte, pas pour prendre la place.
 */

export const LOCALES = ["fr", "en"] as const;
export type Locale = (typeof LOCALES)[number];

const catalogues: Record<Locale, Record<MessageKey, string>> = { fr, en };

const STORAGE_KEY = "boxincloud.locale";

/**
 * La langue à utiliser, déduite dans cet ordre : choix explicite, langue du
 * navigateur, français.
 *
 * Le choix explicite prime sur le navigateur, et pas l'inverse : quelqu'un qui
 * a cliqué sur « English » l'a fait en connaissance de cause, y compris sur un
 * navigateur configuré en français.
 */
export function detectLocale(stored: string | null, languages: readonly string[]): Locale {
  if (isLocale(stored)) return stored;

  for (const language of languages) {
    const base = language.slice(0, 2).toLowerCase();
    if (isLocale(base)) return base;
  }

  return "fr";
}

function isLocale(value: string | null): value is Locale {
  return value !== null && (LOCALES as readonly string[]).includes(value);
}

type Translate = (key: MessageKey) => string;

const LocaleContext = createContext<{
  locale: Locale;
  setLocale: (locale: Locale) => void;
  t: Translate;
}>({ locale: "fr", setLocale: () => {}, t: (key) => fr[key] });

export function LocaleProvider({ children }: { children: React.ReactNode }) {
  // Le français au premier rendu, toujours. L'export statique produit une page
  // unique servie à tout le monde : deviner la langue avant l'hydratation
  // ferait diverger le HTML du serveur et celui du client.
  const [locale, setLocaleState] = useState<Locale>("fr");

  useEffect(() => {
    const detected = detectLocale(localStorage.getItem(STORAGE_KEY), navigator.languages ?? []);
    setLocaleState(detected);
    document.documentElement.lang = detected;
  }, []);

  const setLocale = useCallback((next: Locale) => {
    localStorage.setItem(STORAGE_KEY, next);
    document.documentElement.lang = next;
    setLocaleState(next);
  }, []);

  const t = useCallback<Translate>((key) => catalogues[locale][key] ?? fr[key], [locale]);

  return (
    <LocaleContext.Provider value={{ locale, setLocale, t }}>
      {children}
    </LocaleContext.Provider>
  );
}

export function useLocale() {
  return useContext(LocaleContext);
}

/** Raccourci pour le cas courant : on ne veut que traduire. */
export function useT(): Translate {
  return useContext(LocaleContext).t;
}

export type { MessageKey };
