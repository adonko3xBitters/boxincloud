/**
 * Catalogue français — la référence.
 *
 * C'est ce fichier qui définit les clés : `en.ts` est typé d'après lui, et une
 * clé traduite nulle part ailleurs devient une erreur de compilation plutôt
 * qu'un mot français au milieu d'une interface anglaise.
 *
 * Les clés sont hiérarchiques par point, mais l'objet reste plat. Une
 * arborescence obligerait à naviguer pour lire une chaîne, et le seul gain
 * serait esthétique.
 *
 * Le français est la langue par défaut parce que c'est celle dans laquelle le
 * projet a été écrit, et qu'une traduction faite après coup est toujours moins
 * juste que l'original.
 */
export const fr = {
  // ── Général ────────────────────────────────────────────────────────────
  "app.tagline": "Votre bibliothèque de BD, comics et mangas",
  "action.cancel": "Annuler",
  "action.close": "Fermer",
  "action.save": "Enregistrer",
  "action.delete": "Supprimer",
  "action.create": "Créer",
  "action.retry": "Réessayer",
  "action.copy": "Copier",
  "action.copied": "Copié",
  "action.confirm": "Confirmer",
  "state.loading": "Chargement…",
  "error.generic": "Une erreur est survenue.",

  // ── Connexion ──────────────────────────────────────────────────────────
  "auth.signIn": "Se connecter",
  "auth.signOut": "Se déconnecter",
  "auth.username": "Identifiant",
  "auth.password": "Mot de passe",
  "auth.displayName": "Nom affiché",
  "auth.email": "Adresse e-mail",
  "auth.role": "Rôle",
  "auth.roleAdmin": "Administrateur",
  "auth.roleUser": "Utilisateur",

  // ── Menu du compte ─────────────────────────────────────────────────────
  "account.label": "Compte",
  "account.mobileApp": "Application mobile",
  "account.devices": "Appareils connectés",
  "account.storage": "Stockage",
  "account.accounts": "Comptes",
  "account.theme": "Changer de thème",

  // ── Appareils ──────────────────────────────────────────────────────────
  "devices.title": "Appareils connectés",
  "devices.empty": "Aucun appareil enregistré.",
  "devices.current": "celui-ci",
  "devices.unnamed": "Appareil sans nom",
  "devices.revoke": "Révoquer",
  "devices.revokeAll": "Tout déconnecter",
  "devices.hint":
    "Révoquer un appareil coupe son accès immédiatement, sans toucher aux autres.",
  "devices.platform.web": "Navigateur",
  "devices.platform.android": "Android",
  "devices.platform.ios": "iOS",
  "devices.platform.desktop": "Ordinateur",
  "devices.seen.now": "à l'instant",
  "devices.seen.unknown": "date inconnue",

  // ── Cache ──────────────────────────────────────────────────────────────
  "cache.tab": "Cache",
  "cache.entries": "Entrées",
  "cache.hits": "Lectures servies",
  "cache.oldest": "Plus ancienne",
  "cache.unbounded": "cache non borné",
  "cache.of": "sur",
  "cache.purge": "Vider le cache",
  "cache.purgeConfirm": "Confirmer la purge",
  "cache.explain":
    "Vignettes, couvertures et pages transcodées. Tout s'y régénère depuis les archives d'origine : vider ce cache ne perd aucune donnée, cela coûte seulement une régénération à la prochaine lecture.",

  // ── Stockage ───────────────────────────────────────────────────────────
  "storage.title": "Stockage",
  "storage.libraries": "Bibliothèques",
  "storage.backends": "Espaces de stockage",

  // ── Application mobile ─────────────────────────────────────────────────
  "mobile.title": "Application mobile",
  "mobile.scanHint": "Scannez avec l'appareil photo du téléphone.",
  "mobile.linkLabel": "La page ouverte par le code",
  "mobile.download": "Télécharger pour Android",
  "mobile.serverAddress": "L'adresse de ce serveur",
  "mobile.serverAddressHint": "À saisir dans l'application, au premier lancement.",
} as const;

/** Les clés du catalogue. Toute traduction doit les couvrir toutes. */
export type MessageKey = keyof typeof fr;
