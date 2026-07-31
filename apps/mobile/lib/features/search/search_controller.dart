import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/client.dart';
import '../../core/auth/session.dart';
import '../../core/db/database.dart';
import '../../core/db/mapping.dart';
import '../../core/search/local_search.dart';
import '../library/library_controller.dart';

/// Une série trouvée, quelle qu'en soit la provenance.
///
/// Le réseau renvoie un `api.Series`, le cache un `CachedSeriesRow` : deux types
/// pour la même chose. L'écran n'a pas à connaître les deux.
class SeriesHit {
  final String id;
  final String name;
  final int comicCount;
  final String coverPath;

  const SeriesHit({
    required this.id,
    required this.name,
    required this.comicCount,
    required this.coverPath,
  });
}

/// Ce qu'une recherche a produit, et d'où cela vient.
class SearchOutcome {
  final List<CachedComic> comics;
  final List<SeriesHit> series;

  /// Vrai quand ces résultats sortent du cache faute de réseau. L'écran le dit,
  /// parce qu'une liste courte a deux explications possibles — peu de résultats,
  /// ou peu de cache — et que l'utilisateur doit pouvoir les distinguer.
  final bool offline;

  const SearchOutcome({
    this.comics = const [],
    this.series = const [],
    this.offline = false,
  });

  bool get isEmpty => comics.isEmpty && series.isEmpty;
}

/*
Recherche : le serveur d'abord, le cache en repli.

L'ordre est l'inverse de celui de la bibliothèque, qui sert le cache d'abord.
La raison tient à ce qu'on attend de chacun : une bibliothèque doit s'ouvrir
instantanément sur ce qu'on avait déjà, une recherche doit trouver — y compris
un album téléversé ce matin, absent du cache. Attendre le réseau est acceptable
quand on vient de taper une requête ; il ne l'est pas à l'ouverture.

Le repli local n'est pas un lot de consolation : hors ligne, c'est exactement
l'ensemble sur lequel on peut lire, donc le seul qui vaille la peine d'être
cherché.
*/
final searchProvider =
    FutureProvider.autoDispose.family<SearchOutcome, String>((ref, query) async {
  final session = ref.watch(sessionProvider);
  if (session is! SessionActive) return const SearchOutcome();

  if (fold(query).length < 2) return const SearchOutcome();

  final db = ref.watch(databaseProvider);
  final serverId = session.server.id;

  Future<SearchOutcome> fromCache() async {
    final comics = await db.comicsOf(serverId);
    final series = await db.seriesOf(serverId);

    return SearchOutcome(
      comics: rank(comics, query, (c) => [c.title, c.seriesName]),
      series: rank(series, query, (s) => [s.name])
          .map((s) => SeriesHit(
                id: s.id,
                name: s.name,
                comicCount: s.comicCount,
                coverPath: s.coverPath,
              ))
          .toList(),
      offline: true,
    );
  }

  try {
    final results = await session.client.search(query);

    return SearchOutcome(
      comics: results.comics.map((c) => cachedFromApi(c, serverId)).toList(),
      series: results.series
          .map((s) => SeriesHit(
                id: s.id,
                name: s.name,
                comicCount: s.comicCount,
                coverPath: s.coverPath ?? '',
              ))
          .toList(),
    );
  } on NetworkException {
    return fromCache();
  } on ApiException {
    return fromCache();
  }
});

/// Toutes les séries du serveur, cache d'abord.
final seriesListProvider = FutureProvider<List<SeriesHit>>((ref) async {
  final session = ref.watch(sessionProvider);
  if (session is! SessionActive) return const [];

  final db = ref.watch(databaseProvider);
  final rows = await db.seriesOf(session.server.id);

  return rows
      .map((s) => SeriesHit(
            id: s.id,
            name: s.name,
            comicCount: s.comicCount,
            coverPath: s.coverPath,
          ))
      .toList();
});
