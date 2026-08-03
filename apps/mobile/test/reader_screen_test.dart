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

import 'dart:convert';
import 'dart:io';

import 'package:cached_network_image/cached_network_image.dart';
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

  group('défilement continu', () {
    /// Choisit une option des réglages, en ouvrant le panneau au besoin.
    ///
    /// Le bouton bascule : appuyer alors que le panneau est déjà ouvert le
    /// referme. C'est ce qui arrive quand on enchaîne deux choix.
    Future<void> choose(WidgetTester tester, String label) async {
      if (find.text('RÉGLAGES DE LECTURE').evaluate().isEmpty) {
        await tester.tap(find.byIcon(Icons.tune_rounded));
        await settle(tester);
      }
      await tester.tap(find.text(label));
      await settle(tester);
    }

    testWidgets('le mode par défaut reste planche par planche', (tester) async {
      await openReader(tester);

      expect(find.byType(PageView), findsOneWidget);
    });

    testWidgets('le mode remplace la pagination par une liste verticale',
        (tester) async {
      await openReader(tester);
      await choose(tester, 'Défilement continu');

      // La pagination disparaît : les deux modes ne coexistent pas, sans quoi
      // deux contrôleurs se disputeraient la position de lecture.
      expect(find.byType(PageView), findsNothing);

      final list = tester.widget<ListView>(find.byType(ListView));
      expect(list.scrollDirection, Axis.vertical);
    });

    testWidgets('le sens de lecture disparaît en défilement continu',
        (tester) async {
      await openReader(tester);

      await tester.tap(find.byIcon(Icons.tune_rounded));
      await settle(tester);
      expect(find.text('Sens de lecture'), findsOneWidget);

      await tester.tap(find.text('Défilement continu'));
      await settle(tester);

      // La lecture y est verticale : montrer le réglage ferait croire à une
      // option sans effet.
      expect(find.text('Sens de lecture'), findsNothing);
    });

    testWidgets('le mode choisi est retenu', (tester) async {
      await openReader(tester);
      await choose(tester, 'Défilement continu');

      expect(await db.preference('reader.mode'), 'webtoon');
    });

    testWidgets('revenir en arrière rétablit la pagination', (tester) async {
      await openReader(tester);
      await choose(tester, 'Défilement continu');
      expect(find.byType(PageView), findsNothing);

      await choose(tester, 'Planche par planche');
      expect(find.byType(PageView), findsOneWidget);
    });

    /*
      Les hauteurs sont réservées d'après les proportions du manifeste, et c'est
      ce calcul qui dit quelle planche on lit. Il n'a rien de visible : une
      erreur de décalage donnerait un compteur faux et une reprise au mauvais
      endroit, sans que l'affichage bouge d'un pixel.

      L'écran de test fait 800 de large ; une planche de 1000×1500 y occupe donc
      1200 de haut.
    */
    const pageHeight = 800 * 1500 / 1000;

    testWidgets('faire défiler d\'une planche avance le compteur',
        (tester) async {
      await openReader(tester);
      await choose(tester, 'Défilement continu');
      expect(find.text('1 / $pageCount'), findsOneWidget);

      await tester.drag(
        find.byType(ListView),
        const Offset(0, -pageHeight - 50),
      );
      await settle(tester);

      expect(find.text('2 / $pageCount'), findsOneWidget);
    });

    testWidgets('rester à l\'intérieur d\'une planche ne change pas le compteur',
        (tester) async {
      await openReader(tester);
      await choose(tester, 'Défilement continu');

      await tester.drag(find.byType(ListView), const Offset(0, -pageHeight / 2));
      await settle(tester);

      // Une planche de webtoon dépasse largement l'écran : la moitié parcourue
      // n'est pas une page tournée.
      expect(find.text('1 / $pageCount'), findsOneWidget);
    });

    testWidgets('choisir une vignette fait défiler jusqu\'à la planche',
        (tester) async {
      await openReader(tester);
      await choose(tester, 'Défilement continu');

      await tester.tap(find.byIcon(Icons.grid_view_rounded));
      await settle(tester);
      await tester.tap(find.text('5').last);
      await settle(tester);

      // Le curseur et les vignettes désignent une planche ; c'est au lecteur de
      // savoir que cela veut dire faire défiler, ici, et non tourner.
      expect(find.text('5 / $pageCount'), findsOneWidget);
    });
  });

  group('marge de lecture', () {
    Future<double> imageWidth(WidgetTester tester) async =>
        tester.getSize(find.byType(CachedNetworkImage).first).width;

    testWidgets('sans marge, la planche occupe toute la largeur',
        (tester) async {
      await openReader(tester);

      final screen = tester.getSize(find.byType(PageView)).width;
      expect(await imageWidth(tester), screen);
    });

    testWidgets('une marge large rétrécit la planche', (tester) async {
      await openReader(tester);

      final screen = tester.getSize(find.byType(PageView)).width;

      await tester.tap(find.byIcon(Icons.tune_rounded));
      await settle(tester);
      await tester.tap(find.text('Large'));
      await settle(tester);

      // 16 % retirés de chaque côté.
      expect(await imageWidth(tester), closeTo(screen * 0.68, 0.5));
    });

    testWidgets('la marge ne rétrécit pas les zones de toucher',
        (tester) async {
      await openReader(tester);

      await tester.tap(find.byIcon(Icons.tune_rounded));
      await settle(tester);
      await tester.tap(find.text('Large'));
      await settle(tester);
      await tester.tap(find.byIcon(Icons.tune_rounded));
      await settle(tester);

      // Le bord extrême est hors de l'image, mais dans la zone qui tourne la
      // page : rétrécir les gestes avec l'image éloignerait ce bord du pouce,
      // ce qui est l'inverse du but de la marge.
      final box = tester.getRect(find.byType(PageView));
      await tester.tapAt(Offset(box.right - 2, box.center.dy));
      await settle(tester);

      expect(find.text('2 / $pageCount'), findsOneWidget);
    });

    testWidgets('la marge choisie est retenue', (tester) async {
      await openReader(tester);

      await tester.tap(find.byIcon(Icons.tune_rounded));
      await settle(tester);
      await tester.tap(find.text('Moyenne'));
      await settle(tester);

      expect(await db.preference('reader.margin'), 'medium');
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
