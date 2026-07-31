import 'package:flutter_test/flutter_test.dart';

import 'package:boxincloud/core/search/local_search.dart';

/// La recherche locale décide de ce qu'on trouve hors ligne. Elle échoue en
/// silence : un résultat manquant ressemble à un album absent du cache.
void main() {
  group('fold', () {
    test('replie les accents', () {
      expect(fold('Astérix'), 'asterix');
      expect(fold('Persépolis'), 'persepolis');
      expect(fold('Château'), 'chateau');
      expect(fold('Naïve'), 'naive');
    });

    test('replie les ligatures et l\'eszett', () {
      expect(fold('Cœur'), 'coeur');
      expect(fold('Æon'), 'aeon');
      expect(fold('Straße'), 'strasse');
    });

    test('écarte la ponctuation sans coller les mots', () {
      expect(fold("L'Œil du loup"), 'l oeil du loup');
      expect(fold('Blake & Mortimer'), 'blake mortimer');
      expect(fold('Tome  7 :  la  suite'), 'tome 7 la suite');
    });

    test('conserve les chiffres', () => expect(fold('Tome 10'), 'tome 10'));
  });

  group('score', () {
    test('classe exact, puis début, puis milieu', () {
      final exact = score('Astérix', fold('asterix'))!;
      final prefix = score('Astérix et les Normands', fold('asterix'))!;
      final word = score('Le retour d\'Astérix', fold('asterix'))!;

      expect(exact, lessThan(prefix));
      expect(prefix, lessThan(word));
    });

    test('à rang égal, le plus court gagne', () {
      final short = score('Astérix légionnaire', fold('asterix'))!;
      final long = score('Astérix et le chaudron magique', fold('asterix'))!;

      expect(short, lessThan(long));
    });

    test('un début de mot bat un fragment au milieu', () {
      // « rix » est un fragment dans « Astérix », un début de mot dans « Rixe ».
      final fragment = score('Astérix', fold('rix'))!;
      final wordStart = score('Grande Rixe', fold('rix'))!;

      expect(wordStart, lessThan(fragment));
    });

    test('refuse ce qui ne correspond pas', () {
      expect(score('Astérix', fold('tintin')), isNull);
      expect(score('Astérix', fold('')), isNull);
    });

    test('ne pardonne pas les fautes de frappe — c\'est le rôle du serveur', () {
      expect(score('Astérix', fold('asterics')), isNull);
    });
  });

  group('rank', () {
    final albums = [
      ('Astérix le Gaulois', 'Astérix'),
      ('Le Combat des chefs', 'Astérix'),
      ('Persépolis', ''),
      ('Blacksad', 'Blacksad'),
    ];

    List<String> titlesFor(String query, {int limit = 50}) => rank(
          albums,
          query,
          (a) => [a.$1, a.$2],
          limit: limit,
        ).map((a) => a.$1).toList();

    test('trouve par le titre comme par la série', () {
      // « Le Combat des chefs » ne contient pas « astérix » : c'est sa série
      // qui le fait remonter.
      expect(titlesFor('asterix'), ['Astérix le Gaulois', 'Le Combat des chefs']);
    });

    test('ignore les accents dans la requête et dans les données', () {
      expect(titlesFor('persepolis'), ['Persépolis']);
      expect(titlesFor('PERSÉPOLIS'), ['Persépolis']);
    });

    test('exige deux caractères', () {
      expect(titlesFor('a'), isEmpty);
      expect(titlesFor(' é '), isEmpty);
      expect(titlesFor('bl'), ['Blacksad']);
    });

    test('respecte la limite', () {
      expect(titlesFor('asterix', limit: 1), ['Astérix le Gaulois']);
    });

    test('ne retourne rien plutôt que tout sur une requête vide', () {
      expect(titlesFor(''), isEmpty);
    });
  });
}
