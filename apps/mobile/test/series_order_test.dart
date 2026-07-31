import 'package:flutter_test/flutter_test.dart';

import 'package:boxincloud/features/library/series_screen.dart';

/*
Le tri par tome.

C'est ce qu'une série apporte qu'un dossier ne donne pas. Le numéro est une
chaîne — « 7 », « 07 », « HS 2 » — et un tri textuel placerait « 10 » entre
« 1 » et « 2 », ce qui rend une série illisible.
*/
void main() {
  group('ordre des tomes', () {
    test('le tri est numérique, pas alphabétique', () {
      final tomes = ['10', '2', '1', '21', '3'];
      tomes.sort(compareTome);

      expect(tomes, ['1', '2', '3', '10', '21']);
    });

    test('les zéros de tête ne changent rien', () {
      final tomes = ['07', '10', '1'];
      tomes.sort(compareTome);

      expect(tomes, ['1', '07', '10']);
    });

    test('un numéro décimal s\'intercale', () {
      // Les tomes « 7.5 » existent : hors-série intercalé, ou double album.
      final tomes = ['8', '7.5', '7'];
      tomes.sort(compareTome);

      expect(tomes, ['7', '7.5', '8']);
    });

    /*
      Un album sans numéro passe en dernier.

      Un hors-série ou une intégrale n'a pas à s'intercaler entre deux tomes :
      il n'a pas de place dans la suite, et lui en inventer une déplacerait tout
      ce qui suit.
    */
    test('les albums sans numéro ferment la marche', () {
      final tomes = ['', '2', '', '1'];
      tomes.sort(compareTome);

      expect(tomes, ['1', '2', '', '']);
    });

    test('les numéros non numériques restent alphabétiques entre eux', () {
      final tomes = ['HS 2', 'HS 1'];
      tomes.sort(compareTome);

      expect(tomes, ['HS 1', 'HS 2']);
    });
  });
}
