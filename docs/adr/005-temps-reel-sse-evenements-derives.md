# ADR-005 — Temps réel : SSE, et des événements dérivés d'instantanés

> Statut : accepté. Étape 0 du module eD2k/Kad Center.

## Contexte

Une interface de téléchargement doit vivre : les barres avancent, les vitesses
changent, une source apparaît, Kad se connecte. Deux questions distinctes se
posent, et il est utile de ne pas les confondre.

**D'où viennent les changements ?** Du démon amuled, par le protocole EC.

**Comment atteignent-ils le navigateur ?** Il n'y avait, jusqu'ici, aucun
mécanisme dans le projet : la grille d'albums sonde toutes les deux secondes
tant qu'un album n'est pas indexé, et cesse dès qu'ils le sont tous. Cela
convient à un état qui change trois fois par minute ; pas à des débits.

## Le fait à regarder en face : EC ne pousse rien

EC est un protocole **requête/réponse**. amuled n'émet aucun message spontané :
il n'y a ni abonnement, ni rappel, ni socket d'événements. `amuleweb` et
`amulegui` scrutent en boucle, et `EC_OP_GET_UPDATE` ne renverse pas le sens du
dialogue — il économise seulement la bande passante en ne renvoyant que ce qui a
changé depuis la requête précédente.

Il s'ensuit que `DownloadStarted`, `DownloadCompleted` ou `KadConnected`
**n'existent nulle part dans EC**. Ce ne sont pas des messages à relayer : ce
sont des conclusions à tirer.

## Décision

**Deux mécanismes, chacun à sa place.**

1. **Vers amuled : scrutation, à cadence adaptative.** Une seconde quand quelque
   chose bouge, cinq quand la file est au repos, **rien du tout quand aucun
   navigateur n'est abonné**. Une instance que personne ne regarde ne doit
   produire aucun trafic EC — c'est la même règle que le sondage conditionnel
   déjà en place dans la grille d'albums.

2. **Vers le navigateur : Server-Sent Events.** Un flux `text/event-stream`
   servi par `net/http`, donc par Chi, donc sans dépendance nouvelle. Le
   concentrateur vit dans `internal/platform/sse` : il n'a rien de spécifique à
   eD2k, et la progression d'un scan de bibliothèque en sera le second usage.

**Les événements sont dérivés, pas relayés.** La couche d'intégration compare
deux instantanés successifs et en déduit ce qui s'est passé : un téléchargement
qui quitte la file à 100 % est un `DownloadCompleted`, un `connstate` qui bascule
est un `ServerConnected`. La table de correspondance est explicite, et elle se
teste en comparant deux instantanés figés — un test qui ne demande ni réseau ni
démon.

**Une scrutation pour tout le monde.** Le concentrateur interroge amuled une
fois et diffuse à tous les abonnés. Vingt onglets ouverts ne font pas vingt
connexions EC.

## Conséquences

**La latence d'un événement vaut la période de scrutation.** C'est une propriété
assumée du montage, pas un défaut à corriger : personne ne fait autrement,
`amuleweb` compris. Il faut donc éviter de promettre dans l'interface une
réactivité que le protocole ne permet pas — une barre qui avance par paliers
d'une seconde est honnête, une animation qui interpole entre deux mesures ment.

**Un événement peut être manqué.** Si un téléchargement démarre et se termine
entre deux instantanés, la dérivation ne verra qu'un fichier terminé. Pour
l'affichage c'est sans conséquence ; pour le pont vers la bibliothèque, qui
arrive à l'étape 7, la détection ne doit donc pas reposer sur l'événement mais
sur l'état — un fichier présent dans le répertoire d'arrivée et pas encore
publié, quelle que soit la manière dont il y est arrivé.

**SSE plutôt que WebSocket**, pour trois raisons : le besoin est
unidirectionnel — les commandes partent en REST ordinaire, où elles bénéficient
de l'authentification, de la limitation de débit et du format d'erreur du
projet ; SSE se reconnecte tout seul, là où un WebSocket demande d'écrire cette
logique ; et SSE ne coûte aucune dépendance.

**Le jeton passe en paramètre d'URL.** `EventSource` ne sait pas porter
d'en-tête `Authorization`. La route rejoint donc le groupe déjà prévu pour les
balises `<img>` et `sendBeacon` — celui dont `router.go` documente précisément
la portée : on l'accepte là où il n'y a pas d'alternative, nulle part ailleurs.

**Les proxys mettent en tampon.** La réponse porte `X-Accel-Buffering: no` et
`Cache-Control: no-store`, et un battement de cœur périodique traverse les
intermédiaires qui coupent une connexion muette. Sans cela, le flux fonctionne
en développement et meurt derrière le premier nginx.
