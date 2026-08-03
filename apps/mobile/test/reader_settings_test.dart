/*
Réglages du lecteur, et leur relecture.

Ces trois préférences vivent en base et traversent les versions. Ce qui se
teste ici est surtout la relecture de valeurs inattendues : une préférence
écrite par une version ultérieure ne doit pas empêcher d'ouvrir un album.
*/

import 'package:drift/native.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:boxincloud/core/db/database.dart';
import 'package:boxincloud/features/library/library_controller.dart';
import 'package:boxincloud/features/reader/reader_settings.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  late BoxDatabase db;
  late ProviderContainer container;

  setUp(() {
    db = BoxDatabase.forTesting(NativeDatabase.memory());
    container = ProviderContainer(
      overrides: [databaseProvider.overrideWithValue(db)],
    );
  });

  tearDown(() async {
    container.dispose();
    await db.close();
  });

  ReaderSettingsNotifier notifier() =>
      container.read(readerSettingsProvider.notifier);

  ReaderSettings settings() => container.read(readerSettingsProvider);

  test('les défauts conviennent à une bande dessinée occidentale', () {
    expect(settings().mode, ReaderMode.paged);
    expect(settings().direction, ReadingDirection.leftToRight);
    expect(settings().margin, ReaderMargin.none);
  });

  test('chaque réglage survit à une relecture', () async {
    await notifier().setMode(ReaderMode.webtoon);
    await notifier().setDirection(ReadingDirection.rightToLeft);
    await notifier().setMargin(ReaderMargin.large);

    // Un second conteneur : c'est ce que fait un relancement de
    // l'application, pas un simple `state =`.
    final second = ProviderContainer(
      overrides: [databaseProvider.overrideWithValue(db)],
    );
    addTearDown(second.dispose);

    await second.read(readerSettingsProvider.notifier).restore();
    final restored = second.read(readerSettingsProvider);

    expect(restored.mode, ReaderMode.webtoon);
    expect(restored.direction, ReadingDirection.rightToLeft);
    expect(restored.margin, ReaderMargin.large);
  });

  test('le sens de lecture garde la clé et les valeurs d\'origine', () async {
    // Des installations existantes ont écrit ceci avant que les deux autres
    // réglages n'existent. Changer la clé ou le nom des valeurs perdrait
    // silencieusement la préférence d'un lecteur de mangas.
    await notifier().setDirection(ReadingDirection.rightToLeft);
    expect(await db.preference('reader.direction'), 'rightToLeft');
  });

  test('une valeur inconnue retombe sur le défaut, sans lever', () async {
    // Ce que produirait une version ultérieure qui aurait ajouté un mode.
    await db.setPreference('reader.mode', 'panorama');
    await db.setPreference('reader.margin', 'énorme');
    await db.setPreference('reader.direction', 'bottomToTop');

    await notifier().restore();

    expect(settings().mode, ReaderMode.paged);
    expect(settings().margin, ReaderMargin.none);
    expect(settings().direction, ReadingDirection.leftToRight);
  });

  test('une préférence absente laisse le défaut', () async {
    await db.setPreference('reader.mode', 'webtoon');
    await notifier().restore();

    expect(settings().mode, ReaderMode.webtoon);
    // Les deux autres n'ont jamais été écrites.
    expect(settings().margin, ReaderMargin.none);
    expect(settings().direction, ReadingDirection.leftToRight);
  });

  test('les marges vont en croissant et commencent à zéro', () {
    expect(ReaderMargin.none.fraction, 0);

    var previous = -1.0;
    for (final margin in ReaderMargin.values) {
      expect(margin.fraction, greaterThan(previous));
      // Deux fois la marge doit laisser de la place à la planche.
      expect(margin.fraction * 2, lessThan(1));
      previous = margin.fraction;
    }
  });
}
