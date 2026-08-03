import 'dart:convert';
import 'dart:io';

import 'package:drift/native.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:path_provider_platform_interface/path_provider_platform_interface.dart';
import 'package:plugin_platform_interface/plugin_platform_interface.dart';

import 'package:boxincloud/core/api/client.dart';
import 'package:boxincloud/core/api/models.dart';
import 'package:boxincloud/core/auth/servers.dart';
import 'package:boxincloud/core/auth/session.dart';
import 'package:boxincloud/core/db/database.dart';
import 'package:boxincloud/features/library/library_controller.dart';
import 'package:boxincloud/features/reader/reader_screen.dart';
import 'package:boxincloud/shared/theme.dart';

/*
Le lecteur, éprouvé sur ses gestes.

Ces tests existent à cause d'un défaut signalé à l'usage : agrandi sur un
détail, tirer pour voir le reste de la planche changeait de page. Les deux
gestes sont le même — un glissement horizontal — et le PageView l'emportait sur
l'image.

Rien dans le code ne le disait, et rien ne pouvait le dire : les deux
comportements sont légitimes, seule leur priorité était fausse. D'où des tests
qui portent sur la priorité, pas sur la présence des widgets.
*/
/*
Avance le temps sans attendre le repos.

`pumpAndSettle` ne convient pas ici : une planche dont l'image n'arrive jamais
garde son indicateur de chargement, qui tourne indéfiniment. Attendre la
quiescence reviendrait donc à attendre pour toujours — et c'est le cas normal
d'un lecteur, pas une anomalie du test.

Six cents millisecondes couvrent largement l'animation de changement de page,
qui dure deux cent vingt.
*/
Future<void> settle(WidgetTester tester) async {
  for (var i = 0; i < 12; i++) {
    await tester.pump(const Duration(milliseconds: 50));
  }
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  late Directory temporary;
  late BoxDatabase db;

  setUp(() async {
    temporary = await Directory.systemTemp.createTemp('boxincloud-reader');
    PathProviderPlatform.instance = _FakePathProvider(temporary.path);
    db = BoxDatabase.forTesting(NativeDatabase.memory());
  });

  tearDown(() async {
    await db.close();
    if (await temporary.exists()) await temporary.delete(recursive: true);
  });

  const pageCount = 12;

  /// Un serveur qui rend un manifeste et refuse tout le reste.
  ///
  /// Les images échouent volontairement : ce qui est mesuré ici est le geste,
  /// et une planche cassée occupe exactement la même surface qu'une planche
  /// affichée.
  ApiClient clientWith({int pages = pageCount}) => ApiClient(
        baseUrl: 'https://exemple.test',
        httpClient: MockClient((request) async {
          if (request.url.path.endsWith('/manifest')) {
            return http.Response(
              jsonEncode({
                'comicId': 'c1',
                'pageCount': pages,
                'pages': [
                  for (var i = 0; i < pages; i++)
                    {'index': i, 'width': 1000, 'height': 1500, 'isDouble': false},
                ],
              }),
              200,
              headers: {'content-type': 'application/json'},
            );
          }
          return http.Response('{}', 404);
        }),
      );

  Widget host(ApiClient client) {
    final session = SessionActive(
      server: const ServerAccount(
        id: 's1',
        baseUrl: 'https://exemple.test',
        label: 'exemple.test',
        username: 'niando',
      ),
      user: const User(id: 'u1', username: 'niando', role: 'admin', restricted: false),
      client: client,
      servers: const [],
    );

    return ProviderScope(
      overrides: [
        databaseProvider.overrideWithValue(db),
        sessionProvider.overrideWith(() => _FixedSession(session)),
      ],
      child: MaterialApp(
        theme: boxTheme(Brightness.dark),
        home: const ReaderScreen(comicId: 'c1', title: 'Album'),
      ),
    );
  }

  /// Monte le lecteur et attend que le manifeste soit arrivé.
  Future<void> openReader(WidgetTester tester) async {
    await tester.pumpWidget(host(clientWith()));
    await settle(tester);
  }

  PageView readerPageView(WidgetTester tester) =>
      tester.widget<PageView>(find.byType(PageView));

  /// Double tap au point donné, avec l'intervalle qu'attend le détecteur.
  Future<void> doubleTapAt(WidgetTester tester, Offset position) async {
    await tester.tapAt(position);
    await tester.pump(kDoubleTapMinTime);
    await tester.tapAt(position);
    await settle(tester);
  }

  testWidgets('le lecteur s\'ouvre sur la première planche', (tester) async {
    await openReader(tester);

    expect(find.byType(PageView), findsOneWidget);
    expect(find.text('1 / $pageCount'), findsOneWidget);
  });

  group('zoom et défilement', () {
    testWidgets('au repos, les pages défilent', (tester) async {
      await openReader(tester);

      expect(readerPageView(tester).physics, isA<PageScrollPhysics>());
    });

    testWidgets('agrandi, le glissement horizontal n\'appartient plus aux pages',
        (tester) async {
      await openReader(tester);

      final centre = tester.getCenter(find.byType(PageView));
      await doubleTapAt(tester, centre);

      // Le cœur du défaut signalé : sans ceci, tirer pour explorer la planche
      // agrandie tournait la page.
      expect(readerPageView(tester).physics, isA<NeverScrollableScrollPhysics>());
    });

    testWidgets('revenir à l\'échelle 1 rend le défilement aux pages',
        (tester) async {
      await openReader(tester);

      final centre = tester.getCenter(find.byType(PageView));
      await doubleTapAt(tester, centre);
      expect(readerPageView(tester).physics, isA<NeverScrollableScrollPhysics>());

      // Un second double tap remet à plat.
      await doubleTapAt(tester, centre);
      expect(readerPageView(tester).physics, isA<PageScrollPhysics>());
    });

    testWidgets('agrandi, un toucher de bord ne tourne pas la page',
        (tester) async {
      await openReader(tester);

      final box = tester.getRect(find.byType(PageView));
      await doubleTapAt(tester, box.center);

      await tester.tapAt(Offset(box.right - 8, box.center.dy));
      await settle(tester);

      expect(find.text('1 / $pageCount'), findsOneWidget);
    });
  });

  group('toucher des bords', () {
    testWidgets('le bord droit avance en lecture classique', (tester) async {
      await openReader(tester);

      final box = tester.getRect(find.byType(PageView));
      await tester.tapAt(Offset(box.right - 8, box.center.dy));
      await settle(tester);

      expect(find.text('2 / $pageCount'), findsOneWidget);
    });

    testWidgets('le bord gauche ne recule pas depuis la première planche',
        (tester) async {
      await openReader(tester);

      final box = tester.getRect(find.byType(PageView));
      await tester.tapAt(Offset(box.left + 8, box.center.dy));
      await settle(tester);

      // Aucune page avant la première : le toucher est sans effet, et surtout
      // sans erreur.
      expect(find.text('1 / $pageCount'), findsOneWidget);
    });
  });

  group('vignettes', () {
    testWidgets('la bande n\'est pas montée tant qu\'on ne l\'ouvre pas',
        (tester) async {
      await openReader(tester);

      // Un album de deux cents planches déclencherait autant de requêtes.
      expect(find.text('$pageCount pages'), findsNothing);
    });

    testWidgets('le bouton ouvre la bande', (tester) async {
      await openReader(tester);

      await tester.tap(find.byIcon(Icons.grid_view_rounded));
      await settle(tester);

      expect(find.text('$pageCount pages'), findsOneWidget);
    });

    testWidgets('choisir une vignette va à la planche et referme la bande',
        (tester) async {
      await openReader(tester);

      await tester.tap(find.byIcon(Icons.grid_view_rounded));
      await settle(tester);

      // Le numéro sous la vignette de la cinquième planche.
      await tester.tap(find.text('5').last);
      await settle(tester);

      expect(find.text('5 / $pageCount'), findsOneWidget);
      expect(find.text('$pageCount pages'), findsNothing);
    });
  });
}

/// Session déjà ouverte, sans passer par le stockage sécurisé.
class _FixedSession extends SessionController {
  final SessionState _state;

  _FixedSession(this._state);

  @override
  SessionState build() => _state;
}

class _FakePathProvider extends PathProviderPlatform
    with MockPlatformInterfaceMixin {
  final String root;

  _FakePathProvider(this.root);

  @override
  Future<String?> getApplicationDocumentsPath() async => root;

  @override
  Future<String?> getApplicationSupportPath() async => root;

  @override
  Future<String?> getTemporaryPath() async => root;
}
