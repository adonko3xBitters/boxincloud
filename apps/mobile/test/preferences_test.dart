import 'package:drift/native.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:boxincloud/core/db/database.dart';

/// Les préférences ne valent que si elles survivent à la fermeture de
/// l'application — c'est leur unique raison d'être.
void main() {
  late BoxDatabase db;

  setUp(() => db = BoxDatabase.forTesting(NativeDatabase.memory()));
  tearDown(() => db.close());

  test('une préférence absente ne vaut rien', () async {
    expect(await db.preference('reader.direction'), isNull);
  });

  test('une préférence se relit telle qu\'écrite', () async {
    await db.setPreference('reader.direction', 'rightToLeft');
    expect(await db.preference('reader.direction'), 'rightToLeft');
  });

  test('réécrire remplace au lieu d\'ajouter', () async {
    await db.setPreference('reader.direction', 'rightToLeft');
    await db.setPreference('reader.direction', 'leftToRight');

    expect(await db.preference('reader.direction'), 'leftToRight');
    expect(await db.select(db.preferences).get(), hasLength(1));
  });

  test('les clés ne se marchent pas dessus', () async {
    await db.setPreference('a', '1');
    await db.setPreference('b', '2');

    expect(await db.preference('a'), '1');
    expect(await db.preference('b'), '2');
  });

  test('oublier un serveur ne touche pas aux préférences', () async {
    // Le sens de lecture est une habitude de personne, pas de bibliothèque :
    // se déconnecter d'une instance ne doit pas le remettre à zéro.
    await db.setPreference('reader.direction', 'rightToLeft');
    await db.forgetServer('serveur-1');

    expect(await db.preference('reader.direction'), 'rightToLeft');
  });
}
