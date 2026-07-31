import 'package:flutter_test/flutter_test.dart';

import 'package:boxincloud/core/auth/servers.dart';

/*
Normalisation de l'adresse d'un serveur.

Elle est tapée à la main, au clavier d'un téléphone, souvent de mémoire. La
refuser sur la forme n'apprend rien à personne et bloque au tout premier écran :
mieux vaut accepter les formes courantes et les ramener à quelque chose
d'utilisable.
*/
void main() {
  group('adresse de serveur', () {
    test('https est ajouté par défaut', () {
      expect(normalizeServerUrl('bd.exemple.fr'), 'https://bd.exemple.fr');
    });

    test('http explicite est respecté', () {
      // Sur un réseau local, http est le cas courant : forcer https rendrait
      // le serveur inatteignable.
      expect(normalizeServerUrl('http://192.168.1.10:8070'), 'http://192.168.1.10:8070');
    });

    test('le port est conservé', () {
      expect(normalizeServerUrl('bd.exemple.fr:8070'), 'https://bd.exemple.fr:8070');
    });

    test('les espaces autour sont ignorés', () {
      expect(normalizeServerUrl('  bd.exemple.fr  '), 'https://bd.exemple.fr');
    });

    test('le chemin est retiré', () {
      // Coller l'adresse d'un écran de l'interface web ne doit pas produire un
      // serveur introuvable : l'API vit toujours sous /api/v1.
      expect(
        normalizeServerUrl('https://bd.exemple.fr/partage?t=abc'),
        'https://bd.exemple.fr',
      );
    });

    test('une adresse vide ou absurde est rejetée', () {
      expect(normalizeServerUrl(''), '');
      expect(normalizeServerUrl('   '), '');
      expect(normalizeServerUrl('https://'), '');
    });
  });
}
