/**
 * Stockage des jetons de session.
 *
 * `localStorage` plutôt qu'un cookie : l'application est un export statique
 * servi par le binaire Go, il n'y a pas de rendu serveur qui aurait besoin de
 * lire la session. Le compromis est connu — un XSS donnerait accès au jeton —
 * et couvert par une CSP stricte plus l'échappement de React.
 *
 * L'`identifiant d'appareil` est conservé séparément et survit à la
 * déconnexion : c'est ce qui permet au serveur de reconnaître le navigateur
 * plutôt que d'accumuler un appareil par connexion.
 */

import type { components } from "./schema";

type Tokens = components["schemas"]["Tokens"];

const TOKENS_KEY = "boxincloud.tokens";
const DEVICE_KEY = "boxincloud.deviceId";

export type StoredTokens = {
  accessToken: string;
  refreshToken: string;
  expiresAt: string;
};

/** Notifie l'application d'un changement de session. */
const listeners = new Set<() => void>();

function notify() {
  for (const listener of listeners) listener();
}

export function subscribeToTokens(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function getTokens(): StoredTokens | null {
  if (typeof window === "undefined") return null;

  const raw = window.localStorage.getItem(TOKENS_KEY);
  if (!raw) return null;

  try {
    const parsed = JSON.parse(raw) as StoredTokens;
    if (!parsed.accessToken || !parsed.refreshToken) return null;
    return parsed;
  } catch {
    // Contenu corrompu — une version antérieure, une édition manuelle : on
    // repart proprement plutôt que de planter au démarrage.
    window.localStorage.removeItem(TOKENS_KEY);
    return null;
  }
}

export function setTokens(tokens: Tokens): void {
  if (typeof window === "undefined") return;

  const stored: StoredTokens = {
    accessToken: tokens.accessToken,
    refreshToken: tokens.refreshToken,
    expiresAt: tokens.expiresAt,
  };
  window.localStorage.setItem(TOKENS_KEY, JSON.stringify(stored));

  if (tokens.deviceId) {
    window.localStorage.setItem(DEVICE_KEY, tokens.deviceId);
  }
  notify();
}

export function clearTokens(): void {
  if (typeof window === "undefined") return;
  // L'identifiant d'appareil est délibérément conservé : il n'est pas secret,
  // et le préserver évite de créer un appareil à chaque reconnexion.
  window.localStorage.removeItem(TOKENS_KEY);
  notify();
}

export function getDeviceId(): string | undefined {
  if (typeof window === "undefined") return undefined;
  return window.localStorage.getItem(DEVICE_KEY) ?? undefined;
}

export function isAuthenticated(): boolean {
  return getTokens() !== null;
}

/**
 * Nom d'appareil déduit du navigateur.
 *
 * Approximatif par nature — l'user-agent ment. L'utilisateur pourra le
 * renommer depuis la gestion des appareils ; l'objectif est seulement qu'une
 * liste de sessions soit lisible plutôt que remplie de « Appareil inconnu ».
 */
export function guessDeviceName(): string {
  if (typeof navigator === "undefined") return "Navigateur";

  const ua = navigator.userAgent;
  const os =
    /Mac OS X/.test(ua) ? "macOS"
    : /Windows/.test(ua) ? "Windows"
    : /Android/.test(ua) ? "Android"
    : /iPhone|iPad/.test(ua) ? "iOS"
    : /Linux/.test(ua) ? "Linux"
    : "";

  const browser =
    /Edg\//.test(ua) ? "Edge"
    : /Chrome\//.test(ua) ? "Chrome"
    : /Safari\//.test(ua) ? "Safari"
    : /Firefox\//.test(ua) ? "Firefox"
    : "Navigateur";

  return os ? `${browser} sur ${os}` : browser;
}
