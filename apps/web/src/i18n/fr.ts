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
  "ingest.formats": "CBZ, CBR, CB7, PDF, EPUB — et leurs équivalents ZIP, RAR et 7z",
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

  // ── Comptes ────────────────────────────────────────────────────────────
  "accounts.title": "Comptes",
  "accounts.new": "Nouveau compte",
  "accounts.pickOne": "Sélectionnez un compte",
  "accounts.passwordHint":
    "Douze caractères minimum. La longueur protège mieux qu'une exigence de majuscules et de chiffres, qui pousse surtout à des variantes prévisibles du même mot.",
  "accounts.create": "Créer le compte",
  "accounts.selfRoleHint":
    "Vous ne pouvez pas modifier votre propre rôle : une erreur de ligne vous coûterait l'accès à l'administration.",
  "accounts.restricted": "Profil restreint",
  "accounts.restrictedHint": "Masque les albums dont la classification dépasse la limite.",
  "accounts.maxRating": "Classification maximale",
  "accounts.years": "ans",
  "accounts.newPassword": "Nouveau mot de passe",
  "accounts.newPasswordHint":
    "Laissez vide pour ne pas le changer. Les sessions ouvertes ne sont pas fermées : c'est une action distincte, pour ne pas déconnecter partout quelqu'un qui a simplement oublié son mot de passe.",
  "accounts.disabling": "Désactivation…",
  "accounts.disable": "Désactiver ce compte",
  "accounts.cannotDisableSelf": "Vous ne pouvez pas désactiver votre propre compte.",
  "accounts.disableHint":
    "La progression de lecture, les favoris et les notes sont conservés. Les sessions ouvertes sont fermées.",
  "accounts.libraryAccess": "Accès aux bibliothèques",
  "accounts.libraryAccessHint":
    "Une bibliothèque sans aucun accès explicite est visible de tous. En accorder un ici la referme pour tous les autres comptes.",
  "accounts.accessTo": "Accès à {name}",
  "accounts.noLibrary": "Aucune bibliothèque.",
  "accounts.roleReader": "Lecteur",
  "accounts.suffixRestricted": " · restreint",

  // ── Barre d'outils ─────────────────────────────────────────────────────
  "toolbar.read": "Lire",
  "toolbar.markRead": "Marquer lu",
  "toolbar.markUnread": "Marquer non lu",
  "toolbar.unfavorite": "Retirer des favoris",
  "toolbar.favorite": "Ajouter aux favoris",
  "toolbar.moveToFolder": "Ranger dans un dossier",
  "toolbar.removeFromLibrary": "Retirer de la bibliothèque",
  "toolbar.readStatus": "Lecture",
  "toolbar.all": "Tous",
  "toolbar.unread": "Non lus",
  "toolbar.inProgress": "En cours",
  "toolbar.done": "Lus",
  "toolbar.sort": "Tri",
  "toolbar.sortAdded": "Ajout",
  "toolbar.sortTitle": "Titre",
  "toolbar.sortReleased": "Parution",
  "toolbar.viewGrid": "Grille",
  "toolbar.viewList": "Liste",
  "toolbar.viewCoverflow": "Carrousel",
  "toolbar.selectedOne": "{count} sélectionné",
  "toolbar.selectedOther": "{count} sélectionnés",
  "toolbar.clearSelection": "annuler",
  "toolbar.selectAll": "tout sélectionner",

  // ── Fiche d'album ──────────────────────────────────────────────────────
  "detail.resume": "Reprendre p. {page}",
  "detail.editMetadata": "Modifier les métadonnées",
  "detail.pageOf": "Page {page} / {total}",
  "detail.rate": "Noter {step} sur 5",
  "detail.rating": "Note",
  "detail.number": "Numéro",
  "detail.pages": "Pages",
  "detail.format": "Format",
  "detail.size": "Taille",
  "detail.released": "Parution",
  "detail.language": "Langue",
  "detail.file": "Fichier",
  "detail.title": "Titre",
  "detail.summary": "Résumé",

  // ── Retirer / ranger ───────────────────────────────────────────────────
  "manage.removeOne": "Retirer l'album",
  "manage.removeMany": "Retirer {count} albums",
  "manage.fromCatalog": "Retirer du catalogue",
  "manage.fromCatalogHint":
    "Le fichier reste dans votre stockage. La progression de lecture, les favoris et les notes sont conservés. Un nouveau parcours ne le fera pas réapparaître.",
  "manage.deleteFile": "Supprimer aussi le fichier",
  "manage.deleteFileHint":
    "Le fichier est effacé du stockage. Irréversible : boxincloud ne conserve aucune copie.",
  "manage.working": "En cours…",
  "manage.remove": "Retirer",
  "manage.moveOne": "Ranger l'album",
  "manage.moveMany": "Ranger {count} albums",
  "manage.rootPlaceholder": "Laisser vide pour la racine",
  "manage.moving": "Déplacement…",
  "manage.move": "Ranger",

  // ── Recherche ──────────────────────────────────────────────────────────
  "search.action": "Rechercher",
  "search.shortcut": "Rechercher (/)",
  "search.center": "Centre de recherche",
  "search.placeholder": "Titre, série, numéro…",
  "search.series": "Séries",
  "search.albums": "Albums",

  // ── Tableau ────────────────────────────────────────────────────────────
  "table.title": "Titre",
  "table.series": "Série",
  "table.number": "N°",
  "table.progress": "Page",
  "table.pages": "Feuilles",
  "table.size": "Taille",
  "table.released": "Parution",
  "table.read": "Lu",
  "table.rating": "Note",
  "table.selectAll": "Tout sélectionner",
  "table.select": "Sélectionner {title}",

  // ── Carrousel ──────────────────────────────────────────────────────────
  "coverflow.label": "Carrousel de couvertures",
  "coverflow.previous": "Couverture précédente",
  "coverflow.next": "Couverture suivante",

  // ── Menu d'un album ────────────────────────────────────────────────────
  "comicMenu.readNamed": "Lire « {title} »",
  "comicMenu.markRead": "Marquer comme lu",
  "comicMenu.markReadCount": "Marquer comme lu ({count})",
  "comicMenu.markUnread": "Marquer comme non lu",
  "comicMenu.move": "Ranger dans un dossier…",
  "comicMenu.moveCount": "Ranger dans un dossier… ({count})",
  "comicMenu.remove": "Retirer de la bibliothèque…",
  "comicMenu.removeCount": "Retirer de la bibliothèque… ({count})",

  // ── Lecteur ────────────────────────────────────────────────────────────
  "reader.hideChrome": "Masquer l'interface",
  "reader.showChrome": "Afficher l'interface",
  "reader.close": "Fermer le lecteur",
  "reader.settings": "Réglages de lecture",
  "reader.position": "Position dans l'album",
  "reader.thumbnails": "Pages de l'album (t)",
  "reader.mode": "Mode",
  "reader.modeSingle": "Page simple",
  "reader.modeDouble": "Double page",
  "reader.modeScroll": "Défilement",
  "reader.fit": "Ajustement",
  "reader.fitWidth": "Largeur",
  "reader.fitHeight": "Hauteur",
  "reader.fitPage": "Page entière",
  "reader.direction": "Sens de lecture",
  "reader.ltr": "Gauche → droite",
  "reader.rtl": "Droite → gauche",
  "reader.shortcuts":
    "Flèches ou espace pour tourner · Début et Fin pour les extrémités · t pour les vignettes · + − 0 pour le zoom · double-clic ou pincement pour agrandir · Échap pour sortir",
  "reader.previousPage": "Page précédente",
  "reader.nextPage": "Page suivante",

  // ── Espace de travail ──────────────────────────────────────────────────
  "workspace.emptyTitle": "Aucun album ici",
  "workspace.emptyDetail": "Changez de dossier ou élargissez les filtres.",
  "workspace.loadMore": "Charger la suite",
  "workspace.readNamed": "Lire {title}",

  // ── Connexion et installation ──────────────────────────────────────────
  "login.failed": "Connexion impossible. Le serveur est-il joignable ?",
  "setup.mismatch": "Les mots de passe ne correspondent pas.",
  "setup.unreachable": "Le serveur est-il joignable ?",
  "setup.usernameHint": "Lettres, chiffres, tiret, point et souligné.",
  "setup.emailOptional": "Adresse e-mail (facultative)",
  "setup.confirm": "Confirmation",

  // ── Lien partagé ───────────────────────────────────────────────────────
  "shared.invalid": "Ce lien n'est pas valide",
  "shared.expiredOrRevoked": "Il a peut-être expiré, ou été révoqué.",
  "shared.expiredOrRevokedBy":
    "Il a peut-être expiré, ou été révoqué par la personne qui vous l'a transmis.",
  "shared.albums": "Albums partagés",
  "shared.album": "Album partagé",
  "shared.nothingTitle": "Rien à voir",
  "shared.nothingDetail": "Ce partage ne contient aucun album.",

  // ── Bande de vignettes ─────────────────────────────────────────────────
  "filmstrip.label": "Pages de l'album",
  "filmstrip.close": "Fermer la bande de vignettes",

  // ── États d'erreur ─────────────────────────────────────────────────────
  "error.pageFailed": "Impossible de charger cette page",

  // ── Portées ────────────────────────────────────────────────────────────
  "scope.library": "Bibliothèque",
  "scope.root": "Racine",
  "scope.reading": "En cours de lecture",
  "scope.recent": "Récemment ajouté",

  "login.title": "Connexion",
  "setup.welcome": "Bienvenue",
  "setup.intro": "Cette instance est neuve. Créez le compte administrateur pour commencer.",
  "setup.willBeAdmin": "Ce compte sera administrateur. Vous pourrez en créer d'autres ensuite.",
  "setup.created": "Compte créé",
  "setup.nextSteps":
    "Il reste à connecter un espace de stockage et à créer une bibliothèque. Tout se fait depuis l'interface, sous « Stockage ».",
  "setup.fromServer": "Depuis le serveur",
  "filmstrip.goToPage": "Aller à la page {page}",
  "mobile.qrAlt": "Code QR vers {url}",
  "mobile.directApk": "Télécharger l'APK directement",
  "mobile.pageExplains":
    "Elle propose le téléchargement pour Android et rappelle l'adresse de ce serveur, à saisir à la première connexion.",
  "login.badCredentials": "Nom d'utilisateur ou mot de passe incorrect.",
  "storage.endpoint": "Endpoint",
  "storage.bucket": "Bucket",
  "account.language": "Langue",

  "login.subtitle": "Accédez à votre bibliothèque.",
  "setup.createAccount": "Créer le compte",
  "action.continue": "Continuer",
  "download.title": "boxincloud sur votre téléphone",
  "download.subtitle":
    "Votre bibliothèque, lisible hors ligne, avec la progression synchronisée avec ce serveur.",
  "download.androidWarning":
    "Android demandera d'autoriser l'installation depuis cette source : l'application n'est pas distribuée par le Play Store.",
  "download.allVersions": "Toutes les versions",
  "download.noFile": "Le bouton ne trouve pas de fichier ?",
  "download.noFileDetail":
    "Aucune version signée n'a encore été publiée. Une version de test existe peut-être. Elle s'installe et fonctionne, mais ne se mettra pas à jour vers la version définitive : il faudra la désinstaller le moment venu.",
  "download.testVersion": "version de test",
  "download.iosTitle": "iPhone et iPad",
  "download.iosDetail":
    "L'application iOS n'est pas encore publiée. En attendant, ce site fonctionne sur mobile : ouvrez-le et ajoutez-le à votre écran d'accueil depuis le menu de partage.",
  "accounts.pickFromLeft": "Choisissez un compte à gauche, ou créez-en un.",
  "accounts.saved": "Enregistré.",
  "detail.pickAlbum": "Sélectionnez un album pour voir son détail",
  "detail.lockedFields":
    "Les champs modifiés sont verrouillés : un nouveau scan ne les écrasera pas.",
  "manage.destination": "Dossier de destination",
  "manage.typeWord": "Tapez",
  "search.hint": "Cherchez dans toute la bibliothèque",
  "search.tolerance":
    "Les accents et les fautes de frappe sont tolérés — « asterics » trouve « Astérix ».",
  "ingest.formatsShort": "CBZ, CBR, CB7, PDF, EPUB",

  "select.toggle": "Sélectionner {title}",
  "select.hint": "Maj+clic pour une plage, {modifier}+clic pour ajouter",

  "manage.targetLibrary": "Bibliothèque",
  "manage.sameLibrary": "Ne pas changer",
  "manage.crossBackend":
    "Changer de bibliothèque peut changer d'espace de stockage. Les octets transitent alors par le serveur, faute de copie possible entre deux backends distincts — c'est plus lent, et cela se voit sur une intégrale.",
  "manage.moveHint":
    "Le fichier est déplacé dans votre stockage. Au sein d'un même espace de stockage, la copie se fait côté serveur : les octets ne transitent pas par boxincloud.",

  // ── Erreurs du serveur ─────────────────────────────────────────────────
  //
  // Une clé par type de problème RFC 7807. Le serveur ne traduit rien : il
  // renvoie un type stable, et c'est l'interface — qui seule connaît la langue
  // du lecteur — qui le met en mots.
  "problem.not-found": "Cet élément n'existe plus.",
  "problem.bad-request": "La demande n'a pas pu être comprise.",
  "problem.validation": "Un ou plusieurs champs sont invalides.",
  "problem.unauthorized": "Vous n'êtes plus connecté.",
  "problem.token-expired": "Votre session a expiré.",
  "problem.session-revoked": "Cette session a été révoquée.",
  "problem.forbidden": "Vous n'avez pas les droits pour cette action.",
  "problem.method-not-allowed": "Cette action n'est pas possible ici.",
  "problem.too-many-requests": "Trop de tentatives. Patientez un instant.",
  "problem.service-unavailable": "Le serveur ne répond pas pour le moment.",
  "problem.internal": "Une erreur inattendue est survenue.",
  "problem.folder-not-empty": "Ce dossier n'est pas vide.",
  "problem.folder-read-only": "Ce dossier est en lecture seule.",
  "problem.backend-in-use": "Cet espace de stockage est utilisé par une bibliothèque.",
  "problem.not-indexed": "Cet album n'est pas encore indexé.",
  "problem.network": "Le serveur est injoignable.",

  // ── Erreurs par champ ──────────────────────────────────────────────────
  //
  // Le serveur envoie la RÈGLE enfreinte, pas une phrase : « taken », pas
  // « already taken ». C'est l'interface qui la met en mots. Un client tiers y
  // gagne aussi — un code se traduit, une phrase anglaise se subit.
  "field.required": "Ce champ est obligatoire.",
  "field.invalid": "Cette valeur n'est pas valide.",
  "field.unknown": "Cet élément n'existe pas.",
  "field.taken": "Déjà utilisé.",
  "field.exists": "Un élément portant ce nom existe déjà ici.",
  "field.format": "Caractères non autorisés.",
  "field.mismatch": "Le contenu du fichier ne correspond pas à son extension.",
  "field.range": "Valeur hors des limites permises.",
  "field.one-of": "Valeur non reconnue.",
  "field.self": "Cette action vous viserait vous-même.",
  "field.protected": "Cet élément ne peut pas être modifié.",
  "field.no-code": "Ce dossier n'a pas de code d'accès.",
  "field.wrong-code": "Code d'accès incorrect.",

  "comic.indexing": "Indexation…",
  "comic.hydrating": "Conversion…",
  "comic.failed": "Échec de l'indexation",

  "download.notBundled": "Cette instance n'embarque pas l'application",
  "download.notBundledDetail":
    "L'application Android est construite avec le serveur. Cette instance a été compilée sans elle — `make build-apk` la produit, et l'image officielle l'embarque toujours.",
  "download.servedByInstance":
    "L'application est servie par ce serveur, pas par un tiers : rien ne sort de votre réseau, et sa version correspond exactement à celle de votre instance.",
  "download.size": "{size}",

  // ── Rapprochement de métadonnées ───────────────────────────────────────────
  "metadata.find": "Chercher une fiche",
  "metadata.searching": "Interrogation des bases\u2026",
  "metadata.none": "Aucune fiche trouvée",
  "metadata.noneHint":
    "Ces bases couvrent mal la bande dessinée franco-belge. Essayez le titre de la série plut\u00f4t que celui du tome.",
  "metadata.offline":
    "Aucune base de métadonnées n'est activée sur cette instance.",
  "metadata.apply": "Utiliser cette fiche",
  "metadata.applied": "Champs remplis — relisez avant d'enregistrer.",
  "metadata.confidence": "{percent} % de correspondance",
  "metadata.from": "Depuis {source}",
  "metadata.openSource": "Voir la fiche d'origine",
  "metadata.partial":
    "Certaines bases n'ont pas répondu : la liste est incompl\u00e8te.",
  "metadata.explain":
    "Les propositions viennent de bases publiques. Rien n'est enregistré tant que vous n'avez pas validé.",

  // ── Configuration ──────────────────────────────────────────────────────────
  "settings.title": "Configuration",
  "settings.open": "Configuration",
  "settings.back": "Retour",
  "settings.storage.hint": "Espaces de stockage, biblioth\u00e8ques et cache",
  "settings.accounts.hint": "Comptes de l'instance et leurs droits",
  "settings.sources.hint": "Catalogues OPDS interrogés par la recherche fédérée",
  "settings.devices.hint": "Sessions ouvertes, et comment les révoquer",
  "settings.mobile.hint": "Installer l'application Android servie par cette instance",
  "settings.discover": "Découvrir",
  "settings.discoverHint": "Chercher dans les catalogues fédérés",

  // ── Recherche fédérée ──────────────────────────────────────────────────────
  "discovery.soon": "Fonctionnalité à venir",
  "discovery.soonHint":
    "La recherche dans des catalogues extérieurs n'est pas encore disponible. Le raccourci reste en place : cet écran s'ouvrira ici le jour où elle arrivera.",
  "discovery.title": "Découvrir",
  "discovery.dialogLabel": "Recherche fédérée",

  // États d'un catalogue, traduits ici : le serveur rend un code stable.



  // Codes d'échec rendus par le serveur, traduits ici.
} as const;

/** Les clés du catalogue. Toute traduction doit les couvrir toutes. */
export type MessageKey = keyof typeof fr;
