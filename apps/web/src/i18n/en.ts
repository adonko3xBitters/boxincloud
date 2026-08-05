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
  "field.unreachable": "Unreachable with these settings.",
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

  // ── Metadata matching ──────────────────────────────────────────────────────
  "metadata.find": "Find a record",
  "metadata.searching": "Querying the databases\u2026",
  "metadata.none": "No record found",
  "metadata.noneHint":
    "These databases cover Franco-Belgian comics poorly. Try the series title rather than the volume title.",
  "metadata.offline": "No metadata database is enabled on this instance.",
  "metadata.apply": "Use this record",
  "metadata.applied": "Fields filled in \u2014 review before saving.",
  "metadata.confidence": "{percent}% match",
  "metadata.from": "From {source}",
  "metadata.openSource": "Open the original record",
  "metadata.partial": "Some databases did not answer: the list is incomplete.",
  "metadata.explain":
    "Suggestions come from public databases. Nothing is saved until you confirm.",

  // ── Settings ───────────────────────────────────────────────────────────────
  "settings.title": "Settings",
  "settings.open": "Settings",
  "settings.back": "Back",
  "settings.storage.hint": "Storage backends, libraries and cache",
  "settings.accounts.hint": "Instance accounts and their permissions",
  "settings.sources.hint": "OPDS catalogues queried by federated search",
  "settings.devices.hint": "Open sessions, and how to revoke them",
  "settings.mobile.hint": "Install the Android app served by this instance",
  "settings.discover": "Discover",
  "settings.discoverHint": "Search the federated catalogues",

  // ── Federated search ───────────────────────────────────────────────────────
  "discovery.soon": "Coming soon",
  "discovery.soonHint":
    "Searching external catalogues is not available yet. The shortcut stays in place: this is where it will open when it lands.",
  "discovery.title": "Discover",
  "discovery.dialogLabel": "Federated search",

  // ── eD2k / Kad module ──────────────────────────────────────────────────────
  "ed2k.title": "eD2k / Kad",
  "ed2k.menuHint": "Drive an aMule daemon from this instance",
  "ed2k.intro":
    "This module drives an aMule daemon over its External Connections protocol. boxincloud reimplements no peer-to-peer protocol: aMule stays the engine, the interface lives here.",
  "ed2k.adminOnly": "Administrators only",
  "ed2k.adminOnlyHint":
    "This module commits the instance's bandwidth, ports and IP address. Driving it requires an administrator account.",

  "ed2k.state.disabled": "Disabled",
  "ed2k.state.unconfigured": "No daemon declared",
  "ed2k.state.disconnected": "Daemon declared",
  "ed2k.state.connecting": "Connecting…",
  "ed2k.state.connected": "Connected",

  "ed2k.disabledTitle": "Module disabled on this instance",
  "ed2k.disabledHint":
    "Nothing is running and no port is open. To turn it on, set BOXINCLOUD_ED2K_ENABLED to true in the server configuration, then restart it.",

  "ed2k.daemon.title": "aMule daemon",
  "ed2k.daemon.hint":
    "The address must be reachable from the SERVER, not from this browser — “amuled” on an internal network, or the address of the NAS already running aMule.",
  "ed2k.daemon.host": "Host",
  "ed2k.daemon.port": "Port",
  "ed2k.daemon.portHint": "The daemon's External Connections port. 4712 on aMule.",
  "ed2k.daemon.password": "EC password",
  "ed2k.daemon.passwordHint":
    "Encrypted at rest and never shown again: it must be typed in on every save.",
  "ed2k.daemon.label": "Name",
  "ed2k.daemon.labelHint": "Optional, to tell instances apart in the logs.",
  "ed2k.daemon.save": "Save",
  "ed2k.daemon.forget": "Forget this daemon",
  "ed2k.daemon.forgetConfirm":
    "Forget this daemon? It keeps running and no download is touched: only the boxincloud-side declaration goes away.",
  "ed2k.daemon.lastSeen": "Last contact",
  "ed2k.daemon.never": "never",
  "ed2k.daemon.declared": "Daemon declared: {host}",

  "ed2k.incoming": "Incoming directory",
  "ed2k.incomingHint":
    "As the server sees it, read-only. A missing mount here is the first cause of “the download finished but nothing arrived”.",

  "ed2k.next.title": "What is not here yet",
  "ed2k.next.hint":
    "This step reads and does not act: no button pauses, resumes, connects or removes anything. Commands, search and eD2k link submission land in the next step.",

  "ed2k.nav.label": "Module sections",
  "ed2k.section.dashboard": "Dashboard",
  "ed2k.section.downloads": "Downloads",
  "ed2k.section.uploads": "Uploads",
  "ed2k.section.shared": "Shared",
  "ed2k.section.servers": "Servers",
  "ed2k.section.kad": "Kad",
  "ed2k.section.stats": "Statistics",
  "ed2k.section.settings": "Settings",

  "ed2k.needsDaemon": "No daemon declared",
  "ed2k.needsDaemonHint":
    "There is nothing to read until an aMule daemon is declared. Fill in its address and External Connections password under Settings.",
  "ed2k.openSettings": "Open settings",
  "ed2k.takenAt": "Read at {time}",
  "ed2k.readOnly": "Read-only",
  "ed2k.yes": "yes",
  "ed2k.no": "no",

  "ed2k.unit.byte": "B",
  "ed2k.unit.kilo": "kB",
  "ed2k.unit.mega": "MB",
  "ed2k.unit.giga": "GB",
  "ed2k.unit.tera": "TB",
  "ed2k.unit.perSecond": "{value}/s",
  "ed2k.unit.second": "{value} s",
  "ed2k.unit.minute": "{value} min",
  "ed2k.unit.hour": "{value} h",
  "ed2k.unit.day": "{value} d",

  "ed2k.col.name": "Name",
  "ed2k.col.size": "Size",
  "ed2k.col.progress": "Progress",
  "ed2k.col.received": "Done",
  "ed2k.col.speed": "Speed",
  "ed2k.col.eta": "Left",
  "ed2k.col.sources": "Sources",
  "ed2k.col.parts": "Parts",
  "ed2k.col.priority": "Priority",
  "ed2k.col.status": "Status",
  "ed2k.col.peer": "Peer",
  "ed2k.col.software": "Client",
  "ed2k.col.address": "Address",
  "ed2k.col.rank": "Rank",
  "ed2k.col.available": "Available",
  "ed2k.col.file": "File",
  "ed2k.col.sent": "Sent",
  "ed2k.col.session": "Session",
  "ed2k.col.total": "Total",
  "ed2k.col.score": "Score",
  "ed2k.col.waiting": "Waiting since",
  "ed2k.col.path": "Path",
  "ed2k.col.requests": "Requests",
  "ed2k.col.accepted": "Served",
  "ed2k.col.complete": "Complete",
  "ed2k.col.ping": "Ping",
  "ed2k.col.users": "Users",
  "ed2k.col.files": "Files",
  "ed2k.col.failed": "Failures",
  "ed2k.col.static": "Pinned",
  "ed2k.col.description": "Description",

  "ed2k.status.waiting": "Waiting",
  "ed2k.status.downloading": "Downloading",
  "ed2k.status.paused": "Paused",
  "ed2k.status.stopped": "Stopped",
  "ed2k.status.erroneous": "Failed",
  "ed2k.status.completing": "Completing",
  "ed2k.status.completed": "Completed",
  "ed2k.status.hashing": "Hashing",
  "ed2k.status.allocating": "Allocating",
  "ed2k.status.unknown": "Unknown state",

  "ed2k.priority.verylow": "Very low",
  "ed2k.priority.low": "Low",
  "ed2k.priority.normal": "Normal",
  "ed2k.priority.high": "High",
  "ed2k.priority.veryhigh": "Very high",
  "ed2k.priority.auto": "Auto",

  "ed2k.id.high": "HighID",
  "ed2k.id.low": "LowID",
  "ed2k.id.none": "No ID",
  "ed2k.id.lowHint":
    "Incoming connections do not get through: sources go via a server, are fewer and drop more often. Open the daemon's TCP port to fix it.",

  "ed2k.dashboard.serverNetwork": "Server network",
  "ed2k.dashboard.kadNetwork": "Kad network",
  "ed2k.dashboard.noServer": "No server",
  "ed2k.dashboard.connecting": "Connecting",
  "ed2k.dashboard.notConnected": "Not connected",
  "ed2k.dashboard.activeDownloads": "Active downloads",
  "ed2k.dashboard.queueLength": "Peers waiting",
  "ed2k.dashboard.sharedCount": "Shared files",
  "ed2k.dashboard.knownSources": "Known sources",
  "ed2k.dashboard.transfers": "What is moving",
  "ed2k.dashboard.transfersEmpty": "Nothing is moving",
  "ed2k.dashboard.transfersEmptyHint":
    "No file is receiving or sending data right now. The queue may be empty, or every source may be queued.",
  "ed2k.dashboard.seeAll": "See the whole queue",

  "ed2k.downloads.hint":
    "What the daemon is receiving. A finished file stays here until it is assembled and moved to the incoming directory.",
  "ed2k.downloads.empty": "Empty queue",
  "ed2k.downloads.emptyHint":
    "The daemon is downloading nothing. A file added from aMule, or through an eD2k link, will show up in this list.",
  "ed2k.downloads.expand": "Show the sources of {name}",
  "ed2k.downloads.sourcesEmpty": "No source",
  "ed2k.downloads.sourcesEmptyHint":
    "None of the peers reached holds this file. A server or Kad has to find one first.",
  "ed2k.downloads.wasted":
    "{value} received in excess: corrupted data was rejected on verification.",

  "ed2k.uploads.hint":
    "What the daemon is sending, and who is waiting. What you give decides what you get: eD2k credits reward the peers you have already served.",
  "ed2k.uploads.active": "Outgoing transfers",
  "ed2k.uploads.queue": "Waiting queue",
  "ed2k.uploads.empty": "No upload",
  "ed2k.uploads.emptyHint":
    "Nobody is downloading from this instance right now, and nobody is waiting. That is expected if nothing is shared, or if the daemon just started.",
  "ed2k.uploads.queueEmpty": "Empty waiting queue",

  "ed2k.shared.hint":
    "What this instance offers, and what each file actually returned. A file still downloading is shared too — that is how the network works.",
  "ed2k.shared.empty": "Nothing is shared",
  "ed2k.shared.emptyHint":
    "The daemon publishes no file. Check its shared directories in its own configuration: boxincloud reads them, it does not set them.",
  "ed2k.shared.partial": "partial",

  "ed2k.servers.hint":
    "Known servers, not just the one in use. A dead server only differs from a never-tried one by its failure counter.",
  "ed2k.servers.empty": "No known server",
  "ed2k.servers.emptyHint":
    "The daemon's list is empty. It fills from a server.met file, loaded by aMule at startup or when its list is updated.",
  "ed2k.servers.connected": "Connected",

  "ed2k.kad.hint":
    "Kad needs no server: peers find each other. That is what keeps things running when every server goes down.",
  "ed2k.kad.running": "Engine running",
  "ed2k.kad.connected": "Network found",
  "ed2k.kad.firewalled": "Incoming connections blocked",
  "ed2k.kad.firewalledUdp": "UDP blocked",
  "ed2k.kad.firewalledHint":
    "You can search, but you are harder to find: other peers cannot reach this host.",
  "ed2k.kad.udpHint":
    "Kad searches over UDP and transfers over TCP. Both are blocked separately, and a closed UDP port stops every search.",
  "ed2k.kad.searchingHint":
    "The engine is running but has not found its peers yet. That takes several minutes on a first start.",
  "ed2k.kad.stopped": "Kad is stopped",
  "ed2k.kad.stoppedHint":
    "The Kademlia engine is not running on this daemon. It is started from aMule's own configuration; boxincloud only reads its state at this step.",
  "ed2k.kad.users": "Estimated users",
  "ed2k.kad.files": "Estimated files",

  "ed2k.stats.hint":
    "The daemon's counters, as it reports them. Network sizes are estimates it computes itself, not measurements.",
  "ed2k.stats.transfer": "Throughput",
  "ed2k.stats.network": "Networks",
  "ed2k.stats.peers": "Peers",
  "ed2k.stats.downSpeed": "Download",
  "ed2k.stats.upSpeed": "Upload",
  "ed2k.stats.downLimit": "Download cap",
  "ed2k.stats.upLimit": "Upload cap",
  "ed2k.stats.downOverhead": "Protocol in",
  "ed2k.stats.upOverhead": "Protocol out",
  "ed2k.stats.overheadHint":
    "What the protocol itself consumes, beyond file data. It is the gap between the cap you set and what the router reports.",
  "ed2k.stats.unlimited": "no cap",
  "ed2k.stats.totalSources": "Known sources",
  "ed2k.stats.bannedPeers": "Banned peers",
  "ed2k.stats.uploadQueue": "Peers waiting",
  "ed2k.stats.ed2kUsers": "eD2k users",
  "ed2k.stats.kadUsers": "Kad users",
  "ed2k.stats.ed2kFiles": "eD2k files",
  "ed2k.stats.kadFiles": "Kad files",
};
