package amule

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/adonko3xBitters/boxincloud/server/internal/amule/ec"
)

/*
Les tests de traduction des téléchargements.

Ils construisent les paquets à la main plutôt que d'interroger un démon : c'est
la seule façon de provoquer à volonté les combinaisons rares — un fichier
« prêt » ET arrêté, une priorité automatique, un état venu d'une version
inconnue — qu'un amuled au repos ne produira jamais.

Le pendant vit dans mapping_downloads_integration_test.go : lui vérifie que la
forme supposée ici est bien celle qu'un vrai démon envoie.
*/

// ─── Fabriques de paquets ────────────────────────────────────────────────────

// dlPartfileTag fabrique un tag de fichier tel qu'amuled l'écrit : sa valeur est
// l'ECID, ses champs sont ses enfants.
func dlPartfileTag(ecid uint64, children ...ec.Tag) ec.Tag {
	tag := ec.Uint(ec.TagPartfile, ecid)
	tag.Children = children
	return tag
}

func dlClientTag(ecid uint64, children ...ec.Tag) ec.Tag {
	tag := ec.Uint(ec.TagClient, ecid)
	tag.Children = children
	return tag
}

// dlDoubleTag fabrique un tag flottant : type « double », valeur en texte.
func dlDoubleTag(name ec.TagName, text string) ec.Tag {
	return ec.Tag{Name: name, Type: ec.TypeDouble, Value: append([]byte(text), 0)}
}

// dlHashTag fabrique un tag d'empreinte depuis sa forme hexadécimale.
func dlHashTag(t *testing.T, name ec.TagName, hexHash string) ec.Tag {
	t.Helper()

	raw, err := hex.DecodeString(hexHash)
	if err != nil {
		t.Fatalf("empreinte de test illisible : %v", err)
	}
	return ec.Hash(name, raw)
}

const (
	dlHashA = "0123456789abcdef0123456789abcdef"
	dlHashB = "fedcba9876543210fedcba9876543210"
)

// ─── File de téléchargement ──────────────────────────────────────────────────

func TestDecodeDownloadsFileVide(t *testing.T) {
	// Le cas nominal du démon au repos, et celui qui doit rester silencieux :
	// une file vide n'est pas une anomalie.
	downloads, err := decodeDownloads(ec.New(ec.OpDloadQueue))
	if err != nil {
		t.Fatalf("une file vide ne doit pas produire d'erreur : %v", err)
	}
	if downloads == nil {
		t.Fatal("tranche nil rendue : l'appelant devrait pouvoir itérer sans vérifier")
	}
	if len(downloads) != 0 {
		t.Fatalf("%d téléchargements rendus pour une file vide", len(downloads))
	}
}

func TestDecodeDownloadsRefuseUnAutreOpcode(t *testing.T) {
	// Un opcode inattendu signifie que la réponse ne correspond pas à la
	// requête. Le décoder quand même rendrait une file vide plausible et
	// fausse.
	if _, err := decodeDownloads(ec.New(ec.OpStats)); err == nil {
		t.Fatal("une réponse OpStats a été acceptée comme file de téléchargement")
	}
}

func TestDecodeDownloadsTraduitTousLesChamps(t *testing.T) {
	packet := ec.New(ec.OpDloadQueue, dlPartfileTag(42,
		dlHashTag(t, ec.TagPartfileHash, dlHashA),
		ec.Text(ec.TagPartfileName, "tome-01.cbz"),
		ec.Uint(ec.TagPartfileSizeFull, 300_000_000),
		ec.Uint(ec.TagPartfileSizeDone, 100_000_000),
		ec.Uint(ec.TagPartfileSizeXfer, 110_000_000),
		ec.Uint(ec.TagPartfileSpeed, 500_000),
		ec.Uint(ec.TagPartfileStatus, psReady),
		ec.Uint(ec.TagPartfilePrio, prHigh),
		ec.Uint(ec.TagPartfileCat, 3),
		ec.Uint(ec.TagPartfileSourceCount, 40),
		ec.Uint(ec.TagPartfileSourceCountNotCurrent, 12),
		ec.Uint(ec.TagPartfileSourceCountXfer, 4),
		ec.Uint(ec.TagPartfileSourceCountA4AF, 7),
		ec.Uint(ec.TagPartfileAvailableParts, 25),
		ec.Uint(ec.TagPartfileLastSeenComp, 1_700_000_000),
	))

	downloads, err := decodeDownloads(packet)
	if err != nil {
		t.Fatalf("décodage : %v", err)
	}
	if len(downloads) != 1 {
		t.Fatalf("%d téléchargements rendus, un seul attendu", len(downloads))
	}
	d := downloads[0]

	if d.Hash != dlHashA {
		t.Errorf("empreinte %q, attendu %q", d.Hash, dlHashA)
	}
	if d.Name != "tome-01.cbz" {
		t.Errorf("nom %q", d.Name)
	}
	if d.Size != 300_000_000 || d.SizeDone != 100_000_000 || d.SizeXfer != 110_000_000 {
		t.Errorf("tailles : %d / %d / %d", d.Size, d.SizeDone, d.SizeXfer)
	}
	if d.Speed != 500_000 {
		t.Errorf("débit %d", d.Speed)
	}
	// Quatre sources transfèrent : l'état « prêt » devient « en cours ».
	if d.Status != DownloadDownloading {
		t.Errorf("état %q, attendu %q", d.Status, DownloadDownloading)
	}
	if d.Priority != PriorityHigh {
		t.Errorf("priorité %q", d.Priority)
	}
	if d.Category != 3 {
		t.Errorf("catégorie %d", d.Category)
	}
	want := SourceCounts{Total: 40, NotCurrent: 12, Transferring: 4, A4AF: 7}
	if d.Sources != want {
		t.Errorf("sources %+v, attendu %+v", d.Sources, want)
	}
	if d.AvailableParts != 25 {
		t.Errorf("parties disponibles %d", d.AvailableParts)
	}
	// 300 000 000 / 9 728 000 = 30,8 → 31 parties.
	if d.PartCount != 31 {
		t.Errorf("nombre de parties %d, attendu 31", d.PartCount)
	}
	if d.LastSeenComplete == nil || d.LastSeenComplete.Unix() != 1_700_000_000 {
		t.Errorf("dernière vue complète %v", d.LastSeenComplete)
	}
	// (300 000 000 - 100 000 000) / 500 000 = 400 secondes.
	if d.ETA == nil || *d.ETA != 400*time.Second {
		t.Errorf("ETA %v, attendu 400s", d.ETA)
	}
}

func TestDecodeDownloadsRendLEmpreinteEnMinuscules(t *testing.T) {
	// L'empreinte est notre clé stable : deux casses feraient deux clés, et le
	// démon ne garantit rien sur ce point puisqu'il envoie des octets bruts.
	packet := ec.New(ec.OpDloadQueue, dlPartfileTag(1,
		dlHashTag(t, ec.TagPartfileHash, "ABCDEF0123456789ABCDEF0123456789"),
	))

	downloads, err := decodeDownloads(packet)
	if err != nil {
		t.Fatalf("décodage : %v", err)
	}
	if got := downloads[0].Hash; got != "abcdef0123456789abcdef0123456789" {
		t.Errorf("empreinte %q, attendue en minuscules", got)
	}
}

func TestDecodeDownloadsTolereLesTagsAbsents(t *testing.T) {
	// Le cas qui arrive vraiment en production : aux niveaux incrémentaux,
	// amuled ne répète que ce qui a changé. Un champ manquant doit laisser le
	// zéro, jamais faire échouer la lecture de la file entière.
	packet := ec.New(ec.OpDloadQueue, dlPartfileTag(7,
		ec.Uint(ec.TagPartfileSpeed, 1024),
	))

	downloads, err := decodeDownloads(packet)
	if err != nil {
		t.Fatalf("un fichier sans ses champs a fait échouer le décodage : %v", err)
	}
	if len(downloads) != 1 {
		t.Fatalf("%d téléchargements rendus", len(downloads))
	}

	d := downloads[0]
	if d.Hash != "" || d.Name != "" || d.Size != 0 {
		t.Errorf("champs absents non nuls : %+v", d)
	}
	if d.Speed != 1024 {
		t.Errorf("le seul champ envoyé n'a pas été lu : débit %d", d.Speed)
	}
	// Sans code d'état ni drapeau d'arrêt, il ne reste rien qui dise autre
	// chose que « en file ».
	if d.Status != DownloadWaiting {
		t.Errorf("état %q, attendu %q", d.Status, DownloadWaiting)
	}
	// Le domaine n'a pas de priorité « inconnue » : l'absence laisse le vide.
	if d.Priority != "" {
		t.Errorf("priorité %q inventée pour un tag absent", d.Priority)
	}
	// Taille inconnue : impossible de calculer un reste, donc pas d'ETA.
	if d.ETA != nil {
		t.Errorf("ETA %v calculée sans connaître la taille", *d.ETA)
	}
}

func TestDecodeDownloadsIgnoreLesMarqueursDePresence(t *testing.T) {
	// Un tag de fichier sans enfant dit « ce fichier existe toujours », rien
	// de plus. Le traduire produirait une entrée sans nom ni taille.
	packet := ec.New(ec.OpDloadQueue,
		dlPartfileTag(11), // marqueur
		dlPartfileTag(12, dlHashTag(t, ec.TagPartfileHash, dlHashB)),
	)

	downloads, err := decodeDownloads(packet)
	if err != nil {
		t.Fatalf("décodage : %v", err)
	}
	if len(downloads) != 1 {
		t.Fatalf("%d téléchargements rendus, le marqueur n'a pas été écarté", len(downloads))
	}
	if downloads[0].Hash != dlHashB {
		t.Errorf("empreinte %q, attendu %q", downloads[0].Hash, dlHashB)
	}
}

func TestDecodeDownloadsIgnoreLesFichiersPartages(t *testing.T) {
	// La mise à jour globale mêle fichiers partagés et fichiers en cours dans
	// un même paquet. Seuls les seconds nous concernent ici.
	packet := ec.New(ec.OpSharedFiles,
		ec.Uint(ec.TagKnownfile, 1),
		dlPartfileTag(2, dlHashTag(t, ec.TagPartfileHash, dlHashA)),
	)

	downloads, err := decodeDownloads(packet)
	if err != nil {
		t.Fatalf("décodage : %v", err)
	}
	if len(downloads) != 1 {
		t.Fatalf("%d téléchargements rendus", len(downloads))
	}
}

func TestDecodeDownloadIDsApparieEmpreinteEtECID(t *testing.T) {
	packet := ec.New(ec.OpDloadQueue,
		dlPartfileTag(42, dlHashTag(t, ec.TagPartfileHash, dlHashA)),
		// Sans empreinte, rien à apparier : le fichier est écarté plutôt que
		// rattaché à une clé vide.
		dlPartfileTag(43, ec.Text(ec.TagPartfileName, "sans-empreinte")),
	)

	ids, err := decodeDownloadIDs(packet)
	if err != nil {
		t.Fatalf("décodage : %v", err)
	}
	if len(ids) != 1 {
		t.Fatalf("%d correspondances, une seule attendue : %v", len(ids), ids)
	}
	if ids[42] != dlHashA {
		t.Errorf("ECID 42 → %q, attendu %q", ids[42], dlHashA)
	}
}

// ─── Table des états ─────────────────────────────────────────────────────────

/*
TestMappingEtatsDuDemon parcourt la table des états, cas par cas.

C'est la traduction la plus exposée du fichier : les codes sont des entiers nus,
aucune erreur ne se voit à la lecture d'une trame, et une inversion se
manifesterait en interface par un fichier « en attente » qui télécharge.

Référence : CPartFile::getPartfileStatus, src/PartFile.cpp.
*/
func TestMappingEtatsDuDemon(t *testing.T) {
	cases := []struct {
		nom          string
		code         uint64
		stopped      bool
		transferring int
		want         DownloadStatus
	}{
		// Les codes 0, 1 et 6 disent tous « la file s'en occupe ». C'est le
		// nombre de sources qui transfèrent qui départage.
		{"prêt sans source active", psReady, false, 0, DownloadWaiting},
		{"prêt avec une source active", psReady, false, 1, DownloadDownloading},
		{"vide sans source active", psEmpty, false, 0, DownloadWaiting},
		{"vide avec sources actives", psEmpty, false, 3, DownloadDownloading},
		{"indéterminé sans source active", psUnknown, false, 0, DownloadWaiting},
		{"indéterminé avec source active", psUnknown, false, 2, DownloadDownloading},

		{"en pause", psPaused, false, 0, DownloadPaused},
		{"en erreur", psError, false, 0, DownloadErroneous},
		{"assemblage", psCompleting, false, 0, DownloadCompleting},
		{"terminé", psComplete, false, 0, DownloadCompleted},
		{"vérification", psHashing, false, 0, DownloadHashing},
		{"attente de vérification", psWaitingForHash, false, 0, DownloadHashing},
		{"réservation d'espace", psAllocating, false, 0, DownloadAllocating},

		// Le domaine n'a pas de constante pour le disque plein. C'est bien une
		// erreur : le fichier n'avancera plus tant que rien n'est fait.
		{"disque plein", psInsufficient, false, 0, DownloadErroneous},

		// « Arrêté » est un tag SÉPARÉ. Il l'emporte sur l'état, y compris sur
		// « prêt » — c'est tout l'intérêt du piège.
		{"prêt et arrêté", psReady, true, 0, DownloadStopped},
		{"prêt, arrêté, sources actives", psReady, true, 5, DownloadStopped},
		{"en pause et arrêté", psPaused, true, 0, DownloadStopped},
		{"en erreur et arrêté", psError, true, 0, DownloadStopped},
		{"assemblage et arrêté", psCompleting, true, 0, DownloadStopped},

		// Trois exceptions au drapeau. « Terminé » d'abord : aMule refuse
		// explicitement d'afficher « arrêté » sur un fichier fini.
		{"terminé et arrêté", psComplete, true, 0, DownloadCompleted},
		// Vérification et réservation ensuite : ce sont des traitements
		// locaux, que l'arrêt du transfert ne suspend pas.
		{"vérification et arrêté", psHashing, true, 0, DownloadHashing},
		{"réservation et arrêté", psAllocating, true, 0, DownloadAllocating},

		// Un code venu d'une version plus récente ne se devine pas.
		{"code inconnu", 42, false, 0, DownloadUnknown},
		{"code inconnu et arrêté", 42, true, 0, DownloadUnknown},
	}

	for _, c := range cases {
		t.Run(c.nom, func(t *testing.T) {
			got := mapPartfileStatus(c.code, true, c.stopped, c.transferring)
			if got != c.want {
				t.Errorf("code %d (arrêté=%v, %d sources actives) → %q, attendu %q",
					c.code, c.stopped, c.transferring, got, c.want)
			}
		})
	}
}

func TestMappingEtatSansCode(t *testing.T) {
	// Le tag d'état peut manquer, comme n'importe quel autre. Il reste alors
	// le drapeau d'arrêt, qui, lui, se suffit.
	if got := mapPartfileStatus(0, false, false, 0); got != DownloadWaiting {
		t.Errorf("sans code ni drapeau → %q, attendu %q", got, DownloadWaiting)
	}
	if got := mapPartfileStatus(0, false, true, 0); got != DownloadStopped {
		t.Errorf("sans code mais arrêté → %q, attendu %q", got, DownloadStopped)
	}
	// Le zéro d'un tag absent ne doit surtout pas être lu comme PS_READY avec
	// des sources actives.
	if got := mapPartfileStatus(0, false, false, 5); got != DownloadWaiting {
		t.Errorf("sans code, sources actives → %q, attendu %q", got, DownloadWaiting)
	}
}

// ─── Table des priorités ─────────────────────────────────────────────────────

/*
TestMappingPriorites parcourt la table des priorités.

Le piège est le décalage de dix : amuled n'envoie pas de drapeau « auto », il
l'ajoute à la priorité. Un lecteur naïf verrait des priorités 11 ou 12, qui
n'existent pas, et n'afficherait jamais « auto ».

Référence : ECSpecialCoreTags.cpp (EC_TAG_PARTFILE_PRIO) et src/Constants.h.
*/
func TestMappingPriorites(t *testing.T) {
	cases := []struct {
		nom  string
		code uint64
		want Priority
	}{
		{"basse", prLow, PriorityLow},
		{"normale", prNormal, PriorityNormal},
		{"haute", prHigh, PriorityHigh},
		{"très haute", prVeryHigh, PriorityVeryHigh},
		// Quatre et non moins un : la sérialisation d'aMule ne gardait pas le
		// signe, la constante a été déplacée.
		{"très basse", prVeryLow, PriorityVeryLow},
		{"automatique explicite", prAuto, PriorityAuto},

		// Le décalage de dix, cas par cas.
		{"auto, calculée basse", prAutoOffset + prLow, PriorityAuto},
		{"auto, calculée normale", prAutoOffset + prNormal, PriorityAuto},
		{"auto, calculée haute", prAutoOffset + prHigh, PriorityAuto},
		{"auto, calculée très haute", prAutoOffset + prVeryHigh, PriorityAuto},
		{"auto, calculée très basse", prAutoOffset + prVeryLow, PriorityAuto},

		// Le domaine n'a pas de « inconnue » : mieux vaut le vide qu'une
		// « normale » inventée qui ferait taire un vrai écart.
		{"code non attribué", 7, ""},
		{"partage forcé, réservé aux envois", 6, ""},
	}

	for _, c := range cases {
		t.Run(c.nom, func(t *testing.T) {
			if got := mapPartfilePriority(c.code); got != c.want {
				t.Errorf("code %d → %q, attendu %q", c.code, got, c.want)
			}
		})
	}
}

// ─── ETA et parties ──────────────────────────────────────────────────────────

func TestMappingETA(t *testing.T) {
	cases := []struct {
		nom    string
		size   int64
		done   int64
		speed  int64
		status DownloadStatus
		want   *time.Duration
	}{
		{
			nom: "en cours", size: 1000, done: 200, speed: 8,
			status: DownloadDownloading, want: dlPtr(100 * time.Second),
		},
		{
			nom: "débit nul", size: 1000, done: 200, speed: 0,
			status: DownloadDownloading, want: nil,
		},
		{
			nom: "en attente, sans débit", size: 1000, done: 0, speed: 0,
			status: DownloadWaiting, want: nil,
		},
		// Les états où l'attente n'a pas de fin prévisible : rendre une durée
		// obligerait l'interface à savoir qu'elle ne veut rien dire.
		{
			nom: "en pause", size: 1000, done: 200, speed: 5000,
			status: DownloadPaused, want: nil,
		},
		{
			nom: "arrêté", size: 1000, done: 200, speed: 5000,
			status: DownloadStopped, want: nil,
		},
		{
			nom: "terminé", size: 1000, done: 1000, speed: 5000,
			status: DownloadCompleted, want: nil,
		},
		{
			nom: "en erreur", size: 1000, done: 200, speed: 5000,
			status: DownloadErroneous, want: nil,
		},
		// Le démon compte parfois plus d'octets acquis que la taille annoncée
		// le temps d'un tour : une durée négative serait pire que rien.
		{
			nom: "plus rien à recevoir", size: 1000, done: 1000, speed: 10,
			status: DownloadDownloading, want: nil,
		},
	}

	for _, c := range cases {
		t.Run(c.nom, func(t *testing.T) {
			got := partfileETA(c.size, c.done, c.speed, c.status)
			switch {
			case c.want == nil && got != nil:
				t.Errorf("ETA %v rendue, nil attendue", *got)
			case c.want != nil && got == nil:
				t.Errorf("ETA nil rendue, %v attendue", *c.want)
			case c.want != nil && *got != *c.want:
				t.Errorf("ETA %v, attendue %v", *got, *c.want)
			}
		})
	}
}

func TestMappingETANeDebordePasSurUnGrosFichier(t *testing.T) {
	// Calculer en nanosecondes avant de diviser déborderait un entier signé
	// de 64 bits dès neuf gigaoctets restants — une taille banale ici.
	const dixGio = int64(10) << 30

	got := partfileETA(dixGio, 0, 1_000_000, DownloadDownloading)
	if got == nil {
		t.Fatal("ETA nil sur un gros fichier en cours")
	}
	if want := time.Duration(dixGio/1_000_000) * time.Second; *got != want {
		t.Errorf("ETA %v, attendue %v", *got, want)
	}
}

func TestMappingNombreDeParties(t *testing.T) {
	const partSize = int64(ed2kPartSize)

	cases := []struct {
		nom  string
		size int64
		want int
	}{
		{"taille inconnue", 0, 0},
		{"un octet", 1, 1},
		{"une partie moins un octet", partSize - 1, 1},
		// Le rattrapage d'aMule : un multiple exact n'ajoute pas une partie
		// vide. La formule brute (taille / partie + 1) en donnerait deux.
		{"exactement une partie", partSize, 1},
		{"une partie et un octet", partSize + 1, 2},
		{"exactement deux parties", 2 * partSize, 2},
	}

	for _, c := range cases {
		t.Run(c.nom, func(t *testing.T) {
			if got := ed2kPartCount(c.size); got != c.want {
				t.Errorf("taille %d → %d parties, attendu %d", c.size, got, c.want)
			}
		})
	}
}

// ─── Requêtes ────────────────────────────────────────────────────────────────

func TestMappingRequeteDeFile(t *testing.T) {
	req := requestDownloads()

	if req.Op != ec.OpGetDloadQueue {
		t.Errorf("opcode %s, attendu %s", req.Op, ec.OpGetDloadQueue)
	}
	// Le niveau complet n'est pas négociable : les niveaux allégés font
	// retourner amuled avant d'écrire l'empreinte, qui est notre clé.
	level, ok := req.Uint(ec.TagDetailLevel)
	if !ok {
		t.Fatal("aucun niveau de détail demandé")
	}
	if level != uint64(ec.DetailFull) {
		t.Errorf("niveau de détail %d, attendu %d", level, ec.DetailFull)
	}
}

func TestMappingRequeteDeSources(t *testing.T) {
	req := requestSources(dlHashA)

	// Le protocole n'a pas d'opération « sources de ce fichier » : la mise à
	// jour globale est le seul endroit où amuled décrit les pairs un par un.
	if req.Op != ec.OpGetUpdate {
		t.Errorf("opcode %s, attendu %s", req.Op, ec.OpGetUpdate)
	}
	// Et elle n'existe qu'au niveau incrémental : amuled ne répond rien aux
	// autres.
	level, ok := req.Uint(ec.TagDetailLevel)
	if !ok {
		t.Fatal("aucun niveau de détail demandé")
	}
	if level != uint64(ec.DetailIncUpdate) {
		t.Errorf("niveau de détail %d, attendu %d", level, ec.DetailIncUpdate)
	}

	tag, ok := req.Find(ec.TagPartfile)
	if !ok {
		t.Fatal("l'empreinte demandée n'apparaît pas dans la requête")
	}
	raw, ok := tag.Hash()
	if !ok || hex.EncodeToString(raw) != dlHashA {
		t.Errorf("empreinte transmise %x, attendu %s", raw, dlHashA)
	}
}

func TestMappingRequeteDeSourcesIgnoreUneEmpreinteIllisible(t *testing.T) {
	// L'empreinte n'a de toute façon aucun effet côté démon : une saisie
	// fautive ne doit pas produire un tag tronqué qui embrouillerait une
	// capture réseau.
	for _, bad := range []string{"", "pas-hexadecimal", "abcd"} {
		req := requestSources(bad)
		if _, ok := req.Find(ec.TagPartfile); ok {
			t.Errorf("empreinte %q transmise malgré son invalidité", bad)
		}
	}
}

// ─── Sources ─────────────────────────────────────────────────────────────────

func TestDecodeSourcesFileVide(t *testing.T) {
	sources, err := decodeSources(ec.New(ec.OpSharedFiles))
	if err != nil {
		t.Fatalf("aucune source ne doit pas produire d'erreur : %v", err)
	}
	if sources == nil {
		t.Fatal("tranche nil rendue")
	}
	if len(sources) != 0 {
		t.Fatalf("%d sources rendues", len(sources))
	}
}

func TestDecodeSourcesRefuseUnAutreOpcode(t *testing.T) {
	if _, err := decodeSources(ec.New(ec.OpDloadQueue)); err == nil {
		t.Fatal("une file de téléchargement a été acceptée comme liste de pairs")
	}
}

func TestDecodeSourcesTraduitTousLesChamps(t *testing.T) {
	// La mise à jour globale range les pairs sous un tag CONTENEUR qui porte
	// le même nom qu'eux. C'est la forme que le démon envoie réellement.
	packet := ec.New(ec.OpSharedFiles, dlContainerTag(ec.TagClient,
		dlClientTag(100,
			dlHashTag(t, ec.TagClientHash, dlHashB),
			ec.Text(ec.TagClientName, "un-pair"),
			ec.Uint(ec.TagClientSoftware, 3), // aMule
			ec.Text(ec.TagClientSoftVerStr, "2.3.3"),
			ec.Uint(ec.TagClientUserID, 0x0100007F), // au-dessus du seuil : HighID
			ec.Uint(ec.TagClientUserIP, 0x0100007F),
			ec.Uint(ec.TagClientUserPort, 4662),
			ec.Uint(ec.TagClientRemoteQueueRank, 17),
			ec.Uint(ec.TagClientAvailableParts, 12),
			ec.Uint(ec.TagClientDownloadState, dsDownloading),
			dlDoubleTag(ec.TagClientDownSpeed, "12.5"),
			ec.Uint(ec.TagClientRequestFile, 42),
		),
	))

	sources, err := decodeSources(packet)
	if err != nil {
		t.Fatalf("décodage : %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("%d sources rendues", len(sources))
	}
	s := sources[0]

	if s.UserHash != dlHashB {
		t.Errorf("empreinte du pair %q, attendu %q", s.UserHash, dlHashB)
	}
	if s.Name != "un-pair" {
		t.Errorf("nom %q", s.Name)
	}
	if s.Software != "aMule" {
		t.Errorf("logiciel %q", s.Software)
	}
	if s.Version != "2.3.3" {
		t.Errorf("version %q", s.Version)
	}
	if s.LowID {
		t.Error("pair marqué LowID alors que son identifiant est au-dessus du seuil")
	}
	// amuled range l'adresse octet de poids faible en tête : la lire comme un
	// gros-boutiste rendrait 1.0.0.127.
	if s.IP != "127.0.0.1" {
		t.Errorf("adresse %q, attendu 127.0.0.1", s.IP)
	}
	if s.Port != 4662 {
		t.Errorf("port %d", s.Port)
	}
	if s.QueueRank != 17 {
		t.Errorf("rang en file %d", s.QueueRank)
	}
	if s.AvailableParts != 12 {
		t.Errorf("parties détenues %d", s.AvailableParts)
	}
	if !s.Downloading {
		t.Error("pair non marqué comme transférant alors que son état le dit")
	}
	// 12,5 KiO/s en octets par seconde.
	if s.Speed != 12800 {
		t.Errorf("débit %d, attendu 12800", s.Speed)
	}
}

func TestDecodeSourcesMasqueLAdresseDUnPairLowID(t *testing.T) {
	// Un pair en LowID n'a pas d'adresse joignable : publier la sienne
	// laisserait croire qu'on peut l'atteindre.
	packet := ec.New(ec.OpSharedFiles, dlContainerTag(ec.TagClient,
		dlClientTag(1,
			ec.Uint(ec.TagClientUserID, 12345), // sous le seuil : LowID
			ec.Uint(ec.TagClientUserIP, 0x0100007F),
			ec.Uint(ec.TagClientRequestFile, 42),
		),
	))

	sources, err := decodeSources(packet)
	if err != nil {
		t.Fatalf("décodage : %v", err)
	}
	if !sources[0].LowID {
		t.Error("pair non marqué LowID alors que son identifiant est sous le seuil")
	}
	if sources[0].IP != "" {
		t.Errorf("adresse %q publiée pour un pair en LowID", sources[0].IP)
	}
}

func TestDecodeSourcesNInventePasUnTransfert(t *testing.T) {
	// L'état « en train de télécharger » vaut ZÉRO côté démon. Sans le témoin
	// de présence du tag, un pair muet passerait pour actif.
	packet := ec.New(ec.OpSharedFiles, dlContainerTag(ec.TagClient,
		dlClientTag(1,
			ec.Text(ec.TagClientName, "muet"),
			ec.Uint(ec.TagClientRequestFile, 42),
		),
	))

	sources, err := decodeSources(packet)
	if err != nil {
		t.Fatalf("décodage : %v", err)
	}
	if sources[0].Downloading {
		t.Error("pair marqué comme transférant alors qu'il n'a pas envoyé son état")
	}
	if sources[0].Speed != 0 {
		t.Errorf("débit %d inventé", sources[0].Speed)
	}
}

func TestDecodeSourcesLitLaFileDEnvoi(t *testing.T) {
	// La file d'envoi place les tags de pair au PREMIER niveau, sans
	// conteneur. La même fonction doit lire les deux formes.
	packet := ec.New(ec.OpUloadQueue,
		dlClientTag(1,
			ec.Text(ec.TagClientName, "premier-niveau"),
			ec.Uint(ec.TagClientRequestFile, 42),
		),
	)

	sources, err := decodeSources(packet)
	if err != nil {
		t.Fatalf("décodage : %v", err)
	}
	if len(sources) != 1 || sources[0].Name != "premier-niveau" {
		t.Fatalf("sources rendues : %+v", sources)
	}
}

func TestDecodeSourcesEcarteLesPairsSansFichierDemande(t *testing.T) {
	// Un pair à qui nous envoyons seulement ne demande aucun fichier : ce
	// n'est pas une source.
	packet := ec.New(ec.OpSharedFiles, dlContainerTag(ec.TagClient,
		dlClientTag(1, ec.Text(ec.TagClientName, "receveur")),
		dlClientTag(2,
			ec.Text(ec.TagClientName, "receveur-explicite"),
			ec.Uint(ec.TagClientRequestFile, 0),
		),
		dlClientTag(3,
			ec.Text(ec.TagClientName, "source"),
			ec.Uint(ec.TagClientRequestFile, 42),
		),
	))

	sources, err := decodeSources(packet)
	if err != nil {
		t.Fatalf("décodage : %v", err)
	}
	if len(sources) != 1 || sources[0].Name != "source" {
		t.Fatalf("sources rendues : %+v", sources)
	}
}

func TestDecodeSourcesGroupeParFichier(t *testing.T) {
	// Le seul moyen de savoir de quel fichier une source est la source :
	// l'ECID que porte chaque pair. Rendre la table est ce qui permet ensuite
	// de l'apparier aux empreintes de decodeDownloadIDs.
	packet := ec.New(ec.OpSharedFiles, dlContainerTag(ec.TagClient,
		dlClientTag(1, ec.Text(ec.TagClientName, "a"), ec.Uint(ec.TagClientRequestFile, 42)),
		dlClientTag(2, ec.Text(ec.TagClientName, "b"), ec.Uint(ec.TagClientRequestFile, 42)),
		dlClientTag(3, ec.Text(ec.TagClientName, "c"), ec.Uint(ec.TagClientRequestFile, 43)),
	))

	byFile, err := decodeSourcesByFile(packet)
	if err != nil {
		t.Fatalf("décodage : %v", err)
	}
	if len(byFile) != 2 {
		t.Fatalf("%d fichiers dans la table, deux attendus", len(byFile))
	}
	if len(byFile[42]) != 2 {
		t.Errorf("fichier 42 : %d sources, deux attendues", len(byFile[42]))
	}
	if len(byFile[43]) != 1 {
		t.Errorf("fichier 43 : %d sources, une attendue", len(byFile[43]))
	}
}

// ─── Détails de lecture ──────────────────────────────────────────────────────

func TestMappingLogicielDuPair(t *testing.T) {
	cases := map[uint64]string{
		0x00: "eMule",
		0x35: "eMule", // ancien champ d'identification, même logiciel
		0x03: "aMule",
		0x04: "Shareaza",
		0x44: "Shareaza", // successeur du même champ
		0x33: "eDonkey",
		0x99: "", // code non attribué
	}

	for code, want := range cases {
		if got := mapClientSoftware(code); got != want {
			t.Errorf("code 0x%02X → %q, attendu %q", code, got, want)
		}
	}
}

func TestMappingAdresseIPv4(t *testing.T) {
	// L'ordre des octets est celui d'amuled : poids faible en tête.
	cases := map[uint32]string{
		0x0100007F: "127.0.0.1",
		0x0101A8C0: "192.168.1.1",
		0x00000000: "0.0.0.0",
		0xFFFFFFFF: "255.255.255.255",
	}

	for raw, want := range cases {
		if got := dlIPv4(raw); got != want {
			t.Errorf("0x%08X → %q, attendu %q", raw, got, want)
		}
	}
}

func TestMappingLectureDunFlottant(t *testing.T) {
	// amuled écrit les flottants en TEXTE et pose le type « double ». La
	// notation dépend de la bibliothèque C++ d'en face.
	cases := []struct {
		text string
		want float64
	}{
		{"12.5", 12.5},
		{"0", 0},
		{"1.23457e+06", 1234570},
	}

	for _, c := range cases {
		tag := ec.Tag{Children: []ec.Tag{dlDoubleTag(ec.TagClientDownSpeed, c.text)}}
		got, ok := dlChildFloat(tag, ec.TagClientDownSpeed)
		if !ok {
			t.Errorf("%q illisible", c.text)
			continue
		}
		if got != c.want {
			t.Errorf("%q → %v, attendu %v", c.text, got, c.want)
		}
	}

	// Un texte qui n'est pas un nombre ne doit pas passer pour zéro : le zéro
	// est une vitesse plausible, et l'erreur resterait invisible.
	tag := ec.Tag{Children: []ec.Tag{dlDoubleTag(ec.TagClientDownSpeed, "n'importe quoi")}}
	if _, ok := dlChildFloat(tag, ec.TagClientDownSpeed); ok {
		t.Error("un flottant illisible a été accepté")
	}
}

// dlContainerTag fabrique un tag conteneur : sans valeur, ses enfants portent son
// propre nom. C'est ainsi qu'amuled groupe les pairs dans la mise à jour
// globale.
func dlContainerTag(name ec.TagName, children ...ec.Tag) ec.Tag {
	tag := ec.Empty(name)
	tag.Children = children
	return tag
}

func dlPtr[T any](v T) *T { return &v }
