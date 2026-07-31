import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/auth/session.dart';
import '../../core/db/database.dart';
import '../../core/offline/download_manager.dart';
import '../library/library_controller.dart';

/// Le gestionnaire de téléchargements de la session courante.
///
/// Un seul par serveur, partagé : deux instances videraient la même file en
/// parallèle, chacune ignorant que l'autre travaille sur le même album.
final downloadManagerProvider = Provider<DownloadManager?>((ref) {
  final session = ref.watch(sessionProvider);
  if (session is! SessionActive) return null;

  return DownloadManager(
    db: ref.watch(databaseProvider),
    client: session.client,
    serverId: session.server.id,
  );
});

/// L'état de téléchargement d'un album, en direct.
///
/// Un flux plutôt qu'un `Future` : la progression change des centaines de fois
/// pendant un téléchargement, et l'écran doit suivre sans qu'on l'invalide à
/// chaque page.
final downloadProvider = StreamProvider.family<Download?, String>((ref, comicId) {
  final session = ref.watch(sessionProvider);
  if (session is! SessionActive) return const Stream.empty();

  return ref.watch(databaseProvider).watchDownload(session.server.id, comicId);
});

/// Tous les téléchargements du serveur courant.
final downloadsProvider = StreamProvider<List<Download>>((ref) {
  final session = ref.watch(sessionProvider);
  if (session is! SessionActive) return const Stream.empty();

  return ref.watch(databaseProvider).watchDownloads(session.server.id);
});

/// Occupation disque et budget, pour l'indicateur de l'écran des téléchargements.
final diskUsageProvider = FutureProvider<({int used, int budget})>((ref) async {
  final manager = ref.watch(downloadManagerProvider);
  final session = ref.watch(sessionProvider);
  if (manager == null || session is! SessionActive) {
    return (used: 0, budget: defaultBudgetBytes);
  }

  // On suit les téléchargements de la session pour se rafraîchir quand ils
  // avancent : l'indicateur d'occupation figé serait pire que pas d'indicateur.
  ref.watch(downloadsProvider);

  return (
    used: await ref.watch(databaseProvider).downloadedBytes(session.server.id),
    budget: await manager.budgetBytes(),
  );
});

/// Taille lisible, en unités binaires.
String formatBytes(int bytes) {
  if (bytes < 1024) return '$bytes o';

  const units = ['ko', 'Mo', 'Go', 'To'];
  var value = bytes / 1024;
  var unit = 0;

  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }

  // Une décimale au-delà du mégaoctet, aucune en dessous : « 3,4 Go » informe,
  // « 512,0 ko » fait du bruit.
  return unit >= 1
      ? '${value.toStringAsFixed(1)} ${units[unit]}'
      : '${value.round()} ${units[unit]}';
}
