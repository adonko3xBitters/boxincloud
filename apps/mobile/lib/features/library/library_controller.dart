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

/// Portée courante de l'affichage.
class LibraryScope {
  final String? libraryId;
  final String? folderPath;
  final String? seriesId;
  final String title;

  const LibraryScope({
    this.libraryId,
    this.folderPath,
    this.seriesId,
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

  Future<LibraryView> fromCache({bool offline = false}) async => LibraryView(
        comics: await db.comicsOf(
          serverId,
          libraryId: scope.libraryId,
          folderPath: scope.folderPath,
          seriesId: scope.seriesId,
        ),
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
