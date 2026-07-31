# Gabarits livrés

Ce répertoire contient les gabarits que **le projet** livre dans le binaire. Ils
sont revus comme du code : un gabarit fautif empêche le démarrage.

**Il est vide aujourd'hui**, et la raison mérite d'être écrite plutôt que
découverte.

## Critère d'admission

Celui de `docs/04-roadmap.md`, sans exception : la diffusion des œuvres doit
être autorisée — domaine public, licence libre, autorisation de l'auteur, ou
accès fourni par l'utilisateur avec ses propres identifiants.

Un moteur de parsing configurable ne déplace pas cette frontière. Il évite
d'écrire du Go pour chaque site qui la respecte, rien de plus.

## Pourquoi les deux sites visés n'y sont pas

La feuille de route désignait Digital Comic Museum et Comic Book Plus comme les
deux sources du domaine public sans API ni OPDS. Vérification faite (juillet
2026), aucune des deux n'est lisible aujourd'hui :

**Digital Comic Museum** répond `403` à toute requête, y compris sa page
d'accueil, dès lors que le client n'est pas un navigateur — un défi Cloudflare
(« Just a moment »). Son `robots.txt` nous autorise pourtant explicitement
(`User-agent: *` puis `Allow: /`), ce qui rend le blocage d'autant plus net : il
est posé par l'infrastructure, pas par une politique du site.

Le contourner demanderait de se faire passer pour un navigateur. C'est
précisément ce que `fetch.go` refuse, et le refus vaut plus qu'un gabarit : un
site qui ne veut pas de clients automatisés a le droit de ne pas en avoir. La
démarche correcte est de leur écrire, pas de se déguiser.

**Comic Book Plus** est joignable et son `robots.txt` est raisonnable — il
ferme une liste de pages de listage, qu'il faut donc respecter. Mais son moteur
de recherche est un Google Custom Search côté navigateur : il n'existe aucun
point d'entrée de recherche sur le serveur à interroger. Un gabarit devrait
parcourir les pages de catalogue, ce qui est un autre travail que celui décrit
ici, et bien plus coûteux pour le site.

## En attendant

Le format est documenté dans `docs/06-gabarits-scraper.md`, et un gabarit de
référence complet vit dans `../testdata/comicshelf.yaml` — celui que la suite de
tests exécute de bout en bout.

Un administrateur qui a son propre site à interroger — l'intranet d'une
médiathèque, un catalogue local — peut déposer ses gabarits dans le répertoire
désigné par `BOXINCLOUD_SCRAPER_TEMPLATES_DIR`, sous sa responsabilité.
