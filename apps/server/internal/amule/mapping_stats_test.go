package amule

import (
	"encoding/hex"
	"testing"

	"github.com/adonko3xBitters/boxincloud/server/internal/amule/ec"
)

/*
Ces tests figent la TRADUCTION, pas le protocole.

Les paquets y sont construits à la main, ce qui permet de fabriquer des cas
qu'un démon de test ne produira jamais : un pair banni au milieu d'une file
d'envoi, un logiciel client inconnu, une priorité automatique. La contrepartie
est qu'ils ne prouvent rien sur la forme réelle des réponses — c'est le rôle des
tests d'intégration du fichier voisin.
*/

// hashTag fabrique un tag d'empreinte à partir de sa graphie hexadécimale.
func hashTag(t *testing.T, name ec.TagName, hexDigits string) ec.Tag {
	t.Helper()

	raw, err := hex.DecodeString(hexDigits)
	if err != nil {
		t.Fatalf("empreinte de test illisible : %v", err)
	}
	if len(raw) != 16 {
		t.Fatalf("empreinte de test sur %d octets, attendu 16", len(raw))
	}
	return ec.Hash(name, raw)
}

// ─── Statistiques ────────────────────────────────────────────────────────────

// TestDecodeStatsRefuseUnAutreOpcode : le protocole n'a pas d'identifiant de
// corrélation, et un paquet décalé d'un cran doit se voir plutôt que se
// traduire en compteurs à zéro.
func TestDecodeStatsRefuseUnAutreOpcode(t *testing.T) {
	if _, err := decodeStats(ec.New(ec.OpUloadQueue)); err == nil {
		t.Fatal("une réponse d'un autre opcode a été acceptée")
	}
}

/*
TestDecodeStatsDebitsEnOctetsSansConversion est le test de l'unité.

Le démon envoie déjà des octets par seconde, y compris pour les plafonds, qui se
configurent pourtant en kilo-octets. Un décodeur qui « corrigerait » l'unité
rendrait ici 1024 fois trop.

Les quatre largeurs d'entier sont exercées au passage : amuled écrit chaque
valeur sur le plus petit type qui la contienne, et un même compteur arrive donc
sur un, deux, quatre ou huit octets selon le moment de la journée.
*/
func TestDecodeStatsDebitsEnOctetsSansConversion(t *testing.T) {
	const (
		upSpeed   = 200        // tient sur un octet
		downSpeed = 40_000     // deux octets
		upLimit   = 20 * 1024  // 20 ko/s configurés, déjà convertis par le démon
		downLimit = 300 * 1024 // quatre octets
	)

	p := ec.New(ec.OpStats,
		ec.Uint(ec.TagStatsUlSpeed, upSpeed),
		ec.Uint(ec.TagStatsDlSpeed, downSpeed),
		ec.Uint(ec.TagStatsUlSpeedLimit, upLimit),
		ec.Uint(ec.TagStatsDlSpeedLimit, downLimit),
	)

	stats, err := decodeStats(p)
	if err != nil {
		t.Fatalf("décodage refusé : %v", err)
	}

	if stats.UpSpeed != upSpeed {
		t.Errorf("débit montant : %d, attendu %d", stats.UpSpeed, upSpeed)
	}
	if stats.DownSpeed != downSpeed {
		t.Errorf("débit descendant : %d, attendu %d", stats.DownSpeed, downSpeed)
	}
	if stats.UpLimit != upLimit {
		t.Errorf("plafond montant : %d octets/s, attendu %d — l'unité a été convertie deux fois",
			stats.UpLimit, upLimit)
	}
	if stats.DownLimit != downLimit {
		t.Errorf("plafond descendant : %d octets/s, attendu %d", stats.DownLimit, downLimit)
	}
}

// TestDecodeStatsCompteurs couvre les champs que seul le niveau de détail
// complet fournit, ainsi qu'un compteur assez grand pour occuper huit octets.
func TestDecodeStatsCompteurs(t *testing.T) {
	const kadFiles = 5_000_000_000 // au-delà de quatre milliards : huit octets

	p := ec.New(ec.OpStats,
		ec.Uint(ec.TagStatsUpOverhead, 1_500),
		ec.Uint(ec.TagStatsDownOverhead, 900),
		ec.Uint(ec.TagStatsBannedCount, 3),
		ec.Uint(ec.TagStatsTotalSrcCount, 412),
		ec.Uint(ec.TagStatsUlQueueLen, 17),
		ec.Uint(ec.TagStatsEd2kUsers, 1_200_000),
		ec.Uint(ec.TagStatsKadUsers, 800_000),
		ec.Uint(ec.TagStatsEd2kFiles, 250_000_000),
		ec.Uint(ec.TagStatsKadFiles, kadFiles),
	)

	stats, err := decodeStats(p)
	if err != nil {
		t.Fatalf("décodage refusé : %v", err)
	}

	if stats.UpOverhead != 1_500 || stats.DownOverhead != 900 {
		t.Errorf("surdébit : %d montant, %d descendant", stats.UpOverhead, stats.DownOverhead)
	}
	if stats.BannedPeers != 3 {
		t.Errorf("pairs bannis : %d, attendu 3", stats.BannedPeers)
	}
	if stats.TotalSources != 412 {
		t.Errorf("sources : %d, attendu 412", stats.TotalSources)
	}
	if stats.UploadQueueLength != 17 {
		t.Errorf("file d'envoi : %d, attendu 17", stats.UploadQueueLength)
	}
	if stats.Ed2kUsers != 1_200_000 || stats.KadUsers != 800_000 {
		t.Errorf("utilisateurs : %d eD2k, %d Kad", stats.Ed2kUsers, stats.KadUsers)
	}
	if stats.Ed2kFiles != 250_000_000 || stats.KadFiles != kadFiles {
		t.Errorf("fichiers : %d eD2k, %d Kad", stats.Ed2kFiles, stats.KadFiles)
	}
}

/*
TestDecodeStatsIgnoreEtatDeConnexion vérifie la frontière entre deux décodeurs.

Le démon agrafe l'état de connexion à toute réponse aux statistiques, et ce tag
est imbriqué : il porte le serveur joint en sous-tag. Le laisser traverser ce
décodeur sans effet est ce qui garantit qu'il n'existe qu'une seule source de
vérité pour l'état — celle de decodeConnection.
*/
func TestDecodeStatsIgnoreEtatDeConnexion(t *testing.T) {
	connState := ec.Uint(ec.TagConnstate, 0x01|0x04)
	connState.Children = []ec.Tag{
		ec.Text(ec.TagServerName, "un serveur"),
	}

	p := ec.New(ec.OpStats,
		ec.Uint(ec.TagStatsUlSpeed, 42),
		connState,
		// Le journal : un tag imbriqué de chaînes, sans valeur entière.
		ec.Tag{Name: ec.TagStatsLoggerMessage, Type: ec.TypeCustom, Children: []ec.Tag{
			ec.Text(ec.TagString, "une ligne de journal"),
		}},
	)

	stats, err := decodeStats(p)
	if err != nil {
		t.Fatalf("un tag imbriqué a fait échouer le décodage : %v", err)
	}
	if stats.UpSpeed != 42 {
		t.Errorf("débit montant : %d, attendu 42", stats.UpSpeed)
	}
	if (stats != Stats{UpSpeed: 42}) {
		t.Errorf("des champs ont été renseignés hors des compteurs : %+v", stats)
	}
}

// TestDecodeStatsTagsAbsents : une réponse dépouillée n'est pas une erreur.
func TestDecodeStatsTagsAbsents(t *testing.T) {
	stats, err := decodeStats(ec.New(ec.OpStats))
	if err != nil {
		t.Fatalf("une réponse sans tag a été refusée : %v", err)
	}
	if (stats != Stats{}) {
		t.Errorf("compteurs inventés à partir de rien : %+v", stats)
	}
}

// ─── Envois ──────────────────────────────────────────────────────────────────

// clientTag fabrique un tag de pair tel qu'amuled l'écrit dans la file d'envoi.
func clientTag(children ...ec.Tag) ec.Tag {
	return ec.Tag{
		Name:     ec.TagClient,
		Type:     ec.TypeUint32,
		Value:    []byte{0, 0, 0, 7}, // numéro interne du pair, sans usage ici
		Children: children,
	}
}

func TestDecodeUploadsRefuseUnAutreOpcode(t *testing.T) {
	if _, _, err := decodeUploads(ec.New(ec.OpStats)); err == nil {
		t.Fatal("une réponse d'un autre opcode a été acceptée")
	}
}

/*
TestDecodeUploadsSepareTransfertsEtAttente est le test du critère.

Les deux populations arrivent sous des tags identiques, dans le même paquet.
Seul l'état d'envoi les distingue : zéro pour un transfert en cours, tout autre
code pour un pair qui occupe une place sans rien recevoir.

Le pair en attente porte ici un débit non nul — cas absurde mais possible juste
après la perte de son créneau. Il vérifie qu'on trie bien sur l'état et non sur
le débit.
*/
func TestDecodeUploadsSepareTransfertsEtAttente(t *testing.T) {
	p := ec.New(ec.OpUloadQueue,
		clientTag(
			ec.Text(ec.TagClientName, "en cours"),
			ec.Uint(ec.TagClientUploadState, 0), // US_UPLOADING
			ec.Uint(ec.TagClientUpSpeed, 0),     // créneau tout juste obtenu
		),
		clientTag(
			ec.Text(ec.TagClientName, "en file"),
			ec.Uint(ec.TagClientUploadState, 1), // US_ONUPLOADQUEUE
			ec.Uint(ec.TagClientUpSpeed, 4_096),
			ec.Uint(ec.TagClientScore, 780),
		),
		clientTag(
			ec.Text(ec.TagClientName, "banni"),
			ec.Uint(ec.TagClientUploadState, 6), // US_BANNED
		),
	)

	uploads, queued, err := decodeUploads(p)
	if err != nil {
		t.Fatalf("décodage refusé : %v", err)
	}

	if len(uploads) != 1 || uploads[0].Name != "en cours" {
		t.Fatalf("transferts : %+v, attendu le seul pair en état d'envoi", uploads)
	}
	if len(queued) != 2 {
		t.Fatalf("file d'attente : %d pairs, attendu 2", len(queued))
	}
	if queued[0].Name != "en file" || queued[1].Name != "banni" {
		t.Errorf("ordre du démon perdu : %+v", queued)
	}
	if queued[0].Score != 780 {
		t.Errorf("note du pair en attente : %d, attendu 780", queued[0].Score)
	}
}

// TestDecodeUploadsEtatAbsent : sans tag d'état, le pair compte comme un
// transfert. C'est la valeur nulle du code, et c'est aussi ce que cette réponse
// contient majoritairement.
func TestDecodeUploadsEtatAbsent(t *testing.T) {
	p := ec.New(ec.OpUloadQueue,
		clientTag(ec.Text(ec.TagClientName, "sans état")),
	)

	uploads, queued, err := decodeUploads(p)
	if err != nil {
		t.Fatalf("décodage refusé : %v", err)
	}
	if len(uploads) != 1 || len(queued) != 0 {
		t.Fatalf("%d transferts et %d attentes, attendu 1 et 0", len(uploads), len(queued))
	}
}

// TestDecodeUploadsListesVides : une file d'envoi vide se décode sans erreur et
// sans inventer d'entrée. C'est l'état normal d'un démon au repos.
func TestDecodeUploadsListesVides(t *testing.T) {
	uploads, queued, err := decodeUploads(ec.New(ec.OpUloadQueue))
	if err != nil {
		t.Fatalf("une réponse vide a été refusée : %v", err)
	}
	if len(uploads) != 0 || len(queued) != 0 {
		t.Errorf("%d transferts et %d attentes tirés d'une réponse vide", len(uploads), len(queued))
	}
}

/*
TestDecodeUploadChamps couvre la traduction d'un transfert complet.

Deux détails s'y jouent, et chacun produirait une donnée plausible et fausse :

  - l'empreinte du pair est rendue en hexadécimal minuscule ;
  - l'adresse se lit à l'envers de l'intuition, l'octet de poids faible en tête.
*/
func TestDecodeUploadChamps(t *testing.T) {
	const userHash = "0a1b2c3d4e5f60718293a4b5c6d7e8f9"

	// 192.168.1.10 sous la convention du démon : 192 en poids faible.
	const ip = 192 | 168<<8 | 1<<16 | 10<<24

	p := ec.New(ec.OpUloadQueue,
		clientTag(
			hashTag(t, ec.TagClientHash, userHash),
			ec.Text(ec.TagClientName, "un pair"),
			ec.Uint(ec.TagClientSoftware, 3), // aMule
			ec.Text(ec.TagClientSoftVerStr, "2.3.3"),
			ec.Uint(ec.TagClientUserIP, ip),
			ec.Uint(ec.TagClientUserPort, 4662),
			ec.Text(ec.TagPartfileName, "album.cbz"),
			ec.Uint(ec.TagClientUpSpeed, 51_200),
			ec.Uint(ec.TagClientUploadSession, 3_000_000),
			ec.Uint(ec.TagClientUploadTotal, 90_000_000),
		),
	)

	uploads, _, err := decodeUploads(p)
	if err != nil {
		t.Fatalf("décodage refusé : %v", err)
	}
	if len(uploads) != 1 {
		t.Fatalf("%d transferts, attendu 1", len(uploads))
	}
	up := uploads[0]

	if up.UserHash != userHash {
		t.Errorf("empreinte : %q, attendu %q", up.UserHash, userHash)
	}
	if up.IP != "192.168.1.10" {
		t.Errorf("adresse : %q, attendu 192.168.1.10 — les octets ont été lus à l'envers", up.IP)
	}
	if up.Port != 4662 {
		t.Errorf("port : %d, attendu 4662", up.Port)
	}
	if up.Software != "aMule" || up.Version != "2.3.3" {
		t.Errorf("logiciel : %q %q, attendu aMule 2.3.3", up.Software, up.Version)
	}
	if up.FileName != "album.cbz" {
		t.Errorf("fichier servi : %q, attendu album.cbz", up.FileName)
	}
	if up.Speed != 51_200 {
		t.Errorf("débit : %d octets/s, attendu 51200", up.Speed)
	}
	if up.SessionUploaded != 3_000_000 || up.Transferred != 3_000_000 {
		t.Errorf("envoi de session : %d, transféré : %d, attendu 3000000 pour les deux",
			up.SessionUploaded, up.Transferred)
	}
	if up.TotalUploaded != 90_000_000 {
		t.Errorf("cumul envoyé : %d, attendu 90000000", up.TotalUploaded)
	}
	if up.FileHash != "" {
		t.Errorf("empreinte de fichier %q inventée : le démon ne l'envoie pas", up.FileHash)
	}
}

// TestMappingLogicielClient couvre la table des logiciels, y compris sa règle de
// repli : un code hors table est rendu en clair plutôt que traduit par un mot
// qui ne dit rien.
func TestMappingLogicielClient(t *testing.T) {
	cases := []struct {
		code uint64
		want string
	}{
		{0x00, "eMule"},
		{0x35, "eMule"}, // ancien schéma de version
		{0x03, "aMule"},
		{0x04, "Shareaza"},
		{0x44, "Shareaza"}, // nouveau code, même programme
		{0x0a, "MLDonkey"},
		{0x33, "eDonkey"},
		{0xff, "eMule compatible"},
		{0x36, "54"}, // « inconnu » chez amuled : on garde le code
		{42, "42"},   // jamais vu : idem
	}

	for _, c := range cases {
		if got := clientSoftwareName(c.code); got != c.want {
			t.Errorf("code %d : %q, attendu %q", c.code, got, c.want)
		}
	}
}

// ─── Fichiers partagés ───────────────────────────────────────────────────────

// knownfileTag fabrique un tag de fichier partagé.
func knownfileTag(children ...ec.Tag) ec.Tag {
	return ec.Tag{
		Name:     ec.TagKnownfile,
		Type:     ec.TypeUint32,
		Value:    []byte{0, 0, 0, 12}, // numéro interne du fichier
		Children: children,
	}
}

func TestDecodeSharedFilesRefuseUnAutreOpcode(t *testing.T) {
	if _, err := decodeSharedFiles(ec.New(ec.OpStats)); err == nil {
		t.Fatal("une réponse d'un autre opcode a été acceptée")
	}
}

// TestDecodeSharedFilesListeVide : aucun partage n'est un état normal, pas une
// erreur.
func TestDecodeSharedFilesListeVide(t *testing.T) {
	files, err := decodeSharedFiles(ec.New(ec.OpSharedFiles))
	if err != nil {
		t.Fatalf("une réponse vide a été refusée : %v", err)
	}
	if len(files) != 0 {
		t.Errorf("%d fichiers tirés d'une réponse vide", len(files))
	}
}

/*
TestDecodeSharedFileChamps couvre un partage complet.

Les compteurs sont pris en CUMUL et non en session : le paquet porte les deux, et
lire les mauvais donnerait des chiffres qui retombent à zéro à chaque
redémarrage du démon.
*/
func TestDecodeSharedFileChamps(t *testing.T) {
	const fileHash = "aabbccddeeff00112233445566778899"

	p := ec.New(ec.OpSharedFiles,
		knownfileTag(
			hashTag(t, ec.TagPartfileHash, fileHash),
			ec.Text(ec.TagPartfileName, "tome-01.cbz"),
			ec.Uint(ec.TagPartfileSizeFull, 180_000_000),
			ec.Text(ec.TagKnownfileFilename, "/downloads/incoming"),
			ec.Uint(ec.TagKnownfilePrio, sharePrioHigh),

			// Compteurs de session : ils ne doivent PAS être retenus.
			ec.Uint(ec.TagKnownfileReqCount, 2),
			ec.Uint(ec.TagKnownfileAcceptCount, 1),
			ec.Uint(ec.TagKnownfileXferred, 1_000),

			ec.Uint(ec.TagKnownfileReqCountAll, 340),
			ec.Uint(ec.TagKnownfileAcceptCountAll, 120),
			ec.Uint(ec.TagKnownfileXferredAll, 4_000_000_000),
		),
	)

	files, err := decodeSharedFiles(p)
	if err != nil {
		t.Fatalf("décodage refusé : %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("%d fichiers, attendu 1", len(files))
	}
	f := files[0]

	if f.Hash != fileHash {
		t.Errorf("empreinte : %q, attendu %q", f.Hash, fileHash)
	}
	if f.Name != "tome-01.cbz" || f.Size != 180_000_000 {
		t.Errorf("nom %q, taille %d", f.Name, f.Size)
	}
	if f.Path != "/downloads/incoming" {
		t.Errorf("chemin : %q", f.Path)
	}
	if f.Priority != PriorityHigh {
		t.Errorf("priorité : %q, attendu %q", f.Priority, PriorityHigh)
	}
	if f.Requests != 340 || f.Accepted != 120 || f.Transferred != 4_000_000_000 {
		t.Errorf("compteurs de session lus au lieu des cumuls : %d/%d/%d",
			f.Requests, f.Accepted, f.Transferred)
	}
	if !f.Complete {
		t.Error("un fichier rangé dans un vrai répertoire a été pris pour un téléchargement en cours")
	}
}

/*
TestDecodeSharedFileCompletude couvre le seul indice disponible.

Aucun tag ne dit si un fichier est entier. Le démon envoie, à la place du
chemin, le nom du fichier de suivi d'un téléchargement en cours : un numéro
d'ordre suivi de « .part ». Le contrôle porte sur les deux moitiés — un vrai
partage nommé « mon film.part » ne doit pas être pris pour un téléchargement.
*/
func TestDecodeSharedFileCompletude(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/home/amule/Incoming", true},
		{"/downloads/incoming", true},
		{"012.part", false},     // fichier de suivi : téléchargement en cours
		{"7.part", false},       // numéro à un chiffre
		{"mon film.part", true}, // vrai partage dont le nom finit en .part
		{".part", true},         // pas de numéro : ce n'est pas un fichier de suivi
		{"012.part.met", true},  // le démon retire toujours l'extension .met
		{"", false},             // chemin absent : on n'affirme rien
	}

	for _, c := range cases {
		p := ec.New(ec.OpSharedFiles,
			knownfileTag(ec.Text(ec.TagKnownfileFilename, c.path)),
		)

		files, err := decodeSharedFiles(p)
		if err != nil {
			t.Fatalf("décodage refusé pour %q : %v", c.path, err)
		}
		if files[0].Complete != c.want {
			t.Errorf("chemin %q : complet=%v, attendu %v", c.path, files[0].Complete, c.want)
		}
	}
}

// TestMappingPrioritePartage couvre la table des priorités et son décalage.
//
// Le décalage de dix est la partie qui ne se devine pas : il ne code pas une
// priorité de plus mais un MODE, celui où le démon choisit lui-même.
func TestMappingPrioritePartage(t *testing.T) {
	cases := []struct {
		code uint64
		want Priority
	}{
		{0, PriorityLow},
		{1, PriorityNormal},
		{2, PriorityHigh},
		{3, PriorityVeryHigh},
		{4, PriorityVeryLow},  // hors ordre : « très basse » vaut plus que « très haute »
		{5, PriorityAuto},     // code direct
		{6, PriorityVeryHigh}, // « diffusion », sans équivalent au domaine
		{10, PriorityAuto},    // basse, gérée par le démon
		{11, PriorityAuto},
		{12, PriorityAuto},
		{99, PriorityAuto},     // décalage sans code connu : reste « automatique »
		{7, PriorityNormal},    // hors table : repli du démon lui-même
		{0xFF, PriorityNormal}, // au-delà d'un octet, mais sous le décalage ? non : > 10
	}

	for _, c := range cases {
		// 0xFF dépasse le décalage : le cas est corrigé ici plutôt que dans la
		// table, pour que la lecture de la table reste celle des codes réels.
		want := c.want
		if c.code >= sharePrioAutoOffset {
			want = PriorityAuto
		}
		if got := sharedFilePriority(c.code); got != want {
			t.Errorf("code %d : %q, attendu %q", c.code, got, want)
		}
	}
}

// TestDecodeSharedFilePrioriteAbsente : le tag manquant vaut « normale », comme
// pour un fichier fraîchement partagé.
func TestDecodeSharedFilePrioriteAbsente(t *testing.T) {
	files, err := decodeSharedFiles(ec.New(ec.OpSharedFiles, knownfileTag()))
	if err != nil {
		t.Fatalf("décodage refusé : %v", err)
	}
	if files[0].Priority != PriorityNormal {
		t.Errorf("priorité : %q, attendu %q", files[0].Priority, PriorityNormal)
	}
}
