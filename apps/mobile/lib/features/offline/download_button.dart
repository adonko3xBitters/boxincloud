import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/db/database.dart';
import '../../shared/theme.dart';
import '../../shared/tokens.dart';
import 'downloads_controller.dart';

/// Bouton de téléchargement d'un album, avec sa progression.
///
/// Un seul contrôle pour quatre états — absent, en cours, complet, en échec —
/// parce que c'est une seule question : cet album est-il lisible sans réseau ?
class DownloadButton extends ConsumerWidget {
  final CachedComic comic;

  const DownloadButton({super.key, required this.comic});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = context.colors;
    final manager = ref.watch(downloadManagerProvider);
    final download = ref.watch(downloadProvider(comic.id)).valueOrNull;

    if (manager == null) return const SizedBox.shrink();

    if (download == null) {
      return OutlinedButton.icon(
        onPressed: () => manager.enqueue(
          comicId: comic.id,
          title: comic.title,
          seriesName: comic.seriesName,
          coverPath: comic.coverPath,
          pageCount: comic.pageCount,
        ),
        icon: const Icon(Icons.download_outlined, size: 20),
        label: const Text('Télécharger'),
      );
    }

    switch (download.state) {
      case 'complete':
        return Row(
          children: [
            Icon(Icons.offline_pin, size: 20, color: colors.success),
            const SizedBox(width: BoxSpace.s2),
            Expanded(
              child: Text(
                'Disponible hors ligne · ${formatBytes(download.bytes)}',
                style: TextStyle(color: colors.textMuted, fontSize: 13),
              ),
            ),
            TextButton(
              onPressed: () => _confirmRemoval(context, ref, download.bytes),
              child: const Text('Supprimer'),
            ),
          ],
        );

      case 'failed':
        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              download.error ?? 'Le téléchargement a échoué.',
              style: TextStyle(color: colors.danger, fontSize: 13),
            ),
            const SizedBox(height: BoxSpace.s1),
            OutlinedButton.icon(
              onPressed: () => manager.enqueue(
                comicId: comic.id,
                title: comic.title,
                seriesName: comic.seriesName,
                coverPath: comic.coverPath,
                pageCount: comic.pageCount,
              ),
              icon: const Icon(Icons.refresh, size: 20),
              label: const Text('Réessayer'),
            ),
          ],
        );

      default:
        // En file, en cours, ou interrompu : la progression est la même
        // information, et le bouton ne diffère que par ce qu'il propose.
        final ratio = download.pageCount > 0
            ? download.pagesDone / download.pageCount
            : 0.0;

        return Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            ClipRRect(
              borderRadius: BorderRadius.circular(BoxRadius.full),
              child: LinearProgressIndicator(
                value: ratio,
                minHeight: 4,
                backgroundColor: colors.border,
              ),
            ),
            const SizedBox(height: BoxSpace.s2),
            Row(
              children: [
                Expanded(
                  child: Text(
                    download.state == 'paused'
                        ? 'Interrompu à ${download.pagesDone}/${download.pageCount}'
                        : '${download.pagesDone}/${download.pageCount} pages',
                    style: TextStyle(color: colors.textMuted, fontSize: 13),
                  ),
                ),
                if (download.state == 'paused')
                  TextButton(
                    onPressed: manager.start,
                    child: const Text('Reprendre'),
                  )
                else
                  TextButton(
                    onPressed: manager.pause,
                    child: const Text('Interrompre'),
                  ),
              ],
            ),
          ],
        );
    }
  }

  Future<void> _confirmRemoval(
    BuildContext context,
    WidgetRef ref,
    int bytes,
  ) async {
    final manager = ref.read(downloadManagerProvider);
    if (manager == null) return;

    // Une confirmation, parce que le geste est destructif et que retélécharger
    // coûte du temps et des données — pas parce qu'il serait dangereux.
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Supprimer le téléchargement ?'),
        content: Text(
          'L\'album restera lisible tant que le serveur répond. '
          '${formatBytes(bytes)} seront libérés.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('Annuler'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, true),
            child: const Text('Supprimer'),
          ),
        ],
      ),
    );

    if (confirmed == true) await manager.remove(comic.id);
  }
}
