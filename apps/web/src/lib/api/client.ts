/**
 * Client HTTP de l'API boxincloud.
 *
 * Les types viennent de `schema.d.ts`, généré depuis `api/openapi.yaml` :
 * aucun type d'API n'est écrit à la main, et un changement de contrat casse la
 * compilation plutôt que de se manifester à l'exécution.
 */

import type { components } from "./schema";
import { getTokens, setTokens, clearTokens } from "./tokens";

export type Schemas = components["schemas"];

export type User = Schemas["User"];
export type Tokens = Schemas["Tokens"];
export type Device = Schemas["Device"];
export type Library = Schemas["Library"];
export type Comic = Schemas["Comic"];
export type Series = Schemas["Series"];
export type Manifest = Schemas["Manifest"];
export type Progress = Schemas["Progress"];
export type Problem = Schemas["Problem"];
export type ComicPage = Schemas["ComicPage"];
export type SeriesPage = Schemas["SeriesPage"];
export type Ed2kStatus = Schemas["Ed2kStatus"];
export type Ed2kState = Schemas["Ed2kState"];
export type Ed2kDaemon = Schemas["Ed2kDaemon"];

/**
 * Base de l'API.
 *
 * En production le web est servi par le binaire Go : même origine, chemin
 * relatif. En développement, `next dev` tourne sur son propre port et doit
 * pointer vers le serveur.
 */
export const API_BASE =
  process.env.NEXT_PUBLIC_API_BASE?.replace(/\/$/, "") ?? "/api/v1";

/** Erreur d'API portant le corps RFC 9457. */
export class ApiError extends Error {
  readonly status: number;
  readonly problem: Problem | null;

  constructor(status: number, problem: Problem | null, fallback: string) {
    super(problem?.detail || problem?.title || fallback);
    this.name = "ApiError";
    this.status = status;
    this.problem = problem;
  }

  /** Erreurs de validation par champ, pour annoter un formulaire. */
  get fieldErrors(): Record<string, string> {
    return this.problem?.errors ?? {};
  }

  get isUnauthorized() {
    return this.status === 401;
  }

  /**
   * Une session révoquée impose une reconnexion complète : une réutilisation
   * de refresh token a été détectée côté serveur, toute la chaîne est tombée.
   */
  get isSessionRevoked() {
    return this.problem?.type?.endsWith("/session-revoked") ?? false;
  }
}

type RequestOptions = {
  method?: string;
  body?: unknown;
  query?: Record<string, string | number | boolean | undefined | null>;
  /** Requête publique : ni jeton, ni tentative de rafraîchissement. */
  anonymous?: boolean;
  signal?: AbortSignal;
};

function buildURL(path: string, query?: RequestOptions["query"]): string {
  const url = `${API_BASE}${path}`;
  if (!query) return url;

  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    // `false` et la chaîne vide sont écartés : ils signifient « pas de
    // filtre ». Une exception — `folder: ""` désigne la racine, et le
    // paramètre doit alors être présent ; l'appelant passe donc `folder`
    // explicitement via une clé dédiée quand c'est le cas.
    if (value === undefined || value === null || value === "" || value === false) continue;
    params.set(key, String(value));
  }
  const qs = params.toString();
  return qs ? `${url}?${qs}` : url;
}

async function parseProblem(res: Response): Promise<Problem | null> {
  try {
    return (await res.json()) as Problem;
  } catch {
    return null;
  }
}

/**
 * Rafraîchissement des jetons, dédupliqué.
 *
 * Sans cette déduplication, une page qui déclenche cinq requêtes en parallèle
 * après expiration lancerait cinq rafraîchissements concurrents. Comme le
 * serveur fait tourner le refresh token à chaque usage, quatre d'entre eux
 * présenteraient un jeton déjà consommé — ce que le serveur interprète, à
 * juste titre, comme un vol de jeton, et il révoquerait toute la session.
 */
let refreshInFlight: Promise<string | null> | null = null;

async function refreshAccessToken(): Promise<string | null> {
  if (refreshInFlight) return refreshInFlight;

  refreshInFlight = (async () => {
    const stored = getTokens();
    if (!stored?.refreshToken) return null;

    try {
      const res = await fetch(buildURL("/auth/refresh"), {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ refreshToken: stored.refreshToken }),
      });

      if (!res.ok) {
        clearTokens();
        return null;
      }

      const tokens = (await res.json()) as Tokens;
      setTokens(tokens);
      return tokens.accessToken;
    } catch {
      return null;
    } finally {
      refreshInFlight = null;
    }
  })();

  return refreshInFlight;
}

/**
 * Rafraîchit le jeton d'accès et retourne le nouveau, ou null.
 *
 * Exposé pour le téléversement, qui passe par XMLHttpRequest et ne peut donc
 * pas emprunter `request` : il lui faut néanmoins la même gestion du jeton
 * expiré, sans dupliquer la garde contre les rafraîchissements concurrents.
 */
export function refreshToken(): Promise<string | null> {
  return refreshAccessToken();
}

/** Exécute une requête, en rafraîchissant les jetons au besoin. */
export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { method = "GET", body, query, anonymous = false, signal } = options;
  const url = buildURL(path, query);

  const send = (accessToken: string | null): Promise<Response> => {
    const headers: Record<string, string> = {};
    if (body !== undefined) headers["Content-Type"] = "application/json";
    if (accessToken) headers["Authorization"] = `Bearer ${accessToken}`;

    return fetch(url, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
      signal,
    });
  };

  let res = await send(anonymous ? null : (getTokens()?.accessToken ?? null));

  // Un 401 sur une requête authentifiée signifie presque toujours « jeton
  // expiré » : on rafraîchit une fois et on rejoue, de façon invisible.
  if (res.status === 401 && !anonymous) {
    const fresh = await refreshAccessToken();
    if (fresh) {
      res = await send(fresh);
    }
  }

  if (!res.ok) {
    const problem = await parseProblem(res);
    const error = new ApiError(res.status, problem, `${method} ${path} a échoué`);

    // Session définitivement perdue : on efface, l'application redirigera vers
    // la connexion.
    if (error.isUnauthorized) clearTokens();
    throw error;
  }

  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

/**
 * URL d'une image servie par l'API.
 *
 * Une balise `<img>` ne peut pas porter d'en-tête `Authorization` : le jeton
 * passe donc en paramètre de requête, ce que le serveur accepte explicitement
 * pour ces routes.
 */
export function imageURL(path: string, params: Record<string, string | number> = {}): string {
  const token = getTokens()?.accessToken;
  const search = new URLSearchParams();

  for (const [key, value] of Object.entries(params)) {
    search.set(key, String(value));
  }
  if (token) search.set("token", token);

  // coverPath arrive déjà préfixé par /api/v1 : on ne le préfixe pas deux fois.
  const base = path.startsWith("/api/") ? path : `${API_BASE}${path}`;
  const qs = search.toString();
  return qs ? `${base}?${qs}` : base;
}
