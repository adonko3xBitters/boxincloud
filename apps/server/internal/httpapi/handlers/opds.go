package handlers

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/adonko3xBitters/boxincloud/server/internal/catalog"
	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi/problem"
	"github.com/adonko3xBitters/boxincloud/server/internal/reader"
)

/*
Catalogue OPDS sortant.

C'est le pendant de la fédération : boxincloud sait interroger des catalogues
OPDS depuis le début, il sait maintenant en être un. Ce qui ouvre l'instance à
tous les lecteurs tiers existants — Chunky, Panels, KyBook, Moon+ Reader — sans
qu'aucun d'eux ait à connaître boxincloud.

# OPDS 1.2, et pas 2.0

Le client de ce projet lit les deux. Le serveur ne publie que le premier, et
c'est un choix, pas un raccourci : OPDS 2.0 est plus propre, mais les lecteurs
de bande dessinée installés sur les téléphones ne le comprennent pas. Publier du
JSON qu'aucun d'eux ne lit reviendrait à n'ouvrir l'instance à personne.

# Deux natures de flux, et la confusion à éviter

Un flux de NAVIGATION mène à d'autres flux : les bibliothèques, les séries. Un
flux d'ACQUISITION contient des œuvres téléchargeables. Les mélanger est le
défaut qu'on a corrigé du côté client, où les entrées de navigation
encombraient les résultats en menant à du XML brut — le faire ici les
imposerait à tous les lecteurs tiers.

Le type de contenu les distingue, et les lecteurs s'y fient : `kind=navigation`
ou `kind=acquisition` dans le profil.

# Les droits sont ceux du reste

Chaque flux passe par le même `catalog.Service` et le même `Viewer` que
l'interface web. Un profil restreint voit par OPDS exactement ce qu'il voit
ailleurs — dossiers verrouillés masqués, classification d'âge appliquée. Rien
n'est réimplémenté ici, ce qui est la seule façon que les deux ne divergent pas.
*/

// Types de contenu du profil OPDS.
const (
	opdsNavigation  = "application/atom+xml;profile=opds-catalog;kind=navigation"
	opdsAcquisition = "application/atom+xml;profile=opds-catalog;kind=acquisition"
	opdsSearchDesc  = "application/opensearchdescription+xml"
)

// Relations OPDS.
const (
	relSelf        = "self"
	relStart       = "start"
	relUp          = "up"
	relSearch      = "search"
	relSubsection  = "subsection"
	relAcquisition = "http://opds-spec.org/acquisition"
	relImage       = "http://opds-spec.org/image"
	relThumbnail   = "http://opds-spec.org/image/thumbnail"
)

// opdsFeedLimit borne une page de flux.
//
// Les lecteurs tiers paginent mal ou pas du tout : beaucoup chargent la première
// page et s'arrêtent là. Une page large leur donne un catalogue utilisable ;
// trop large, elle fait un document de plusieurs méga-octets qu'un téléphone
// met dix secondes à analyser.
const opdsFeedLimit = 60

// OPDS sert le catalogue de l'instance au format OPDS 1.2.
type OPDS struct {
	catalog *catalog.Service
	reader  *reader.Service
	// baseURL est l'adresse publique de l'instance. Les flux OPDS doivent
	// porter des adresses ABSOLUES : un lecteur tiers résout mal le relatif, et
	// certains ne le résolvent pas du tout.
	baseURL string
}

func NewOPDS(cat *catalog.Service, rdr *reader.Service, baseURL string) *OPDS {
	return &OPDS{catalog: cat, reader: rdr, baseURL: strings.TrimRight(baseURL, "/")}
}

// ─── Représentation Atom ─────────────────────────────────────────────────────

type opdsLink struct {
	Rel   string `xml:"rel,attr"`
	Href  string `xml:"href,attr"`
	Type  string `xml:"type,attr"`
	Title string `xml:"title,attr,omitempty"`
}

type opdsAuthor struct {
	Name string `xml:"name"`
}

type opdsEntry struct {
	Title   string       `xml:"title"`
	ID      string       `xml:"id"`
	Updated string       `xml:"updated"`
	Authors []opdsAuthor `xml:"author,omitempty"`
	// Content porte le résumé. `type="text"` plutôt que `html` : les résumés
	// viennent de ComicInfo.xml et de bases tierces, et les servir en HTML
	// laisserait passer leur balisage dans un lecteur qui l'interpréterait.
	Content   *opdsContent `xml:"content,omitempty"`
	Language  string       `xml:"http://purl.org/dc/terms/ language,omitempty"`
	Issued    string       `xml:"http://purl.org/dc/terms/ issued,omitempty"`
	Publisher string       `xml:"http://purl.org/dc/terms/ publisher,omitempty"`
	Links     []opdsLink   `xml:"link"`
}

type opdsContent struct {
	Type string `xml:"type,attr"`
	Text string `xml:",chardata"`
}

type opdsFeed struct {
	XMLName xml.Name `xml:"http://www.w3.org/2005/Atom feed"`
	// Les espaces de noms sont déclarés à la main : l'encodeur de la
	// bibliothèque standard ne sait pas les factoriser sur la racine, et un
	// préfixe répété sur chaque élément gonflerait le document sans profit.
	XMLNSDC   string      `xml:"xmlns:dcterms,attr"`
	XMLNSOPDS string      `xml:"xmlns:opds,attr"`
	ID        string      `xml:"id"`
	Title     string      `xml:"title"`
	Updated   string      `xml:"updated"`
	Author    *opdsAuthor `xml:"author,omitempty"`
	Links     []opdsLink  `xml:"link"`
	Entries   []opdsEntry `xml:"entry"`
}

// ─── Flux ────────────────────────────────────────────────────────────────────

/*
Root est le point d'entrée du catalogue.

C'est l'adresse que l'utilisateur colle dans son lecteur. Elle ne contient que
des rubriques : un lecteur qui la charge doit voir immédiatement par où entrer,
pas les six mille albums de l'instance.
*/
func (h *OPDS) Root(w http.ResponseWriter, r *http.Request) {
	v, ok := viewerFrom(w, r)
	if !ok {
		return
	}

	libraries, err := h.catalog.ListLibraries(r.Context(), v)
	if err != nil {
		writeInternal(w, r, err)
		return
	}

	feed := h.newFeed(r, "urn:boxincloud:opds:root", "boxincloud", "/opds")
	feed.Links = append(feed.Links, opdsLink{
		Rel:  relSearch,
		Href: h.abs(r, "/opds/search.xml"),
		Type: opdsSearchDesc,
	})

	feed.Entries = append(feed.Entries, h.navigationEntry(r,
		"urn:boxincloud:opds:recent", "Ajouts récents",
		"Les derniers albums entrés dans la bibliothèque.", "/opds/recent"))

	for _, library := range libraries {
		feed.Entries = append(feed.Entries, h.navigationEntry(r,
			"urn:boxincloud:opds:library:"+library.ID.String(),
			library.Name,
			fmt.Sprintf("%d albums", library.ComicCount),
			"/opds/libraries/"+library.ID.String()))
	}

	h.write(w, r, feed, opdsNavigation)
}

// Recent sert les derniers albums ajoutés.
func (h *OPDS) Recent(w http.ResponseWriter, r *http.Request) {
	h.acquisitionFeed(w, r, acquisitionRequest{
		id:    "urn:boxincloud:opds:recent",
		title: "Ajouts récents",
		self:  "/opds/recent",
		query: catalog.ListComicsQuery{Sort: "added", Limit: opdsFeedLimit},
	})
}

// Library sert les albums d'une bibliothèque.
func (h *OPDS) Library(w http.ResponseWriter, r *http.Request) {
	libraryID, err := uuid.Parse(chi.URLParam(r, "libraryID"))
	if err != nil {
		problem.Write(w, r, problem.BadRequest("invalid library id"))
		return
	}

	h.acquisitionFeed(w, r, acquisitionRequest{
		id:    "urn:boxincloud:opds:library:" + libraryID.String(),
		title: "Bibliothèque",
		self:  "/opds/libraries/" + libraryID.String(),
		query: catalog.ListComicsQuery{
			LibraryID: &libraryID,
			Sort:      "title",
			Limit:     opdsFeedLimit,
			Cursor:    r.URL.Query().Get("cursor"),
		},
	})
}

/*
SearchDescription publie le document OpenSearch.

Sans lui, aucun lecteur tiers ne sait chercher : la spécification OPDS n'a pas
d'URL de recherche conventionnelle, elle est annoncée. C'est exactement le
chemin que le client de ce projet parcourt pour interroger un catalogue
distant — servi ici dans l'autre sens.
*/
func (h *OPDS) SearchDescription(w http.ResponseWriter, r *http.Request) {
	if _, ok := viewerFrom(w, r); !ok {
		return
	}

	document := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<OpenSearchDescription xmlns="http://a9.com/-/spec/opensearch/1.1/">
  <ShortName>boxincloud</ShortName>
  <Description>Recherche dans la bibliothèque</Description>
  <InputEncoding>UTF-8</InputEncoding>
  <OutputEncoding>UTF-8</OutputEncoding>
  <Url type="%s" template="%s?q={searchTerms}"/>
</OpenSearchDescription>`, opdsAcquisition, h.abs(r, "/opds/search"))

	w.Header().Set("Content-Type", opdsSearchDesc+"; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	_, _ = w.Write([]byte(document))
}

// Search sert les résultats d'une recherche.
func (h *OPDS) Search(w http.ResponseWriter, r *http.Request) {
	v, ok := viewerFrom(w, r)
	if !ok {
		return
	}

	terms := strings.TrimSpace(r.URL.Query().Get("q"))
	feed := h.newFeed(r, "urn:boxincloud:opds:search", "Recherche : "+terms, "/opds/search")

	if terms != "" {
		found, err := h.catalog.Search(r.Context(), v, terms, nil, opdsFeedLimit)
		if err != nil {
			writeInternal(w, r, err)
			return
		}
		for _, comic := range found.Comics {
			feed.Entries = append(feed.Entries, h.comicEntry(r, comic))
		}
	}

	h.write(w, r, feed, opdsAcquisition)
}

type acquisitionRequest struct {
	id    string
	title string
	self  string
	query catalog.ListComicsQuery
}

func (h *OPDS) acquisitionFeed(w http.ResponseWriter, r *http.Request, req acquisitionRequest) {
	v, ok := viewerFrom(w, r)
	if !ok {
		return
	}

	page, err := h.catalog.ListComics(r.Context(), v, req.query)
	if err != nil {
		writeCatalogError(w, r, err)
		return
	}

	feed := h.newFeed(r, req.id, req.title, req.self)
	for _, comic := range page.Items {
		feed.Entries = append(feed.Entries, h.comicEntry(r, comic))
	}

	/*
		La page suivante est annoncée quand elle existe.

		Beaucoup de lecteurs l'ignorent et s'arrêtent à la première page — d'où
		une page large — mais ceux qui la suivent doivent pouvoir tout
		parcourir, et l'omettre amputerait silencieusement le catalogue.
	*/
	if page.NextCursor != "" {
		feed.Links = append(feed.Links, opdsLink{
			Rel:  "next",
			Href: h.abs(r, req.self+"?cursor="+url.QueryEscape(page.NextCursor)),
			Type: opdsAcquisition,
		})
	}

	h.write(w, r, feed, opdsAcquisition)
}

// ─── Entrées ─────────────────────────────────────────────────────────────────

func (h *OPDS) navigationEntry(r *http.Request, id, title, summary, href string) opdsEntry {
	return opdsEntry{
		Title:   title,
		ID:      id,
		Updated: nowRFC3339(),
		Content: &opdsContent{Type: "text", Text: summary},
		Links: []opdsLink{{
			Rel:  relSubsection,
			Href: h.abs(r, href),
			Type: opdsNavigation,
		}},
	}
}

/*
comicEntry décrit un album téléchargeable.

Le lien d'acquisition porte le type MIME du format, et pas un type générique :
c'est lui qui permet à un lecteur de savoir s'il sait ouvrir le fichier avant de
le télécharger. Un `application/octet-stream` ferait apparaître tous les albums
comme des fichiers inconnus.
*/
func (h *OPDS) comicEntry(r *http.Request, comic catalog.Comic) opdsEntry {
	id := comic.ID.String()

	title := comic.Title
	if comic.SeriesName != "" && comic.Number != "" {
		// Le lecteur tiers n'a pas la notion de série : la porter dans le titre
		// est le seul moyen que ses albums se classent correctement chez lui.
		title = fmt.Sprintf("%s - %s - %s", comic.SeriesName, comic.Number, comic.Title)
	}

	entry := opdsEntry{
		Title:    title,
		ID:       "urn:boxincloud:comic:" + id,
		Updated:  comic.CreatedAt.UTC().Format(time.RFC3339),
		Language: comic.Language,
		Links: []opdsLink{
			{
				Rel:  relAcquisition,
				Href: h.abs(r, "/opds/comics/"+id+"/file"),
				Type: opdsContentType(comic.Format),
			},
			{
				Rel:  relThumbnail,
				Href: h.abs(r, "/opds/comics/"+id+"/cover?width=320"),
				Type: "image/jpeg",
			},
			{
				Rel:  relImage,
				Href: h.abs(r, "/opds/comics/"+id+"/cover?width=640"),
				Type: "image/jpeg",
			},
		},
	}

	if comic.Summary != "" {
		entry.Content = &opdsContent{Type: "text", Text: comic.Summary}
	}
	if comic.ReleasedAt != nil {
		entry.Issued = comic.ReleasedAt.UTC().Format("2006-01-02")
	}

	return entry
}

// ─── Fichiers ────────────────────────────────────────────────────────────────

/*
File sert l'archive d'un album.

C'est la seule route du projet qui rende le fichier ENTIER, et elle n'existe que
pour l'OPDS : un lecteur tiers ne sait pas lire page par page, il télécharge
l'album puis l'ouvre lui-même.

Le flux passe par le service de lecture, donc par le même contrôle de visibilité
que le reste. Aucun octet n'est mis en mémoire : l'archive traverse le serveur.
*/
func (h *OPDS) File(w http.ResponseWriter, r *http.Request) {
	v, ok := viewerFrom(w, r)
	if !ok {
		return
	}

	comicID, err := uuid.Parse(chi.URLParam(r, "comicID"))
	if err != nil {
		problem.Write(w, r, problem.BadRequest("invalid comic id"))
		return
	}

	comic, err := h.catalog.GetComic(r.Context(), v, comicID)
	if err != nil {
		writeCatalogError(w, r, err)
		return
	}

	content, err := h.reader.OpenArchive(r.Context(), comicID)
	if err != nil {
		writeReaderError(w, r, err)
		return
	}
	defer func() { _ = content.Body.Close() }()

	w.Header().Set("Content-Type", opdsContentType(comic.Format))
	// Le nom de fichier compte : c'est lui que le lecteur tiers affichera et
	// classera. Un identifiant technique rendrait la bibliothèque illisible
	// chez lui.
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename*=UTF-8''%s`,
			url.PathEscape(opdsFilename(comic))))
	if content.Size > 0 {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", content.Size))
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")

	_, _ = io.Copy(w, content.Body)
}

// Cover sert la couverture d'un album à un lecteur tiers.
func (h *OPDS) Cover(w http.ResponseWriter, r *http.Request) {
	comicID, ok := h.authorized(w, r)
	if !ok {
		return
	}

	content, err := h.reader.GetCover(
		r.Context(), comicID, int(intParam(r, "width", 320)), r.Header.Get("Accept"))
	if err != nil {
		writeReaderError(w, r, err)
		return
	}
	defer func() { _ = content.Body.Close() }()

	writeImage(w, r, content, "private, max-age=86400")
}

func (h *OPDS) authorized(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	v, ok := viewerFrom(w, r)
	if !ok {
		return uuid.UUID{}, false
	}

	comicID, err := uuid.Parse(chi.URLParam(r, "comicID"))
	if err != nil {
		problem.Write(w, r, problem.BadRequest("invalid comic id"))
		return uuid.UUID{}, false
	}

	if _, err := h.catalog.GetComic(r.Context(), v, comicID); err != nil {
		writeCatalogError(w, r, err)
		return uuid.UUID{}, false
	}
	return comicID, true
}

// ─── Communs ─────────────────────────────────────────────────────────────────

func (h *OPDS) newFeed(r *http.Request, id, title, self string) *opdsFeed {
	return &opdsFeed{
		XMLNSDC:   "http://purl.org/dc/terms/",
		XMLNSOPDS: "http://opds-spec.org/2010/catalog",
		ID:        id,
		Title:     title,
		Updated:   nowRFC3339(),
		Author:    &opdsAuthor{Name: "boxincloud"},
		Links: []opdsLink{
			{Rel: relSelf, Href: h.abs(r, self), Type: opdsAcquisition},
			{Rel: relStart, Href: h.abs(r, "/opds"), Type: opdsNavigation},
			{Rel: relUp, Href: h.abs(r, "/opds"), Type: opdsNavigation},
		},
	}
}

func (h *OPDS) write(w http.ResponseWriter, r *http.Request, feed *opdsFeed, contentType string) {
	// Le lien « self » porte le type du flux qu'il désigne : un flux de
	// navigation annoncé comme acquisition ferait chercher des téléchargements
	// à un lecteur qui n'en trouverait pas.
	for i := range feed.Links {
		if feed.Links[i].Rel == relSelf {
			feed.Links[i].Type = contentType
		}
	}

	w.Header().Set("Content-Type", contentType+"; charset=utf-8")
	// Jamais de cache partagé : le flux dépend du compte qui l'a demandé, et
	// deux profils ne voient pas les mêmes albums.
	w.Header().Set("Cache-Control", "private, no-store")

	_, _ = w.Write([]byte(xml.Header))
	encoder := xml.NewEncoder(w)
	encoder.Indent("", "  ")
	if err := encoder.Encode(feed); err != nil {
		// L'en-tête est déjà parti : on ne peut plus répondre une erreur, on
		// journalise et on laisse le document tronqué — que le lecteur
		// rejettera, ce qui est le comportement le moins trompeur.
		writeInternal(w, r, err)
	}
}

/*
abs rend une adresse absolue.

Les flux OPDS doivent en porter : un lecteur tiers résout mal le relatif, et
plusieurs ne le résolvent pas du tout — ils affichent un catalogue vide sans
rien signaler.

L'adresse publique configurée l'emporte quand elle existe, parce qu'elle seule
est correcte derrière un proxy : le serveur y voit un hôte interne que le client
ne saurait pas joindre.

Sans elle, on retombe sur l'hôte de la REQUÊTE. C'est un repli, pas un défaut de
conception : l'adresse par laquelle le lecteur vient d'arriver est
nécessairement une adresse qu'il sait joindre. Sans ce repli, une instance dont
l'opérateur n'a pas renseigné `BOXINCLOUD_PUBLIC_URL` publierait un catalogue
inutilisable, et rien ne le lui dirait.
*/
func (h *OPDS) abs(r *http.Request, path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if h.baseURL != "" {
		return h.baseURL + path
	}
	return originOf(r) + path
}

// originOf reconstitue l'adresse par laquelle le client est arrivé.
//
// Les en-têtes de proxy sont pris en compte : sans eux, une instance derrière
// un terminaison TLS annoncerait des adresses en clair, que les lecteurs
// refusent depuis un catalogue servi en HTTPS.
func originOf(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwarded := r.Header.Get("X-Forwarded-Proto"); forwarded != "" {
		scheme = forwarded
	}

	host := r.Host
	if forwarded := r.Header.Get("X-Forwarded-Host"); forwarded != "" {
		host = forwarded
	}
	return scheme + "://" + host
}

func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }

// opdsContentType donne le type MIME d'un format d'album.
//
// Les types `vnd.comicbook` sont ceux que les lecteurs de bande dessinée
// reconnaissent ; un `application/zip` ferait passer un CBZ pour une archive
// quelconque.
func opdsContentType(format string) string {
	switch strings.ToLower(format) {
	case "cbz":
		return "application/vnd.comicbook+zip"
	case "cbr":
		return "application/vnd.comicbook-rar"
	case "cb7":
		return "application/x-cb7"
	case "pdf":
		return "application/pdf"
	case "epub":
		return "application/epub+zip"
	default:
		return "application/octet-stream"
	}
}

// opdsFilename compose un nom de fichier lisible chez le lecteur tiers.
func opdsFilename(comic catalog.Comic) string {
	name := comic.Title
	if comic.SeriesName != "" && comic.Number != "" {
		name = fmt.Sprintf("%s - %s - %s", comic.SeriesName, comic.Number, comic.Title)
	}

	// Les séparateurs de chemin sont retirés : un titre contenant une barre
	// oblique produirait un nom que certains clients interprètent comme un
	// dossier.
	name = strings.NewReplacer("/", "-", "\\", "-").Replace(name)

	extension := strings.ToLower(comic.Format)
	if extension == "" {
		extension = "cbz"
	}
	return name + "." + extension
}
