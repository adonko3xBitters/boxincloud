import 'package:drift/drift.dart' show Value;
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/client.dart';
import '../../core/auth/session.dart';
import '../../core/db/database.dart';

/// Base locale, partagée par toute l'application.
final databaseProvider = Provider<BoxDatabase>((ref) {
  final db = BoxDatabase();
  ref.onDispose(db.close);
  return db;
});

/// Ce que la bibliothèque a à montrer, et d'où cela vient.
class LibraryView {
  final List<CachedComic> comics;
  final List<CachedFolder> folders;

  /// Vrai quand ces données viennent du cache faute de réseau.
  final bool offline;

  const LibraryView({
    this.comics = const [],
    this.folders = const [],
    this.offline = false,
  });
}

/// Listes de lecture.
///
/// Trois façons de retrouver un album qui ne passent ni par le rangement ni par
/// la série : ce qu'on aime, ce qu'on a commencé, ce qui vient d'arriver.
enum ReadingList { favorites, inProgress, recent }

/// Portée courante de l'affichage.
class LibraryScope {
  final String? libraryId;
  final String? folderPath;
  final String? seriesId;

  /// Quand elle est renseignée, elle prime : une liste de lecture traverse les
  /// bibliothèques et les dossiers, elle ne s'y range pas.
  final ReadingList? list;

  final String title;

  const LibraryScope({
    this.libraryId,
    this.folderPath,
    this.seriesId,
    this.list,
    this.title = 'Tous les albums',
  });
}

final scopeProvider = StateProvider<LibraryScope>((ref) => const LibraryScope());

/*
Chargement de la bibliothèque.

Le cache est servi D'ABORD, toujours, puis rafraîchi si le réseau répond.
L'inverse — attendre le réseau avant d'afficher — donnerait un écran vide de
plusieurs secondes à chaque ouverture, et un écran vide définitif hors ligne.

Une application de lecture doit s'ouvrir sur des couvertures, pas sur un
indicateur de chargement.
*/
final libraryProvider = FutureProvider<LibraryView>((ref) async {
  final session = ref.watch(sessionProvider);
  if (session is! SessionActive) return const LibraryView();

  final db = ref.watch(databaseProvider);
  final scope = ref.watch(scopeProvider);
  final serverId = session.server.id;

  Future<List<CachedComic>> comicsForScope() async {
    switch (scope.list) {
      case ReadingList.favorites:
        return db.favoriteComics(serverId);
      case ReadingList.inProgress:
        return db.inProgressComics(serverId);
      case ReadingList.recent:
        return db.recentComics(serverId);
      case null:
        return db.comicsOf(
          serverId,
          libraryId: scope.libraryId,
          folderPath: scope.folderPath,
          seriesId: scope.seriesId,
        );
    }
  }

  Future<LibraryView> fromCache({bool offline = false}) async => LibraryView(
        comics: await comicsForScope(),
        folders: await db.foldersOf(serverId),
        offline: offline,
      );

  try {
    final libraries = await session.client.libraries();

    await db.replaceLibraries(
      serverId,
      libraries
          .map((l) => CachedLibrariesCompanion.insert(
                id: l.id,
                serverId: serverId,
                name: l.name,
                comicCount: Value(l.comicCount),
              ))
          .toList(),
    );

    final folders = await session.client.folders(libraryId: scope.libraryId);
    await db.replaceFolders(
      serverId,
      folders
          .map((f) => CachedFoldersCompanion.insert(
                serverId: serverId,
                libraryId: f.libraryId,
                path: f.path,
                name: f.name,
                depth: f.depth,
                comicCount: Value(f.comicCount),
              ))
          .toList(),
    );

    // Les favoris appartiennent au compte, pas au catalogue : ils viennent
    // d'un appel à part, et sont conservés dans leur propre table pour
    // survivre au remplacement du cache des albums.
    await db.replaceFavorites(serverId, await session.client.favorites());

    // Les séries sont mises en cache ici plutôt qu'à l'ouverture de l'écran qui
    // les liste : elles servent aussi à la recherche hors ligne, qu'on n'a
    // aucune raison de faire dépendre d'une visite préalable de cet écran.
    final series = await session.client.series(libraryId: scope.libraryId);
    await db.replaceSeries(
      serverId,
      series.items
          .map((s) => CachedSeriesCompanion.insert(
                id: s.id,
                serverId: serverId,
                libraryId: s.libraryId,
                name: s.name,
                comicCount: Value(s.comicCount),
                coverPath: Value(s.coverPath ?? ''),
              ))
          .toList(),
    );

    // Une bibliothèque à la fois : le cache est remplacé par bibliothèque, et
    // mélanger les deux effacerait ce qu'on vient d'écrire.
    for (final library in libraries) {
      if (scope.libraryId != null && scope.libraryId != library.id) continue;

      final page = await session.client.comics(libraryId: library.id, limit: 200);
      await db.replaceComics(
        serverId,
        library.id,
        page.items
            .map((c) => CachedComicsCompanion.insert(
                  id: c.id,
                  serverId: serverId,
                  libraryId: c.libraryId,
                  title: c.title,
                  seriesId: Value(c.seriesId),
                  seriesName: Value(c.seriesName ?? ''),
                  number: Value(c.number ?? ''),
                  folderPath: Value(c.folderPath),
                  pageCount: Value(c.pageCount),
                  coverPath: Value(c.coverPath),
                  coverPlaceholder: Value(c.coverPlaceholder),
                  fileSize: Value(c.fileSize),
                  createdAt: Value(DateTime.tryParse(c.createdAt)?.toUtc()),
                  cachedAt: DateTime.now().toUtc(),
                ))
            .toList(),
      );
    }

    return fromCache();
  } on NetworkException {
    // Hors ligne : le cache fait le travail, et l'interface le dit.
    return fromCache(offline: true);
  } on ApiException {
    return fromCache(offline: true);
  }
});
