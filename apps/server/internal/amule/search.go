package amule

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/adonko3xBitters/boxincloud/server/internal/amule/ec"
)

/*
La recherche.

Elle ne ressemble à rien d'autre dans ce module, pour une raison de fond : elle
est ASYNCHRONE et le protocole ne la suit pas pour nous.

On démarre une recherche, le démon répond « en cours », et c'est tout. Il faut
ensuite demander la progression, puis les résultats, jusqu'à ce qu'elle
s'achève. Il n'y a ni notification, ni identifiant de recherche côté client :
le démon n'en tient qu'UNE à la fois, et en démarrer une seconde efface la
première.

# Une seule recherche à la fois, et c'est le démon qui l'impose

`Get_EC_Response_Search` commence par effacer tous les résultats existants. Ce
n'est donc pas un choix d'interface qu'on pourrait assouplir : deux
utilisateurs qui cherchent en même temps se marcheraient dessus, et le second
verrait les résultats du premier disparaître sous ses yeux.

C'est une conséquence directe du fait qu'amuled est un moteur unique partagé —
la même raison qui fait que le « multi-utilisateurs » du module est une couche
d'autorisation, pas d'isolation.
*/

// SearchNetwork dit où chercher.
type SearchNetwork string

const (
	// SearchServer interroge le serveur eD2k auquel on est connecté. Rapide,
	// mais limité à ce que CE serveur indexe.
	SearchServer SearchNetwork = "server"

	// SearchGlobal interroge tous les serveurs connus, en UDP. Plus large et
	// plus lent ; c'est le mode habituel.
	SearchGlobal SearchNetwork = "global"

	// SearchKad interroge le réseau Kademlia. Ne demande aucun serveur, et
	// trouve ce que les serveurs ignorent.
	SearchKad SearchNetwork = "kad"
)

// Codes attendus par le démon, transcrits de EC_SEARCH_TYPE.
var searchNetworkCodes = map[SearchNetwork]uint64{
	SearchServer: uint64(ec.SearchLocal),
	SearchGlobal: uint64(ec.SearchGlobal),
	SearchKad:    uint64(ec.SearchKad),
}

/*
SearchParams décrit ce qu'on cherche.

Les filtres facultatifs ne sont PAS envoyés quand ils sont vides, et ce n'est
pas une optimisation : amuled distingue « pas de filtre de taille » de « taille
minimale nulle », et envoyer systématiquement un zéro changerait le sens de la
requête.
*/
type SearchParams struct {
	Query string

	Network SearchNetwork

	// FileType restreint par catégorie. Les valeurs sont celles d'aMule —
	// « Audio », « Video », « Image », « Doc », « Pro », « Iso » — et vides
	// pour ne pas filtrer.
	FileType string

	// Extension filtre sur le suffixe du nom, sans le point.
	Extension string

	MinSize int64
	MaxSize int64

	// Availability est le nombre minimal de sources. Zéro ne filtre pas.
	Availability int
}

/*
SearchStatus décrit l'avancement de la recherche en cours.

Il n'y a PAS de champ « une recherche tourne-t-elle », et c'est une limite du
protocole, pas un oubli : le démon rend 0 aussi bien pour « aucune recherche »
que pour « recherche qui vient de démarrer ». Les deux sont indiscernables.

Ce n'est pas gênant en pratique — celui qui sonde la progression vient de lancer
la recherche, il sait donc qu'elle tourne — mais inventer un booléen à partir de
ce zéro produirait une valeur fausse la moitié du temps.
*/
type SearchStatus struct {
	// Progress va de 0 à 100.
	Progress int

	// Complete dit que la recherche est arrivée à son terme. Vrai aussi quand
	// aucune recherche ne tourne, faute de pouvoir distinguer les deux.
	Complete bool
}

// SearchResult est un fichier trouvé.
type SearchResult struct {
	Hash string
	Name string
	Size int64

	// Sources est le nombre de pairs qui détiennent tout ou partie du fichier ;
	// CompleteSources ceux qui en ont la totalité.
	//
	// C'est le second qui compte pour savoir si un fichier finira : cent
	// sources partielles qui n'ont jamais la même partie ne complètent rien.
	Sources         int
	CompleteSources int

	// AlreadyQueued dit que ce fichier est déjà dans la file. L'interface
	// grise alors le bouton, plutôt que de laisser l'ajouter deux fois.
	AlreadyQueued bool
}

// ErrSearchRefused signale un refus explicite du démon.
var ErrSearchRefused = errors.New("amule : recherche refusée")

// ─── Requêtes ────────────────────────────────────────────────────────────────

/*
requestSearch construit la demande de recherche.

Le tag racine porte le RÉSEAU comme valeur, et les critères comme enfants. Cette
forme surprend — on attendrait un tag « type » parmi les autres — mais c'est
celle qu'attend le démon.
*/
func requestSearch(p SearchParams) (ec.Packet, error) {
	code, ok := searchNetworkCodes[p.Network]
	if !ok {
		return ec.Packet{}, fmt.Errorf("amule : réseau de recherche inconnu %q", p.Network)
	}

	query := strings.TrimSpace(p.Query)
	if query == "" {
		return ec.Packet{}, errors.New("amule : recherche sans terme")
	}

	root := ec.Uint(ec.TagSearchType, code)
	children := []ec.Tag{
		ec.Text(ec.TagSearchName, query),
		ec.Text(ec.TagSearchFileType, p.FileType),
	}

	// Les filtres absents ne sont pas envoyés : le démon distingue « pas de
	// filtre » d'un filtre à zéro.
	if p.Extension != "" {
		children = append(children, ec.Text(ec.TagSearchExtension, strings.TrimPrefix(p.Extension, ".")))
	}
	if p.Availability > 0 {
		children = append(children, ec.Uint(ec.TagSearchAvailability, uint64(p.Availability)))
	}
	if p.MinSize > 0 {
		children = append(children, ec.Uint(ec.TagSearchMinSize, uint64(p.MinSize)))
	}
	if p.MaxSize > 0 {
		children = append(children, ec.Uint(ec.TagSearchMaxSize, uint64(p.MaxSize)))
	}

	root.Children = children
	return ec.New(ec.OpSearchStart, root), nil
}

func requestSearchStatus() ec.Packet  { return ec.New(ec.OpSearchProgress) }
func requestSearchResults() ec.Packet { return ec.New(ec.OpSearchResults) }

// ─── Décodage ────────────────────────────────────────────────────────────────

/*
decodeSearchStatus traduit l'avancement.

Deux valeurs demandent un traitement, et aucune des deux n'est évidente :

  - 0xFFFF signifie « aucune recherche en cours ». Le prendre au pied de la
    lettre afficherait une progression de 65535 %.
  - 0 est AMBIGU : c'est ce que rend le démon au repos comme au tout début
    d'une recherche. On ne prétend donc pas trancher — voir SearchStatus.
*/
func decodeSearchStatus(p ec.Packet) (SearchStatus, error) {
	if p.Op != ec.OpSearchProgress {
		return SearchStatus{}, fmt.Errorf("réponse %s, attendu %s", p.Op, ec.OpSearchProgress)
	}

	raw, ok := p.Uint(ec.TagSearchStatus)
	if !ok {
		return SearchStatus{}, nil
	}

	const noSearch = 0xFFFF
	if raw >= noSearch {
		return SearchStatus{Progress: 100, Complete: true}, nil
	}

	progress := asInt(raw)
	if progress > 100 {
		progress = 100
	}
	return SearchStatus{Progress: progress, Complete: progress >= 100}, nil
}

/*
decodeSearchResults traduit les fichiers trouvés.

Chaque résultat est un tag dont la valeur est le numéro interne du démon, et
dont les enfants portent les champs. Le numéro n'est PAS conservé : il change
d'une recherche à l'autre, alors que l'empreinte identifie le fichier sur le
réseau. C'est elle qui sert à demander le téléchargement.
*/
func decodeSearchResults(p ec.Packet) ([]SearchResult, error) {
	if p.Op != ec.OpSearchResults {
		return nil, fmt.Errorf("réponse %s, attendu %s", p.Op, ec.OpSearchResults)
	}

	results := make([]SearchResult, 0, len(p.Tags))
	for _, tag := range p.Tags {
		if tag.Name != ec.TagSearchfile {
			continue
		}
		results = append(results, decodeSearchResult(tag))
	}
	return results, nil
}

func decodeSearchResult(tag ec.Tag) SearchResult {
	var out SearchResult

	for _, child := range tag.Children {
		switch child.Name {
		case ec.TagPartfileName:
			if v, ok := child.Text(); ok {
				out.Name = v
			}
		case ec.TagPartfileSizeFull:
			if v, ok := child.Uint(); ok {
				out.Size = asInt64(v)
			}
		case ec.TagPartfileHash:
			if v, ok := child.Hash(); ok {
				out.Hash = hex.EncodeToString(v)
			}
		case ec.TagPartfileSourceCount:
			if v, ok := child.Uint(); ok {
				out.Sources = asInt(v)
			}
		case ec.TagPartfileSourceCountXfer:
			if v, ok := child.Uint(); ok {
				out.CompleteSources = asInt(v)
			}
		case ec.TagPartfileStatus:
			/*
				Le démon réutilise ici le champ d'état d'un téléchargement pour
				dire si le fichier est DÉJÀ en file. Une valeur non nulle
				signifie qu'il l'est — sous une forme ou une autre, en cours ou
				en pause.

				C'est ce qui permet à l'interface de griser le bouton plutôt que
				de laisser ajouter deux fois le même fichier.
			*/
			if v, ok := child.Uint(); ok {
				out.AlreadyQueued = v != 0
			}
		}
	}
	return out
}

// ─── Service ─────────────────────────────────────────────────────────────────

/*
StartSearch lance une recherche.

Efface la précédente : le démon n'en tient qu'une, et c'est lui qui l'impose.
L'appelant sondera ensuite SearchStatus puis SearchResults.
*/
func (s *Service) StartSearch(ctx context.Context, p SearchParams) error {
	req, err := requestSearch(p)
	if err != nil {
		return err
	}

	/*
		Le démon accepte en répondant OpStrings — « Search in progress » — et
		refuse en répondant OpFailed, que la couche EC traduit déjà en erreur.
		Il n'y a donc rien à vérifier de plus ici : une réponse sans erreur
		signifie que la recherche est partie.
	*/
	return s.do(ctx, req)
}

// StopSearch interrompt la recherche en cours.
func (s *Service) StopSearch(ctx context.Context) error {
	return s.do(ctx, ec.New(ec.OpSearchStop))
}

/*
SearchStatus et SearchResults interrogent le démon à la demande.

Hors de l'instantané, délibérément : une recherche est un geste ponctuel, et la
scrutation n'a aucune raison de redemander des résultats à quelqu'un qui a fermé
l'onglet depuis une heure.
*/
func (s *Service) SearchStatus(ctx context.Context) (SearchStatus, error) {
	resp, err := s.query(ctx, requestSearchStatus())
	if err != nil {
		return SearchStatus{}, err
	}
	return decodeSearchStatus(resp)
}

func (s *Service) SearchResults(ctx context.Context) ([]SearchResult, error) {
	resp, err := s.query(ctx, requestSearchResults())
	if err != nil {
		return nil, err
	}
	return decodeSearchResults(resp)
}

/*
DownloadSearchResult met un résultat en file.

Par son empreinte, jamais par le numéro interne du démon : celui-ci change
d'une recherche à l'autre, et un client qui l'aurait retenu téléchargerait le
mauvais fichier.
*/
func (s *Service) DownloadSearchResult(ctx context.Context, hash string) error {
	raw, err := parseHash(hash)
	if err != nil {
		return err
	}
	return s.do(ctx, ec.New(ec.OpDownloadSearchResult, ec.Hash(ec.TagKnownfile, raw)))
}

/*
query fait un aller-retour qui RAPPORTE une réponse.

Jumelle de `do`, qui ne rapporte rien. La différence tient au réveil : une
lecture ne change rien à l'état du démon, et réveiller la scrutation après
chaque sondage de progression la ferait tourner à plein régime pendant toute
une recherche.
*/
func (s *Service) query(ctx context.Context, req ec.Packet) (ec.Packet, error) {
	if !s.opts.Enabled {
		return ec.Packet{}, ErrDisabled
	}

	s.session.mu.Lock()
	p := s.session.poller
	s.session.mu.Unlock()

	if p != nil {
		if conn := p.Session(); conn != nil {
			resp, err := conn.Do(ctx, req)
			return resp, translateEC(err)
		}
	}

	conn, err := serviceDialer{svc: s}.Open(ctx)
	if err != nil {
		return ec.Packet{}, err
	}
	defer func() { _ = conn.Close() }()

	resp, err := conn.Do(ctx, req)
	return resp, translateEC(err)
}
