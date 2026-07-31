import 'dart:io';
import 'dart:typed_data';

import 'package:drift/drift.dart' show Value;
import 'package:drift/native.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:path_provider_platform_interface/path_provider_platform_interface.dart';
import 'package:plugin_platform_interface/plugin_platform_interface.dart';

import 'package:boxincloud/core/api/client.dart';
import 'package:boxincloud/core/db/database.dart';
import 'package:boxincloud/core/offline/download_manager.dart';
import 'package:boxincloud/core/offline/storage.dart';

/// Le gestionnaire de téléchargements, face à un serveur simulé.
///
/// C'est la pièce qui décide de ce qu'on peut lire sans réseau, et ses défauts
/// ne se manifestent qu'au mauvais moment : coupure à mi-parcours, disque
/// plein, serveur qui refuse. Les provoquer ici évite de les découvrir dans un
/// train.
void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  late Directory temporary;
  late BoxDatabase db;

  setUp(() async {
    temporary = await Directory.systemTemp.createTemp('boxincloud-test');
    PathProviderPlatform.instance = _FakePathProvider(temporary.path);
    db = BoxDatabase.forTesting(NativeDatabase.memory());
  });

  tearDown(() async {
    await db.close();
    if (await temporary.exists()) await temporary.delete(recursive: true);
  });

  /// Un serveur qui rend une page de `size` octets, ou ce qu'on lui dit.
  DownloadManager managerWith(
    Future<http.Response> Function(http.Request request) handler, {
    String serverId = 's1',
  }) {
    final client = ApiClient(
      baseUrl: 'https://exemple.test',
      httpClient: MockClient((request) => handler(request)),
    );
    return DownloadManager(db: db, client: client, serverId: serverId);
  }

  http.Response page(int size) => http.Response.bytes(
        Uint8List(size),
        200,
        headers: {'content-type': 'image/jpeg'},
      );

  group('téléchargement', () {
    test('écrit chaque page et se déclare complet', () async {
      final manager = managerWith((_) async => page(100));

      await manager.enqueue(
        comicId: 'c1',
        title: 'Album',
        seriesName: '',
        coverPath: '',
        pageCount: 3,
      );
      await manager.whenIdle();

      final download = await db.download('s1', 'c1');
      expect(download?.state, 'complete');
      expect(download?.pagesDone, 3);
      expect(download?.bytes, 300);

      for (var index = 0; index < 3; index += 1) {
        expect(await File(await pagePath('s1', 'c1', index)).exists(), isTrue);
      }
    });

    test('ne laisse aucun fichier temporaire derrière lui', () async {
      // L'écriture passe par un `.part` renommé : en trouver un signifierait
      // qu'une page a été considérée comme écrite alors qu'elle ne l'est pas.
      final manager = managerWith((_) async => page(50));

      await manager.enqueue(
        comicId: 'c1',
        title: 'Album',
        seriesName: '',
        coverPath: '',
        pageCount: 2,
      );
      await manager.whenIdle();

      final directory = await comicDirectory('s1', 'c1');
      final leftovers = await directory
          .list()
          .where((e) => e.path.endsWith('.part'))
          .toList();
      expect(leftovers, isEmpty);
    });

    test('une coupure réseau interrompt sans perdre ce qui est écrit', () async {
      var served = 0;
      final manager = managerWith((_) async {
        served += 1;
        if (served > 2) throw http.ClientException('réseau coupé');
        return page(100);
      });

      await manager.enqueue(
        comicId: 'c1',
        title: 'Album',
        seriesName: '',
        coverPath: '',
        pageCount: 10,
      );
      await manager.whenIdle();

      final download = await db.download('s1', 'c1');
      expect(download?.state, 'paused');
      expect(download?.pagesDone, 2);
      expect(download?.bytes, 200);
    });

    test('la reprise repart de la page suivante, pas du début', () async {
      final requested = <String>[];
      var fail = true;

      final manager = managerWith((request) async {
        requested.add(request.url.path);
        if (fail && requested.length > 2) {
          throw http.ClientException('réseau coupé');
        }
        return page(100);
      });

      await manager.enqueue(
        comicId: 'c1',
        title: 'Album',
        seriesName: '',
        coverPath: '',
        pageCount: 5,
      );
      await manager.whenIdle();
      expect((await db.download('s1', 'c1'))?.pagesDone, 2);

      fail = false;
      requested.clear();
      manager.start();
      await manager.whenIdle();

      final download = await db.download('s1', 'c1');
      expect(download?.state, 'complete');
      expect(download?.pagesDone, 5);
      // Les pages 0 et 1 ne sont pas redemandées.
      expect(requested.map((p) => p.split('/').last), ['2', '3', '4']);
    });

    test('un refus du serveur échoue franchement, sans réessayer', () async {
      var calls = 0;
      final manager = managerWith((_) async {
        calls += 1;
        return http.Response('{"title":"interdit"}', 403);
      });

      await manager.enqueue(
        comicId: 'c1',
        title: 'Album',
        seriesName: '',
        coverPath: '',
        pageCount: 5,
      );
      await manager.whenIdle();

      expect((await db.download('s1', 'c1'))?.state, 'failed');
      expect(calls, 1);
    });

    test('un album déjà complet n\'est pas retéléchargé', () async {
      var calls = 0;
      final manager = managerWith((_) async {
        calls += 1;
        return page(100);
      });

      Future<void> ask() => manager.enqueue(
            comicId: 'c1',
            title: 'Album',
            seriesName: '',
            coverPath: '',
            pageCount: 2,
          );

      await ask();
      await manager.whenIdle();
      expect(calls, 2);

      await ask();
      await manager.whenIdle();
      expect(calls, 2);
    });
  });

  group('budget', () {
    test('évince un album lu pour faire de la place', () async {
      final manager = managerWith((_) async => page(100));
      await manager.setBudgetBytes(500);

      // Un album déjà là, terminé et lu : le premier sacrifiable.
      await db.upsertDownload(DownloadsCompanion.insert(
        serverId: 's1',
        comicId: 'ancien',
        title: 'Ancien',
        pageCount: 5,
        pagesDone: const Value(5),
        bytes: const Value(500),
        state: 'complete',
        requestedAt: DateTime.utc(2025, 1, 1),
        completedAt: Value(DateTime.utc(2025, 1, 1)),
      ));
      await db.saveProgress(
          serverId: 's1', comicId: 'ancien', page: 4, pageCount: 5, status: 'read');

      await manager.enqueue(
        comicId: 'nouveau',
        title: 'Nouveau',
        seriesName: '',
        coverPath: '',
        pageCount: 2,
      );
      await manager.whenIdle();

      expect(await db.download('s1', 'ancien'), isNull);
      expect((await db.download('s1', 'nouveau'))?.state, 'complete');
    });

    test('échoue plutôt que d\'évincer l\'album en cours', () async {
      final manager = managerWith((_) async => page(400));
      await manager.setBudgetBytes(500);

      await manager.enqueue(
        comicId: 'c1',
        title: 'Album',
        seriesName: '',
        coverPath: '',
        pageCount: 5,
      );
      await manager.whenIdle();

      final download = await db.download('s1', 'c1');
      expect(download?.state, 'failed');
      expect(download?.error, contains('Budget'));
      // Ce qui a été écrit avant la bascule reste : l'album est partiellement
      // lisible plutôt que rien du tout.
      expect(download!.pagesDone, greaterThan(0));
    });

    test('le budget par défaut s\'applique sans réglage', () async {
      final manager = managerWith((_) async => page(1));
      expect(await manager.budgetBytes(), defaultBudgetBytes);
    });

    test('le budget choisi survit à un nouveau gestionnaire', () async {
      final first = managerWith((_) async => page(1));
      await first.setBudgetBytes(2048);

      final second = managerWith((_) async => page(1));
      expect(await second.budgetBytes(), 2048);
    });
  });

  group('suppression', () {
    test('efface les fichiers et le registre', () async {
      final manager = managerWith((_) async => page(100));

      await manager.enqueue(
        comicId: 'c1',
        title: 'Album',
        seriesName: '',
        coverPath: '',
        pageCount: 2,
      );
      await manager.whenIdle();

      final directory = await comicDirectory('s1', 'c1');
      expect(await directory.exists(), isTrue);

      await manager.remove('c1');

      expect(await db.download('s1', 'c1'), isNull);
      expect(await directory.exists(), isFalse);
    });
  });
}

class _FakePathProvider extends PathProviderPlatform with MockPlatformInterfaceMixin {
  final String root;

  _FakePathProvider(this.root);

  @override
  Future<String?> getApplicationSupportPath() async => root;
}
