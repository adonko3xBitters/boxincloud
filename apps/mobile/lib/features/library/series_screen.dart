import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/client.dart';
import '../../core/auth/session.dart';
import '../../core/db/database.dart';
import '../../core/db/mapping.dart';
import '../../shared/theme.dart';
import '../offline/downloads_controller.dart';
import '../../shared/tokens.dart';
import 'comic_detail_screen.dart';
import 'library_controller.dart';

/// Les albums d'une série, dans l'ordre des tomes.
///
/// C'est ce qu'une série apporte qu'un dossier ne donne pas : le tri par numéro
/// plutôt qu'alphabétique. « T10 » vient après « T9 », pas entre « T1 » et
/// « T2 » — ce qu'un classement par nom de fichier ferait pourtant.
class SeriesScreen extends ConsumerWidget {
  final String seriesId;
  final String name;

  const SeriesScreen({super.key, required this.seriesId, required this.name});

  /// Met toute la série en file, dans l'ordre des tomes.
  ///
  /// L'ordre compte : la file se vide séquentiellement, et quelqu'un qui
  /// télécharge une série veut commencer par le tome 1 pendant que le reste
  /// arrive.
  Future<void> _downloadAll(BuildContext context, WidgetRef ref) async {
    final manager = ref.read(downloadManagerProvider);
    final comics = ref.read(seriesComicsProvider(seriesId)).valueOrNull;
    if (manager == null || comics == null || comics.isEmpty) return;

    final messenger = ScaffoldMessenger.of(context);

    for (final comic in comics) {
      await manager.enqueue(
        comicId: comic.id,
        title: comic.title,
        seriesName: comic.seriesName,
        coverPath: comic.coverPath,
        pageCount: comic.pageCount,
      );
    }

    messenger.showSnackBar(
      SnackBar(
        content: Text(comics.length == 1
            ? '1 album en file de téléchargement.'
            : '${comics.length} albums en file de téléchargement.'),
      ),
    );
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final session = ref.watch(sessionProvider);
    if (session is! SessionActive) return const SizedBox.shrink();

    final colors = context.colors;
    final comics = ref.watch(seriesComicsProvider(seriesId));

    return Scaffold(
      appBar: AppBar(
        title: Text(name),
        actions: [
          // Télécharger une série entière, parce que c'est ainsi qu'on lit :
          // on n'emporte pas le tome 4 en vacances, on emporte Blacksad.
          IconButton(
            tooltip: 'Tout télécharger',
            icon: const Icon(Icons.download_outlined),
            onPressed: () => _downloadAll(context, ref),
          ),
        ],
      ),
      body: comics.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, _) => Center(
          child: Padding(
            padding: const EdgeInsets.all(BoxSpace.s8),
            child: Text('$error', textAlign: TextAlign.center),
          ),
        ),
        data: (list) {
          if (list.isEmpty) {
            return Center(
              child: Text(
                'Aucun album dans cette série.',
                style: TextStyle(color: colors.textMuted),
              ),
            );
          }

          return ListView.separated(
            itemCount: list.length,
            separatorBuilder: (_, _) => Divider(height: 1, color: colors.border),
            itemBuilder: (context, index) {
              final comic = list[index];

              return ListTile(
                contentPadding: const EdgeInsets.symmetric(
                  horizontal: BoxSpace.s4,
                  vertical: BoxSpace.s2,
                ),
                leading: SizedBox(
                  width: 40,
                  height: 56,
                  child: ClipRRect(
                    borderRadius: BorderRadius.circular(BoxRadius.sm),
                    child: CachedNetworkImage(
                      imageUrl: session.client.imageUrl(comic.coverPath, width: 160),
                      fit: BoxFit.cover,
                      placeholder: (_, _) => ColoredBox(color: colors.surfaceSunken),
                      errorWidget: (_, _, _) => ColoredBox(color: colors.surfaceSunken),
                    ),
                  ),
                ),
                title: Text(comic.title, maxLines: 1, overflow: TextOverflow.ellipsis),
                subtitle: Text(
                  comic.number.isNotEmpty
                      ? 'Tome ${comic.number} · ${comic.pageCount} p.'
                      : '${comic.pageCount} p.',
                  style: TextStyle(color: colors.textSubtle, fontSize: 12),
                ),
                onTap: () => Navigator.of(context).push(
                  MaterialPageRoute<void>(
                    builder: (_) => ComicDetailScreen(comic: comic),
                  ),
                ),
              );
            },
          );
        },
      ),
    );
  }
}

/*
Albums d'une série, triés par numéro de tome.

Le tri se fait ici plutôt qu'en base : le numéro est une chaîne — « 7 », « 07 »,
« HS 2 » — et un ORDER BY textuel placerait « 10 » avant « 2 ». La comparaison
numérique quand c'est possible, alphabétique sinon, donne l'ordre qu'un lecteur
attend.
*/
final seriesComicsProvider =
    FutureProvider.family<List<CachedComic>, String>((ref, seriesId) async {
  final session = ref.watch(sessionProvider);
  if (session is! SessionActive) return const [];

  final db = ref.watch(databaseProvider);
  var comics = await db.comicsOf(session.server.id, seriesId: seriesId);

  // Le cache ne retient qu'une partie de chaque bibliothèque, et une série
  // atteinte depuis une recherche en ligne peut n'y figurer par aucun tome.
  // Afficher « aucun album » alors que le serveur répond serait faux.
  if (comics.isEmpty) {
    try {
      final page = await session.client.comics(seriesId: seriesId, limit: 200);
      comics = page.items.map((c) => cachedFromApi(c, session.server.id)).toList();
    } on NetworkException {
      // Hors ligne : la liste vide est alors la réponse honnête.
    } on ApiException {
      // Idem — mieux vaut une série vide qu'un écran d'erreur.
    }
  }

  final sorted = [...comics]..sort((a, b) => compareTome(a.number, b.number));
  return sorted;
});

/// Compare deux numéros de tome.
///
/// Les deux numériques : ordre numérique. Sinon, ordre alphabétique, et un
/// numéro absent passe en dernier — un hors-série sans numéro n'a pas à
/// s'intercaler entre deux tomes.
int compareTome(String a, String b) {
  if (a.isEmpty && b.isEmpty) return 0;
  if (a.isEmpty) return 1;
  if (b.isEmpty) return -1;

  final na = double.tryParse(a);
  final nb = double.tryParse(b);
  if (na != null && nb != null) return na.compareTo(nb);

  return a.compareTo(b);
}
