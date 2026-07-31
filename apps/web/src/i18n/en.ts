import type { MessageKey } from "./fr";

/**
 * English catalogue.
 *
 * Typed as `Record<MessageKey, string>` on purpose: adding a key to `fr.ts`
 * without translating it here breaks the build. A missing translation that
 * merely fell back to French would ship silently, and nobody reviews an
 * interface in a language they do not read.
 */
export const en: Record<MessageKey, string> = {
  // ── General ────────────────────────────────────────────────────────────
  "app.tagline": "Your comics, graphic novels and manga library",
  "action.cancel": "Cancel",
  "action.close": "Close",
  "action.save": "Save",
  "action.delete": "Delete",
  "action.create": "Create",
  "action.retry": "Try again",
  "action.copy": "Copy",
  "action.copied": "Copied",
  "action.confirm": "Confirm",
  "state.loading": "Loading…",
  "error.generic": "Something went wrong.",

  // ── Sign in ────────────────────────────────────────────────────────────
  "auth.signIn": "Sign in",
  "auth.signOut": "Sign out",
  "auth.username": "Username",
  "auth.password": "Password",
  "auth.displayName": "Display name",
  "auth.email": "Email address",
  "auth.role": "Role",
  "auth.roleAdmin": "Administrator",
  "auth.roleUser": "User",

  // ── Account menu ───────────────────────────────────────────────────────
  "account.label": "Account",
  "account.mobileApp": "Mobile app",
  "account.devices": "Connected devices",
  "account.storage": "Storage",
  "account.accounts": "Accounts",
  "account.theme": "Switch theme",

  // ── Devices ────────────────────────────────────────────────────────────
  "devices.title": "Connected devices",
  "devices.empty": "No devices registered.",
  "devices.current": "this one",
  "devices.unnamed": "Unnamed device",
  "devices.revoke": "Revoke",
  "devices.revokeAll": "Sign out everywhere",
  "devices.hint":
    "Revoking a device cuts its access immediately, leaving the others untouched.",
  "devices.platform.web": "Browser",
  "devices.platform.android": "Android",
  "devices.platform.ios": "iOS",
  "devices.platform.desktop": "Desktop",
  "devices.seen.now": "just now",
  "devices.seen.unknown": "unknown date",

  // ── Cache ──────────────────────────────────────────────────────────────
  "cache.tab": "Cache",
  "cache.entries": "Entries",
  "cache.hits": "Reads served",
  "cache.oldest": "Oldest",
  "cache.unbounded": "unbounded cache",
  "cache.of": "of",
  "cache.purge": "Clear cache",
  "cache.purgeConfirm": "Confirm clearing",
  "cache.explain":
    "Thumbnails, covers and transcoded pages. Everything here is rebuilt from the original archives: clearing this cache loses no data, it only costs a regeneration on the next read.",

  // ── Storage ────────────────────────────────────────────────────────────
  "storage.title": "Storage",
  "storage.libraries": "Libraries",
  "storage.backends": "Storage backends",

  // ── Mobile app ─────────────────────────────────────────────────────────
  "mobile.title": "Mobile app",
  "mobile.scanHint": "Scan with your phone's camera.",
  "mobile.linkLabel": "The page this code opens",
  "mobile.download": "Download for Android",
  "mobile.serverAddress": "This server's address",
  "mobile.serverAddressHint": "Enter it in the app on first launch.",

  // ── Storage: libraries ─────────────────────────────────────────────────
  "storage.dialogLabel": "Storage and libraries",
  "storage.newLibrary": "New library",
  "storage.noLibraries": "No libraries yet.",
  "storage.noBackendTitle": "No storage backend",
  "storage.noBackendDetail":
    "A library points at a location inside a storage backend. Declare one first, under the \u201cStorage backends\u201d tab.",
  "storage.fieldName": "Name",
  "storage.libraryNamePlaceholder": "My comics",
  "storage.backendField": "Storage backend",
  "storage.subfolder": "Subfolder",
  "storage.subfolderHint":
    "Location inside the backend. Leave empty to take everything.",
  "storage.creating": "Creating\u2026",
  "storage.albumOne": "{count} album",
  "storage.albumOther": "{count} albums",
  "storage.lastScan": "last scan {status}",
  "storage.scan": "Scan",
  "storage.edit": "Edit",
  "storage.rootPrefix": "Root prefix",
  "storage.unchanged": "unchanged",
  "storage.rootPrefixHint":
    "Changing the prefix moves nothing: already indexed albums still point at the old one. The change says where to look from now on, and a new scan rebuilds the catalogue.",
  "storage.saving": "Saving\u2026",
  "storage.deleteLibrary": "Delete this library",
  "storage.deleteLibraryWarning":
    "Albums, folders, reading progress, favourites, ratings and shares all go. Your files stay untouched \u2014 recreating the library on the same prefix finds every one of them. The reading history does not come back.",
  "storage.typeToConfirm": "Type {word} to confirm.",
  "storage.confirmWord": "delete",
  "storage.deleteForever": "Delete permanently",

  "cache.freed": "{entries} entries removed, {bytes} freed.",

  // ── Storage: scans ─────────────────────────────────────────────────────
  "scan.hide": "Hide",
  "scan.show": "Show",
  "scan.lastRuns": "the last {count} scans",
  "scan.counts": "{seen} seen · {added} added · {updated} updated",
  "scan.removed": " · {count} gone",
  "scan.errors": " · {count} errors",
  "scan.status.success": "succeeded",
  "scan.status.running": "running",
  "scan.status.failed": "failed",
  "scan.status.cancelled": "cancelled",

  // ── Storage: backends ──────────────────────────────────────────────────
  "storage.newBackend": "New storage backend",
  "storage.noBackendsDetail":
    "A storage backend is where your files actually live: a folder on the server, or an S3 / MinIO bucket. boxincloud hosts none of it \u2014 it reads yours.",
  "storage.backendNamePlaceholder": "Living room NAS",
  "storage.type": "Type",
  "storage.kindLocal": "Server folder",
  "storage.kindS3": "S3 / MinIO",
  "storage.folderPath": "Folder path",
  "storage.folderPathHint": "The path as the SERVER sees it, not your machine.",
  "storage.accessKey": "Access key",
  "storage.secretKey": "Secret key",
  "storage.checking": "Checking the backend\u2026",
  "storage.declare": "Declare",
  "storage.checkedBeforeSaving":
    "The backend is reached before being saved: a wrong path or wrong credentials are reported right away, not at the first scan.",
  "storage.isDefault": "default",
  "storage.readOnly": "read-only",
  "storage.localFolder": "Local folder",
  "storage.test": "Test",
  "storage.reachable": "The backend responds.",
  "storage.unreachable": "Unreachable.",
  "storage.keepSecrets":
    "Leave empty to keep the current credentials: they never come back out of the database, not even for an administrator.",
  "storage.setDefault": "Make default",
  "storage.verifying": "Checking\u2026",
  "storage.backendFooter":
    "The backend is reached before being saved. Deleting it is refused while a library relies on it \u2014 your files are never touched.",

  // ── Adding content ─────────────────────────────────────────────────────
  "ingest.title": "Add content",
  "ingest.dropHere": "Drop your albums here, or click to pick them",
  "ingest.formats": "CBZ, CBR, PDF \u2014 and their ZIP and RAR equivalents",
  "ingest.library": "Library",
  "ingest.folder": "Folder",
  "ingest.optional": "(optional)",
  "ingest.folderPlaceholder": "Tintin",
  "ingest.waiting": "{count} waiting",
  "ingest.sending": "Sending\u2026",
  "ingest.send": "Send",
  "ingest.sendWithCount": "Send ({count})",
  "ingest.fileOne": "{count} file",
  "ingest.fileOther": "{count} files",
  "ingest.clearDoneOne": "clear the {count} sent",
  "ingest.clearDoneOther": "clear the {count} sent",
  "ingest.markSent": "Sent",
  "ingest.markFailed": "Failed",
  "ingest.markSending": "In progress",
  "ingest.markPending": "Waiting",
  "ingest.add": "Add",
  "ingest.dropToAdd": "Drop to add",

  // ── First library ──────────────────────────────────────────────────────
  "first.title": "First library",
  "first.defaultName": "My library",
  "first.storageType": "Storage type",
  "first.subfolder": "Subfolder",
  "first.create": "Create the library",

  // ── Scanning ───────────────────────────────────────────────────────────
  "scan.button": "Scan the backend",
  "scan.queued": "Scan started",
  "scan.failed": "Failed",
  "scan.tooltip": "Look for files added outside boxincloud",
  "first.intro":
    "Say where your albums live. The backend is reached before being saved \u2014 a wrong path or wrong credentials are reported right away, not at the first scan.",
  "first.localPath": "Folder path, on the server",

  // ── Sidebar ────────────────────────────────────────────────────────────
  "sidebar.libraries": "Libraries",
  "sidebar.allAlbums": "All albums",
  "sidebar.folders": "Folders",
  "sidebar.newRootFolder": "New folder at the root",
  "sidebar.newFolder": "New folder",
  "sidebar.series": "Series",
  "sidebar.lists": "Reading lists",
  "sidebar.favorites": "Favourites",
  "sidebar.reading": "Reading",
  "sidebar.recent": "Recent",
  "sidebar.noFolders": "No folders",
  "sidebar.expand": "Expand",
  "sidebar.collapse": "Collapse",
  "sidebar.readOnly": "Read-only",
  "sidebar.unlockedForNow": "Unlocked for now",
  "sidebar.actionsOn": "Actions on {name}",
  "sidebar.root": "the root",

  // ── Folder menu ────────────────────────────────────────────────────────
  "folder.open": "Open",
  "folder.newChild": "New subfolder\u2026",
  "folder.rename": "Rename\u2026",
  "folder.lock": "Lock\u2026",
  "folder.share": "Share\u2026",
  "folder.delete": "Delete\u2026",

  // ── Folder dialogs ─────────────────────────────────────────────────────
  "folderDialog.newTitle": "New folder",
  "folderDialog.namePlaceholder": "Tintin",
  "folderDialog.createdInRoot": "It will be created at the root of the library.",
  "folderDialog.createdIn": "It will be created in",
  "folderDialog.noWriteYet":
    "Nothing is written to your storage: an object store has no directories. The folder comes into being with the first album dropped into it.",
  "folderDialog.renameTitle": "Rename the folder",
  "folderDialog.renaming": "Renaming\u2026",
  "folderDialog.rename": "Rename",
  "folderDialog.renameWarning":
    "Every album in the folder is renamed in your storage. On a remote backend, a large branch can take several minutes.",
  "folderDialog.deleteTitle": "Delete the folder",
  "folderDialog.emptyFolder": "This folder is empty. Deleting it touches nothing else.",
  "folderDialog.containsOne": "This folder and its subfolders hold {count} album.",
  "folderDialog.containsOther": "This folder and its subfolders hold {count} albums.",
  "folderDialog.deleteFiles": "Delete the files too",
  "folderDialog.deleteFilesHint":
    "Without this, albums are removed from the catalogue and the files stay in your storage. With it, they are erased \u2014 irreversibly.",
  "folderDialog.deleting": "Deleting\u2026",

  // ── Folder locking ─────────────────────────────────────────────────────
  "lock.title": "Lock the folder",
  "lock.readOnly": "Read-only",
  "lock.readOnlyHint":
    "The folder stays visible to everyone, but can no longer be renamed, moved, nor gain or lose an album. The protection extends to subfolders.",
  "lock.accessCode": "Access code",
  "lock.accessCodeHint":
    "Hides the folder and its contents \u2014 lists, search, direct access \u2014 until the code is entered. Useful on a shared server.",
  "lock.removeCode": "Remove the existing code",
  "lock.newCode": "New code",
  "lock.code": "Code",
  "lock.keepCode": "Leave empty to keep it",
  "lock.minLength": "Four characters minimum",
  "unlock.title": "Locked folder",
  "unlock.duration": "The folder stays open for two hours, then closes itself.",
  "unlock.open": "Open",

  // ── Folder sharing ─────────────────────────────────────────────────────
  "share.title": "Share the folder",
  "share.accounts": "Server accounts",
  "share.accountsHint":
    "A folder with no explicit access is visible to everyone who sees the library. Granting one here closes it to everyone else.",
  "share.with": "Share with {name}",
  "share.write": "write",
  "share.noOtherAccount": "No other account.",
  "share.publicLink": "Public link",
  "share.blockedByCode":
    "Unavailable: this folder is hidden behind an access code. Publishing what you just hid would undo the code without saying so.",
  "share.publicWarning":
    "A public link opens this folder with no account at all: whoever has the address sees the contents, and can pass it on.",
  "share.unnamed": "Unnamed",
  "share.expiresOn": "expires on {date}",
  "share.openedOne": "{count} open",
  "share.openedOther": "{count} opens",
  "share.revoke": "Revoke",
  "share.copyNow": "Copy this link now: it will not be shown again.",
  "share.labelPlaceholder": "For Camille",
  "share.expiresIn": "Expires in",
  "share.oneDay": "1 day",
  "share.sevenDays": "7 days",
  "share.thirtyDays": "30 days",
  "share.oneYear": "1 year",
  "share.createLink": "Create a link",
};
