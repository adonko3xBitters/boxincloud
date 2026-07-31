// Package indexer découvre et indexe les archives d'une bibliothèque.
//
// Deux jobs enchaînés :
//
//	ScanLibrary  parcourt le backend, crée ou met à jour les comics, et enfile
//	             un IndexComic pour chacun de ceux qui en ont besoin.
//	IndexComic   lit l'index de l'archive, persiste comic_pages, extrait les
//	             métadonnées et génère les vignettes de couverture.
//
// Les deux sont idempotents : rejouer un scan ne crée pas de doublon et ne
// perd aucune saisie manuelle. C'est la condition pour qu'un scan interrompu
// puisse simplement être relancé.
package indexer

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"github.com/adonko3xBitters/boxincloud/server/internal/archive"
	"github.com/adonko3xBitters/boxincloud/server/internal/cache"
	"github.com/adonko3xBitters/boxincloud/server/internal/imaging"
	"github.com/adonko3xBitters/boxincloud/server/internal/library"
	"github.com/adonko3xBitters/boxincloud/server/internal/storage"
)

// Deps rassemble ce dont l'indexeur a besoin.
type Deps struct {
	Libraries *library.Service
	Repo      Repository
	Cache     *cache.Cache
	Imaging   imaging.Processor
	Log       *slog.Logger

	// Folders réconcilie l'arborescence après un parcours. Facultatif : le
	// pipeline d'indexation fonctionne sans, les dossiers étant alors seulement
	// déduits à l'affichage.
	Folders FolderObserver
}

// ─── ScanLibrary ─────────────────────────────────────────────────────────────

// ScanLibraryArgs déclenche le parcours d'une bibliothèque.
type ScanLibraryArgs struct {
	LibraryID uuid.UUID `json:"library_id"`
}

func (ScanLibraryArgs) Kind() string { return "scan_library" }

type ScanLibraryWorker struct {
	river.WorkerDefaults[ScanLibraryArgs]
	deps Deps
}

// Timeout généreux : parcourir un bucket de plusieurs dizaines de milliers
// d'objets sur un backend distant prend du temps, et l'interrompre à mi-course
// n'apporterait rien — le scan est reprenable, mais autant le laisser finir.
func (w *ScanLibraryWorker) Timeout(*river.Job[ScanLibraryArgs]) time.Duration {
	return 2 * time.Hour
}

func (w *ScanLibraryWorker) Work(ctx context.Context, job *river.Job[ScanLibraryArgs]) error {
	_, err := w.scan(ctx, job.Args.LibraryID)
	return err
}

// scan porte le corps du parcours et retourne son bilan.
//
// Séparé de Work pour que DirectRunner puisse récupérer les statistiques : le
// contrat de River ne rend que l'erreur.
func (w *ScanLibraryWorker) scan(ctx context.Context, libraryID uuid.UUID) (ScanStats, error) {
	log := w.deps.Log.With(slog.String("library_id", libraryID.String()))

	lib, err := w.deps.Libraries.GetLibrary(ctx, libraryID)
	if err != nil {
		return ScanStats{}, err
	}

	provider, err := w.deps.Libraries.ProviderForLibrary(ctx, lib)
	if err != nil {
		return ScanStats{}, err
	}

	run, err := w.deps.Repo.StartScanRun(ctx, lib.ID)
	if err != nil {
		return ScanStats{}, err
	}

	log.Info("scan démarré",
		slog.String("bibliothèque", lib.Name),
		slog.String("préfixe", lib.RootPrefix),
	)

	stats := ScanStats{}
	seen := make([]string, 0, 256)
	folders := make([]string, 0, 32)
	start := time.Now()

	err = provider.List(ctx, lib.RootPrefix, func(obj storage.ObjectInfo) error {
		if err := ctx.Err(); err != nil {
			return err
		}

		// Un bucket contient couramment autre chose que des archives :
		// couvertures exportées, notes, fichiers de configuration. Ce n'est pas
		// une anomalie, on passe.
		format, ok := detectComicFormat(obj.Key)
		if !ok {
			return nil
		}
		stats.ObjectsSeen++
		seen = append(seen, obj.Key)
		folders = append(folders, FolderOf(obj.Key, lib.RootPrefix))

		meta := ParseFilename(obj.Key)
		title := meta.Title
		if title == "" {
			title = obj.Key
		}

		comic, inserted, err := w.deps.Repo.UpsertComic(ctx, UpsertComicParams{
			LibraryID:  lib.ID,
			ObjectKey:  obj.Key,
			FileSize:   obj.Size,
			FileETag:   obj.ETag,
			Format:     string(format),
			Title:      title,
			FolderPath: FolderOf(obj.Key, lib.RootPrefix),
		})
		if err != nil {
			stats.Errors++
			log.Warn("ingestion impossible", slog.String("key", obj.Key), slog.Any("err", err))
			return nil // un objet fautif ne doit pas interrompre le scan
		}

		if err := w.deps.Repo.SetFolder(ctx, comic.ID, FolderOf(obj.Key, lib.RootPrefix)); err != nil {
			log.Debug("dossier non enregistré", slog.String("key", obj.Key), slog.Any("err", err))
		}

		if inserted {
			stats.Added++
		} else if comic.NeedsIndexing {
			stats.Updated++
		}

		// Seuls les comics nouveaux ou modifiés sont réindexés : un rescan
		// d'une bibliothèque inchangée ne coûte qu'un listage.
		if comic.NeedsIndexing {
			if err := w.deps.Repo.EnqueueIndexComic(ctx, comic.ID); err != nil {
				stats.Errors++
				log.Warn("mise en file d'indexation impossible",
					slog.String("comic_id", comic.ID.String()), slog.Any("err", err))
			}
		}
		return nil
	})
	if err != nil {
		_ = w.deps.Repo.FinishScanRun(ctx, run.ID, "failed", stats, err.Error())
		return stats, fmt.Errorf("parcours de la bibliothèque %q : %w", lib.Name, err)
	}

	// Les objets absents du parcours ont disparu du backend. On les marque
	// plutôt que de les supprimer : un bucket momentanément démonté ne doit
	// pas détruire la progression de lecture des utilisateurs.
	removed, err := w.deps.Repo.MarkMissingDeleted(ctx, lib.ID, seen)
	if err != nil {
		log.Warn("marquage des objets disparus impossible", slog.Any("err", err))
	}
	stats.Removed = int(removed)

	if err := w.deps.Repo.RefreshSeriesCounts(ctx, lib.ID); err != nil {
		log.Warn("rafraîchissement des compteurs de séries impossible", slog.Any("err", err))
	}

	// L'arborescence est réconciliée avec ce que le parcours a trouvé : les
	// dossiers rencontrés sont inscrits, ceux qui n'existaient que par la
	// présence de fichiers désormais partis sont élagués. Les dossiers créés à
	// la main survivent — c'est justement ce qui les distingue.
	if w.deps.Folders != nil {
		if err := w.deps.Folders.Observe(ctx, lib.ID, folders); err != nil {
			log.Warn("réconciliation de l'arborescence impossible", slog.Any("err", err))
		}
	}
	if err := w.deps.Repo.SetLibraryScanResult(ctx, lib.ID, "success"); err != nil {
		log.Warn("enregistrement du résultat de scan impossible", slog.Any("err", err))
	}
	if err := w.deps.Repo.FinishScanRun(ctx, run.ID, "success", stats, ""); err != nil {
		log.Warn("clôture du scan impossible", slog.Any("err", err))
	}

	log.Info("scan terminé",
		slog.Int("objets", stats.ObjectsSeen),
		slog.Int("ajoutés", stats.Added),
		slog.Int("modifiés", stats.Updated),
		slog.Int("disparus", stats.Removed),
		slog.Int("erreurs", stats.Errors),
		slog.Duration("durée", time.Since(start)),
	)
	return stats, nil
}

// ScanStats résume un scan.
type ScanStats struct {
	ObjectsSeen int
	Added       int
	Updated     int
	Removed     int
	Errors      int
}

// ─── IndexComic ──────────────────────────────────────────────────────────────

// IndexComicArgs déclenche l'indexation d'une archive.
type IndexComicArgs struct {
	ComicID uuid.UUID `json:"comic_id"`
}

func (IndexComicArgs) Kind() string { return "index_comic" }

type IndexComicWorker struct {
	river.WorkerDefaults[IndexComicArgs]
	deps Deps
}

func (w *IndexComicWorker) Timeout(*river.Job[IndexComicArgs]) time.Duration {
	return 10 * time.Minute
}

func (w *IndexComicWorker) Work(ctx context.Context, job *river.Job[IndexComicArgs]) error {
	log := w.deps.Log.With(slog.String("comic_id", job.Args.ComicID.String()))

	comic, err := w.deps.Repo.GetComic(ctx, job.Args.ComicID)
	if err != nil {
		return err
	}

	lib, err := w.deps.Libraries.GetLibrary(ctx, comic.LibraryID)
	if err != nil {
		return err
	}
	provider, err := w.deps.Libraries.ProviderForLibrary(ctx, lib)
	if err != nil {
		return err
	}

	if err := w.deps.Repo.SetComicState(ctx, comic.ID, "indexing", ""); err != nil {
		return err
	}

	fail := func(err error) error {
		if setErr := w.deps.Repo.SetComicState(ctx, comic.ID, "error", err.Error()); setErr != nil {
			log.Warn("enregistrement de l'état d'erreur impossible", slog.Any("err", setErr))
		}
		return err
	}

	format, err := archive.DetectFormat(comic.ObjectKey)
	if err != nil {
		return fail(err)
	}

	start := time.Now()

	/*
		Les formats sans accès aléatoire sont convertis avant d'être indexés.

		Le RAR ne permettra jamais de servir une page par une requête Range —
		ses archives solides compressent les fichiers comme un flux continu — et
		le PDF demanderait un moteur de rendu qu'on ne veut pas embarquer. On
		les réécrit donc une fois en CBZ dans le cache dérivé, et tout ce qui
		suit ne connaît plus qu'un CBZ.

		L'archive hydratée devient la source des offsets : `indexKey` et
		`indexProvider` désignent à partir d'ici l'objet réellement indexé, qui
		n'est plus celui de l'utilisateur.
	*/
	indexKey, indexSize, indexProvider := comic.ObjectKey, comic.FileSize, provider

	if !format.SupportsRandomAccess() {
		if err := w.deps.Repo.SetComicState(ctx, comic.ID, "hydrating", ""); err != nil {
			return err
		}

		log.Info("hydratation",
			slog.String("format", string(format)),
			slog.String("key", comic.ObjectKey))

		cacheProvider := w.deps.Cache.Provider()

		hydrated, err := Hydrate(ctx, provider, cacheProvider, comic.ID, comic.ObjectKey, format)
		if err != nil {
			return fail(fmt.Errorf("hydratation de %q : %w", comic.ObjectKey, err))
		}

		info, err := cacheProvider.Stat(ctx, hydrated)
		if err != nil {
			return fail(fmt.Errorf("archive hydratée illisible : %w", err))
		}

		// Enregistrée AVANT l'indexation des pages : les offsets qui suivent
		// désignent cette archive, et l'ordre inverse laisserait, en cas
		// d'interruption, des pages pointant vers une archive que rien ne
		// référence.
		if err := w.deps.Repo.SetComicHydrated(ctx, comic.ID, hydrated); err != nil {
			return fail(err)
		}

		indexKey, indexSize, indexProvider = hydrated, info.Size, cacheProvider

		log.Info("album hydraté",
			slog.String("key", comic.ObjectKey),
			slog.String("archive", hydrated),
			slog.Int64("octets", info.Size),
			slog.Duration("durée", time.Since(start)))
	}

	idx, err := archive.ReadZipIndex(ctx, indexProvider, indexKey, indexSize)
	if err != nil {
		return fail(fmt.Errorf("indexation de %q : %w", indexKey, err))
	}

	// Tout ce qui suit lit des pages, donc l'archive indexée — l'originale pour
	// un CBZ, l'hydratée sinon. Passer `provider` et `comic.ObjectKey` ici
	// donnerait des offsets valides appliqués au mauvais fichier : la lecture
	// rendrait des octets arbitraires plutôt qu'une erreur.
	pages, err := w.buildPages(ctx, indexProvider, indexKey, idx)
	if err != nil {
		return fail(err)
	}

	if err := w.deps.Repo.ReplaceComicPages(ctx, comic.ID, pages); err != nil {
		return fail(err)
	}

	if err := w.applyMetadata(ctx, indexProvider, comic, lib, idx); err != nil {
		// Des métadonnées manquantes ne rendent pas l'album illisible : on
		// signale sans faire échouer l'indexation.
		log.Warn("métadonnées non appliquées", slog.Any("err", err))
	}

	if err := w.generateCover(ctx, indexProvider, comic, idx); err != nil {
		log.Warn("couverture non générée", slog.Any("err", err))
	}

	if err := w.deps.Repo.SetComicIndexed(ctx, comic.ID, len(pages)); err != nil {
		return err
	}

	log.Info("comic indexé",
		slog.String("key", comic.ObjectKey),
		slog.Int("pages", len(pages)),
		slog.Duration("durée", time.Since(start)),
	)
	return nil
}

// buildPages convertit l'index de l'archive en lignes comic_pages, en lisant au
// passage les dimensions de chaque page.
//
// Les dimensions permettent au client de réserver la mise en page avant
// réception de l'image — donc aucun décalage visuel pendant la lecture. Elles
// coûtent une lecture d'en-tête par page, soit quelques centaines d'octets :
// image.DecodeConfig n'a pas besoin de l'image entière.
func (w *IndexComicWorker) buildPages(ctx context.Context, provider storage.Provider, key string, idx *archive.Index) ([]Page, error) {
	pages := make([]Page, 0, len(idx.Pages))

	for i, entry := range idx.Pages {
		page := Page{
			Index:       i,
			EntryName:   entry.Name,
			DataOffset:  entry.DataOffset,
			DataSize:    entry.DataSize,
			Size:        entry.Size,
			Compression: uint16ToInt16(uint16(entry.Compression)),
		}

		if info, err := w.inspectPage(ctx, provider, key, entry); err == nil {
			page.Width = info.Width
			page.Height = info.Height
			page.IsDouble = info.IsDouble()
		} else {
			// Une page illisible ne doit pas empêcher d'indexer l'album : le
			// lecteur s'adaptera, et l'utilisateur verra le problème sur cette
			// page seulement.
			w.deps.Log.Debug("dimensions illisibles",
				slog.String("entry", entry.Name), slog.Any("err", err))
		}

		pages = append(pages, page)
	}
	return pages, nil
}

func (w *IndexComicWorker) inspectPage(ctx context.Context, provider storage.Provider, key string, entry archive.Entry) (imaging.Info, error) {
	r, err := archive.OpenEntry(ctx, provider, key, entry)
	if err != nil {
		return imaging.Info{}, err
	}
	defer func() { _ = r.Close() }()

	return w.deps.Imaging.Inspect(r)
}

// applyMetadata fusionne les métadonnées de ComicInfo.xml et du nom de fichier.
//
// Ordre de priorité : ComicInfo.xml prime sur le nom de fichier, et les champs
// verrouillés (saisie manuelle) priment sur tout — cette dernière règle est
// appliquée en SQL, pour qu'aucun chemin de code ne puisse la contourner.
func (w *IndexComicWorker) applyMetadata(ctx context.Context, provider storage.Provider, comic Comic, lib library.Library, idx *archive.Index) error {
	meta := ParseFilename(comic.ObjectKey)
	var rawXML []byte

	if idx.ComicInfo != nil {
		r, err := archive.OpenEntry(ctx, provider, comic.ObjectKey, *idx.ComicInfo)
		if err != nil {
			return err
		}
		info, raw, err := ParseComicInfo(r)
		_ = r.Close()

		if err != nil {
			w.deps.Log.Debug("ComicInfo.xml ignoré",
				slog.String("key", comic.ObjectKey), slog.Any("err", err))
		} else {
			rawXML = raw
			meta = mergeMetadata(info.ToMetadata(), meta)
		}
	}

	var seriesID *uuid.UUID
	if meta.Series != "" {
		s, err := w.deps.Repo.UpsertSeries(ctx, lib.ID, meta.Series, SortName(meta.Series))
		if err != nil {
			return err
		}
		seriesID = &s
	}

	metadataJSON := []byte("{}")
	if len(rawXML) > 0 {
		// On conserve le XML brut : un jalon ultérieur pourra en exploiter des
		// champs que le modèle actuel ignore, sans avoir à tout réindexer.
		if b, err := json.Marshal(map[string]string{"comicinfo_xml": string(rawXML)}); err == nil {
			metadataJSON = b
		}
	}

	return w.deps.Repo.ApplyMetadata(ctx, comic.ID, meta, seriesID, metadataJSON)
}

// mergeMetadata complète les champs vides de primary avec ceux de fallback.
func mergeMetadata(primary, fallback Metadata) Metadata {
	if primary.Title == "" {
		primary.Title = fallback.Title
	}
	if primary.Series == "" {
		primary.Series = fallback.Series
	}
	if primary.Number == "" {
		primary.Number = fallback.Number
		primary.NumberSort = fallback.NumberSort
	}
	if primary.NumberSort == nil {
		primary.NumberSort = fallback.NumberSort
	}
	if primary.Summary == "" {
		primary.Summary = fallback.Summary
	}
	if primary.Language == "" {
		primary.Language = fallback.Language
	}
	if primary.Volume == nil {
		primary.Volume = fallback.Volume
	}
	if primary.AgeRating == nil {
		primary.AgeRating = fallback.AgeRating
	}
	return primary
}

// generateCover produit les vignettes de couverture dans les trois tailles.
func (w *IndexComicWorker) generateCover(ctx context.Context, provider storage.Provider, comic Comic, idx *archive.Index) error {
	if len(idx.Pages) == 0 {
		return archive.ErrNoPages
	}

	r, err := archive.OpenEntry(ctx, provider, comic.ObjectKey, idx.Pages[0])
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()

	// La page source est lue une seule fois puis réutilisée pour les trois
	// tailles : une lecture distante au lieu de trois. La borne protège d'une
	// entrée d'archive délibérément énorme — une planche réelle reste très en
	// deçà de 64 Mio.
	original, err := io.ReadAll(io.LimitReader(r, maxCoverBytes))
	if err != nil {
		return fmt.Errorf("lecture de la page de couverture : %w", err)
	}

	for _, width := range imaging.ThumbSizes {
		var buf bytes.Buffer

		if _, err := w.deps.Imaging.Transform(&buf, bytes.NewReader(original), imaging.Options{
			Width:  width,
			Format: imaging.FormatJPEG,
		}); err != nil {
			return fmt.Errorf("vignette %dpx : %w", width, err)
		}

		key := cache.CoverKey(comic.ID, width, imaging.FormatJPEG)
		if err := w.deps.Cache.Put(ctx, key, comic.ID, buf.Bytes(), imaging.FormatJPEG.ContentType()); err != nil {
			return err
		}
	}

	// Aperçu de chargement (LQIP). Stocké en base plutôt que dans le cache : il
	// doit arriver AVEC la liste d'albums, en une seule requête — le chercher
	// ailleurs annulerait tout son intérêt.
	if placeholder, err := w.buildPlaceholder(original); err != nil {
		w.deps.Log.Debug("aperçu de couverture non généré",
			slog.String("comic_id", comic.ID.String()), slog.Any("err", err))
	} else if err := w.deps.Repo.SetCoverPlaceholder(ctx, comic.ID, placeholder); err != nil {
		return err
	}

	return nil
}

// maxPlaceholderBytes borne la taille du LQIP.
//
// Il voyage dans chaque élément de la liste d'albums : au-delà, une page de
// soixante couvertures paierait plus cher en JSON que ce que l'aperçu fait
// gagner en confort. Un JPEG de 16 px reste très en deçà.
const maxPlaceholderBytes = 2048

// buildPlaceholder produit un data-URI d'un JPEG de 16 px de large.
func (w *IndexComicWorker) buildPlaceholder(original []byte) (string, error) {
	var buf bytes.Buffer

	if _, err := w.deps.Imaging.Transform(&buf, bytes.NewReader(original), imaging.Options{
		Width:  imaging.PlaceholderWidth,
		Format: imaging.FormatJPEG,
		// Qualité basse assumée : l'image sera floutée et étirée, les artefacts
		// de compression disparaissent entièrement.
		Quality: 45,
	}); err != nil {
		return "", err
	}

	if buf.Len() > maxPlaceholderBytes {
		return "", fmt.Errorf("aperçu trop volumineux : %d octets", buf.Len())
	}

	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// maxCoverBytes borne la lecture d'une page de couverture.
const maxCoverBytes = 64 << 20

// ─── Enregistrement des workers ──────────────────────────────────────────────

// Register déclare les workers de l'indexeur auprès de River.
func Register(workers *river.Workers, deps Deps) {
	river.AddWorker(workers, &ScanLibraryWorker{deps: deps})
	river.AddWorker(workers, &IndexComicWorker{deps: deps})
}
