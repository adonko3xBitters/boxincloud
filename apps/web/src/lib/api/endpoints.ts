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
  Ed2kConnection,
  Ed2kDaemon,
  Ed2kDownload,
  Ed2kQueuedPeer,
  Ed2kServer,
  Ed2kSharedFile,
  Ed2kSnapshot,
  Ed2kSource,
  Ed2kStats,
  Ed2kStatus,
  Ed2kPriority,
  Ed2kUpload,
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

/**
 * Révoque un seul appareil.
 *
 * Le geste après un téléphone perdu. `logoutAllDevices` existe à côté, mais
 * oblige à se reconnecter partout, y compris là où on est en train de lire.
 */
export const revokeDevice = (deviceId: string) =>
  request<{ revokedSessions: number }>(`/me/devices/${deviceId}`, {
    method: "DELETE",
  });

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
  id: string;
  libraryId: string;
  path: string;
  name: string;
  depth: number;
  comicCount: number;
  /** Vrai pour un dossier créé à la main, qui survit au fait d'être vide. */
  explicit: boolean;
  /** Protégé contre les modifications. Ne masque rien. */
  readOnly: boolean;
  /** Masqué par un code d'accès. */
  hasCode: boolean;
  /** Le code a été saisi et n'a pas expiré. */
  unlocked: boolean;
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

// ─── Comptes ─────────────────────────────────────────────────────────────────

export type Account = {
  id: string;
  username: string;
  email?: string;
  role: "admin" | "user";
  displayName?: string;
  restricted: boolean;
  maxAgeRating?: number;
  lastLoginAt?: string;
  createdAt: string;
};

export type LibraryGrant = {
  libraryId: string;
  userId: string;
  canWrite: boolean;
};

export const listAccounts = () => request<{ accounts: Account[] }>("/accounts");

export const createAccount = (input: {
  username: string;
  password: string;
  email?: string;
  role?: "admin" | "user";
  displayName?: string;
}) => request<Account>("/accounts", { method: "POST", body: input });

export const updateAccount = (
  userId: string,
  input: {
    displayName?: string;
    email?: string;
    role?: "admin" | "user";
    restricted?: boolean;
    maxAgeRating?: number;
    password?: string;
  },
) => request<Account>(`/accounts/${userId}`, { method: "PATCH", body: input });

export const deleteAccount = (userId: string) =>
  request<void>(`/accounts/${userId}`, { method: "DELETE" });

export const listAccountAccess = (userId: string) =>
  request<{ grants: LibraryGrant[] }>(`/accounts/${userId}/library-access`);

export const listLibraryAccess = (libraryId: string) =>
  request<{ grants: LibraryGrant[] }>(`/libraries/${libraryId}/access`);

export const grantLibraryAccess = (libraryId: string, userId: string, canWrite: boolean) =>
  request<LibraryGrant>(`/libraries/${libraryId}/access`, {
    method: "POST",
    body: { userId, canWrite },
  });

export const revokeLibraryAccess = (libraryId: string, userId: string) =>
  request<void>(`/libraries/${libraryId}/access/${userId}`, { method: "DELETE" });

// ─── Suppression et déplacement ──────────────────────────────────────────────

/**
 * Retire un album.
 *
 * `deleteFile` est faux par défaut, et ce défaut compte : retirer un album d'un
 * catalogue se rattrape, effacer un fichier non.
 */
export const deleteComic = (comicId: string, deleteFile = false) =>
  request<void>(`/comics/${comicId}`, {
    method: "DELETE",
    query: deleteFile ? { deleteFile: true } : undefined,
  });

export const moveComic = (comicId: string, folder: string) =>
  request<{ folderPath: string }>(`/comics/${comicId}/folder`, {
    method: "PUT",
    body: { folder },
  });

export type ManageAction = "delete" | "move";

export const manageComics = (
  action: ManageAction,
  ids: string[],
  options: { folder?: string; deleteFile?: boolean; libraryId?: string } = {},
) =>
  request<{ affected: number }>("/comics/manage", {
    method: "POST",
    body: { action, ids, ...options },
  });

// ─── Dossiers ────────────────────────────────────────────────────────────────

export const createFolder = (libraryId: string, path: string) =>
  request<Folder>("/folders", { method: "POST", body: { libraryId, path } });

/** Renommer et déplacer sont le même geste : seul le chemin complet change. */
export const relocateFolder = (libraryId: string, path: string, newPath: string) =>
  request<Folder>("/folders/path", { method: "PUT", body: { libraryId, path, newPath } });

export const deleteFolder = (
  libraryId: string,
  path: string,
  options: { deleteComics?: boolean; deleteFiles?: boolean } = {},
) =>
  request<{ removedComics: number }>(`/libraries/${libraryId}/folders`, {
    method: "DELETE",
    query: { path, ...options },
  });

export const setFolderLock = (
  libraryId: string,
  path: string,
  lock: { readOnly?: boolean; code?: string },
) => request<Folder>("/folders/lock", { method: "PUT", body: { libraryId, path, ...lock } });

export const unlockFolder = (libraryId: string, path: string, code: string) =>
  request<{ unlockedUntil: string }>("/folders/unlock", {
    method: "POST",
    body: { libraryId, path, code },
  });

export const relockFolder = (libraryId: string, path: string) =>
  request<void>(`/libraries/${libraryId}/folders/unlock`, {
    method: "DELETE",
    query: { path },
  });

// ─── Partage ─────────────────────────────────────────────────────────────────

export type FolderGrant = {
  userId: string;
  username?: string;
  displayName?: string;
  canWrite: boolean;
};

export const listFolderGrants = (libraryId: string, path: string) =>
  request<{ grants: FolderGrant[] }>(`/libraries/${libraryId}/folders/access`, {
    query: { path },
  });

export const grantFolderAccess = (
  libraryId: string,
  path: string,
  userId: string,
  canWrite: boolean,
) => request<FolderGrant>("/folders/access", {
  method: "POST",
  body: { libraryId, path, userId, canWrite },
});

export const revokeFolderAccess = (libraryId: string, path: string, userId: string) =>
  request<void>(`/libraries/${libraryId}/folders/access/${userId}`, {
    method: "DELETE",
    query: { path },
  });

export type ShareLink = {
  id: string;
  libraryId: string;
  folderPath?: string;
  comicId?: string;
  label?: string;
  expiresAt: string;
  createdAt: string;
  lastUsedAt?: string;
  useCount: number;
  /** Présent uniquement à la création : le jeton n'est pas relisible ensuite. */
  token?: string;
};

export const listShareLinks = () => request<{ links: ShareLink[] }>("/share-links");

export const createShareLink = (input: {
  libraryId: string;
  folderPath?: string;
  comicId?: string;
  label?: string;
  expiresAt: string;
}) => request<ShareLink>("/share-links", { method: "POST", body: input });

export const revokeShareLink = (shareId: string) =>
  request<void>(`/share-links/${shareId}`, { method: "DELETE" });

export type SharedContent = {
  scope: "folder" | "comic";
  label?: string;
  expiresAt: string;
  comics: Array<{
    id: string;
    title: string;
    seriesName?: string;
    number?: string;
    pageCount: number;
    coverPath: string;
  }>;
};

/** Consultation publique : aucun jeton d'authentification n'est envoyé. */
export const getSharedContent = (token: string) =>
  request<SharedContent>(`/share/${token}`, { anonymous: true });

// ─── Administration du stockage ──────────────────────────────────────────────

export const updateBackend = (
  backendId: string,
  input: {
    name?: string;
    config?: Record<string, string>;
    /** Omis, les identifiants existants sont conservés. */
    secrets?: Record<string, string>;
    readOnly?: boolean;
  },
) => request<StorageBackend>(`/storage-backends/${backendId}`, { method: "PATCH", body: input });

export const deleteBackend = (backendId: string) =>
  request<void>(`/storage-backends/${backendId}`, { method: "DELETE" });

export const setDefaultBackend = (backendId: string) =>
  request<void>(`/storage-backends/${backendId}/default`, { method: "PUT" });

export const updateLibrary = (
  libraryId: string,
  input: { name?: string; rootPrefix?: string },
) => request<LibraryDetail>(`/libraries/${libraryId}`, { method: "PATCH", body: input });

export const deleteLibrary = (libraryId: string) =>
  request<void>(`/libraries/${libraryId}`, { method: "DELETE" });

export type ScanRun = {
  id: string;
  status: string;
  startedAt: string;
  finishedAt?: string;
  objectsSeen: number;
  added: number;
  updated: number;
  removed: number;
  errors: number;
  detail?: string;
};

export const listScanRuns = (libraryId: string) =>
  request<{ runs: ScanRun[] }>(`/libraries/${libraryId}/scans`);

// ─── Cache dérivé ────────────────────────────────────────────────────────────

export type CacheStats = {
  entries: number;
  bytes: number;
  hits: number;
  maxBytes?: number;
  oldestAt?: string;
  newestHitAt?: string;
};

export const getCacheStats = () => request<CacheStats>("/cache");

export const purgeCache = () =>
  request<{ entries: number; bytes: number }>("/cache", { method: "DELETE" });

// ─── Module eD2k/Kad ─────────────────────────────────────────────────────────

export const getEd2kStatus = () => request<Ed2kStatus>("/ed2k/status");

export const getEd2kDaemon = () => request<Ed2kDaemon>("/ed2k/daemon");

export const setEd2kDaemon = (input: {
  host: string;
  port: number;
  password: string;
  label?: string;
}) => request<Ed2kDaemon>("/ed2k/daemon", { method: "PUT", body: input });

export const forgetEd2kDaemon = () => request<void>("/ed2k/daemon", { method: "DELETE" });

/*
  Lecture du module.

  Chaque réponse porte `takenAt` : le serveur rend le dernier instantané scruté,
  pas une mesure faite à la demande. Sans cette date, l'interface afficherait
  des chiffres figés depuis dix minutes avec l'assurance de chiffres frais.
*/

export const getEd2kSnapshot = () => request<Ed2kSnapshot>("/ed2k/snapshot");

export const listEd2kDownloads = () =>
  request<{ takenAt: string; downloads: Ed2kDownload[] }>("/ed2k/downloads");

export const listEd2kDownloadSources = (hash: string) =>
  request<{ takenAt: string; sources: Ed2kSource[] }>(`/ed2k/downloads/${hash}/sources`);

export const listEd2kUploads = () =>
  request<{ takenAt: string; uploads: Ed2kUpload[]; queuedPeers: Ed2kQueuedPeer[] }>(
    "/ed2k/uploads",
  );

export const listEd2kServers = () =>
  request<{ takenAt: string; servers: Ed2kServer[] }>("/ed2k/servers");

export const listEd2kSharedFiles = () =>
  request<{ takenAt: string; files: Ed2kSharedFile[] }>("/ed2k/shared");

export const getEd2kStats = () =>
  request<{ takenAt: string; stats: Ed2kStats; connection: Ed2kConnection }>("/ed2k/stats");

/**
 * Adresse du flux d'événements du module.
 *
 * Le jeton passe en paramètre d'URL, parce qu'`EventSource` ne sait pas porter
 * d'en-tête `Authorization` — le serveur l'accepte explicitement sur cette
 * route, et sur celles-là seulement.
 */
export function ed2kEventsURL(): string {
  const token = getAccessToken();
  const url = `${API_BASE}/ed2k/events`;
  return token ? `${url}?token=${encodeURIComponent(token)}` : url;
}

// ─── Module eD2k : commandes ─────────────────────────────────────────────────
//
// Toutes répondent 202 sans corps : amuled n'accuse que réception, jamais
// l'effet. C'est l'instantané suivant qui dit ce qui s'est passé — et il arrive
// dans la seconde, la commande réveillant la scrutation.

export type Ed2kDownloadAction = "pause" | "resume" | "stop" | "cancel";

export const actOnEd2kDownload = (hash: string, action: Ed2kDownloadAction) =>
  request<void>(`/ed2k/downloads/${hash}/action`, { method: "POST", body: { action } });

export const setEd2kDownloadPriority = (hash: string, priority: Ed2kPriority) =>
  request<void>(`/ed2k/downloads/${hash}/priority`, { method: "PUT", body: { priority } });

export const addEd2kLink = (link: string) =>
  request<void>("/ed2k/links", { method: "POST", body: { link } });

export const connectEd2kServer = (target?: { ip: string; port: number }) =>
  request<void>("/ed2k/servers/connect", { method: "POST", body: target ?? {} });

export const disconnectEd2kServer = () =>
  request<void>("/ed2k/servers/disconnect", { method: "POST" });

export const setEd2kKadRunning = (running: boolean) =>
  request<void>("/ed2k/kad", { method: "POST", body: { running } });

export const reloadEd2kSharedFiles = () =>
  request<void>("/ed2k/shared/reload", { method: "POST" });
