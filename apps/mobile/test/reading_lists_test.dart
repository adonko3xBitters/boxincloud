import 'package:drift/drift.dart' show Value;
import 'package:drift/native.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:boxincloud/core/db/database.dart';

/// Les trois listes de lecture se lisent en local. Ce sont des requêtes, et une
/// requête fausse ne se voit pas : elle rend une liste plausible mais fausse.
void main() {
  late BoxDatabase db;

  setUp(() => db = BoxDatabase.forTesting(NativeDatabase.memory()));
  tearDown(() => db.close());

  Future<void> cache(
    String id, {
    String server = 's1',
    String title = 'Album',
    DateTime? createdAt,
  }) =>
      db.into(db.cachedComics).insert(CachedComicsCompanion.insert(
            id: id,
            serverId: server,
            libraryId: 'lib',
            title: title,
            createdAt: Value(createdAt),
            cachedAt: DateTime.utc(2026, 1, 1),
          ));

  group('en cours', () {
    test('ne retient que les albums commencés et non finis', () async {
      await cache('a');
      await cache('b');
      await cache('c');

      await db.saveProgress(
          serverId: 's1', comicId: 'a', page: 5, pageCount: 40, status: 'in_progress');
      await db.saveProgress(
          serverId: 's1', comicId: 'b', page: 39, pageCount: 40, status: 'read');
      await db.saveProgress(
          serverId: 's1', comicId: 'c', page: 0, pageCount: 40, status: 'unread');

      final list = await db.inProgressComics('s1');
      expect(list.map((c) => c.id), ['a']);
    });

    test('le plus récemment lu vient en tête', () async {
      await cache('vieux');
      await cache('recent');

      await db.saveProgress(
          serverId: 's1', comicId: 'vieux', page: 2, pageCount: 40, status: 'in_progress');
      // `saveProgress` horodate à l'instant : un aller-retour explicite garantit
      // deux valeurs distinctes sans dépendre de la résolution de l'horloge.
      await db.into(db.localProgress).insertOnConflictUpdate(
            LocalProgressCompanion.insert(
              comicId: 'vieux',
              serverId: 's1',
              page: 2,
              pageCount: 40,
              status: 'in_progress',
              updatedAt: DateTime.utc(2026, 1, 1),
            ),
          );
      await db.into(db.localProgress).insertOnConflictUpdate(
            LocalProgressCompanion.insert(
              comicId: 'recent',
              serverId: 's1',
              page: 8,
              pageCount: 40,
              status: 'in_progress',
              updatedAt: DateTime.utc(2026, 6, 1),
            ),
          );

      final list = await db.inProgressComics('s1');
      expect(list.map((c) => c.id), ['recent', 'vieux']);
    });

    test('une progression sans album en cache ne fait rien apparaître', () async {
      // La jointure est interne : une progression orpheline — l'album a
      // disparu du serveur — ne doit pas produire de ligne fantôme.
      await db.saveProgress(
          serverId: 's1', comicId: 'inconnu', page: 3, pageCount: 40, status: 'in_progress');

      expect(await db.inProgressComics('s1'), isEmpty);
    });

    test('la progression d\'un autre serveur reste chez lui', () async {
      await cache('a', server: 's1');
      await cache('a', server: 's2');

      await db.saveProgress(
          serverId: 's2', comicId: 'a', page: 5, pageCount: 40, status: 'in_progress');

      expect(await db.inProgressComics('s1'), isEmpty);
      expect((await db.inProgressComics('s2')).map((c) => c.id), ['a']);
    });
  });

  group('favoris', () {
    test('un remplacement efface ce qui n\'est plus favori', () async {
      await cache('a');
      await cache('b');

      await db.replaceFavorites('s1', ['a', 'b']);
      expect((await db.favoriteComics('s1')).length, 2);

      await db.replaceFavorites('s1', ['b']);
      expect((await db.favoriteComics('s1')).map((c) => c.id), ['b']);
    });

    test('la marque survit au remplacement du cache des albums', () async {
      // C'est la raison d'être de la table séparée : `replaceComics` vide et
      // réécrit le catalogue à chaque rafraîchissement.
      await cache('a');
      await db.replaceFavorites('s1', ['a']);

      await db.replaceComics('s1', 'lib', [
        CachedComicsCompanion.insert(
          id: 'a',
          serverId: 's1',
          libraryId: 'lib',
          title: 'Album',
          cachedAt: DateTime.utc(2026, 2, 1),
        ),
      ]);

      expect(await db.isFavorite('s1', 'a'), isTrue);
      expect((await db.favoriteComics('s1')).map((c) => c.id), ['a']);
    });

    test('la bascule locale s\'écrit et s\'annule', () async {
      await cache('a');

      await db.setFavorite('s1', 'a', true);
      expect(await db.isFavorite('s1', 'a'), isTrue);

      await db.setFavorite('s1', 'a', false);
      expect(await db.isFavorite('s1', 'a'), isFalse);
      expect(await db.favoriteComics('s1'), isEmpty);
    });

    test('marquer deux fois ne crée pas deux lignes', () async {
      await cache('a');
      await db.setFavorite('s1', 'a', true);
      await db.setFavorite('s1', 'a', true);

      expect((await db.favoriteComics('s1')).length, 1);
    });

    test('un favori sans album en cache ne remonte pas', () async {
      await db.replaceFavorites('s1', ['fantome']);
      expect(await db.favoriteComics('s1'), isEmpty);
      // La marque existe pourtant : elle reprendra sens au prochain cache.
      expect(await db.favoriteIds('s1'), {'fantome'});
    });

    test('oublier un serveur emporte ses favoris', () async {
      await cache('a');
      await db.setFavorite('s1', 'a', true);

      await db.forgetServer('s1');
      expect(await db.favoriteIds('s1'), isEmpty);
    });
  });

  group('ajouts récents', () {
    test('le plus récemment ajouté vient en tête', () async {
      await cache('vieux', createdAt: DateTime.utc(2024, 1, 1));
      await cache('recent', createdAt: DateTime.utc(2026, 1, 1));
      await cache('moyen', createdAt: DateTime.utc(2025, 1, 1));

      final list = await db.recentComics('s1');
      expect(list.map((c) => c.id), ['recent', 'moyen', 'vieux']);
    });

    test('un album sans date passe en dernier, pas à la trappe', () async {
      // Les lignes écrites avant l'existence de la colonne : elles sont
      // réelles, et le prochain rafraîchissement leur rendra leur date.
      await cache('date', createdAt: DateTime.utc(2025, 1, 1));
      await cache('sans-date');

      final list = await db.recentComics('s1');
      expect(list.map((c) => c.id), ['date', 'sans-date']);
    });

    test('la limite tronque sans changer l\'ordre', () async {
      for (var year = 2020; year < 2026; year += 1) {
        await cache('a$year', createdAt: DateTime.utc(year, 1, 1));
      }

      final list = await db.recentComics('s1', limit: 2);
      expect(list.map((c) => c.id), ['a2025', 'a2024']);
    });
  });
}
