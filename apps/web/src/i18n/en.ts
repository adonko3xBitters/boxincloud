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
};
