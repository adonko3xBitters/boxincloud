package amule

import (
	"context"
	"fmt"
	"strings"

	"github.com/adonko3xBitters/boxincloud/server/internal/amule/ec"
)

/*
Les journaux du démon.

amuled tient deux journaux distincts, et la nuance compte : celui d'exploitation
— connexions, serveurs, fichiers terminés — et celui de mise au point, beaucoup
plus bavard et rarement utile à qui n'écrit pas d'aMule.

Seul le premier est exposé. Le second est disponible par le même protocole, mais
l'afficher noierait ce qu'on vient y chercher.

# Ils ne sont pas conservés

Le démon garde une fenêtre glissante, et la perd à chaque redémarrage. On ne
recopie rien en base à cette étape : ce serait une table qui grossit sans borne
pour un contenu qu'on consulte deux fois par an, et l'archivage a une réponse
propre — la journalisation du serveur lui-même — le jour où le besoin se
présentera vraiment.
*/

// LogLine est une ligne du journal du démon.
type LogLine struct {
	// Text est la ligne telle que le démon l'a écrite, horodatage compris.
	//
	// Non découpée : le format n'est pas stable d'une version à l'autre, et une
	// analyse qui se tromperait rendrait des lignes tronquées plutôt qu'un
	// texte brut parfaitement lisible.
	Text string
}

// Logs rend le journal d'exploitation, le plus ancien d'abord.
func (s *Service) Logs(ctx context.Context) ([]LogLine, error) {
	resp, err := s.query(ctx, ec.New(ec.OpGetLog))
	if err != nil {
		return nil, err
	}
	return decodeLogs(resp)
}

// ClearLogs vide le journal du démon.
func (s *Service) ClearLogs(ctx context.Context) error {
	return s.do(ctx, ec.New(ec.OpResetLog))
}

/*
decodeLogs traduit la réponse.

Le démon envoie le journal en UN SEUL tag, dont la valeur porte toutes les
lignes séparées par des sauts de ligne — et non un tag par ligne, comme on
l'attendrait. Rendre ce tag tel quel donnerait une « ligne » de trente
kilo-octets, illisible dans n'importe quel tableau.

Le découpage se fait donc ici. Il tolère plusieurs tags au cas où une version
d'aMule changerait d'avis : chacun est découpé de la même façon, et le résultat
reste le même.

Ni horodatage structuré ni niveau : le démon n'en fournit pas, et une analyse du
préfixe se tromperait au premier changement de format. Le texte brut est
parfaitement lisible.
*/
func decodeLogs(p ec.Packet) ([]LogLine, error) {
	if p.Op != ec.OpLog {
		return nil, fmt.Errorf("réponse %s, attendu %s", p.Op, ec.OpLog)
	}

	var lines []LogLine
	for _, tag := range p.Tags {
		text, ok := tag.Text()
		if !ok {
			continue
		}
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimRight(line, "\r")
			if strings.TrimSpace(line) == "" {
				continue
			}
			lines = append(lines, LogLine{Text: line})
		}
	}
	return lines, nil
}
