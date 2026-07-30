import 'package:drift/drift.dart';

import '../api/client.dart';
import '../db/database.dart';

/*
Synchronisation de la progression.

La progression est ce qui se perd le plus facilement et se remarque le plus vite :
reprendre un album à la page 3 alors qu'on en était à la 87 gâche une soirée de
lecture. Elle est donc écrite LOCALEMENT d'abord, tout de suite, et poussée
ensuite — jamais l'inverse.

La règle de résolution est « la page la plus avancée gagne ». Elle est
volontairement simple et non chronologique : les horloges d'un téléphone et d'un
serveur divergent, et un fuseau mal réglé ferait reculer une lecture. Avancer
par erreur coûte de sauter quelques pages ; reculer par erreur coûte de relire
tout un album en cherchant où l'on en était.
*/

/// Résultat d'une synchronisation.
class SyncOutcome {
  final int pushed;
  final int pulled;
  final bool offline;

  const SyncOutcome({this.pushed = 0, this.pulled = 0, this.offline = false});
}

class ProgressSync {
  final BoxDatabase db;
  final String serverId;

  const ProgressSync({required this.db, required this.serverId});

  /// Enregistre une position de lecture.
  ///
  /// Écrite localement quoi qu'il arrive, puis poussée si le réseau répond. Un
  /// échec réseau n'est pas une erreur : la ligne reste marquée à envoyer, et
  /// partira à la prochaine occasion.
  Future<void> record(
    ApiClient? client, {
    required String comicId,
    required int page,
    required int pageCount,
  }) async {
    final status = statusFor(page, pageCount);

    await db.saveProgress(
      serverId: serverId,
      comicId: comicId,
      page: page,
      pageCount: pageCount,
      status: status,
    );

    if (client == null) return;

    try {
      await client.saveProgress(
        comicId,
        page: page,
        pageCount: pageCount,
        status: status,
      );
      await db.markSynced(serverId, [comicId]);
    } on NetworkException {
      // Hors ligne : la file s'en chargera.
    } on ApiException {
      // Album disparu ou droits retirés : insister à chaque page serait
      // inutile. La ligne reste en attente et sera rejouée à la prochaine
      // synchronisation complète, qui décidera.
    }
  }

  /// Position à ouvrir pour un album, locale d'abord.
  ///
  /// Le local prime : il est forcément au moins aussi avancé que le serveur,
  /// puisque toute lecture y est écrite avant d'être envoyée.
  Future<int> resumePage(ApiClient? client, String comicId) async {
    final local = await db.progressOf(serverId, comicId);
    if (local != null) return local.page;

    if (client == null) return 0;

    try {
      final remote = await client.progress(comicId);
      await db.saveProgress(
        serverId: serverId,
        comicId: comicId,
        page: remote.page,
        pageCount: remote.pageCount,
        status: remote.status,
        pending: false,
      );
      return remote.page;
    } catch (_) {
      return 0;
    }
  }

  /*
    Vide la file d'attente vers le serveur.

    En un seul lot plutôt qu'une requête par album : quelqu'un qui a lu trois
    albums en avion produit trois lignes, et trois allers-retours à la
    reconnexion coûtent plus que le trajet lui-même sur un réseau mobile.

    Les lignes ne sont marquées envoyées qu'APRÈS confirmation du serveur. Les
    marquer d'avance perdrait la progression si l'envoi échouait à mi-chemin,
    ce qui est précisément le scénario que cette file existe pour couvrir.
  */
  Future<SyncOutcome> push(ApiClient client) async {
    final pending = await db.pendingProgress(serverId);
    if (pending.isEmpty) return const SyncOutcome();

    final updates = pending
        .map((p) => {
              'comicId': p.comicId,
              'page': p.page,
              'pageCount': p.pageCount,
              'status': p.status,
            })
        .toList();

    try {
      await client.pushSync(updates);
      await db.markSynced(serverId, pending.map((p) => p.comicId).toList());
      return SyncOutcome(pushed: pending.length);
    } on NetworkException {
      return const SyncOutcome(offline: true);
    }
  }
}

/// Déduit le statut d'une position.
///
/// La dernière page marque l'album comme lu : attendre un geste explicite
/// laisserait des albums terminés indéfiniment « en cours », et fausserait
/// l'étagère « reprendre la lecture ».
String statusFor(int page, int pageCount) {
  if (pageCount <= 0) return 'unread';
  if (page >= pageCount - 1) return 'read';
  if (page <= 0) return 'unread';
  return 'in_progress';
}

/// Retient la position la plus avancée entre deux progressions.
///
/// Utilisée à la réconciliation. Volontairement non chronologique : les horloges
/// d'un téléphone et d'un serveur divergent, et un fuseau mal réglé ferait
/// reculer une lecture. Avancer par erreur coûte quelques pages sautées ;
/// reculer coûte de relire un album entier en cherchant où l'on en était.
int furthest(int local, int remote) => local >= remote ? local : remote;

/// Fusionne une progression distante dans le cache local.
Future<void> mergeRemote(
  BoxDatabase db,
  String serverId, {
  required String comicId,
  required int remotePage,
  required int pageCount,
}) async {
  final local = await db.progressOf(serverId, comicId);
  final page = furthest(local?.page ?? 0, remotePage);

  await db.saveProgress(
    serverId: serverId,
    comicId: comicId,
    page: page,
    pageCount: pageCount,
    status: statusFor(page, pageCount),
    // Si le local était en avance, il reste à pousser.
    pending: (local?.page ?? 0) > remotePage,
  );
}

/// Adapte un album de l'API vers une ligne de cache.
CachedComicsCompanion cachedComicFrom(
  dynamic comic,
  String serverId,
) {
  return CachedComicsCompanion.insert(
    id: comic.id as String,
    serverId: serverId,
    libraryId: comic.libraryId as String,
    title: comic.title as String,
    seriesId: Value(comic.seriesId as String?),
    seriesName: Value((comic.seriesName as String?) ?? ''),
    number: Value((comic.number as String?) ?? ''),
    folderPath: Value(comic.folderPath as String),
    pageCount: Value(comic.pageCount as int),
    coverPath: Value(comic.coverPath as String),
    coverPlaceholder: Value(comic.coverPlaceholder as String?),
    fileSize: Value(comic.fileSize as int),
    cachedAt: DateTime.now().toUtc(),
  );
}
