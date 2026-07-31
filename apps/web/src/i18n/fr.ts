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

  // ── Stockage : bibliothèques ───────────────────────────────────────────
  "storage.dialogLabel": "Stockage et bibliothèques",
  "storage.newLibrary": "Nouvelle bibliothèque",
  "storage.noLibraries": "Aucune bibliothèque pour l'instant.",
  "storage.noBackendTitle": "Aucun espace de stockage",
  "storage.noBackendDetail":
    "Une bibliothèque désigne un emplacement dans un espace de stockage. Commencez par en déclarer un dans l'onglet « Espaces de stockage ».",
  "storage.fieldName": "Nom",
  "storage.libraryNamePlaceholder": "Mes BD",
  "storage.backendField": "Espace de stockage",
  "storage.subfolder": "Sous-dossier",
  "storage.subfolderHint":
    "Emplacement dans le stockage. Laissez vide pour prendre tout le contenu.",
  "storage.creating": "Création…",
  "storage.albumOne": "{count} album",
  "storage.albumOther": "{count} albums",
  "storage.lastScan": "dernier parcours {status}",
  "storage.scan": "Analyser",
  "storage.edit": "Modifier",
  "storage.rootPrefix": "Préfixe racine",
  "storage.unchanged": "inchangé",
  "storage.rootPrefixHint":
    "Changer le préfixe ne déplace rien : les albums déjà indexés pointent l'ancien. Le changement dit où chercher désormais, et un nouveau parcours reconstruit le catalogue.",
  "storage.saving": "Enregistrement…",
  "storage.deleteLibrary": "Supprimer cette bibliothèque",
  "storage.deleteLibraryWarning":
    "Albums, dossiers, progression de lecture, favoris, notes et partages disparaissent. Vos fichiers restent intacts — recréer la bibliothèque sur le même préfixe les retrouve tous. L'historique de lecture, lui, ne revient pas.",
  "storage.typeToConfirm": "Tapez {word} pour confirmer.",
  "storage.confirmWord": "supprimer",
  "storage.deleteForever": "Supprimer définitivement",

  "cache.freed": "{entries} entrées supprimées, {bytes} libérés.",

  // ── Stockage : parcours ────────────────────────────────────────────────
  "scan.hide": "Masquer",
  "scan.show": "Voir",
  "scan.lastRuns": "les {count} derniers parcours",
  "scan.counts": "{seen} vus · {added} ajoutés · {updated} modifiés",
  "scan.removed": " · {count} disparus",
  "scan.errors": " · {count} erreurs",
  "scan.status.success": "réussi",
  "scan.status.running": "en cours",
  "scan.status.failed": "en échec",
  "scan.status.cancelled": "interrompu",

  // ── Stockage : espaces ─────────────────────────────────────────────────
  "storage.newBackend": "Nouvel espace de stockage",
  "storage.noBackendsDetail":
    "Un espace de stockage est l'endroit où vivent réellement vos fichiers : un dossier du serveur, ou un bucket S3 / MinIO. boxincloud n'en héberge aucun — il lit le vôtre.",
  "storage.backendNamePlaceholder": "NAS du salon",
  "storage.type": "Type",
  "storage.kindLocal": "Dossier du serveur",
  "storage.kindS3": "S3 / MinIO",
  "storage.folderPath": "Chemin du dossier",
  "storage.folderPathHint": "Chemin tel que le SERVEUR le voit, pas votre poste.",
  "storage.accessKey": "Clé d'accès",
  "storage.secretKey": "Clé secrète",
  "storage.checking": "Vérification du stockage…",
  "storage.declare": "Déclarer",
  "storage.checkedBeforeSaving":
    "Le stockage est joint avant d'être enregistré : un chemin ou des identifiants erronés sont signalés tout de suite, pas au premier scan.",
  "storage.isDefault": "par défaut",
  "storage.readOnly": "lecture seule",
  "storage.localFolder": "Dossier local",
  "storage.test": "Tester",
  "storage.reachable": "Le stockage répond.",
  "storage.unreachable": "Injoignable.",
  "storage.keepSecrets":
    "Laissez vides pour conserver les identifiants actuels : ils ne ressortent jamais de la base, pas même pour un administrateur.",
  "storage.setDefault": "Définir par défaut",
  "storage.verifying": "Vérification…",
  "storage.backendFooter":
    "Le stockage est joint avant d'être enregistré. Sa suppression est refusée tant qu'une bibliothèque s'y appuie — vos fichiers ne sont jamais touchés.",

  // ── Dépôt de contenu ───────────────────────────────────────────────────
  "ingest.title": "Ajouter du contenu",
  "ingest.dropHere": "Glissez vos albums ici, ou cliquez pour choisir",
  "ingest.formats": "CBZ, CBR, PDF — et leurs équivalents ZIP et RAR",
  "ingest.library": "Bibliothèque",
  "ingest.folder": "Dossier",
  "ingest.optional": "(optionnel)",
  "ingest.folderPlaceholder": "Tintin",
  "ingest.waiting": "{count} en attente",
  "ingest.sending": "Envoi…",
  "ingest.send": "Envoyer",
  "ingest.sendWithCount": "Envoyer ({count})",
  "ingest.fileOne": "{count} fichier",
  "ingest.fileOther": "{count} fichiers",
  "ingest.clearDoneOne": "retirer le {count} envoyé",
  "ingest.clearDoneOther": "retirer les {count} envoyés",
  "ingest.markSent": "Envoyé",
  "ingest.markFailed": "Échec",
  "ingest.markSending": "En cours",
  "ingest.markPending": "En attente",
  "ingest.add": "Ajouter",
  "ingest.dropToAdd": "Déposez pour ajouter",

  // ── Première bibliothèque ──────────────────────────────────────────────
  "first.title": "Première bibliothèque",
  "first.defaultName": "Ma bibliothèque",
  "first.storageType": "Type de stockage",
  "first.subfolder": "Sous-dossier",
  "first.create": "Créer la bibliothèque",

  // ── Parcours du stockage ───────────────────────────────────────────────
  "scan.button": "Analyser le stockage",
  "scan.queued": "Parcours lancé",
  "scan.failed": "Échec",
  "scan.tooltip": "Rechercher les fichiers ajoutés en dehors de boxincloud",
  "first.intro":
    "Indiquez où vos albums sont stockés. Le stockage est vérifié avant d'être enregistré — un chemin ou des identifiants erronés sont signalés tout de suite, pas au premier scan.",
  "first.localPath": "Chemin du dossier, sur le serveur",

  // ── Barre latérale ─────────────────────────────────────────────────────
  "sidebar.libraries": "Bibliothèques",
  "sidebar.allAlbums": "Tous les albums",
  "sidebar.folders": "Dossiers",
  "sidebar.newRootFolder": "Nouveau dossier à la racine",
  "sidebar.newFolder": "Nouveau dossier",
  "sidebar.series": "Séries",
  "sidebar.lists": "Listes de lecture",
  "sidebar.favorites": "Favoris",
  "sidebar.reading": "En cours",
  "sidebar.recent": "Récents",
  "sidebar.noFolders": "Aucun dossier",
  "sidebar.expand": "Déplier",
  "sidebar.collapse": "Replier",
  "sidebar.readOnly": "Lecture seule",
  "sidebar.unlockedForNow": "Déverrouillé pour le moment",
  "sidebar.actionsOn": "Actions sur {name}",
  "sidebar.root": "la racine",

  // ── Menu d'un dossier ──────────────────────────────────────────────────
  "folder.open": "Ouvrir",
  "folder.newChild": "Nouveau sous-dossier…",
  "folder.rename": "Renommer…",
  "folder.lock": "Verrouiller…",
  "folder.share": "Partager…",
  "folder.delete": "Supprimer…",

  // ── Dossiers : dialogues ───────────────────────────────────────────────
  "folderDialog.newTitle": "Nouveau dossier",
  "folderDialog.namePlaceholder": "Tintin",
  "folderDialog.createdInRoot": "Sera créé à la racine de la bibliothèque.",
  "folderDialog.createdIn": "Sera créé dans",
  "folderDialog.noWriteYet":
    "Rien n'est écrit dans votre stockage : un magasin d'objets n'a pas de répertoires. Le dossier prendra corps au premier album déposé.",
  "folderDialog.renameTitle": "Renommer le dossier",
  "folderDialog.renaming": "Renommage…",
  "folderDialog.rename": "Renommer",
  "folderDialog.renameWarning":
    "Chaque album du dossier est renommé dans votre stockage. Sur un backend distant, une branche volumineuse peut demander plusieurs minutes.",
  "folderDialog.deleteTitle": "Supprimer le dossier",
  "folderDialog.emptyFolder": "Ce dossier est vide. Sa suppression ne touche à rien d'autre.",
  "folderDialog.containsOne": "Ce dossier et ses sous-dossiers contiennent {count} album.",
  "folderDialog.containsOther": "Ce dossier et ses sous-dossiers contiennent {count} albums.",
  "folderDialog.deleteFiles": "Supprimer aussi les fichiers",
  "folderDialog.deleteFilesHint":
    "Sans cette option, les albums sont retirés du catalogue et les fichiers restent dans votre stockage. Avec, ils sont effacés — irréversible.",
  "folderDialog.deleting": "Suppression…",

  // ── Dossiers : verrouillage ────────────────────────────────────────────
  "lock.title": "Verrouiller le dossier",
  "lock.readOnly": "Lecture seule",
  "lock.readOnlyHint":
    "Le dossier reste visible de tous, mais ne peut plus être renommé, déplacé, ni recevoir ou perdre un album. La protection s'étend aux sous-dossiers.",
  "lock.accessCode": "Code d'accès",
  "lock.accessCodeHint":
    "Masque le dossier et son contenu — listes, recherche, accès direct — tant que le code n'a pas été saisi. Utile sur un serveur partagé.",
  "lock.removeCode": "Retirer le code existant",
  "lock.newCode": "Nouveau code",
  "lock.code": "Code",
  "lock.keepCode": "Laisser vide pour ne pas changer",
  "lock.minLength": "Quatre caractères minimum",
  "unlock.title": "Dossier verrouillé",
  "unlock.duration": "Le dossier reste ouvert deux heures, puis se referme de lui-même.",
  "unlock.open": "Ouvrir",

  // ── Dossiers : partage ─────────────────────────────────────────────────
  "share.title": "Partager le dossier",
  "share.accounts": "Comptes du serveur",
  "share.accountsHint":
    "Un dossier sans accès explicite est visible de tous ceux qui voient la bibliothèque. En accorder un ici le referme pour tous les autres.",
  "share.with": "Partager avec {name}",
  "share.write": "écriture",
  "share.noOtherAccount": "Aucun autre compte.",
  "share.publicLink": "Lien public",
  "share.blockedByCode":
    "Indisponible : ce dossier est masqué par un code d'accès. Publier ce qu'on vient de cacher annulerait le code sans le dire.",
  "share.publicWarning":
    "Un lien public ouvre ce dossier sans aucun compte : qui a l'adresse voit le contenu, et peut la transmettre.",
  "share.unnamed": "Sans nom",
  "share.expiresOn": "expire le {date}",
  "share.openedOne": "{count} ouverture",
  "share.openedOther": "{count} ouvertures",
  "share.revoke": "Révoquer",
  "share.copyNow": "Copiez ce lien maintenant : il ne sera plus affiché.",
  "share.labelPlaceholder": "Pour Camille",
  "share.expiresIn": "Expire dans",
  "share.oneDay": "1 jour",
  "share.sevenDays": "7 jours",
  "share.thirtyDays": "30 jours",
  "share.oneYear": "1 an",
  "share.createLink": "Créer un lien",
} as const;

/** Les clés du catalogue. Toute traduction doit les couvrir toutes. */
export type MessageKey = keyof typeof fr;
