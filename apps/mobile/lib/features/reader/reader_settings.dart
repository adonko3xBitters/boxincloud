import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../library/library_controller.dart';

/// Sens de lecture.
enum ReadingDirection { leftToRight, rightToLeft }

/*
Mode de lecture.

Deux modes, et pas les trois du web : la double page suppose un écran large,
qu'un téléphone n'a pas. La proposer ici donnerait deux planches illisibles.

Le défilement continu existe pour les webtoons, dont les planches sont des
bandes verticales sans découpage : les tourner une par une coupe le récit au
milieu d'une case.
*/
enum ReaderMode { paged, webtoon }

/*
Marge de lecture.

Sur un grand écran tenu à une main, une planche à fond perdu pousse le regard
jusqu'aux bords, là où la paume et la courbure de la dalle sont. La marge
ramène la planche dans la zone confortable, au prix d'une image plus petite.

Elle ne touche pas les zones de toucher, qui restent sur toute la largeur : les
rétrécir éloignerait le bord qui tourne la page, ce qui est exactement l'inverse
du but.
*/
enum ReaderMargin {
  none(0),
  small(0.05),
  medium(0.10),
  large(0.16);

  /// Part de la largeur retirée de chaque côté.
  final double fraction;

  const ReaderMargin(this.fraction);
}

/// Réglages du lecteur, retenus d'une session à l'autre.
class ReaderSettings {
  final ReadingDirection direction;
  final ReaderMode mode;
  final ReaderMargin margin;

  const ReaderSettings({
    this.direction = ReadingDirection.leftToRight,
    this.mode = ReaderMode.paged,
    this.margin = ReaderMargin.none,
  });

  ReaderSettings copyWith({
    ReadingDirection? direction,
    ReaderMode? mode,
    ReaderMargin? margin,
  }) => ReaderSettings(
    direction: direction ?? this.direction,
    mode: mode ?? this.mode,
    margin: margin ?? this.margin,
  );
}

/*
Réglages du lecteur, retenus d'une session à l'autre.

Quelqu'un qui lit des mangas en lit rarement un seul, et quelqu'un qui lit des
webtoons non plus. Un réglage oublié à chaque lancement obligerait à le refaire
avant chaque album — une correction manuelle, systématique, d'un défaut de
mémoire de l'application.

Ces réglages ne sont pas rattachés à un serveur : ce sont des habitudes de
personne, pas de bibliothèque. Ils ne sont pas non plus synchronisés entre
appareils, pour la raison qu'énonce le lecteur web — on veut le défilement
continu sur téléphone et autre chose sur un écran large.
*/
class ReaderSettingsNotifier extends Notifier<ReaderSettings> {
  // Cette clé et ses valeurs sont antérieures aux deux autres réglages et
  // restent telles quelles : des installations existantes les ont écrites.
  static const _directionKey = 'reader.direction';
  static const _modeKey = 'reader.mode';
  static const _marginKey = 'reader.margin';

  @override
  ReaderSettings build() => const ReaderSettings();

  /// Relit les préférences enregistrées.
  ///
  /// Appelée par le lecteur pendant son chargement, donc avant que les réglages
  /// ne servent à quoi que ce soit : les lire dans `build()` ferait rendre une
  /// première planche dans le mauvais mode, puis basculer sous les yeux.
  Future<void> restore() async {
    final db = ref.read(databaseProvider);

    final direction = await db.preference(_directionKey);
    final mode = await db.preference(_modeKey);
    final margin = await db.preference(_marginKey);

    state = ReaderSettings(
      direction:
          _parse(ReadingDirection.values, direction) ??
          ReadingDirection.leftToRight,
      mode: _parse(ReaderMode.values, mode) ?? ReaderMode.paged,
      margin: _parse(ReaderMargin.values, margin) ?? ReaderMargin.none,
    );
  }

  /// Retrouve une valeur d'énumération par son nom, sans lever sur l'inconnu.
  ///
  /// Une préférence écrite par une version ultérieure — un mode qui n'existe
  /// pas encore ici — ne doit pas empêcher d'ouvrir un album.
  static T? _parse<T extends Enum>(List<T> values, String? stored) {
    if (stored == null) return null;
    for (final value in values) {
      if (value.name == stored) return value;
    }
    return null;
  }

  Future<void> toggleDirection() => setDirection(
    state.direction == ReadingDirection.leftToRight
        ? ReadingDirection.rightToLeft
        : ReadingDirection.leftToRight,
  );

  Future<void> setDirection(ReadingDirection value) async {
    state = state.copyWith(direction: value);
    await ref.read(databaseProvider).setPreference(_directionKey, value.name);
  }

  Future<void> setMode(ReaderMode value) async {
    state = state.copyWith(mode: value);
    await ref.read(databaseProvider).setPreference(_modeKey, value.name);
  }

  Future<void> setMargin(ReaderMargin value) async {
    state = state.copyWith(margin: value);
    await ref.read(databaseProvider).setPreference(_marginKey, value.name);
  }
}

final readerSettingsProvider =
    NotifierProvider<ReaderSettingsNotifier, ReaderSettings>(
      ReaderSettingsNotifier.new,
    );
