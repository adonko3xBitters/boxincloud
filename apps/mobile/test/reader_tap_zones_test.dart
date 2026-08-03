import 'package:flutter_test/flutter_test.dart';

import 'package:boxincloud/features/reader/reader_screen.dart';

/*
Zones de toucher du lecteur.

Les bords tournent la page, le centre appelle l'habillage. Ce qui se teste ici
est l'inversion en lecture manga : elle produit une application qui tourne les
pages dans les deux cas, donc une erreur de sens ne se voit qu'en lisant un
manga jusqu'au bout et en trouvant la fin au début.
*/
void main() {
  const width = 400.0;

  ReaderTapZone zoneAt(double x, {required bool rightToLeft}) =>
      readerTapZone(x: x, width: width, rightToLeft: rightToLeft);

  group('lecture classique, de gauche à droite', () {
    test('le bord droit avance', () {
      expect(zoneAt(380, rightToLeft: false), ReaderTapZone.forward);
    });

    test('le bord gauche recule', () {
      expect(zoneAt(20, rightToLeft: false), ReaderTapZone.backward);
    });
  });

  group('lecture manga, de droite à gauche', () {
    test('le bord gauche avance — la lecture va vers la gauche', () {
      expect(zoneAt(20, rightToLeft: true), ReaderTapZone.forward);
    });

    test('le bord droit recule', () {
      expect(zoneAt(380, rightToLeft: true), ReaderTapZone.backward);
    });
  });

  group('le tiers central appelle l\'habillage', () {
    test('au milieu exact', () {
      expect(zoneAt(200, rightToLeft: false), ReaderTapZone.chrome);
      expect(zoneAt(200, rightToLeft: true), ReaderTapZone.chrome);
    });

    // Le centre ne dépend pas du sens : c'est la seule zone dont le rôle est le
    // même dans les deux modes, et l'y voir change interdit une régression où
    // le basculement manga déplacerait la zone d'habillage.
    test('juste après le premier tiers, dans les deux sens', () {
      expect(zoneAt(width * 0.34, rightToLeft: false), ReaderTapZone.chrome);
      expect(zoneAt(width * 0.34, rightToLeft: true), ReaderTapZone.chrome);
    });

    test('juste avant le dernier tiers, dans les deux sens', () {
      expect(zoneAt(width * 0.66, rightToLeft: false), ReaderTapZone.chrome);
      expect(zoneAt(width * 0.66, rightToLeft: true), ReaderTapZone.chrome);
    });
  });

  group('bornes', () {
    // Le tiers appartient au bord, pas au centre : à 0.33 pile, on tourne.
    test('la limite du tiers appartient au bord', () {
      expect(zoneAt(width * 0.33, rightToLeft: false), ReaderTapZone.backward);
      expect(zoneAt(width * 0.67, rightToLeft: false), ReaderTapZone.forward);
    });

    test('les extrémités exactes de l\'écran', () {
      expect(zoneAt(0, rightToLeft: false), ReaderTapZone.backward);
      expect(zoneAt(width, rightToLeft: false), ReaderTapZone.forward);
    });

    // Une largeur nulle survient pendant la première image d'une rotation.
    // Diviser par elle donnerait NaN, et NaN échoue toutes les comparaisons :
    // le toucher tomberait silencieusement dans « reculer ».
    test('une largeur nulle ne tourne pas la page', () {
      expect(
        readerTapZone(x: 0, width: 0, rightToLeft: false),
        ReaderTapZone.chrome,
      );
    });
  });
}
