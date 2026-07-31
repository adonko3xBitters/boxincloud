import { API_BASE } from "./api/client";

/**
 * Où trouver l'application mobile.
 *
 * **L'instance la sert elle-même.** Le téléphone qui scanne le code QR ne parle
 * jamais à un service tiers, et une installation coupée d'Internet fonctionne
 * exactement comme les autres — ce qui est la moindre des choses pour un projet
 * auto-hébergé qu'aucun magasin d'applications ne distribue.
 *
 * Le bénéfice moins visible est plus durable : l'application et le serveur sont
 * construits ensemble, donc verrouillés sur la même version. Il n'existe aucun
 * couple app/serveur non testé, et donc aucune dérive de compatibilité à gérer.
 *
 * Le prix est une image Docker qui passe de quinze à quatre-vingts méga-octets.
 * C'est le bon échange pour ce public : un self-hosted tire son image une fois,
 * et n'a pas envie de dépendre de la disponibilité d'un tiers pour installer
 * l'application de sa propre bibliothèque.
 */

export const REPO_URL = "https://github.com/adonko3xBitters/boxincloud";

/** L'APK, servi par cette instance. */
export const ANDROID_APK_URL = `${API_BASE}/app/android.apk`;

/** Ce que l'instance embarque, pour ne proposer que ce qui existe. */
export type AppInfo = {
  android: boolean;
  version: string;
  sizeBytes: number;
};

export type Platform = "android" | "ios" | "other";

/**
 * Devine la plateforme, pour ne montrer que ce qui s'installe dessus.
 *
 * Une devinette, pas une certitude : l'inconnu retombe sur « other », qui
 * montre tout. Se tromper coûte alors un choix de plus, jamais une impasse.
 */
export function detectPlatform(userAgent: string): Platform {
  const ua = userAgent.toLowerCase();

  if (ua.includes("android")) return "android";

  // Un iPad récent s'annonce comme un Macintosh ; le tactile le distingue d'un
  // vrai Mac, qui n'installera de toute façon pas d'APK.
  if (/iphone|ipad|ipod/.test(ua)) return "ios";

  return "other";
}

/** Taille lisible, pour annoncer le poids avant de lancer le téléchargement. */
export function formatSize(bytes: number): string {
  if (bytes <= 0) return "";
  return `${Math.round(bytes / (1024 * 1024))} Mo`;
}
