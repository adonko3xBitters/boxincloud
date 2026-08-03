import type { MessageKey } from "@/i18n";
import { ApiError } from "./client";

/**
 * Met en mots une erreur venue du serveur.
 *
 * Le serveur ne traduit rien, et c'est délibéré. Il renvoie un problème RFC
 * 7807 dont le champ `type` est un identifiant stable — pas une phrase. Traduire
 * côté serveur obligerait à y tenir un second catalogue, en Go, qu'il faudrait
 * garder aligné sur celui de l'interface ; et le serveur devrait deviner la
 * langue du lecteur à partir d'un en-tête, alors que l'interface la connaît.
 *
 * La granularité y perd un peu : « Cet élément n'existe plus » là où le serveur
 * disait « library not found ». C'est acceptable parce que le contexte — la
 * boîte de dialogue ouverte — dit déjà de quel élément il s'agit. Un message
 * précis dans une langue qu'on ne lit pas est moins utile qu'un message vague
 * dans la sienne.
 */

const PROBLEM_PREFIX = "https://boxincloud.dev/problems/";

/** Les types que le serveur produit, et leur clé de traduction. */
const KEYS: Record<string, MessageKey> = {
  "not-found": "problem.not-found",
  "bad-request": "problem.bad-request",
  validation: "problem.validation",
  unauthorized: "problem.unauthorized",
  "token-expired": "problem.token-expired",
  "session-revoked": "problem.session-revoked",
  forbidden: "problem.forbidden",
  "method-not-allowed": "problem.method-not-allowed",
  "too-many-requests": "problem.too-many-requests",
  "service-unavailable": "problem.service-unavailable",
  "folder-not-empty": "problem.folder-not-empty",
  "folder-read-only": "problem.folder-read-only",
  "backend-in-use": "problem.backend-in-use",
  "not-indexed": "problem.not-indexed",
};

/** Règles de validation que le serveur peut renvoyer par champ. */
const FIELD_RULES: Record<string, MessageKey> = {
  required: "field.required",
  invalid: "field.invalid",
  unknown: "field.unknown",
  taken: "field.taken",
  exists: "field.exists",
  format: "field.format",
  mismatch: "field.mismatch",
  range: "field.range",
  "one-of": "field.one-of",
  unreachable: "field.unreachable",
  self: "field.self",
  protected: "field.protected",
  "no-code": "field.no-code",
  "wrong-code": "field.wrong-code",
};

/**
 * Traduit les erreurs par champ d'un formulaire.
 *
 * Une règle inconnue disparaît plutôt que de s'afficher en anglais : le champ
 * reste marqué invalide, et le message général du formulaire dit ce qu'il faut.
 * Montrer un code brut — « one-of » — ne renseignerait personne.
 */
export function describeFields(
  error: unknown,
  t: (key: MessageKey) => string,
): Record<string, string> {
  if (!(error instanceof ApiError)) return {};

  const out: Record<string, string> = {};
  for (const [field, rule] of Object.entries(error.fieldErrors)) {
    const key = FIELD_RULES[rule];
    if (key) out[field] = t(key);
  }
  return out;
}

/**
 * Traduit une erreur, quelle qu'en soit la nature.
 *
 * Trois cas, dans cet ordre. Un problème dont le type est connu : sa clé. Un
 * problème dont le type ne l'est pas — un type ajouté au serveur sans l'être
 * ici : on retombe sur le statut HTTP, qui distingue au moins une panne d'un
 * refus. Une erreur qui n'est pas un problème du tout — le réseau coupé, une
 * réponse illisible : le message générique.
 *
 * Le `detail` du serveur n'est JAMAIS affiché. Il est en anglais, et l'y voir
 * apparaître au détour d'un cas non prévu est précisément ce qu'on corrige.
 */
export function describeError(
  error: unknown,
  t: (key: MessageKey) => string,
): string {
  if (!(error instanceof ApiError)) {
    // Le client lève une ApiError pour toute réponse du serveur : ce qui n'en
    // est pas une n'a pas atteint le serveur, ou n'en est pas revenu.
    return t("problem.network");
  }

  // Une erreur de validation porte souvent une règle par champ, plus précise
  // que le type : « déjà utilisé » vaut mieux que « un ou plusieurs champs
  // sont invalides ».
  const fields = describeFields(error, t);
  const first = Object.values(fields)[0];
  if (first) return first;

  const type = error.problem?.type;
  if (type?.startsWith(PROBLEM_PREFIX)) {
    const key = KEYS[type.slice(PROBLEM_PREFIX.length)];
    if (key) return t(key);
  }

  return t(keyForStatus(error.status));
}

/**
 * Repli sur le statut HTTP.
 *
 * Sert quand le serveur produit un type que cette version de l'interface ne
 * connaît pas — un serveur plus récent que le web qu'il sert, ce qui n'arrive
 * pas avec le binaire unique mais arrive derrière un proxy mal configuré.
 */
function keyForStatus(status: number): MessageKey {
  switch (status) {
    case 401:
      return "problem.unauthorized";
    case 403:
      return "problem.forbidden";
    case 404:
      return "problem.not-found";
    case 405:
      return "problem.method-not-allowed";
    case 422:
      return "problem.validation";
    case 429:
      return "problem.too-many-requests";
    case 503:
      return "problem.service-unavailable";
    default:
      return status >= 500 ? "problem.internal" : "problem.bad-request";
  }
}

/**
 * Le diagnostic BRUT rendu par le serveur, quand il en dit plus que la règle.
 *
 * Une erreur de validation porte un code par champ — « invalid » — que
 * l'interface traduit. Ce code dit qu'il y a un problème, pas lequel : adresse
 * absente, schéma refusé, site qui ne répond pas et gabarit qui ne lit plus la
 * page tombent tous sur le même mot.
 *
 * Le serveur joint désormais sa phrase. Elle n'est pas traduisible — elle cite
 * souvent le service distant, en anglais — et se présente donc comme un
 * diagnostic technique sous un libellé traduit, jamais comme une phrase à lire.
 * C'est le parti déjà pris pour l'essai d'un catalogue.
 *
 * Le texte générique est écarté : il ne dit rien de plus que la règle.
 */
export function rawDetail(error: unknown): string | null {
  if (!(error instanceof ApiError)) return null;

  const detail = error.problem?.detail;
  if (!detail || detail === "One or more fields are invalid.") return null;
  return detail;
}
