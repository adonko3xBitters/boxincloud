// Import ciblé : drift et matcher exportent tous deux `isNull`.
import 'dart:convert';

import 'package:drift/drift.dart' show Value;
import 'package:drift/native.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

import 'package:boxincloud/core/api/client.dart';
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

  /*
    Deux appareils, un même album.

    Le scénario qui a révélé le défaut, et qu'aucun test ne couvrait : toutes
    les vérifications de reprise passaient un client nul, c'est-à-dire un
    téléphone seul au monde. Le code en avait tiré sa règle — « le local est
    forcément au moins aussi avancé que le serveur » — vraie d'un appareil
    isolé, fausse dès qu'il y en a deux.
  */
  group('lecture reprise sur un autre appareil', () {
    test('la position du serveur gagne si elle est plus avancée', () async {
      final sync = ProgressSync(db: db, serverId: 's1');

      // Le matin, dans la voiture : lu jusqu'à la page 19, poussé au serveur.
      await sync.record(null, comicId: 'c1', page: 19, pageCount: 120);

      // Au bureau, sur le web : la lecture continue jusqu'à la page 57. Le
      // téléphone n'en sait rien, et c'est ce qu'on corrige.
      final client = clientServing({'c1': 57}, pageCount: 120);

      expect(await sync.resumePage(client, 'c1'), 57);

      // Et le cache retient la position rapatriée : rouvrir hors ligne ne doit
      // pas renvoyer à la page 19.
      expect(await sync.resumePage(null, 'c1'), 57);
    });

    test('la position locale gagne si le serveur est en retard', () async {
      final sync = ProgressSync(db: db, serverId: 's1');
      await sync.record(null, comicId: 'c1', page: 57, pageCount: 120);

      final client = clientServing({'c1': 19}, pageCount: 120);

      expect(await sync.resumePage(client, 'c1'), 57);

      // La ligne reste à pousser : le serveur ignore encore la page 57.
      final pending = await db.pendingProgress('s1');
      expect(pending.map((p) => p.comicId), contains('c1'));
    });

    test('hors ligne, la position locale répond seule', () async {
      final sync = ProgressSync(db: db, serverId: 's1');
      await sync.record(null, comicId: 'c1', page: 19, pageCount: 120);

      // Un serveur injoignable ne doit pas rouvrir l'album au début : c'est la
      // promesse du mode déconnecté.
      final client = ApiClient(
        baseUrl: 'https://exemple.test',
        httpClient: MockClient((_) => throw const SocketExceptionLike()),
      );

      expect(await sync.resumePage(client, 'c1'), 19);
    });

    test('le rapatriement fusionne un lot et retient le curseur', () async {
      final sync = ProgressSync(db: db, serverId: 's1');
      await sync.record(null, comicId: 'c1', page: 19, pageCount: 120);

      final client = pullServing([
        {'comicId': 'c1', 'page': 57, 'pageCount': 120},
        {'comicId': 'c2', 'page': 4, 'pageCount': 30},
      ], cursor: '2026-08-06T09:00:00Z');

      final outcome = await sync.pull(client);
      expect(outcome.pulled, 2);

      expect(await sync.resumePage(null, 'c1'), 57);
      expect(await sync.resumePage(null, 'c2'), 4);

      // Sans curseur retenu, chaque démarrage retéléchargerait tout
      // l'historique.
      expect(await db.preference('sync.cursor.s1'), '2026-08-06T09:00:00Z');
    });

    test('un rapatriement ne fait jamais reculer une lecture', () async {
      final sync = ProgressSync(db: db, serverId: 's1');
      await sync.record(null, comicId: 'c1', page: 57, pageCount: 120);

      final client = pullServing([
        {'comicId': 'c1', 'page': 19, 'pageCount': 120},
      ], cursor: 'c');

      await sync.pull(client);
      expect(await sync.resumePage(null, 'c1'), 57);
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

/// Un serveur qui connaît la progression de quelques albums.
ApiClient clientServing(Map<String, int> pages, {required int pageCount}) =>
    ApiClient(
      baseUrl: 'https://exemple.test',
      httpClient: MockClient((request) async {
        final match = RegExp(r'/comics/([^/]+)/progress').firstMatch(request.url.path);
        if (match == null) return http.Response('', 404);

        final comicId = match.group(1)!;
        return http.Response(
          jsonEncode(progressJson(comicId, pages[comicId] ?? 0, pageCount)),
          200,
          headers: {'content-type': 'application/json'},
        );
      }),
    );

/// Un serveur qui rend une page de changements, puis plus rien.
ApiClient pullServing(List<Map<String, dynamic>> changes, {required String cursor}) =>
    ApiClient(
      baseUrl: 'https://exemple.test',
      httpClient: MockClient((request) async {
        if (!request.url.path.endsWith('/sync')) return http.Response('', 404);

        return http.Response(
          jsonEncode({
            'changes': [
              for (final c in changes)
                progressJson(c['comicId'] as String, c['page'] as int, c['pageCount'] as int),
            ],
            'cursor': cursor,
            'hasMore': false,
          }),
          200,
          headers: {'content-type': 'application/json'},
        );
      }),
    );

Map<String, dynamic> progressJson(String comicId, int page, int pageCount) => {
      'comicId': comicId,
      'page': page,
      'pageCount': pageCount,
      'percent': pageCount == 0 ? 0.0 : page / pageCount,
      'status': statusFor(page, pageCount),
      'readCount': 0,
      'version': 1,
      'updatedAt': '2026-08-06T09:00:00Z',
    };

/// Une panne réseau, telle que le client la reconnaît.
class SocketExceptionLike implements Exception {
  const SocketExceptionLike();
}
