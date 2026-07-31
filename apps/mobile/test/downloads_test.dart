import 'package:drift/drift.dart' show Value;
import 'package:drift/native.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:boxincloud/core/db/database.dart';

/// Le registre des téléchargements décide de ce qui est lisible sans réseau.
///
/// Ses erreurs ne se voient qu'en mode avion, c'est-à-dire au pire moment :
/// loin d'un réseau, donc loin de tout recours.
void main() {
  late BoxDatabase db;

  setUp(() => db = BoxDatabase.forTesting(NativeDatabase.memory()));
  tearDown(() => db.close());

  Future<void> put(
    String comicId, {
    String server = 's1',
    String state = 'complete',
    int pageCount = 30,
    int pagesDone = 30,
    int bytes = 1000,
    DateTime? requestedAt,
    DateTime? completedAt,
    DateTime? lastReadAt,
  }) =>
      db.upsertDownload(DownloadsCompanion.insert(
        serverId: server,
        comicId: comicId,
        title: 'Album $comicId',
        pageCount: pageCount,
        pagesDone: Value(pagesDone),
        bytes: Value(bytes),
        state: state,
        requestedAt: requestedAt ?? DateTime.utc(2026, 1, 1),
        completedAt: Value(completedAt),
        lastReadAt: Value(lastReadAt),
      ));

  group('file d\'attente', () {
    test('se vide dans l\'ordre où on l\'a remplie', () async {
      await put('second', state: 'queued', requestedAt: DateTime.utc(2026, 3, 1));
      await put('premier', state: 'queued', requestedAt: DateTime.utc(2026, 1, 1));

      final next = await db.nextQueuedDownload('s1');
      expect(next?.comicId, 'premier');
    });

    test('un téléchargement interrompu reprend avant les suivants', () async {
      // `running` est aussi candidat : après une fermeture brutale, l'état
      // reste tel quel, et l'album à moitié téléchargé doit repartir en tête.
      await put('en-cours', state: 'running', requestedAt: DateTime.utc(2026, 1, 1));
      await put('attente', state: 'queued', requestedAt: DateTime.utc(2026, 2, 1));

      expect((await db.nextQueuedDownload('s1'))?.comicId, 'en-cours');
    });

    test('ni les complets ni les échecs ne reviennent dans la file', () async {
      await put('fini', state: 'complete');
      await put('rate', state: 'failed');

      expect(await db.nextQueuedDownload('s1'), isNull);
    });

    test('la file d\'un serveur ignore celle d\'un autre', () async {
      await put('a', server: 's2', state: 'queued');
      expect(await db.nextQueuedDownload('s1'), isNull);
      expect((await db.nextQueuedDownload('s2'))?.comicId, 'a');
    });
  });

  group('occupation', () {
    test('additionne les octets du serveur, et de lui seul', () async {
      await put('a', bytes: 1500);
      await put('b', bytes: 2500);
      await put('c', server: 's2', bytes: 9000);

      expect(await db.downloadedBytes('s1'), 4000);
      expect(await db.downloadedBytes('s2'), 9000);
    });

    test('un téléchargement partiel compte pour ce qu\'il occupe', () async {
      await put('a', state: 'running', pagesDone: 10, bytes: 400);
      expect(await db.downloadedBytes('s1'), 400);
    });
  });

  group('éviction', () {
    test('sacrifie d\'abord ce qui a été lu jusqu\'au bout', () async {
      await put('lu', lastReadAt: DateTime.utc(2026, 6, 1));
      await put('jamais-ouvert', lastReadAt: DateTime.utc(2020, 1, 1));

      await db.saveProgress(
          serverId: 's1', comicId: 'lu', page: 29, pageCount: 30, status: 'read');

      // « lu » a pourtant été ouvert bien plus récemment : l'avoir terminé
      // prime, parce qu'on le regrettera moins.
      final candidates = await db.evictionCandidates('s1');
      expect(candidates.first.comicId, 'lu');
    });

    test('à égalité, le plus anciennement ouvert part en premier', () async {
      await put('ancien', lastReadAt: DateTime.utc(2025, 1, 1));
      await put('recent', lastReadAt: DateTime.utc(2026, 1, 1));

      final candidates = await db.evictionCandidates('s1');
      expect(candidates.map((d) => d.comicId), ['ancien', 'recent']);
    });

    test('jamais ouvert : la date de fin fait foi', () async {
      await put('fini-hier', completedAt: DateTime.utc(2026, 6, 1));
      await put('fini-l-an-dernier', completedAt: DateTime.utc(2025, 6, 1));

      final candidates = await db.evictionCandidates('s1');
      expect(candidates.first.comicId, 'fini-l-an-dernier');
    });

    test('un téléchargement inachevé n\'est jamais candidat', () async {
      // L'évincer libérerait peu et détruirait un travail en cours.
      await put('partiel', state: 'running', pagesDone: 12, bytes: 500);
      await put('en-attente', state: 'queued', pagesDone: 0, bytes: 0);
      await put('complet');

      final candidates = await db.evictionCandidates('s1');
      expect(candidates.map((d) => d.comicId), ['complet']);
    });

    test('les albums d\'un autre serveur ne sont pas touchés', () async {
      await put('a', server: 's2');
      expect(await db.evictionCandidates('s1'), isEmpty);
    });
  });

  group('cycle de vie', () {
    test('supprimer efface le registre', () async {
      await put('a');
      await db.deleteDownload('s1', 'a');

      expect(await db.download('s1', 'a'), isNull);
      expect(await db.downloadedBytes('s1'), 0);
    });

    test('oublier un serveur emporte ses téléchargements', () async {
      await put('a');
      await put('b', server: 's2');

      await db.forgetServer('s1');

      expect(await db.downloadsOf('s1'), isEmpty);
      expect((await db.downloadsOf('s2')).length, 1);
    });

    test('le point de reprise est le nombre de pages écrites', () async {
      await put('a', state: 'paused', pageCount: 40, pagesDone: 17, bytes: 700);

      final resumed = await db.download('s1', 'a');
      expect(resumed?.pagesDone, 17);
      expect(resumed?.state, 'paused');
    });
  });
}
