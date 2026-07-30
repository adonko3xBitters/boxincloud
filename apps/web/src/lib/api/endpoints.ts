/**
 * Fonctions d'appel, une par opération du contrat.
 *
 * Couche mince au-dessus de `request` : elle nomme les endpoints et fixe leurs
 * types, sans logique. Les hooks de `lib/queries` s'en servent.
 */

import { API_BASE, ApiError, refreshToken, request } from "./client";
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
import type { Problem } from "./client";
import { getDeviceId, getTokens, guessDeviceName } from "./tokens";

/** Jeton courant, ou null. Utilisé par le téléversement, hors de `request`. */
const getAccessToken = () => getTokens()?.accessToken ?? null;

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

// ─── Administration et ingestion ─────────────────────────────────────────────

export type StorageBackend = {
  id: string;
  name: string;
  kind: "s3" | "local";
  config: Record<string, string>;
  isDefault: boolean;
  readOnly: boolean;
  status: string;
};

export const listBackends = () =>
  request<{ backends: StorageBackend[] }>("/storage-backends");

export const createBackend = (input: {
  name: string;
  kind: "s3" | "local";
  config?: Record<string, string>;
  secrets?: Record<string, string>;
  isDefault?: boolean;
  readOnly?: boolean;
}) => request<StorageBackend>("/storage-backends", { method: "POST", body: input });

export const testBackend = (backendId: string) =>
  request<{ ok: boolean; detail?: string }>(`/storage-backends/${backendId}/test`, {
    method: "POST",
  });

export type LibraryDetail = {
  id: string;
  backendId: string;
  name: string;
  kind: string;
  rootPrefix: string;
  comicCount: number;
};

export const createLibrary = (input: {
  name: string;
  backendId: string;
  kind?: string;
  rootPrefix?: string;
}) => request<LibraryDetail>("/libraries", { method: "POST", body: input });

export const scanLibrary = (libraryId: string) =>
  request<{ queued: boolean }>(`/libraries/${libraryId}/scan`, { method: "POST" });

export type UploadedComic = {
  comicId: string;
  objectKey: string;
  title: string;
  format: string;
  fileSize: number;
};

/**
 * Téléverse un album.
 *
 * XMLHttpRequest plutôt que fetch : seul le premier expose la progression de
 * l'envoi. Sur une intégrale de plusieurs centaines de méga-octets, une barre
 * qui avance est la différence entre « ça travaille » et « c'est planté ».
 *
 * Le champ `folder` précède le fichier, comme l'exige le contrat : le serveur
 * consomme les parties dans l'ordre, sans mise en tampon.
 */
export function uploadComic(
  libraryId: string,
  file: File,
  options: {
    folder?: string;
    onProgress?: (fraction: number) => void;
    signal?: AbortSignal;
  } = {},
): Promise<UploadedComic> {
  const send = (token: string | null, retryOn401: boolean): Promise<UploadedComic> =>
    new Promise((resolve, reject) => {
      const form = new FormData();
      form.append("folder", options.folder ?? "");
      form.append("file", file, file.name);

      const xhr = new XMLHttpRequest();
      xhr.open("POST", `${API_BASE}/libraries/${libraryId}/upload`);
      if (token) xhr.setRequestHeader("Authorization", `Bearer ${token}`);

      xhr.upload.onprogress = (event) => {
        if (event.lengthComputable) {
          options.onProgress?.(event.loaded / event.total);
        }
      };

      xhr.onload = () => {
        if (xhr.status === 401 && retryOn401) {
          // Un envoi long peut survivre à l'expiration du jeton présenté au
          // départ. On rafraîchit et on rejoue — une fois.
          refreshToken()
            .then((fresh) => (fresh ? send(fresh, false) : Promise.reject(readError(xhr))))
            .then(resolve, reject);
          return;
        }

        if (xhr.status >= 200 && xhr.status < 300) {
          try {
            resolve(JSON.parse(xhr.responseText) as UploadedComic);
          } catch {
            reject(new ApiError(xhr.status, null, "réponse illisible"));
          }
          return;
        }
        reject(readError(xhr));
      };

      xhr.onerror = () => reject(new ApiError(0, null, "envoi interrompu"));
      xhr.onabort = () => reject(new ApiError(0, null, "envoi annulé"));

      options.signal?.addEventListener("abort", () => xhr.abort(), { once: true });

      xhr.send(form);
    });

  return send(getAccessToken(), true);
}

function readError(xhr: XMLHttpRequest): ApiError {
  try {
    const problem = JSON.parse(xhr.responseText) as Problem;
    return new ApiError(xhr.status, problem, problem.title ?? "envoi refusé");
  } catch {
    return new ApiError(xhr.status, null, "envoi refusé");
  }
}
