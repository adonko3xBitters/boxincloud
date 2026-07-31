package archive

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"strings"
)

/*
Lecture d'un EPUB de bande dessinée.

Un EPUB est un ZIP, et on pourrait donc croire qu'il relève du chemin du CBZ.
C'est faux, et le piège mérite d'être nommé : **l'ordre de lecture d'un EPUB
n'est pas l'ordre de ses noms de fichiers.** Il est défini par le `spine` du
document OPF, et rien n'oblige un éditeur à nommer ses images dans cet ordre —
beaucoup ne le font pas.

Indexer un EPUB comme un CBZ donnerait donc un album complet, lisible, et dans
le désordre. C'est la pire des pannes : elle ne ressemble pas à une panne.

D'où l'hydratation, qui suit le spine et renomme les images dans l'ordre qu'il
prescrit. Le CBZ produit se trie alors correctement par nom.

La contrepartie est nommée : un EPUB de texte — un roman — n'a rien à extraire
que des images de couverture et d'illustration. Il est refusé explicitement.
Ce lecteur sert les bandes dessinées, pas les livres.
*/

// container décrit META-INF/container.xml, qui désigne le document OPF.
type container struct {
	Rootfiles []struct {
		FullPath string `xml:"full-path,attr"`
	} `xml:"rootfiles>rootfile"`
}

// opf décrit le document de package : ce que l'EPUB contient, et dans quel ordre.
type opf struct {
	Manifest []struct {
		ID   string `xml:"id,attr"`
		Href string `xml:"href,attr"`
		Type string `xml:"media-type,attr"`
	} `xml:"manifest>item"`

	Spine []struct {
		IDRef string `xml:"idref,attr"`
	} `xml:"spine>itemref"`
}

/*
WalkEPUB parcourt les images d'un EPUB, dans l'ordre du spine.

Les noms émis portent leur rang sur quatre chiffres : c'est lui qui déterminera
l'ordre de lecture après hydratation, l'index du CBZ triant par ordre naturel
des noms.
*/
func WalkEPUB(r io.ReaderAt, size int64, visit func(ExtractedEntry) error) error {
	reader, err := zip.NewReader(r, size)
	if err != nil {
		return fmt.Errorf("%w : %v", ErrCorrupted, err)
	}

	files := make(map[string]*zip.File, len(reader.File))
	for _, file := range reader.File {
		files[path.Clean(file.Name)] = file
	}

	pkg, base, err := readPackage(files)
	if err != nil {
		return err
	}

	// Le manifeste donne le chemin de chaque ressource ; le spine donne l'ordre.
	// Les deux sont nécessaires : le premier seul n'a pas d'ordre, le second
	// seul n'a pas de chemins.
	hrefs := make(map[string]string, len(pkg.Manifest))
	types := make(map[string]string, len(pkg.Manifest))
	for _, item := range pkg.Manifest {
		hrefs[item.ID] = item.Href
		types[item.ID] = item.Type
	}

	found := 0

	for _, ref := range pkg.Spine {
		href, ok := hrefs[ref.IDRef]
		if !ok {
			continue
		}

		// Une page d'EPUB fixe est le plus souvent un document XHTML qui ne
		// contient qu'une image. On suit donc le spine jusqu'à l'image, en
		// acceptant qu'un item du spine soit lui-même une image — les EPUB de
		// bande dessinée les plus simples sont bâtis ainsi.
		target := path.Join(base, href)

		if !IsImage(target) {
			target, ok = imageInDocument(files, target)
			if !ok {
				continue
			}
		}

		file, ok := files[path.Clean(target)]
		if !ok {
			continue
		}

		entry, err := file.Open()
		if err != nil {
			return fmt.Errorf("%w : %v", ErrCorrupted, err)
		}

		found++
		name := fmt.Sprintf("page-%04d%s", found, path.Ext(target))
		err = visit(ExtractedEntry{Name: name, Reader: entry})
		_ = entry.Close()
		if err != nil {
			return err
		}
	}

	if found == 0 {
		return fmt.Errorf("%w : cet EPUB ne contient aucune image dans son ordre "+
			"de lecture (roman plutôt que bande dessinée ?)", ErrNoPages)
	}
	return nil
}

// readPackage lit le document OPF et retourne son répertoire de base.
//
// Les chemins du manifeste sont relatifs à l'OPF, pas à la racine de l'archive.
// L'oublier donne des chemins qui n'existent pas, et un album vide.
func readPackage(files map[string]*zip.File) (*opf, string, error) {
	entry, ok := files["META-INF/container.xml"]
	if !ok {
		return nil, "", fmt.Errorf("%w : container.xml absent", ErrCorrupted)
	}

	var root container
	if err := readXML(entry, &root); err != nil {
		return nil, "", err
	}
	if len(root.Rootfiles) == 0 {
		return nil, "", fmt.Errorf("%w : aucun document de package", ErrCorrupted)
	}

	opfPath := path.Clean(root.Rootfiles[0].FullPath)
	entry, ok = files[opfPath]
	if !ok {
		return nil, "", fmt.Errorf("%w : document de package introuvable", ErrCorrupted)
	}

	var pkg opf
	if err := readXML(entry, &pkg); err != nil {
		return nil, "", err
	}

	return &pkg, path.Dir(opfPath), nil
}

/*
imageInDocument trouve l'image d'une page XHTML.

Une lecture naïve du balisage plutôt qu'un analyseur HTML : on cherche le
premier `src` ou `xlink:href` qui désigne une image, ce qui suffit pour une page
d'EPUB fixe — elle ne contient qu'elle. Un vrai analyseur coûterait une
dépendance pour traiter des cas qui ne se présentent pas dans une bande
dessinée.
*/
func imageInDocument(files map[string]*zip.File, docPath string) (string, bool) {
	entry, ok := files[path.Clean(docPath)]
	if !ok {
		return "", false
	}

	reader, err := entry.Open()
	if err != nil {
		return "", false
	}
	defer func() { _ = reader.Close() }()

	// Une page d'EPUB fixe pèse quelques kilo-octets : la borne protège d'un
	// document délibérément énorme sans jamais gêner le cas normal.
	body, err := io.ReadAll(io.LimitReader(reader, 1<<20))
	if err != nil {
		return "", false
	}

	base := path.Dir(docPath)
	for _, attribute := range []string{`src="`, `xlink:href="`, `href="`} {
		rest := string(body)
		for {
			at := strings.Index(rest, attribute)
			if at < 0 {
				break
			}
			rest = rest[at+len(attribute):]

			end := strings.IndexByte(rest, '"')
			if end < 0 {
				break
			}

			value := rest[:end]
			if IsImage(value) {
				return path.Join(base, value), true
			}
		}
	}

	return "", false
}

func readXML(file *zip.File, into any) error {
	reader, err := file.Open()
	if err != nil {
		return fmt.Errorf("%w : %v", ErrCorrupted, err)
	}
	defer func() { _ = reader.Close() }()

	// Les EPUB déclarent des espaces de noms que le décodeur strict refuse ;
	// on ne lit que des attributs simples, la tolérance ne coûte rien.
	decoder := xml.NewDecoder(reader)
	decoder.Strict = false

	if err := decoder.Decode(into); err != nil {
		return fmt.Errorf("%w : %v", ErrCorrupted, err)
	}
	return nil
}
