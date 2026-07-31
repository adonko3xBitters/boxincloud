/*
Package scraper lit un site qui n'expose ni API ni flux.

# Pourquoi ce paquet existe

La feuille de route admet cinq sources du domaine public. Deux d'entre elles —
Digital Comic Museum, Comic Book Plus — n'ont ni API REST, ni flux OPDS, ni
export d'aucune sorte. Elles publient des scans du domaine public depuis vingt
ans, dans du HTML écrit à la main. Le client OPDS ne peut rien pour elles, et
écrire un client Go par site donnerait autant de fichiers à recompiler que de
fois où l'un d'eux change de gabarit HTML.

D'où un moteur unique, piloté par des GABARITS DÉCLARATIFS : le code sait
parcourir un document, les fichiers YAML savent où regarder. Ajouter un site ne
demande alors plus de Go, et corriger un sélecteur cassé ne demande plus de
livraison.

# Ce que ce paquet ne fait pas, et ne fera pas

Le mécanisme est générique ; la LISTE des sites ne l'est pas.

Les gabarits livrés sont embarqués dans le binaire (`templates/*.yaml`) et
restent soumis au critère d'admission de `docs/04-roadmap.md` : la diffusion
des œuvres doit être autorisée — domaine public, licence libre, autorisation de
l'auteur, ou accès fourni par l'utilisateur avec ses propres identifiants. Un
moteur de parsing configurable ne change pas ce critère : il rend seulement
inutile d'écrire du Go pour chaque site qui y satisfait.

Un opérateur peut charger ses propres gabarits depuis un répertoire
(`BOXINCLOUD_SCRAPER_TEMPLATES_DIR`), désactivé par défaut. C'est le pendant
exact de la fédération OPDS en configuration libre : l'instance interroge ce que
son administrateur lui désigne, sous sa responsabilité, et boxincloud ne livre
pas l'annuaire.

# Les trois choses qu'un scraper correct doit faire et qu'on oublie

**Espacer ses requêtes.** Un site associatif servi par un mutualisé ne supporte
pas qu'on lui demande trente fiches en deux secondes. Le limiteur sortant du
paquet parent est réutilisé tel quel, avec un compartiment PAR HÔTE : deux
miroirs sont deux machines, et les compter ensemble punirait l'un pour l'autre.

**Lire robots.txt.** Ce n'est pas une obligation légale, c'est la frontière que
le site a lui-même publiée. Un client qui la respecte se fait rarement bloquer ;
un client qui l'ignore se fait bloquer par adresse, ce qui prive tous les
utilisateurs de l'instance et pas seulement celui qui a lancé la recherche.

**Renoncer.** Un site qui ne répond pas doit sortir de la recherche fédérée en
quelques secondes, pas la retenir. Le budget global borne l'ensemble d'une
recherche, y compris le suivi des fiches — sans lui, un gabarit qui suit vingt
fiches à huit secondes chacune ferait attendre trois minutes.

# La résilience des miroirs

Un site du domaine public change de domaine, tombe, ou reste servi par un
miroir quand l'original a disparu. Le gabarit déclare donc une LISTE de bases,
essayées dans l'ordre, et l'administrateur peut en imposer une autre depuis la
configuration de la source — sans recompiler, puisque c'est l'URL de la source
en base qui l'emporte sur le gabarit.

Le repli ne se déclenche que sur ce qui ressemble à une panne : injoignable,
5xx, 429. Un 404 est une réponse, pas une panne ; réessayer ailleurs ferait
quatre requêtes pour apprendre quatre fois la même chose.
*/
package scraper

import "errors"

var (
	// ErrInvalidTemplate signale un gabarit inutilisable.
	//
	// Rendue au chargement, jamais à la recherche : un sélecteur qui ne compile
	// pas doit se voir au démarrage de l'instance, pas six semaines plus tard
	// au milieu d'une recherche qui ne rend rien.
	ErrInvalidTemplate = errors.New("scraper : gabarit invalide")

	// ErrUnknownTemplate signale une source dont le gabarit n'est pas chargé.
	ErrUnknownTemplate = errors.New("scraper : gabarit inconnu")

	// ErrAllMirrorsFailed signale que toutes les bases ont échoué.
	ErrAllMirrorsFailed = errors.New("scraper : aucun miroir n'a répondu")

	// ErrDisallowed signale une adresse que le robots.txt du site interdit.
	//
	// Distincte d'un échec réseau, et volontairement : elle ne doit pas faire
	// essayer le miroir suivant, qui publiera la même règle.
	ErrDisallowed = errors.New("scraper : adresse interdite par robots.txt")
)
