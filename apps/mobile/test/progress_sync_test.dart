// Import ciblé : drift et matcher exportent tous deux `isNull`.
import 'package:drift/drift.dart' show Value;
import 'package:drift/native.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:boxincloud/core/db/database.dart';
import 'package:boxincloud/core/sync/progress_sync.dart';

/*
La progression est ce qui se perd le plus facilement et se remarque le plus vite.

Reprendre un album à la page 3 alors qu'on en était à la 87 gâche une soirée. Ces
tests fixent les deux règles qui l'en empêchent : écrire localement d'abord, et
ne jamais reculer.
*/
void main() {
  late BoxDatabase db;

  setUp(() {
    db = BoxDatabase.forTesting(NativeDatabase.memory());
  });

  tearDown(() => db.close());

  group('statut déduit de la position', () {
    test('la dernière page marque l\'album comme lu', () {
      // Sans cela, un album terminé resterait indéfiniment « en cours » et
      // fausserait l'étagère « reprendre la lecture ».
      expect(statusFor(9, 10), 'read');
      expect(statusFor(10, 10), 'read');
    });

    test('la première page ne compte pas comme une lecture entamée', () {
      expect(statusFor(0, 10), 'unread');
    });

    test('entre les deux, la lecture est en cours', () {
      expect(statusFor(1, 10), 'in_progress');
      expect(statusFor(8, 10), 'in_progress');
    });

    test('un album sans pages connues reste non lu', () {
      expect(statusFor(0, 0), 'unread');
    });
  });

  group('résolution de conflit', () {
    test('la page la plus avancée gagne', () {
      expect(furthest(87, 3), 87);
      expect(furthest(3, 87), 87);
      expect(furthest(42, 42), 42);
    });

    /*
      Volontairement non chronologique.

      Les horloges d'un téléphone et d'un serveur divergent, et un fuseau mal
      réglé ferait reculer une lecture. Avancer par erreur coûte quelques pages
      sautées ; reculer coûte de relire un album entier.
    */
    test('une progression locale en avance n\'est jamais écrasée', () async {
      await db.saveProgress(
        serverId: 's1',
        comicId: 'c1',
        page: 87,
        pageCount: 100,
        status: 'in_progress',
        pending: false,
      );

      await mergeRemote(db, 's1', comicId: 'c1', remotePage: 3, pageCount: 100);

      final merged = await db.progressOf('s1', 'c1');
      expect(merged!.page, 87);
      // Le local était en avance : il reste à pousser.
      expect(merged.pending, isTrue);
    });

    test('une progression distante en avance est adoptée', () async {
      await db.saveProgress(
        serverId: 's1',
        comicId: 'c1',
        page: 5,
        pageCount: 100,
        status: 'in_progress',
        pending: false,
      );

      await mergeRemote(db, 's1', comicId: 'c1', remotePage: 60, pageCount: 100);

      final merged = await db.progressOf('s1', 'c1');
      expect(merged!.page, 60);
      // Rien à pousser : c'est le serveur qui était en avance.
      expect(merged.pending, isFalse);
    });
  });

  group('file d\'attente', () {
    test('une lecture hors ligne reste marquée à envoyer', () async {
      final sync = ProgressSync(db: db, serverId: 's1');

      // Aucun client : c'est exactement le mode avion.
      await sync.record(null, comicId: 'c1', page: 12, pageCount: 40);

      final pending = await db.pendingProgress('s1');
      expect(pending, hasLength(1));
      expect(pending.first.page, 12);
      expect(pending.first.status, 'in_progress');
    });

    test('les lectures s\'accumulent, une ligne par album', () async {
      final sync = ProgressSync(db: db, serverId: 's1');

      await sync.record(null, comicId: 'c1', page: 5, pageCount: 40);
      await sync.record(null, comicId: 'c1', page: 30, pageCount: 40);
      await sync.record(null, comicId: 'c2', page: 2, pageCount: 20);

      final pending = await db.pendingProgress('s1');
      expect(pending, hasLength(2));

      // La dernière position d'un album remplace la précédente : envoyer
      // l'historique complet n'apporterait rien.
      final first = pending.firstWhere((p) => p.comicId == 'c1');
      expect(first.page, 30);
    });

    test('la position rouverte est la position locale', () async {
      final sync = ProgressSync(db: db, serverId: 's1');
      await sync.record(null, comicId: 'c1', page: 33, pageCount: 40);

      expect(await sync.resumePage(null, 'c1'), 33);
    });

    test('un album jamais ouvert commence à zéro', () async {
      final sync = ProgressSync(db: db, serverId: 's1');
      expect(await sync.resumePage(null, 'inconnu'), 0);
    });
  });

  group('cloisonnement des serveurs', () {
    /*
      Deux serveurs ne partagent rien.

      L'application en gère plusieurs — un serveur familial et celui d'un ami.
      Mélanger leurs progressions ferait apparaître, dans l'un, la lecture d'un
      album de l'autre.
    */
    test('la progression d\'un serveur reste chez lui', () async {
      await ProgressSync(db: db, serverId: 's1')
          .record(null, comicId: 'c1', page: 10, pageCount: 40);
      await ProgressSync(db: db, serverId: 's2')
          .record(null, comicId: 'c1', page: 30, pageCount: 40);

      expect((await db.progressOf('s1', 'c1'))!.page, 10);
      expect((await db.progressOf('s2', 'c1'))!.page, 30);
      expect(await db.pendingProgress('s1'), hasLength(1));
    });

    test('oublier un serveur efface tout ce qui lui appartient', () async {
      await ProgressSync(db: db, serverId: 's1')
          .record(null, comicId: 'c1', page: 10, pageCount: 40);
      await db.replaceLibraries('s1', [
        CachedLibrariesCompanion.insert(id: 'l1', serverId: 's1', name: 'BD'),
      ]);

      await db.forgetServer('s1');

      expect(await db.progressOf('s1', 'c1'), isNull);
      expect(await db.librariesOf('s1'), isEmpty);
    });
  });

  group('cache du catalogue', () {
    test('un remplacement efface ce qui a disparu du serveur', () async {
      CachedComicsCompanion comic(String id, String title) =>
          CachedComicsCompanion.insert(
            id: id,
            serverId: 's1',
            libraryId: 'l1',
            title: title,
            cachedAt: DateTime.now().toUtc(),
          );

      await db.replaceComics('s1', 'l1', [comic('a', 'Tintin'), comic('b', 'Astérix')]);
      expect(await db.comicsOf('s1'), hasLength(2));

      // Le serveur fait autorité : un album supprimé chez lui doit disparaître
      // ici, ce qu'une fusion laisserait traîner.
      await db.replaceComics('s1', 'l1', [comic('a', 'Tintin')]);
      expect(await db.comicsOf('s1'), hasLength(1));
    });

    test('le filtre par dossier inclut la descendance', () async {
      CachedComicsCompanion at(String id, String folder) =>
          CachedComicsCompanion.insert(
            id: id,
            serverId: 's1',
            libraryId: 'l1',
            title: id,
            folderPath: Value(folder),
            cachedAt: DateTime.now().toUtc(),
          );

      await db.replaceComics('s1', 'l1', [
        at('a', 'BD'),
        at('b', 'BD/Tintin'),
        at('c', 'Manga'),
      ]);

      // Comme le serveur : cliquer sur un nœud montre sa branche entière.
      final branch = await db.comicsOf('s1', folderPath: 'BD');
      expect(branch.map((c) => c.id), containsAll(['a', 'b']));
      expect(branch.map((c) => c.id), isNot(contains('c')));
    });
  });
}
