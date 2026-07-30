/**
 * Fonctions d'appel, une par opération du contrat.
 *
 * Couche mince au-dessus de `request` : elle nomme les endpoints et fixe leurs
 * types, sans logique. Les hooks de `lib/queries` s'en servent.
 */

import { request } from "./client";
import type {
  Comic,
  ComicPage,
  Device,
  Library,
  Manifest,
  Progress,
  Series,
  SeriesPage,
  Tokens,
  User,
} from "./client";
import { getDeviceId, guessDeviceName } from "./tokens";

// ─── Authentification ────────────────────────────────────────────────────────

export const getAuthStatus = () =>
  request<{ needsSetup: boolean }>("/auth/status", { anonymous: true });

export const setup = (input: { username: string; email?: string; password: string }) =>
  request<User>("/auth/setup", { method: "POST", body: input, anonymous: true });

export const login = (input: { username: string; password: string }) =>
  request<Tokens>("/auth/login", {
    method: "POST",
    anonymous: true,
    body: {
      ...input,
      // L'appareil est repris s'il existe : le serveur reconnaît alors le
      // navigateur au lieu d'accumuler une entrée par connexion.
      deviceId: getDeviceId(),
      deviceName: guessDeviceName(),
      platform: "web",
    },
  });

export const logout = (refreshToken: string) =>
  request<void>("/auth/logout", { method: "POST", body: { refreshToken }, anonymous: true });

export const getCurrentUser = () => request<User>("/me");

export const listDevices = () => request<{ devices: Device[] }>("/me/devices");

export const logoutAllDevices = () =>
  request<{ revokedSessions: number }>("/me/logout-all", { method: "POST" });

// ─── Catalogue ───────────────────────────────────────────────────────────────

export const listLibraries = () => request<{ libraries: Library[] }>("/libraries");

export const getHome = (limit = 20) =>
  request<{ recent: Comic[]; nextInSeries: Comic[] }>("/home", { query: { limit } });

/**
 * Critères de listage des albums.
 *
 * Exporté parce que l'espace de travail les construit ailleurs : sans un type
 * partagé, une portée qui produirait un paramètre mal nommé serait ignorée
 * silencieusement par le serveur au lieu d'échouer à la compilation.
 */
export type ComicQuery = {
  libraryId?: string;
  seriesId?: string;
  /** Dossier et ses sous-dossiers. Chaîne vide = racine. */
  folder?: string;
  favorites?: boolean;
  /** unread | in_progress | read. Vide, aucun filtre. */
  readStatus?: string;
  /** recent | title | released. */
  sort?: string;
  cursor?: string;
  limit?: number;
};

export const listComics = (params: ComicQuery) =>
  request<ComicPage>("/comics", { query: params });

export const getComic = (comicId: string) => request<Comic>(`/comics/${comicId}`);

export const listSeries = (params: { libraryId?: string; cursor?: string; limit?: number }) =>
  request<SeriesPage>("/series", { query: params });

export const getSeries = (seriesId: string) =>
  request<{ series: Series; comics: Comic[] }>(`/series/${seriesId}`);

export const search = (params: { q: string; libraryId?: string; limit?: number }, signal?: AbortSignal) =>
  request<{ comics: Comic[]; series: Series[] }>("/search", { query: params, signal });

// ─── Lecture ─────────────────────────────────────────────────────────────────

export const getManifest = (comicId: string) =>
  request<Manifest>(`/comics/${comicId}/manifest`);

// ─── Progression ─────────────────────────────────────────────────────────────

export const getProgress = (comicId: string) =>
  request<Progress>(`/comics/${comicId}/progress`);

export const updateProgress = (
  comicId: string,
  input: { page: number; pageCount: number; status?: Progress["status"] },
) => request<Progress>(`/comics/${comicId}/progress`, { method: "PUT", body: input });

export const deleteProgress = (comicId: string) =>
  request<void>(`/comics/${comicId}/progress`, { method: "DELETE" });

export const continueReading = (limit = 20) =>
  request<{ items: Progress[] }>("/continue-reading", { query: { limit } });

// ─── Outils de gestion ───────────────────────────────────────────────────────

export type Folder = {
  path: string;
  name: string;
  depth: number;
  comicCount: number;
};

export const listFolders = (libraryId?: string) =>
  request<{ folders: Folder[] }>("/folders", { query: { libraryId } });

/** Favoris et notes en une requête : une grille les affiche ensemble. */
export const getUserMarks = () =>
  request<{ favorites: string[]; ratings: Record<string, number> }>("/me/marks");

export const setFavorite = (comicId: string, favorite: boolean) =>
  request<{ favorite: boolean }>(`/comics/${comicId}/favorite`, {
    method: "PUT",
    body: { favorite },
  });

/** rating de 1 à 5 ; 0 retire la note. */
export const setRating = (comicId: string, rating: number) =>
  request<{ rating: number }>(`/comics/${comicId}/rating`, {
    method: "PUT",
    body: { rating },
  });

export const editComic = (
  comicId: string,
  edit: { title?: string; number?: string; summary?: string; language?: string },
) => request<Comic>(`/comics/${comicId}`, { method: "PATCH", body: edit });

export type BulkAction = "read" | "unread" | "favorite" | "unfavorite";

export const bulk = (action: BulkAction, ids: string[]) =>
  request<{ affected: number }>("/comics/bulk", {
    method: "POST",
    body: { action, ids },
  });
