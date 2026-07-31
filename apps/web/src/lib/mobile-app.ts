/**
 * Où trouver l'application mobile.
 *
 * L'APK est publié sur les versions GitHub du projet plutôt que servi par
 * l'instance : il pèse quelques dizaines de mégaoctets, que chaque installation
 * self-hosted aurait alors à embarquer dans son image — pour un fichier
 * identique partout, et que la plupart des instances ne serviront jamais.
 *
 * L'alias `latest` évite de coupler la page à un numéro de version : une
 * instance qui n'a pas été mise à jour depuis six mois pointe quand même vers
 * l'application courante, ce qui est le comportement voulu — le protocole est
 * versionné, pas le client.
 */
export const REPO_URL = "https://github.com/adonko3xBitters/boxincloud";

export const ANDROID_APK_URL = `${REPO_URL}/releases/latest/download/boxincloud-android.apk`;

/**
 * L'APK de test, signé avec la clé de debug.
 *
 * Publié tant qu'aucune clé de release n'existe, pour que le code QR mène à
 * quelque chose dès la première version. Il porte un autre nom, exprès :
 * Android identifie une application par sa clé autant que par son identifiant,
 * et la clé de debug est publiquement connue — passer ensuite à une vraie clé
 * exigera une désinstallation. Personne ne doit l'installer sans le savoir.
 */
export const ANDROID_TEST_APK_URL = `${REPO_URL}/releases/latest/download/boxincloud-android-non-signe.apk`;

export const RELEASES_URL = `${REPO_URL}/releases/latest`;

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
