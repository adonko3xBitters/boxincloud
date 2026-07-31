import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/auth/session.dart';
import '../../core/db/database.dart';
import '../../core/sync/progress_sync.dart';
import '../../shared/theme.dart';
import '../../shared/tokens.dart';
import '../reader/reader_screen.dart';
import 'library_controller.dart';
import 'series_screen.dart';

/// Fiche d'un album.
///
/// Elle existe pour une seule raison : reprendre là où l'on s'est arrêté. Tout
/// le reste — série, nombre de pages, taille — est du contexte qui aide à
/// reconnaître l'album, pas à décider.
class ComicDetailScreen extends ConsumerWidget {
  final CachedComic comic;

  const ComicDetailScreen({super.key, required this.comic});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final session = ref.watch(sessionProvider);
    if (session is! SessionActive) return const SizedBox.shrink();

    final colors = context.colors;
    final progress = ref.watch(progressProvider(comic.id));

    return Scaffold(
      appBar: AppBar(title: Text(comic.title, overflow: TextOverflow.ellipsis)),
      body: ListView(
        padding: const EdgeInsets.all(BoxSpace.s4),
        children: [
          Center(
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 260),
              child: AspectRatio(
                aspectRatio: 0.7,
                child: ClipRRect(
                  borderRadius: BorderRadius.circular(BoxRadius.md),
                  child: CachedNetworkImage(
                    imageUrl: session.client.imageUrl(comic.coverPath, width: 640),
                    fit: BoxFit.cover,
                    placeholder: (_, _) => ColoredBox(color: colors.surfaceSunken),
                    errorWidget: (_, _, _) => ColoredBox(color: colors.surfaceSunken),
                  ),
                ),
              ),
            ),
          ),
          const SizedBox(height: BoxSpace.s5),

          if (comic.seriesName.isNotEmpty)
            // La série est cliquable : on tombe souvent sur un tome en cherchant
            // la série, et devoir revenir en arrière pour la trouver serait un
            // aller-retour inutile.
            InkWell(
              onTap: () => Navigator.of(context).push(
                MaterialPageRoute<void>(
                  builder: (_) => SeriesScreen(
                    seriesId: comic.seriesId ?? '',
                    name: comic.seriesName,
                  ),
                ),
              ),
              child: Padding(
                padding: const EdgeInsets.symmetric(vertical: BoxSpace.s1),
                child: Text(
                  comic.seriesName.toUpperCase(),
                  style: TextStyle(
                    color: colors.accentText,
                    fontSize: 12,
                    letterSpacing: 0.6,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
            ),

          Text(
            comic.title,
            style: TextStyle(
              color: colors.text,
              fontSize: 20,
              fontWeight: FontWeight.w700,
              height: 1.25,
            ),
          ),
          const SizedBox(height: BoxSpace.s4),

          progress.when(
            loading: () => const _ResumeButton(page: 0, comicId: '', title: ''),
            error: (_, _) => _ResumeButton(
              page: 0,
              comicId: comic.id,
              title: comic.title,
            ),
            data: (page) => Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                if (page > 0) ...[
                  ClipRRect(
                    borderRadius: BorderRadius.circular(BoxRadius.full),
                    child: LinearProgressIndicator(
                      value: comic.pageCount > 0 ? (page + 1) / comic.pageCount : 0,
                      minHeight: 4,
                      backgroundColor: colors.border,
                    ),
                  ),
                  const SizedBox(height: BoxSpace.s2),
                ],
                _ResumeButton(page: page, comicId: comic.id, title: comic.title),
              ],
            ),
          ),

          const SizedBox(height: BoxSpace.s6),
          _Facts(comic: comic),
        ],
      ),
    );
  }
}

class _ResumeButton extends StatelessWidget {
  final int page;
  final String comicId;
  final String title;

  const _ResumeButton({
    required this.page,
    required this.comicId,
    required this.title,
  });

  @override
  Widget build(BuildContext context) {
    return FilledButton.icon(
      onPressed: comicId.isEmpty
          ? null
          : () => Navigator.of(context).push(
                MaterialPageRoute<void>(
                  builder: (_) => ReaderScreen(comicId: comicId, title: title),
                ),
              ),
      icon: const Icon(Icons.menu_book_outlined, size: 20),
      label: Text(page > 0 ? 'Reprendre page ${page + 1}' : 'Lire'),
    );
  }
}

/// Métadonnées, en grille de deux colonnes.
class _Facts extends StatelessWidget {
  final CachedComic comic;

  const _Facts({required this.comic});

  @override
  Widget build(BuildContext context) {
    final colors = context.colors;

    final facts = <(String, String)>[
      if (comic.number.isNotEmpty) ('Numéro', comic.number),
      ('Pages', '${comic.pageCount}'),
      ('Taille', _bytes(comic.fileSize)),
      if (comic.folderPath.isNotEmpty) ('Dossier', comic.folderPath),
    ];

    return Wrap(
      spacing: BoxSpace.s6,
      runSpacing: BoxSpace.s4,
      children: [
        for (final (label, value) in facts)
          SizedBox(
            width: 140,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  label.toUpperCase(),
                  style: TextStyle(
                    color: colors.textSubtle,
                    fontSize: 11,
                    letterSpacing: 0.5,
                  ),
                ),
                const SizedBox(height: 2),
                Text(value, style: TextStyle(color: colors.text)),
              ],
            ),
          ),
      ],
    );
  }
}

String _bytes(int size) {
  if (size < 1024) return '$size o';
  const units = ['ko', 'Mo', 'Go'];
  var value = size / 1024;
  var unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit++;
  }
  return '${value.toStringAsFixed(1)} ${units[unit]}';
}

/// Position enregistrée d'un album, locale d'abord.
final progressProvider = FutureProvider.family<int, String>((ref, comicId) async {
  final session = ref.watch(sessionProvider);
  if (session is! SessionActive) return 0;

  final sync = ProgressSync(db: ref.watch(databaseProvider), serverId: session.server.id);
  return sync.resumePage(session.client, comicId);
});
