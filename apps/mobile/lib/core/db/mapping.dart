import '../api/models.dart' as api;
import 'database.dart';

/// Un album du réseau, présenté comme un album du cache.
///
/// Rien n'est écrit en base. Le reste de l'application manipule des
/// `CachedComic` — c'est le type que la lecture, le détail et les listes
/// attendent — et convertir ici évite de dupliquer chaque écran pour deux types
/// qui portent les mêmes champs.
CachedComic cachedFromApi(api.Comic comic, String serverId) => CachedComic(
      id: comic.id,
      serverId: serverId,
      libraryId: comic.libraryId,
      title: comic.title,
      seriesId: comic.seriesId,
      seriesName: comic.seriesName ?? '',
      number: comic.number ?? '',
      folderPath: comic.folderPath,
      pageCount: comic.pageCount,
      coverPath: comic.coverPath,
      coverPlaceholder: comic.coverPlaceholder,
      fileSize: comic.fileSize,
      cachedAt: DateTime.now().toUtc(),
    );
