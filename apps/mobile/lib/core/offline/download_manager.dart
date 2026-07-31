import 'dart:async';

import 'package:drift/drift.dart';

import '../api/client.dart';
import '../db/database.dart';
import 'storage.dart';

/// Largeur des pages téléchargées, en pixels.
///
/// Assez pour un écran de téléphone en plein zoom, très en deçà de la
/// définition d'un scan d'origine. C'est ce facteur — souvent quatre à cinq —
/// qui rend le hors ligne praticable sur un appareil dont l'espace se compte.
const offlinePageWidth = 1600;

/// Budget disque par défaut : quatre gigaoctets.
///
/// Une valeur qui tient sur tout téléphone récent sans le remplir, et qui
/// représente une bonne centaine d'albums à cette largeur. Modifiable depuis
/// l'écran des téléchargements — c'est bien pour cela qu'elle est ici et non
/// codée en dur ailleurs.
const defaultBudgetBytes = 4 * 1024 * 1024 * 1024;

const _budgetKey = 'offline.budgetBytes';

/*
Gestionnaire de téléchargements.

Un seul album à la fois, dans l'ordre où on les a demandés. Le parallélisme
ferait tout arriver plus tard : sur une connexion domestique, trois
téléchargements simultanés se partagent la même bande passante et aucun n'est
lisible avant que tous ne soient finis. En série, le premier album demandé est
lisible pendant que le deuxième arrive.

La reprise ne demande aucun protocole particulier. Les pages sont des unités
indépendantes, écrites dans l'ordre : le nombre de pages écrites est le point
de reprise, et une interruption ne coûte au pire qu'une page.

Le téléchargement s'arrête quand l'application s'arrête. Un vrai
téléchargement d'arrière-plan — qui survit à la fermeture — demande WorkManager
côté Android et une extension côté iOS ; ce n'est pas fait, et l'écran des
téléchargements le dit plutôt que de le laisser croire.
*/
class DownloadManager {
  final BoxDatabase db;
  final ApiClient client;
  final String serverId;

  DownloadManager({
    required this.db,
    required this.client,
    required this.serverId,
  });

  Future<void>? _running;
  bool _cancelled = false;

  /// Met un album en file. Sans effet s'il y est déjà ou s'il est complet.
  Future<void> enqueue({
    required String comicId,
    required String title,
    required String seriesName,
    required String coverPath,
    required int pageCount,
  }) async {
    final existing = await db.download(serverId, comicId);
    if (existing != null && existing.state == 'complete') return;

    await db.upsertDownload(DownloadsCompanion.insert(
      serverId: serverId,
      comicId: comicId,
      title: title,
      seriesName: Value(seriesName),
      coverPath: Value(coverPath),
      pageCount: pageCount,
      // Une reprise garde les pages déjà écrites ; une première demande part
      // de zéro. Dans les deux cas la valeur vient de la base, jamais du disque.
      pagesDone: Value(existing?.pagesDone ?? 0),
      bytes: Value(existing?.bytes ?? 0),
      width: const Value(offlinePageWidth),
      state: 'queued',
      error: const Value(null),
      requestedAt: existing?.requestedAt ?? DateTime.now().toUtc(),
    ));

    start();
  }

  /// Lance le traitement de la file, s'il ne tourne pas déjà.
  void start() {
    if (_running != null) return;
    _cancelled = false;
    _running = _drain().whenComplete(() => _running = null);
  }

  /// Interrompt le téléchargement en cours ; ce qui est écrit le reste.
  Future<void> pause() async {
    _cancelled = true;
    await _running;
  }

  /// Attend que la file se vide, sans l'interrompre.
  ///
  /// À ne pas confondre avec `pause()`, qui annule : la distinction a coûté
  /// une série de tests qui mesuraient l'état d'un téléchargement qu'ils
  /// venaient eux-mêmes d'arrêter.
  Future<void> whenIdle() => _running ?? Future.value();

  bool get isRunning => _running != null;

  Future<void> _drain() async {
    while (!_cancelled) {
      final next = await db.nextQueuedDownload(serverId);
      if (next == null) return;

      await _process(next);
    }

    // Interrompu : ce qui était en cours redevient une file en attente, pour
    // que la reprise soit un simple `start()` plutôt qu'un état à réparer.
    await (db.update(db.downloads)
          ..where((d) => d.serverId.equals(serverId) & d.state.equals('running')))
        .write(const DownloadsCompanion(state: Value('paused')));
  }

  Future<void> _process(Download item) async {
    await db.updateDownload(serverId, item.comicId,
        const DownloadsCompanion(state: Value('running'), error: Value(null)));

    var done = item.pagesDone;
    var bytes = item.bytes;

    for (var index = done; index < item.pageCount; index += 1) {
      if (_cancelled) return;

      // Le budget est vérifié à chaque page, pas une fois au départ : un album
      // de trois cents pages peut faire basculer au milieu, et s'en apercevoir
      // à la fin obligerait à tout jeter.
      if (!await _ensureRoom(item.comicId)) {
        await db.updateDownload(
          serverId,
          item.comicId,
          DownloadsCompanion(
            state: const Value('failed'),
            pagesDone: Value(done),
            bytes: Value(bytes),
            error: const Value(
                'Budget disque atteint. Augmentez-le ou supprimez des albums.'),
          ),
        );
        return;
      }

      try {
        final page = await client.pageBytes(item.comicId, index,
            width: item.width > 0 ? item.width : offlinePageWidth);

        bytes += await writePage(serverId, item.comicId, index, page);
        done = index + 1;

        await db.updateDownload(
          serverId,
          item.comicId,
          DownloadsCompanion(pagesDone: Value(done), bytes: Value(bytes)),
        );
      } on NetworkException catch (e) {
        // Le réseau reviendra ; l'état conserve le point de reprise.
        await db.updateDownload(
          serverId,
          item.comicId,
          DownloadsCompanion(
            state: const Value('paused'),
            pagesDone: Value(done),
            bytes: Value(bytes),
            error: Value(e.detail),
          ),
        );
        _cancelled = true;
        return;
      } on ApiException catch (e) {
        // Le serveur a refusé : réessayer donnerait le même refus.
        await db.updateDownload(
          serverId,
          item.comicId,
          DownloadsCompanion(
            state: const Value('failed'),
            pagesDone: Value(done),
            bytes: Value(bytes),
            error: Value(e.message),
          ),
        );
        return;
      }
    }

    await db.updateDownload(
      serverId,
      item.comicId,
      DownloadsCompanion(
        state: const Value('complete'),
        pagesDone: Value(done),
        bytes: Value(bytes),
        completedAt: Value(DateTime.now().toUtc()),
        error: const Value(null),
      ),
    );
  }

  /// Libère de la place si le budget est dépassé. Faux s'il n'y a rien à évincer.
  Future<bool> _ensureRoom(String protectedComicId) async {
    final budget = await budgetBytes();
    var used = await db.downloadedBytes(serverId);
    if (used < budget) return true;

    for (final candidate in await db.evictionCandidates(serverId)) {
      if (candidate.comicId == protectedComicId) continue;

      await remove(candidate.comicId);
      used -= candidate.bytes;
      if (used < budget) return true;
    }

    return false;
  }

  /// Supprime un album téléchargé, disque et registre.
  ///
  /// Les fichiers d'abord : l'inverse laisserait, en cas d'interruption, des
  /// octets que plus rien ne référence et que rien ne viendrait jamais nettoyer.
  Future<void> remove(String comicId) async {
    await deleteComicFiles(serverId, comicId);
    await db.deleteDownload(serverId, comicId);
  }

  Future<int> budgetBytes() async {
    final stored = await db.preference(_budgetKey);
    return int.tryParse(stored ?? '') ?? defaultBudgetBytes;
  }

  Future<void> setBudgetBytes(int value) =>
      db.setPreference(_budgetKey, '$value');

  /// Note qu'un album vient d'être ouvert : l'éviction s'en sert.
  Future<void> markRead(String comicId) => db.updateDownload(
        serverId,
        comicId,
        DownloadsCompanion(lastReadAt: Value(DateTime.now().toUtc())),
      );
}
