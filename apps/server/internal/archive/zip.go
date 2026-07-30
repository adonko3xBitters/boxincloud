package archive

import (
	"compress/flate"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"

	"golang.org/x/sync/errgroup"

	"github.com/adonko3xBitters/boxincloud/server/internal/storage"
)

// Signatures et tailles fixes du format ZIP (APPNOTE.TXT, section 4.3).
const (
	sigEOCD        = 0x06054b50 // End Of Central Directory
	sigEOCD64      = 0x06064b50 // ZIP64 End Of Central Directory
	sigEOCD64Loc   = 0x07064b50 // ZIP64 EOCD Locator
	sigCentralFile = 0x02014b50 // Central Directory File Header
	sigLocalFile   = 0x04034b50 // Local File Header

	eocdMinSize      = 22
	eocd64LocSize    = 20
	eocd64MinSize    = 56
	centralHdrSize   = 46
	localHdrSize     = 30
	maxZipCommentLen = 65535

	// Un premier essai sur la fin du fichier suffit dans l'immense majorité des
	// cas : un CBZ n'a pas de commentaire ZIP. On ne lit les 64 Ko complets que
	// si l'EOCD n'y est pas.
	eocdProbeSize = 4 * 1024

	// Concurrence de lecture des en-têtes locaux. Chaque lecture ne fait que
	// 30 octets, mais elles sont dispersées dans le fichier : c'est la latence
	// réseau qui domine, d'où le parallélisme.
	localHeaderConcurrency = 12
)

// zipEnd décrit la fin de l'archive : où trouver le répertoire central.
type zipEnd struct {
	entries  int64
	cdOffset int64
	cdSize   int64
}

// centralEntry est une entrée du répertoire central, avant résolution de
// l'offset de données.
type centralEntry struct {
	name        string
	compression Compression
	compSize    int64
	uncompSize  int64
	localOffset int64
}

// ReadZipIndex analyse une archive ZIP distante et retourne l'index de ses
// pages.
//
// ★ Fonction centrale du projet.
//
// Le format ZIP place son index en fin de fichier, et compresse chaque entrée
// indépendamment. On peut donc :
//
//  1. lire la fin du fichier pour localiser le répertoire central ;
//  2. lire le répertoire central pour connaître toutes les entrées ;
//  3. lire l'en-tête local de chaque page pour calculer l'offset de ses données.
//
// Le résultat est persisté en base (table comic_pages). Servir une page ne
// coûte ensuite plus qu'un seul ReadRange — c'est ce qui rend le stockage objet
// viable pour de la lecture de bande dessinée.
//
// Coût total de l'indexation : 2 requêtes + 1 par page, une seule fois dans la
// vie du fichier.
func ReadZipIndex(ctx context.Context, p storage.Provider, key string, size int64) (*Index, error) {
	if size < eocdMinSize {
		return nil, fmt.Errorf("%w : fichier trop court (%d octets)", ErrCorrupted, size)
	}

	end, err := readZipEnd(ctx, p, key, size)
	if err != nil {
		return nil, err
	}

	entries, err := readCentralDirectory(ctx, p, key, end)
	if err != nil {
		return nil, err
	}

	var (
		pages     []centralEntry
		comicInfo *centralEntry
	)
	for i := range entries {
		e := &entries[i]
		switch {
		case isJunk(e.name):
			continue
		case IsComicInfo(e.name):
			if comicInfo == nil {
				comicInfo = e
			}
		case IsImage(e.name):
			pages = append(pages, *e)
		}
	}

	if len(pages) == 0 {
		return nil, fmt.Errorf("%w dans %s", ErrNoPages, key)
	}

	// L'ordre de lecture est l'ordre naturel des noms, pas l'ordre de stockage
	// dans l'archive — qui n'a aucune raison d'être le bon.
	sort.SliceStable(pages, func(i, j int) bool {
		return compareNatural(pages[i].name, pages[j].name) < 0
	})

	toResolve := pages
	if comicInfo != nil {
		toResolve = append(append([]centralEntry(nil), pages...), *comicInfo)
	}

	resolved, err := resolveDataOffsets(ctx, p, key, toResolve, size)
	if err != nil {
		return nil, err
	}

	idx := &Index{Pages: resolved[:len(pages)]}
	if comicInfo != nil {
		info := resolved[len(pages)]
		idx.ComicInfo = &info
	}
	return idx, nil
}

// ─── Étape 1 : localiser le répertoire central ───────────────────────────────

func readZipEnd(ctx context.Context, p storage.Provider, key string, size int64) (zipEnd, error) {
	// Essai court d'abord : un CBZ n'a normalement pas de commentaire ZIP.
	for _, probe := range []int64{eocdProbeSize, maxZipCommentLen + eocdMinSize} {
		if probe > size {
			probe = size
		}

		tail, err := readRangeFully(ctx, p, key, size-probe, probe)
		if err != nil {
			return zipEnd{}, err
		}

		pos := lastIndexSignature(tail, sigEOCD)
		if pos < 0 {
			if probe >= size || probe >= maxZipCommentLen+eocdMinSize {
				break
			}
			continue // agrandit la fenêtre
		}

		end, err := parseEOCD(tail[pos:], size-probe+int64(pos))
		if err != nil {
			return zipEnd{}, err
		}

		if end.needsZip64() {
			return readZip64End(ctx, p, key, tail, pos, size-probe)
		}
		return end, nil
	}

	return zipEnd{}, fmt.Errorf("%w : signature de fin d'archive (EOCD) introuvable dans %s", ErrCorrupted, key)
}

func (e zipEnd) needsZip64() bool {
	return e.entries == 0xFFFF || e.cdSize == 0xFFFFFFFF || e.cdOffset == 0xFFFFFFFF
}

func parseEOCD(b []byte, _ int64) (zipEnd, error) {
	if len(b) < eocdMinSize {
		return zipEnd{}, fmt.Errorf("%w : EOCD tronqué", ErrCorrupted)
	}
	return zipEnd{
		entries:  int64(binary.LittleEndian.Uint16(b[10:12])),
		cdSize:   int64(binary.LittleEndian.Uint32(b[12:16])),
		cdOffset: int64(binary.LittleEndian.Uint32(b[16:20])),
	}, nil
}

// readZip64End suit le localisateur ZIP64 pour retrouver les vraies valeurs.
//
// Nécessaire au-delà de 65535 entrées ou de 4 Gio — improbable pour un album,
// courant pour une intégrale scannée en haute définition.
func readZip64End(ctx context.Context, p storage.Provider, key string, tail []byte, eocdPos int, tailStart int64) (zipEnd, error) {
	locPos := eocdPos - eocd64LocSize
	if locPos < 0 {
		return zipEnd{}, fmt.Errorf("%w : localisateur ZIP64 absent", ErrCorrupted)
	}

	loc := tail[locPos : locPos+eocd64LocSize]
	if binary.LittleEndian.Uint32(loc[0:4]) != sigEOCD64Loc {
		return zipEnd{}, fmt.Errorf("%w : signature de localisateur ZIP64 invalide", ErrCorrupted)
	}

	eocd64Offset, err := safeInt64(binary.LittleEndian.Uint64(loc[8:16]), "offset ZIP64")
	if err != nil {
		return zipEnd{}, err
	}
	if eocd64Offset >= tailStart+int64(len(tail)) {
		return zipEnd{}, fmt.Errorf("%w : offset ZIP64 hors limites (%d)", ErrCorrupted, eocd64Offset)
	}

	rec, err := readRangeFully(ctx, p, key, eocd64Offset, eocd64MinSize)
	if err != nil {
		return zipEnd{}, err
	}
	if binary.LittleEndian.Uint32(rec[0:4]) != sigEOCD64 {
		return zipEnd{}, fmt.Errorf("%w : signature EOCD64 invalide", ErrCorrupted)
	}

	entries, err := safeInt64(binary.LittleEndian.Uint64(rec[32:40]), "nombre d'entrées ZIP64")
	if err != nil {
		return zipEnd{}, err
	}
	cdSize, err := safeInt64(binary.LittleEndian.Uint64(rec[40:48]), "taille du répertoire central ZIP64")
	if err != nil {
		return zipEnd{}, err
	}
	cdOffset, err := safeInt64(binary.LittleEndian.Uint64(rec[48:56]), "offset du répertoire central ZIP64")
	if err != nil {
		return zipEnd{}, err
	}

	return zipEnd{entries: entries, cdSize: cdSize, cdOffset: cdOffset}, nil
}

// safeInt64 convertit une valeur 64 bits non signée lue dans une archive.
//
// Le contenu d'une archive n'est pas une source de confiance : une valeur
// supérieure à math.MaxInt64 deviendrait un entier négatif et transformerait
// une vérification de bornes en faille. On refuse plutôt que de convertir.
func safeInt64(v uint64, what string) (int64, error) {
	if v > math.MaxInt64 {
		return 0, fmt.Errorf("%w : %s aberrant (%d)", ErrCorrupted, what, v)
	}
	return int64(v), nil
}

// ─── Étape 2 : lire le répertoire central ────────────────────────────────────

func readCentralDirectory(ctx context.Context, p storage.Provider, key string, end zipEnd) ([]centralEntry, error) {
	if end.cdSize <= 0 {
		return nil, fmt.Errorf("%w : répertoire central vide", ErrCorrupted)
	}

	cd, err := readRangeFully(ctx, p, key, end.cdOffset, end.cdSize)
	if err != nil {
		return nil, fmt.Errorf("lecture du répertoire central : %w", err)
	}

	entries := make([]centralEntry, 0, end.entries)
	pos := 0

	for pos+centralHdrSize <= len(cd) {
		if binary.LittleEndian.Uint32(cd[pos:pos+4]) != sigCentralFile {
			break // fin des entrées
		}
		h := cd[pos : pos+centralHdrSize]

		var (
			compression = Compression(binary.LittleEndian.Uint16(h[10:12]))
			compSize    = int64(binary.LittleEndian.Uint32(h[20:24]))
			uncompSize  = int64(binary.LittleEndian.Uint32(h[24:28]))
			nameLen     = int(binary.LittleEndian.Uint16(h[28:30]))
			extraLen    = int(binary.LittleEndian.Uint16(h[30:32]))
			commentLen  = int(binary.LittleEndian.Uint16(h[32:34]))
			localOffset = int64(binary.LittleEndian.Uint32(h[42:46]))
		)

		total := centralHdrSize + nameLen + extraLen + commentLen
		if pos+total > len(cd) {
			return nil, fmt.Errorf("%w : entrée du répertoire central tronquée", ErrCorrupted)
		}

		name := string(cd[pos+centralHdrSize : pos+centralHdrSize+nameLen])
		extra := cd[pos+centralHdrSize+nameLen : pos+centralHdrSize+nameLen+extraLen]

		// Les valeurs à 0xFFFFFFFF sont des sentinelles : la vraie valeur est
		// dans le champ extra ZIP64.
		if uncompSize == 0xFFFFFFFF || compSize == 0xFFFFFFFF || localOffset == 0xFFFFFFFF {
			uncompSize, compSize, localOffset = applyZip64Extra(extra, uncompSize, compSize, localOffset)
		}

		entries = append(entries, centralEntry{
			name:        name,
			compression: compression,
			compSize:    compSize,
			uncompSize:  uncompSize,
			localOffset: localOffset,
		})
		pos += total
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("%w : aucune entrée dans le répertoire central", ErrCorrupted)
	}
	return entries, nil
}

// applyZip64Extra remplace les sentinelles par les valeurs du champ extra
// ZIP64 (identifiant 0x0001), dans l'ordre imposé par la spécification.
func applyZip64Extra(extra []byte, uncompSize, compSize, localOffset int64) (int64, int64, int64) {
	for len(extra) >= 4 {
		id := binary.LittleEndian.Uint16(extra[0:2])
		size := int(binary.LittleEndian.Uint16(extra[2:4]))
		if len(extra) < 4+size {
			break
		}
		if id != 0x0001 {
			extra = extra[4+size:]
			continue
		}

		field := extra[4 : 4+size]
		read := func() (int64, bool) {
			if len(field) < 8 {
				return 0, false
			}
			raw := binary.LittleEndian.Uint64(field[:8])
			field = field[8:]
			if raw > math.MaxInt64 {
				return 0, false // valeur aberrante : on conserve la sentinelle
			}
			return int64(raw), true
		}

		if uncompSize == 0xFFFFFFFF {
			if v, ok := read(); ok {
				uncompSize = v
			}
		}
		if compSize == 0xFFFFFFFF {
			if v, ok := read(); ok {
				compSize = v
			}
		}
		if localOffset == 0xFFFFFFFF {
			if v, ok := read(); ok {
				localOffset = v
			}
		}
		break
	}
	return uncompSize, compSize, localOffset
}

// ─── Étape 3 : résoudre les offsets de données ───────────────────────────────

// resolveDataOffsets calcule, pour chaque entrée, l'offset absolu de ses
// données compressées.
//
// Le répertoire central donne l'offset de l'en-tête *local*, pas des données.
// Or la longueur du champ extra de l'en-tête local peut différer de celle du
// répertoire central — c'est pourquoi il faut le lire, et non le déduire.
//
// On ne lit que les 30 octets fixes de l'en-tête : les longueurs de nom et
// d'extra s'y trouvent, ce qui suffit au calcul. Les lectures sont dispersées
// dans le fichier, donc dominées par la latence : elles sont parallélisées.
func resolveDataOffsets(ctx context.Context, p storage.Provider, key string, entries []centralEntry, size int64) ([]Entry, error) {
	out := make([]Entry, len(entries))

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(localHeaderConcurrency)

	for i := range entries {
		g.Go(func() error {
			e := entries[i]

			if e.localOffset < 0 || e.localOffset+localHdrSize > size {
				return fmt.Errorf("%w : en-tête local hors limites pour %q (offset %d)",
					ErrCorrupted, e.name, e.localOffset)
			}

			hdr, err := readRangeFully(ctx, p, key, e.localOffset, localHdrSize)
			if err != nil {
				return fmt.Errorf("en-tête local de %q : %w", e.name, err)
			}
			if binary.LittleEndian.Uint32(hdr[0:4]) != sigLocalFile {
				return fmt.Errorf("%w : signature d'en-tête local invalide pour %q", ErrCorrupted, e.name)
			}

			nameLen := int64(binary.LittleEndian.Uint16(hdr[26:28]))
			extraLen := int64(binary.LittleEndian.Uint16(hdr[28:30]))
			dataOffset := e.localOffset + localHdrSize + nameLen + extraLen

			if dataOffset+e.compSize > size {
				return fmt.Errorf("%w : données de %q hors limites (offset %d, taille %d, archive %d)",
					ErrCorrupted, e.name, dataOffset, e.compSize, size)
			}

			out[i] = Entry{
				Name:        e.name,
				DataOffset:  dataOffset,
				DataSize:    e.compSize,
				Size:        e.uncompSize,
				Compression: e.compression,
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return out, nil
}

// ─── Extraction d'une entrée ─────────────────────────────────────────────────

// OpenEntry ouvre le contenu décompressé d'une entrée.
//
// ★ Une seule requête Range, quelle que soit la taille de l'archive. C'est la
// promesse du projet, tenue à l'exécution.
//
// L'appelant doit fermer le lecteur retourné.
func OpenEntry(ctx context.Context, p storage.Provider, key string, e Entry) (io.ReadCloser, error) {
	if e.DataSize < 0 {
		return nil, fmt.Errorf("%w : taille de données négative pour %q", ErrCorrupted, e.Name)
	}

	raw, err := p.ReadRange(ctx, key, e.DataOffset, e.DataSize)
	if err != nil {
		return nil, fmt.Errorf("lecture de %q : %w", e.Name, err)
	}

	switch e.Compression {
	case CompressionStore:
		return raw, nil

	case CompressionDeflate:
		// flate travaille en flux : la page se décompresse au fil de sa
		// lecture, sans jamais charger l'archive ni même la page entière.
		return &deflateReader{
			Reader: flate.NewReader(raw),
			raw:    raw,
		}, nil

	default:
		_ = raw.Close()
		return nil, fmt.Errorf("%w : méthode de compression %d non supportée pour %q",
			ErrUnsupportedFormat, e.Compression, e.Name)
	}
}

// deflateReader ferme à la fois le décompresseur et la source distante.
type deflateReader struct {
	io.Reader
	raw io.ReadCloser
}

func (d *deflateReader) Close() error {
	var errs []error
	if c, ok := d.Reader.(io.Closer); ok {
		errs = append(errs, c.Close())
	}
	errs = append(errs, d.raw.Close())
	return errors.Join(errs...)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// readRangeFully lit une plage complète en mémoire.
//
// Réservé aux petites lectures de structure (index, en-têtes). Les données de
// page se lisent en flux, jamais avec ceci.
func readRangeFully(ctx context.Context, p storage.Provider, key string, offset, length int64) ([]byte, error) {
	r, err := p.ReadRange(ctx, key, offset, length)
	if err != nil {
		return nil, err
	}
	defer func() { _ = r.Close() }()

	buf := make([]byte, 0, length)
	b, err := io.ReadAll(io.LimitReader(r, length))
	if err != nil {
		return nil, fmt.Errorf("lecture de la plage [%d, %d) : %w", offset, offset+length, err)
	}
	return append(buf, b...), nil
}

// lastIndexSignature cherche la dernière occurrence d'une signature 32 bits.
//
// On part de la fin : l'EOCD est en fin de fichier, et un fichier ZIP imbriqué
// dans les données pourrait contenir la même signature plus tôt.
func lastIndexSignature(b []byte, sig uint32) int {
	var want [4]byte
	binary.LittleEndian.PutUint32(want[:], sig)

	for i := len(b) - 4; i >= 0; i-- {
		if b[i] == want[0] && b[i+1] == want[1] && b[i+2] == want[2] && b[i+3] == want[3] {
			return i
		}
	}
	return -1
}
