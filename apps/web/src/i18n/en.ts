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
  "ingest.formats": "CBZ, CBR, CB7, PDF, EPUB \u2014 and their ZIP, RAR and 7z equivalents",
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

  // ── Accounts ───────────────────────────────────────────────────────────
  "accounts.title": "Accounts",
  "accounts.new": "New account",
  "accounts.pickOne": "Pick an account",
  "accounts.passwordHint":
    "Twelve characters minimum. Length protects better than a rule about capitals and digits, which mostly produces predictable variants of the same word.",
  "accounts.create": "Create the account",
  "accounts.selfRoleHint":
    "You cannot change your own role: one mistaken click would cost you access to administration.",
  "accounts.restricted": "Restricted profile",
  "accounts.restrictedHint": "Hides albums rated above the limit.",
  "accounts.maxRating": "Maximum rating",
  "accounts.years": "years",
  "accounts.newPassword": "New password",
  "accounts.newPasswordHint":
    "Leave empty to keep it. Open sessions are not closed: that is a separate action, so that someone who merely forgot their password is not signed out everywhere.",
  "accounts.disabling": "Disabling\u2026",
  "accounts.disable": "Disable this account",
  "accounts.cannotDisableSelf": "You cannot disable your own account.",
  "accounts.disableHint":
    "Reading progress, favourites and ratings are kept. Open sessions are closed.",
  "accounts.libraryAccess": "Library access",
  "accounts.libraryAccessHint":
    "A library with no explicit access is visible to everyone. Granting one here closes it to every other account.",
  "accounts.accessTo": "Access to {name}",
  "accounts.noLibrary": "No libraries.",
  "accounts.roleReader": "Reader",
  "accounts.suffixRestricted": " · restricted",

  // ── Toolbar ────────────────────────────────────────────────────────────
  "toolbar.read": "Read",
  "toolbar.markRead": "Mark as read",
  "toolbar.markUnread": "Mark as unread",
  "toolbar.unfavorite": "Remove from favourites",
  "toolbar.favorite": "Add to favourites",
  "toolbar.moveToFolder": "Move to a folder",
  "toolbar.removeFromLibrary": "Remove from the library",
  "toolbar.readStatus": "Reading",
  "toolbar.all": "All",
  "toolbar.unread": "Unread",
  "toolbar.inProgress": "In progress",
  "toolbar.done": "Read",
  "toolbar.sort": "Sort",
  "toolbar.sortAdded": "Added",
  "toolbar.sortTitle": "Title",
  "toolbar.sortReleased": "Released",
  "toolbar.viewGrid": "Grid",
  "toolbar.viewList": "List",
  "toolbar.viewCoverflow": "Coverflow",
  "toolbar.selectedOne": "{count} selected",
  "toolbar.selectedOther": "{count} selected",
  "toolbar.clearSelection": "clear",
  "toolbar.selectAll": "select all",

  // ── Album details ──────────────────────────────────────────────────────
  "detail.resume": "Resume p. {page}",
  "detail.editMetadata": "Edit metadata",
  "detail.pageOf": "Page {page} / {total}",
  "detail.rate": "Rate {step} out of 5",
  "detail.rating": "Rating",
  "detail.number": "Number",
  "detail.pages": "Pages",
  "detail.format": "Format",
  "detail.size": "Size",
  "detail.released": "Released",
  "detail.language": "Language",
  "detail.file": "File",
  "detail.title": "Title",
  "detail.summary": "Summary",

  // ── Remove / move ──────────────────────────────────────────────────────
  "manage.removeOne": "Remove the album",
  "manage.removeMany": "Remove {count} albums",
  "manage.fromCatalog": "Remove from the catalogue",
  "manage.fromCatalogHint":
    "The file stays in your storage. Reading progress, favourites and ratings are kept. A new scan will not bring it back.",
  "manage.deleteFile": "Delete the file too",
  "manage.deleteFileHint":
    "The file is erased from storage. Irreversible: boxincloud keeps no copy.",
  "manage.working": "Working\u2026",
  "manage.remove": "Remove",
  "manage.moveOne": "Move the album",
  "manage.moveMany": "Move {count} albums",
  "manage.rootPlaceholder": "Leave empty for the root",
  "manage.moving": "Moving\u2026",
  "manage.move": "Move",

  // ── Search ─────────────────────────────────────────────────────────────
  "search.action": "Search",
  "search.shortcut": "Search (/)",
  "search.center": "Search",
  "search.placeholder": "Title, series, number\u2026",
  "search.series": "Series",
  "search.albums": "Albums",

  // ── Table ──────────────────────────────────────────────────────────────
  "table.title": "Title",
  "table.series": "Series",
  "table.number": "No.",
  "table.progress": "Page",
  "table.pages": "Sheets",
  "table.size": "Size",
  "table.released": "Released",
  "table.read": "Read",
  "table.rating": "Rating",
  "table.selectAll": "Select all",
  "table.select": "Select {title}",

  // ── Coverflow ──────────────────────────────────────────────────────────
  "coverflow.label": "Cover carousel",
  "coverflow.previous": "Previous cover",
  "coverflow.next": "Next cover",

  // ── Album menu ─────────────────────────────────────────────────────────
  "comicMenu.readNamed": "Read \u201c{title}\u201d",
  "comicMenu.markRead": "Mark as read",
  "comicMenu.markReadCount": "Mark as read ({count})",
  "comicMenu.markUnread": "Mark as unread",
  "comicMenu.move": "Move to a folder\u2026",
  "comicMenu.moveCount": "Move to a folder\u2026 ({count})",
  "comicMenu.remove": "Remove from the library\u2026",
  "comicMenu.removeCount": "Remove from the library\u2026 ({count})",

  // ── Reader ─────────────────────────────────────────────────────────────
  "reader.hideChrome": "Hide the interface",
  "reader.showChrome": "Show the interface",
  "reader.close": "Close the reader",
  "reader.settings": "Reading settings",
  "reader.position": "Position in the album",
  "reader.thumbnails": "Album pages (t)",
  "reader.mode": "Mode",
  "reader.modeSingle": "Single page",
  "reader.modeDouble": "Two pages",
  "reader.modeScroll": "Continuous",
  "reader.fit": "Fit",
  "reader.fitWidth": "Width",
  "reader.fitHeight": "Height",
  "reader.fitPage": "Whole page",
  "reader.direction": "Reading direction",
  "reader.ltr": "Left \u2192 right",
  "reader.rtl": "Right \u2192 left",
  "reader.shortcuts":
    "Arrows or space to turn · Home and End for the ends · t for thumbnails · + \u2212 0 to zoom · double-click or pinch to enlarge · Esc to leave",
  "reader.previousPage": "Previous page",
  "reader.nextPage": "Next page",

  // ── Workspace ──────────────────────────────────────────────────────────
  "workspace.emptyTitle": "No albums here",
  "workspace.emptyDetail": "Switch folder, or widen the filters.",
  "workspace.loadMore": "Load more",
  "workspace.readNamed": "Read {title}",

  // ── Sign in and setup ──────────────────────────────────────────────────
  "login.failed": "Sign-in failed. Is the server reachable?",
  "setup.mismatch": "The passwords do not match.",
  "setup.unreachable": "Is the server reachable?",
  "setup.usernameHint": "Letters, digits, hyphen, dot and underscore.",
  "setup.emailOptional": "Email address (optional)",
  "setup.confirm": "Confirmation",

  // ── Shared link ────────────────────────────────────────────────────────
  "shared.invalid": "This link is not valid",
  "shared.expiredOrRevoked": "It may have expired, or been revoked.",
  "shared.expiredOrRevokedBy":
    "It may have expired, or been revoked by whoever sent it to you.",
  "shared.albums": "Shared albums",
  "shared.album": "Shared album",
  "shared.nothingTitle": "Nothing to see",
  "shared.nothingDetail": "This share holds no albums.",

  // ── Filmstrip ──────────────────────────────────────────────────────────
  "filmstrip.label": "Album pages",
  "filmstrip.close": "Close the filmstrip",

  // ── Error states ───────────────────────────────────────────────────────
  "error.pageFailed": "This page could not be loaded",

  // ── Scopes ─────────────────────────────────────────────────────────────
  "scope.library": "Library",
  "scope.root": "Root",
  "scope.reading": "Currently reading",
  "scope.recent": "Recently added",

  "login.title": "Sign in",
  "setup.welcome": "Welcome",
  "setup.intro": "This instance is brand new. Create the administrator account to begin.",
  "setup.willBeAdmin": "This account will be an administrator. You can create others afterwards.",
  "setup.created": "Account created",
  "setup.nextSteps":
    "Next: connect a storage backend and create a library. It all happens in the interface, under \u201cStorage\u201d.",
  "setup.fromServer": "From the server",
  "filmstrip.goToPage": "Go to page {page}",
  "mobile.qrAlt": "QR code to {url}",
  "mobile.directApk": "Download the APK directly",
  "mobile.pageExplains":
    "It offers the Android download and repeats this server's address, to enter on first launch.",
  "login.badCredentials": "Wrong username or password.",
  "storage.endpoint": "Endpoint",
  "storage.bucket": "Bucket",
  "account.language": "Language",

  "login.subtitle": "Reach your library.",
  "setup.createAccount": "Create the account",
  "action.continue": "Continue",
  "download.title": "boxincloud on your phone",
  "download.subtitle":
    "Your library, readable offline, with progress synchronised with this server.",
  "download.androidWarning":
    "Android will ask you to allow installation from this source: the app is not distributed through the Play Store.",
  "download.allVersions": "All releases",
  "download.noFile": "The button finds no file?",
  "download.noFileDetail":
    "No signed release has been published yet. A test build may exist. It installs and works, but will not update to the final release: you will have to uninstall it when the time comes.",
  "download.testVersion": "test build",
  "download.iosTitle": "iPhone and iPad",
  "download.iosDetail":
    "The iOS app is not published yet. In the meantime this site works on mobile: open it and add it to your home screen from the share menu.",
  "accounts.pickFromLeft": "Pick an account on the left, or create one.",
  "accounts.saved": "Saved.",
  "detail.pickAlbum": "Select an album to see its details",
  "detail.lockedFields":
    "Edited fields are locked: a new scan will not overwrite them.",
  "manage.destination": "Destination folder",
  "manage.typeWord": "Type",
  "search.hint": "Search the whole library",
  "search.tolerance":
    "Accents and typos are tolerated \u2014 \u201casterics\u201d finds \u201cAst\u00e9rix\u201d.",
  "ingest.formatsShort": "CBZ, CBR, CB7, PDF, EPUB",

  "select.toggle": "Select {title}",
  "select.hint": "Shift-click for a range, {modifier}-click to add",

  "manage.targetLibrary": "Library",
  "manage.sameLibrary": "Keep the current one",
  "manage.crossBackend":
    "Changing library may change storage backend. The bytes then travel through the server, as no copy is possible between two distinct backends \u2014 slower, and noticeably so on a large volume.",
  "manage.moveHint":
    "The file is moved inside your storage. Within one backend the copy happens server-side: the bytes never travel through boxincloud.",

  // ── Server errors ──────────────────────────────────────────────────────
  //
  // One key per RFC 7807 problem type. The server translates nothing: it
  // returns a stable type, and the interface \u2014 the only side that knows the
  // reader's language \u2014 puts it into words.
  "problem.not-found": "This item no longer exists.",
  "problem.bad-request": "The request could not be understood.",
  "problem.validation": "One or more fields are invalid.",
  "problem.unauthorized": "You are no longer signed in.",
  "problem.token-expired": "Your session has expired.",
  "problem.session-revoked": "This session has been revoked.",
  "problem.forbidden": "You do not have permission for this action.",
  "problem.method-not-allowed": "That action is not possible here.",
  "problem.too-many-requests": "Too many attempts. Wait a moment.",
  "problem.service-unavailable": "The server is not responding right now.",
  "problem.internal": "Something unexpected went wrong.",
  "problem.folder-not-empty": "This folder is not empty.",
  "problem.folder-read-only": "This folder is read-only.",
  "problem.backend-in-use": "This storage backend is used by a library.",
  "problem.not-indexed": "This album is not indexed yet.",
  "problem.network": "The server is unreachable.",

  // ── Field errors ───────────────────────────────────────────────────────
  //
  // The server sends the RULE that was broken, not a sentence: \u201ctaken\u201d, not
  // \u201calready taken\u201d. The interface puts it into words. Third-party clients
  // gain too \u2014 a code can be translated, an English sentence can only be
  // endured.
  "field.required": "This field is required.",
  "field.invalid": "This value is not valid.",
  "field.unknown": "This item does not exist.",
  "field.taken": "Already in use.",
  "field.exists": "Something with this name already exists here.",
  "field.format": "Characters not allowed.",
  "field.mismatch": "The file contents do not match its extension.",
  "field.range": "Value outside the allowed range.",
  "field.one-of": "Unrecognised value.",
  "field.self": "This action would target yourself.",
  "field.protected": "This item cannot be changed.",
  "field.no-code": "This folder has no access code.",
  "field.wrong-code": "Wrong access code.",

  "comic.indexing": "Indexing\u2026",
  "comic.hydrating": "Converting\u2026",
  "comic.failed": "Indexing failed",

  "download.notBundled": "This instance does not bundle the app",
  "download.notBundledDetail":
    "The Android app is built alongside the server. This instance was compiled without it \u2014 `make build-apk` produces it, and the official image always bundles it.",
  "download.servedByInstance":
    "The app is served by this server, not by a third party: nothing leaves your network, and its version matches your instance exactly.",
  "download.size": "{size}",

  // ── Federated search ───────────────────────────────────────────────────────
  "discovery.title": "Discover",
  "discovery.dialogLabel": "Federated search",
  "discovery.tab.search": "Search",
  "discovery.tab.sources": "Catalogues",
  "discovery.placeholder": "Search the federated catalogues\u2026",
  "discovery.intro":
    "Queries the OPDS catalogues you have declared, all at once: another boxincloud instance, your Komga or Kavita, a public digital library.",
  "discovery.noSources": "No catalogue declared",
  "discovery.noSourcesHint":
    "Add the address of an OPDS feed you already have access to in order to search it.",
  "discovery.noResults": "No results",
  "discovery.noResultsPartial": "No results \u2014 but not every catalogue answered.",
  "discovery.searching": "Querying the catalogues\u2026",
  "discovery.resultCount": "{count} result(s)",
  "discovery.inLibrary": "Already in your library",
  "discovery.download": "Download",
  "discovery.openPage": "Open page",
  "discovery.from": "From {source}",

  "discovery.status.ok": "{count} result(s) in {ms} ms",
  "discovery.status.unreachable": "Unreachable",
  "discovery.status.timeout": "Too slow, gave up",
  "discovery.status.canceled": "Interrupted",
  "discovery.status.no-search": "Exposes no search",
  "discovery.status.invalid": "Invalid address",
  "discovery.partial": "Some catalogues did not answer: the list is incomplete.",

  "discovery.sources.title": "Federated catalogues",
  "discovery.sources.intro":
    "Addresses declared here are fetched by the server. Administrators only.",
  "discovery.sources.empty": "No catalogue",
  "discovery.sources.add": "Add a catalogue",
  "discovery.sources.name": "Name",
  "discovery.sources.url": "OPDS feed address",
  "discovery.sources.urlHint":
    "For example https://komga.example.com/opds/v2 \u2014 OPDS 1.2 and 2.0 are detected automatically.",
  "discovery.sources.username": "Username",
  "discovery.sources.usernameHint": "Leave empty for a public catalogue.",
  "discovery.sources.password": "Password",
  "discovery.sources.passwordKept": "Leave empty to keep the stored password.",
  "discovery.sources.enabled": "Enabled",
  "discovery.sources.test": "Test",
  "discovery.sources.testOk": "The catalogue answers",
  "discovery.sources.testFailed": "The catalogue did not answer",
  "discovery.sources.lastError": "Last failure",
  "discovery.sources.confirmDelete": "Remove this catalogue?",
  "discovery.sources.checking": "Testing\u2026",

  "discovery.import": "Import",
  "discovery.import.title": "Import into a library",
  "discovery.import.library": "Library",
  "discovery.import.folder": "Folder",
  "discovery.import.folderHint": "Leave empty for the library root.",
  "discovery.import.running": "Downloading\u2026",
  "discovery.import.explain":
    "The server downloads from the catalogue and writes straight into your storage: the file does not pass through this browser.",
  "discovery.import.done": "Imported",
  "discovery.import.noLibrary": "No library to import into",
  "discovery.import.queued": "Queued",
  "discovery.import.failed": "Import failed",
  "discovery.import.background":
    "The import continues in the background: you can close this window.",
  "discovery.import.open": "Open album",

  "discovery.import.err.unreachable": "The catalogue did not answer",
  "discovery.import.err.timeout": "The download timed out",
  "discovery.import.err.foreign-host": "Address foreign to the catalogue",
  "discovery.import.err.invalid": "Address refused",
  "discovery.import.err.source-gone": "The catalogue was removed",
  "discovery.import.err.queue": "The job queue did not accept the import",
  "discovery.import.err.unsupported-format": "Unsupported format",
  "discovery.import.err.content-mismatch": "The content does not match its extension",
  "discovery.import.err.exists": "A file with this name already exists",
  "discovery.import.err.too-large": "File too large",
  "discovery.import.err.deposit-failed": "Writing into the library failed",
};
