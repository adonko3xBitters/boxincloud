import 'dart:io';

import 'package:drift/drift.dart';
import 'package:drift/native.dart';
import 'package:path/path.dart' as p;
import 'package:path_provider/path_provider.dart';

part 'database.g.dart';

/*
Cache local du catalogue.

Il ne sert pas à aller plus vite — le serveur répond en quelques millisecondes
sur un réseau correct. Il sert à ce que l'application s'ouvre SANS réseau : dans
le métro, en avion, chez quelqu'un dont le wifi ne porte pas. Sans lui, une
application de lecture affiche un écran vide dès que la connexion tombe, ce qui
est précisément le moment où l'on veut lire.

Le schéma reprend celui du serveur plutôt que d'aplatir : garder les albums, les
séries et les dossiers relationnels permet d'interroger le cache exactement comme
on interroge l'API, sans deuxième logique de filtrage.
*/

/// Albums, tels que le serveur les a décrits au dernier passage.
class CachedComics extends Table {
  TextColumn get id => text()();
  TextColumn get serverId => text()();
  TextColumn get libraryId => text()();
  TextColumn get title => text()();
  TextColumn get seriesId => text().nullable()();
  TextColumn get seriesName => text().withDefault(const Constant(''))();
  TextColumn get number => text().withDefault(const Constant(''))();
  TextColumn get folderPath => text().withDefault(const Constant(''))();
  IntColumn get pageCount => integer().withDefault(const Constant(0))();
  TextColumn get coverPath => text().withDefault(const Constant(''))();

  /// Aperçu flouté encodé en data-URI, affiché pendant le chargement.
  TextColumn get coverPlaceholder => text().nullable()();

  IntColumn get fileSize => integer().withDefault(const Constant(0))();
  DateTimeColumn get cachedAt => dateTime()();

  @override
  Set<Column> get primaryKey => {id, serverId};
}

/// Séries connues.
///
/// Le nom de la classe de ligne est imposé : la singularisation automatique de
/// Drift transforme « Series » en « Sery », qui n'est pas un mot.
@DataClassName('CachedSeriesRow')
class CachedSeries extends Table {
  TextColumn get id => text()();
  TextColumn get serverId => text()();
  TextColumn get libraryId => text()();
  TextColumn get name => text()();
  IntColumn get comicCount => integer().withDefault(const Constant(0))();
  TextColumn get coverPath => text().withDefault(const Constant(''))();

  @override
  Set<Column> get primaryKey => {id, serverId};
}

/// Arborescence des dossiers.
class CachedFolders extends Table {
  TextColumn get serverId => text()();
  TextColumn get libraryId => text()();
  TextColumn get path => text()();
  TextColumn get name => text()();
  IntColumn get depth => integer()();
  IntColumn get comicCount => integer().withDefault(const Constant(0))();

  @override
  Set<Column> get primaryKey => {serverId, libraryId, path};
}

/// Bibliothèques visibles.
class CachedLibraries extends Table {
  TextColumn get id => text()();
  TextColumn get serverId => text()();
  TextColumn get name => text()();
  IntColumn get comicCount => integer().withDefault(const Constant(0))();

  @override
  Set<Column> get primaryKey => {id, serverId};
}

/// Progression de lecture, locale et faisant autorité jusqu'à synchronisation.
class LocalProgress extends Table {
  TextColumn get comicId => text()();
  TextColumn get serverId => text()();
  IntColumn get page => integer()();
  IntColumn get pageCount => integer()();
  TextColumn get status => text()();
  DateTimeColumn get updatedAt => dateTime()();

  /// Vrai tant que le serveur n'a pas confirmé cette progression.
  BoolColumn get pending => boolean().withDefault(const Constant(false))();

  @override
  Set<Column> get primaryKey => {comicId, serverId};
}

/// Préférences locales, en clé-valeur.
///
/// Pas dans le stockage sécurisé, qui garde les jetons : chiffrer un sens de
/// lecture n'apporte rien et le rend plus lent à lire. Pas dans une dépendance
/// de plus non plus — la base est déjà là.
///
/// Volontairement sans `serverId` : le sens de lecture est une préférence de
/// personne, pas de serveur. Quelqu'un qui lit des mangas les lit de droite à
/// gauche sur toutes ses instances.
class Preferences extends Table {
  TextColumn get key => text()();
  TextColumn get value => text()();

  @override
  Set<Column> get primaryKey => {key};
}

@DriftDatabase(
  tables: [
    CachedComics,
    CachedSeries,
    CachedFolders,
    CachedLibraries,
    LocalProgress,
    Preferences,
  ],
)
class BoxDatabase extends _$BoxDatabase {
  BoxDatabase() : super(_open());

  /// Constructeur de test : base en mémoire, jetée avec l'instance.
  BoxDatabase.forTesting(super.executor);

  @override
  int get schemaVersion => 2;

  /*
  Migrations.

  Le cache du catalogue pourrait être jeté à chaque montée de version — il se
  reconstruit depuis le serveur. La progression de lecture, non : une position
  jamais synchronisée n'existe qu'ici, et l'effacer perdrait la page où
  quelqu'un s'est arrêté dans le métro. D'où des migrations réelles plutôt
  qu'un `deleteEverything`.
  */
  @override
  MigrationStrategy get migration => MigrationStrategy(
        onCreate: (m) => m.createAll(),
        onUpgrade: (m, from, to) async {
          if (from < 2) await m.createTable(preferences);
        },
      );

  // ─── Préférences ───────────────────────────────────────────────────────────

  Future<String?> preference(String key) async {
    final row = await (select(preferences)..where((p) => p.key.equals(key)))
        .getSingleOrNull();
    return row?.value;
  }

  Future<void> setPreference(String key, String value) =>
      into(preferences).insertOnConflictUpdate(
        PreferencesCompanion.insert(key: key, value: value),
      );

  // ─── Catalogue ─────────────────────────────────────────────────────────────

  /// Remplace le cache d'une bibliothèque par ce que le serveur vient de dire.
  ///
  /// Un remplacement plutôt qu'une fusion : le serveur fait autorité sur le
  /// catalogue, et fusionner laisserait traîner des albums supprimés depuis.
  Future<void> replaceComics(
    String serverId,
    String libraryId,
    List<CachedComicsCompanion> comics,
  ) async {
    await transaction(() async {
      await (delete(cachedComics)
            ..where((c) => c.serverId.equals(serverId) & c.libraryId.equals(libraryId)))
          .go();
      await batch((b) => b.insertAll(cachedComics, comics));
    });
  }

  Future<List<CachedComic>> comicsOf(
    String serverId, {
    String? libraryId,
    String? folderPath,
    String? seriesId,
  }) {
    final query = select(cachedComics)..where((c) => c.serverId.equals(serverId));

    if (libraryId != null) query.where((c) => c.libraryId.equals(libraryId));
    if (seriesId != null) query.where((c) => c.seriesId.equals(seriesId));

    if (folderPath != null) {
      // Le dossier ET sa descendance, comme le fait le serveur.
      query.where((c) =>
          c.folderPath.equals(folderPath) | c.folderPath.like('$folderPath/%'));
    }

    query.orderBy([(c) => OrderingTerm(expression: c.title)]);
    return query.get();
  }

  Future<CachedComic?> comic(String serverId, String id) =>
      (select(cachedComics)
            ..where((c) => c.serverId.equals(serverId) & c.id.equals(id)))
          .getSingleOrNull();

  Future<void> replaceFolders(
    String serverId,
    List<CachedFoldersCompanion> folders,
  ) async {
    await transaction(() async {
      await (delete(cachedFolders)..where((f) => f.serverId.equals(serverId))).go();
      await batch((b) => b.insertAll(cachedFolders, folders));
    });
  }

  Future<List<CachedFolder>> foldersOf(String serverId) =>
      (select(cachedFolders)
            ..where((f) => f.serverId.equals(serverId))
            ..orderBy([(f) => OrderingTerm(expression: f.path)]))
          .get();

  Future<void> replaceSeries(String serverId, List<CachedSeriesCompanion> series) async {
    await transaction(() async {
      await (delete(cachedSeries)..where((s) => s.serverId.equals(serverId))).go();
      await batch((b) => b.insertAll(cachedSeries, series));
    });
  }

  Future<List<CachedSeriesRow>> seriesOf(String serverId) =>
      (select(cachedSeries)
            ..where((s) => s.serverId.equals(serverId))
            ..orderBy([(s) => OrderingTerm(expression: s.name)]))
          .get();

  Future<void> replaceLibraries(
    String serverId,
    List<CachedLibrariesCompanion> libraries,
  ) async {
    await transaction(() async {
      await (delete(cachedLibraries)..where((l) => l.serverId.equals(serverId))).go();
      await batch((b) => b.insertAll(cachedLibraries, libraries));
    });
  }

  Future<List<CachedLibrary>> librariesOf(String serverId) =>
      (select(cachedLibraries)..where((l) => l.serverId.equals(serverId))).get();

  // ─── Progression ───────────────────────────────────────────────────────────

  Future<LocalProgressData?> progressOf(String serverId, String comicId) =>
      (select(localProgress)
            ..where((p) => p.serverId.equals(serverId) & p.comicId.equals(comicId)))
          .getSingleOrNull();

  /// Enregistre une progression localement, en la marquant à envoyer.
  Future<void> saveProgress({
    required String serverId,
    required String comicId,
    required int page,
    required int pageCount,
    required String status,
    bool pending = true,
  }) =>
      into(localProgress).insertOnConflictUpdate(
        LocalProgressCompanion.insert(
          comicId: comicId,
          serverId: serverId,
          page: page,
          pageCount: pageCount,
          status: status,
          updatedAt: DateTime.now().toUtc(),
          pending: Value(pending),
        ),
      );

  /// Progressions en attente d'envoi, les plus anciennes d'abord.
  Future<List<LocalProgressData>> pendingProgress(String serverId) =>
      (select(localProgress)
            ..where((p) => p.serverId.equals(serverId) & p.pending.equals(true))
            ..orderBy([(p) => OrderingTerm(expression: p.updatedAt)]))
          .get();

  Future<void> markSynced(String serverId, List<String> comicIds) async {
    if (comicIds.isEmpty) return;
    await (update(localProgress)
          ..where((p) => p.serverId.equals(serverId) & p.comicId.isIn(comicIds)))
        .write(const LocalProgressCompanion(pending: Value(false)));
  }

  /// Efface tout ce qui appartient à un serveur oublié.
  Future<void> forgetServer(String serverId) async {
    await transaction(() async {
      await (delete(cachedComics)..where((c) => c.serverId.equals(serverId))).go();
      await (delete(cachedSeries)..where((s) => s.serverId.equals(serverId))).go();
      await (delete(cachedFolders)..where((f) => f.serverId.equals(serverId))).go();
      await (delete(cachedLibraries)..where((l) => l.serverId.equals(serverId))).go();
      await (delete(localProgress)..where((p) => p.serverId.equals(serverId))).go();
    });
  }
}

LazyDatabase _open() {
  return LazyDatabase(() async {
    final dir = await getApplicationDocumentsDirectory();
    return NativeDatabase.createInBackground(File(p.join(dir.path, 'boxincloud.sqlite')));
  });
}
